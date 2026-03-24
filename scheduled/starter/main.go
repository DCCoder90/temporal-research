package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	scheduled "temporal-scheduled/workflow"

	"go.temporal.io/sdk/client"
)

const scheduleID = "scheduled-report"

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal:7233"
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	log.Printf("Connected to Temporal at %s", hostPort)

	ctx := context.Background()
	sc := c.ScheduleClient()

	// Delete any existing schedule so this starter is idempotent.
	if h := sc.GetHandle(ctx, scheduleID); h != nil {
		_ = h.Delete(ctx)
	}

	handle, err := sc.Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			// Trigger a new workflow run every 30 seconds.
			Intervals: []client.ScheduleIntervalSpec{
				{Every: 30 * time.Second},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			Workflow:  scheduled.ScheduledReportWorkflow,
			TaskQueue: "scheduled-task-queue",
		},
	})
	if err != nil {
		log.Fatalf("Failed to create schedule: %v", err)
	}

	fmt.Printf("\n✓ Schedule '%s' created — workflow runs every 30 seconds.\n", handle.GetID())
	fmt.Println("  Watch runs in the Temporal UI: http://localhost:8080")
	fmt.Printf("  To delete the schedule: use the Temporal UI or rerun this starter.\n\n")
}
