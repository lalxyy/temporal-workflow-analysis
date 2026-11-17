// Command loadgen starts change-approval workflows at a steady rate and sends
// each one its approvals after random waits. It writes one CSV line to stdout
// for every call it makes to Temporal:
//
//	time,op,workflow_id,latency_ms,error
//
// op is "start" or "signal". error is empty when the call succeeded. Capture
// the output with: kubectl -n temporal-lab logs deploy/loadgen > run.csv
package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"

	"changelab/approval"
	"changelab/internal/tclient"
)

type config struct {
	rate            float64       // workflows started per second
	approvals       int           // approvals each workflow needs
	minDelay        time.Duration // shortest wait before an approval
	maxDelay        time.Duration // longest wait before an approval
	approvalTimeout time.Duration // when a workflow gives up waiting
	silentPercent   float64       // share of workflows that get no approvals
	callTimeout     time.Duration // limit for one call to Temporal
}

func main() {
	cfg := loadConfig()

	c, err := tclient.Dial("")
	if err != nil {
		log.Fatalln("cannot connect to Temporal:", err)
	}
	defer c.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rec := newRecorder()
	// The run ID keeps workflow IDs unique across restarts of the generator.
	runID := time.Now().UTC().Format("20060102-150405")
	log.Printf("loadgen starting: run=%s rate=%.2f/s approvals=%d delay=%s..%s timeout=%s silent=%.0f%%",
		runID, cfg.rate, cfg.approvals, cfg.minDelay, cfg.maxDelay, cfg.approvalTimeout, cfg.silentPercent)

	ticker := time.NewTicker(time.Duration(float64(time.Second) / cfg.rate))
	defer ticker.Stop()
	for seq := 1; ; seq++ {
		select {
		case <-ctx.Done():
			log.Println("loadgen stopped")
			return
		case <-ticker.C:
		}
		id := fmt.Sprintf("change-%s-%06d", runID, seq)
		go runOne(ctx, c, cfg, rec, id)
	}
}

// runOne starts one workflow and then sends its approvals.
func runOne(ctx context.Context, c client.Client, cfg config, rec *recorder, id string) {
	req := approval.ChangeRequest{
		ID:              id,
		ApprovalsNeeded: cfg.approvals,
		ApprovalTimeout: cfg.approvalTimeout,
	}
	err := rec.timed(ctx, cfg, "start", id, func(ctx context.Context) error {
		_, err := c.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        id,
			TaskQueue: approval.TaskQueue,
		}, approval.ChangeApproval, req)
		return err
	})
	if err != nil {
		return
	}

	// Leave some workflows without approvals so that they run into the timer.
	if rand.Float64()*100 < cfg.silentPercent {
		return
	}

	for i := 1; i <= cfg.approvals; i++ {
		wait := cfg.minDelay + rand.N(cfg.maxDelay-cfg.minDelay+1)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		approver := fmt.Sprintf("approver-%d", i)
		_ = rec.timed(ctx, cfg, "signal", id, func(ctx context.Context) error {
			return c.SignalWorkflow(ctx, id, "", approval.ApproveSignal, approver)
		})
	}
}

// recorder writes one CSV line per call. It is safe for concurrent use.
type recorder struct {
	mu sync.Mutex
	w  *csv.Writer
}

func newRecorder() *recorder {
	r := &recorder{w: csv.NewWriter(os.Stdout)}
	r.write("time", "op", "workflow_id", "latency_ms", "error")
	return r
}

func (r *recorder) write(fields ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.w.Write(fields)
	r.w.Flush()
}

// timed runs one call with a timeout and records how long it took.
func (r *recorder) timed(ctx context.Context, cfg config, op, id string, call func(context.Context) error) error {
	callCtx, cancel := context.WithTimeout(ctx, cfg.callTimeout)
	defer cancel()

	started := time.Now()
	err := call(callCtx)
	elapsed := time.Since(started)

	errText := ""
	if err != nil {
		errText = err.Error()
	}
	r.write(
		started.UTC().Format(time.RFC3339Nano),
		op,
		id,
		strconv.FormatFloat(float64(elapsed.Microseconds())/1000, 'f', 3, 64),
		errText,
	)
	return err
}

func loadConfig() config {
	cfg := config{
		rate:            floatEnv("RATE", 2),
		approvals:       intEnv("APPROVALS", 2),
		minDelay:        durationEnv("MIN_DELAY", 5*time.Second),
		maxDelay:        durationEnv("MAX_DELAY", 30*time.Second),
		approvalTimeout: durationEnv("APPROVAL_TIMEOUT", 3*time.Minute),
		silentPercent:   floatEnv("SILENT_PERCENT", 10),
		callTimeout:     durationEnv("CALL_TIMEOUT", 10*time.Second),
	}
	if cfg.rate <= 0 {
		log.Fatalln("RATE must be greater than zero")
	}
	if cfg.approvals < 1 {
		log.Fatalln("APPROVALS must be at least 1")
	}
	if cfg.maxDelay < cfg.minDelay {
		log.Fatalln("MAX_DELAY must not be shorter than MIN_DELAY")
	}
	return cfg
}

func floatEnv(key string, def float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Fatalf("bad %s: %v", key, err)
	}
	return f
}

func intEnv(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("bad %s: %v", key, err)
	}
	return n
}

func durationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("bad %s: %v", key, err)
	}
	return d
}
