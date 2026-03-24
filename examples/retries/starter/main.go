package main

import (
	"context"
	"fmt"
	"log"
	"os"

	retries "temporal-retries/workflow"

	"go.temporal.io/sdk/client"
)

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
		ID:        "unreliable-workflow",
		TaskQueue: "retries-task-queue",
	}, retries.UnreliableWorkflow)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("UnreliableWorkflow started (WorkflowID: %s, RunID: %s)", we.GetID(), we.GetRunID())
	log.Println("Activity will fail on attempts 1 and 2, then succeed on attempt 3...")

	var result string
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow returned an error: %v", err)
	}

	fmt.Printf("\n✓ Workflow result: %s\n\n", result)
}
