package childworkflows

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// DataPipelineWorkflow is the parent workflow. It fans out to one child
// workflow per item in parallel, collecting all results before returning.
// Observe in Wireshark: you will see separate workflow task completions for
// each child alongside the parent's coordination traffic.
func DataPipelineWorkflow(ctx workflow.Context, items []string) ([]string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("DataPipelineWorkflow started", "itemCount", len(items))

	childOpts := workflow.ChildWorkflowOptions{
		TaskQueue: "child-workflows-task-queue",
	}

	// Launch all child workflows in parallel.
	futures := make([]workflow.ChildWorkflowFuture, len(items))
	for i, item := range items {
		futures[i] = workflow.ExecuteChildWorkflow(
			workflow.WithChildOptions(ctx, childOpts),
			ProcessItemWorkflow,
			item,
		)
	}

	// Collect results in order.
	results := make([]string, len(items))
	for i, f := range futures {
		if err := f.Get(ctx, &results[i]); err != nil {
			return nil, fmt.Errorf("child workflow %d failed: %w", i, err)
		}
	}

	logger.Info("DataPipelineWorkflow completed", "results", results)
	return results, nil
}

// ProcessItemWorkflow is the child workflow. Each instance processes a single
// item and calls ProcessItemActivity.
func ProcessItemWorkflow(ctx workflow.Context, item string) (string, error) {
	ao := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	if err := workflow.ExecuteActivity(ctx, ProcessItemActivity, item).Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

func ProcessItemActivity(ctx context.Context, item string) (string, error) {
	activity.GetLogger(ctx).Info("ProcessItemActivity", "item", item)
	return fmt.Sprintf("processed: %s", item), nil
}
