package approval

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// The activities only sleep. Each one logs its attempt number, so an activity
// that ran more than once shows up in the worker logs as attempt 2 or higher.

// Validate stands in for a quick check of the change.
func Validate(ctx context.Context, req ChangeRequest, reason string) error {
	logAttempt(ctx, "validate", req)
	activity.GetLogger(ctx).Info("activity started",
		"activity", "validate", "change", req.ID, "attempt", activity.GetInfo(ctx).Attempt, "reason", reason)
	return sleep(ctx, 200*time.Millisecond)
}

// Release stands in for the slow step. It takes about five seconds and
// heartbeats once a second, so that a worker killed in the middle of it is
// visible in the workflow's event history.
func Release(ctx context.Context, req ChangeRequest) error {
	logAttempt(ctx, "release", req)
	for step := 1; step <= 5; step++ {
		activity.RecordHeartbeat(ctx, step)
		if err := sleep(ctx, time.Second); err != nil {
			return err
		}
	}
	return nil
}

// Notify stands in for telling downstream systems about the release.
func Notify(ctx context.Context, req ChangeRequest) error {
	logAttempt(ctx, "notify", req)
	return sleep(ctx, 200*time.Millisecond)
}

func logAttempt(ctx context.Context, name string, req ChangeRequest) {
	activity.GetLogger(ctx).Info("activity started",
		"activity", name, "change", req.ID, "attempt", activity.GetInfo(ctx).Attempt)
}

func sleep(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
