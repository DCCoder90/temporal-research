package signals

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

const (
	ApproveSignal = "approve"
	RejectSignal  = "reject"
	StatusQuery   = "status"
)

// ApprovalWorkflow starts in a "pending" state and blocks until it receives
// either an "approve" or "reject" signal. Signals can be sent from the
// signals-approve / signals-reject containers or via the Temporal UI.
func ApprovalWorkflow(ctx workflow.Context, orderID string) (string, error) {
	logger := workflow.GetLogger(ctx)
	status := "pending"

	// Register a query handler so callers can inspect the current status.
	if err := workflow.SetQueryHandler(ctx, StatusQuery, func() (string, error) {
		return status, nil
	}); err != nil {
		return "", err
	}

	logger.Info("ApprovalWorkflow waiting for signal", "orderID", orderID)

	approveC := workflow.GetSignalChannel(ctx, ApproveSignal)
	rejectC := workflow.GetSignalChannel(ctx, RejectSignal)

	// Block until one of the two signals arrives.
	sel := workflow.NewSelector(ctx)
	var approved bool

	sel.AddReceive(approveC, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
		approved = true
		status = "approved"
	})
	sel.AddReceive(rejectC, func(c workflow.ReceiveChannel, _ bool) {
		c.Receive(ctx, nil)
		approved = false
		status = "rejected"
	})
	sel.Select(ctx)

	ao := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	var err error
	if approved {
		err = workflow.ExecuteActivity(ctx, ProcessOrderActivity, orderID).Get(ctx, &result)
	} else {
		err = workflow.ExecuteActivity(ctx, CancelOrderActivity, orderID).Get(ctx, &result)
	}
	return result, err
}

func ProcessOrderActivity(ctx context.Context, orderID string) (string, error) {
	activity.GetLogger(ctx).Info("Processing order", "orderID", orderID)
	return fmt.Sprintf("order %s processed successfully", orderID), nil
}

func CancelOrderActivity(ctx context.Context, orderID string) (string, error) {
	activity.GetLogger(ctx).Info("Cancelling order", "orderID", orderID)
	return fmt.Sprintf("order %s cancelled", orderID), nil
}
