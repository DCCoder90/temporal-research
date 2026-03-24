package main

import (
	"context"
	"fmt"
	"log"
	"os"

	signals "temporal-signals/workflow"

	"go.temporal.io/sdk/client"
)

const workflowID = "approval-workflow"

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

	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "signals-task-queue",
	}, signals.ApprovalWorkflow, "order-123")
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	fmt.Printf("\n✓ ApprovalWorkflow started (WorkflowID: %s, RunID: %s)\n", we.GetID(), we.GetRunID())
	fmt.Println("  The workflow is now blocking, waiting for a signal.")
	fmt.Println("  Approve: docker compose run --rm signals-approve")
	fmt.Println("  Reject:  docker compose run --rm signals-reject\n")
}
