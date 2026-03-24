package main

import (
	"context"
	"fmt"
	"log"
	"os"

	signals "temporal-signals/workflow"

	"go.temporal.io/sdk/client"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal:7233"
	}

	workflowID := os.Getenv("WORKFLOW_ID")
	if workflowID == "" {
		workflowID = "approval-workflow"
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	if err := c.SignalWorkflow(context.Background(), workflowID, "", signals.ApproveSignal, nil); err != nil {
		log.Fatalf("Failed to send approve signal to '%s': %v", workflowID, err)
	}

	fmt.Printf("\n✓ Approve signal sent to workflow '%s'\n\n", workflowID)
}
