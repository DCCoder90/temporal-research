package retries

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// UnreliableWorkflow calls an activity that intentionally fails on the first
// two attempts, then succeeds on the third. The retry policy uses exponential
// backoff. Observe in Wireshark: you will see repeated RespondActivityTaskFailed
// RPCs followed by a final RespondActivityTaskCompleted.
func UnreliableWorkflow(ctx workflow.Context) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    2 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Second,
			MaximumAttempts:    5,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	if err := workflow.ExecuteActivity(ctx, UnreliableActivity).Get(ctx, &result); err != nil {
		return "", err
	}

	workflow.GetLogger(ctx).Info("UnreliableWorkflow completed", "result", result)
	return result, nil
}

// UnreliableActivity fails on attempts 1 and 2, succeeds on attempt 3+.
// activity.GetInfo provides the current attempt number (1-based).
func UnreliableActivity(ctx context.Context) (string, error) {
	info := activity.GetInfo(ctx)
	logger := activity.GetLogger(ctx)
	attempt := info.Attempt

	logger.Info("UnreliableActivity executing", "attempt", attempt)

	if attempt < 3 {
		return "", fmt.Errorf("simulated transient failure on attempt %d (will retry)", attempt)
	}

	return fmt.Sprintf("activity succeeded on attempt %d", attempt), nil
}
