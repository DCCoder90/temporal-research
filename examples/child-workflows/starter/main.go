package main

import (
	"context"
	"fmt"
	"log"
	"os"

	childworkflows "temporal-child-workflows/workflow"

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

	items := []string{"alpha", "beta", "gamma", "delta", "epsilon"}

	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        "data-pipeline-workflow",
		TaskQueue: "child-workflows-task-queue",
	}, childworkflows.DataPipelineWorkflow, items)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("DataPipelineWorkflow started (WorkflowID: %s, RunID: %s)", we.GetID(), we.GetRunID())
	log.Printf("Fanning out %d child workflows in parallel...", len(items))

	var results []string
	if err := we.Get(context.Background(), &results); err != nil {
		log.Fatalf("Workflow returned an error: %v", err)
	}

	fmt.Printf("\n✓ DataPipelineWorkflow completed — %d items processed:\n", len(results))
	for i, r := range results {
		fmt.Printf("  [%d] %s\n", i+1, r)
	}
	fmt.Println()
}
