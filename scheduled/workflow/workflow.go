package scheduled

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// ScheduledReportWorkflow is triggered by a Temporal Schedule on a fixed
// interval. Each run generates a timestamped report via an activity.
func ScheduledReportWorkflow(ctx workflow.Context) (string, error) {
	ao := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var result string
	if err := workflow.ExecuteActivity(ctx, GenerateReportActivity).Get(ctx, &result); err != nil {
		return "", err
	}

	workflow.GetLogger(ctx).Info("ScheduledReportWorkflow completed", "result", result)
	return result, nil
}

// GenerateReportActivity simulates report generation.
func GenerateReportActivity(ctx context.Context) (string, error) {
	logger := activity.GetLogger(ctx)
	report := fmt.Sprintf("report generated at %s", time.Now().UTC().Format(time.RFC3339))
	logger.Info("GenerateReportActivity", "report", report)
	return report, nil
}
