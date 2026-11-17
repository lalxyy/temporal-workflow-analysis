// Command worker runs the change-approval workflow and its activities.
package main

import (
	"log"
	"time"

	"go.temporal.io/sdk/worker"

	"changelab/approval"
	"changelab/internal/tclient"
)

// buildID is set at build time (see the Dockerfile). It is printed at start-up
// so the logs show which version of the code a pod runs.
var buildID = "dev"

func main() {
	stopTimeout, err := time.ParseDuration(tclient.Env("WORKER_STOP_TIMEOUT", "10s"))
	if err != nil {
		log.Fatalln("bad WORKER_STOP_TIMEOUT:", err)
	}

	c, err := tclient.Dial(tclient.Env("METRICS_ADDR", "0.0.0.0:9090"))
	if err != nil {
		log.Fatalln("cannot connect to Temporal:", err)
	}
	defer c.Close()

	w := worker.New(c, approval.TaskQueue, worker.Options{
		// How long Stop waits for tasks that are already running.
		WorkerStopTimeout: stopTimeout,
	})
	w.RegisterWorkflow(approval.ChangeApproval)
	w.RegisterActivity(approval.Validate)
	w.RegisterActivity(approval.Release)
	w.RegisterActivity(approval.Notify)

	log.Printf("worker starting: build=%s taskQueue=%s stopTimeout=%s", buildID, approval.TaskQueue, stopTimeout)
	// InterruptCh closes on SIGINT or SIGTERM, which is what Kubernetes sends
	// when it replaces the pod.
	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalln("worker stopped with an error:", err)
	}
	log.Println("worker stopped")
}
