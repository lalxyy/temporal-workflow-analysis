// Package approval is the example workflow for the lab: a change that needs a
// number of sign-offs before it is released.
//
// This file is the code under test. Experiment 3 changes it between image
// versions and rolls the change out while workflows are in flight.
package approval

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// TaskQueue is the queue the workers poll and the load generator starts
	// workflows on.
	TaskQueue = "change-approval"

	// ApproveSignal is the name of the signal that carries one approval.
	ApproveSignal = "approve"

	ResultReleased = "released"
	ResultExpired  = "expired"
)

// ChangeRequest is the workflow input.
type ChangeRequest struct {
	ID              string
	ApprovalsNeeded int
	ApprovalTimeout time.Duration
}

// ChangeApproval validates a change, waits for its approvals, then releases it
// and notifies downstream. It returns ResultExpired when the approvals do not
// arrive in time.
func ChangeApproval(ctx workflow.Context, req ChangeRequest) (string, error) {
	retry := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2,
		MaximumInterval:    10 * time.Second,
	}
	quick := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         retry,
	})
	// Release is slow and heartbeats. If its worker dies, Temporal notices
	// after HeartbeatTimeout instead of waiting for StartToCloseTimeout.
	slow := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 60 * time.Second,
		HeartbeatTimeout:    5 * time.Second,
		RetryPolicy:         retry,
	})

	if err := workflow.ExecuteActivity(quick, Validate, req, "hello").Get(ctx, nil); err != nil {
		return "", err
	}

	// Wait until enough approvals have arrived or the timer fires.
	approvals := 0
	expired := false
	timerCtx, cancelTimer := workflow.WithCancel(ctx)
	timer := workflow.NewTimer(timerCtx, req.ApprovalTimeout)

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(workflow.GetSignalChannel(ctx, ApproveSignal), func(c workflow.ReceiveChannel, _ bool) {
		var approver string
		c.Receive(ctx, &approver)
		approvals++
	})
	selector.AddFuture(timer, func(f workflow.Future) {
		if f.Get(ctx, nil) == nil {
			expired = true
		}
	})
	for approvals < req.ApprovalsNeeded && !expired {
		selector.Select(ctx)
	}
	cancelTimer()

	if expired {
		return ResultExpired, nil
	}

	if err := workflow.ExecuteActivity(slow, Release, req).Get(ctx, nil); err != nil {
		return "", err
	}
	if err := workflow.ExecuteActivity(quick, Notify, req).Get(ctx, nil); err != nil {
		return "", err
	}
	return ResultReleased, nil
}
