package helloworld

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// HelloWorldWorkflow is a simple workflow that executes a single activity and
// returns a greeting string.
func HelloWorldWorkflow(ctx workflow.Context, name string) (string, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	err := workflow.ExecuteActivity(ctx, HelloWorldActivity, name).Get(ctx, &result)
	if err != nil {
		return "", err
	}

	workflow.GetLogger(ctx).Info("HelloWorldWorkflow completed", "result", result)
	return result, nil
}

// HelloWorldActivity is the activity executed by the workflow. It builds a
// greeting and logs the activity heartbeat info.
func HelloWorldActivity(ctx context.Context, name string) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("HelloWorldActivity executing", "name", name)

	greeting := fmt.Sprintf("Hello, %s! (from Temporal activity)", name)
	return greeting, nil
}
