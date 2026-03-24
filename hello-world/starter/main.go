package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	helloworld "temporal-helloworld/workflow"

	"go.temporal.io/sdk/client"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "temporal-frontend:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: hostPort,
	})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	log.Printf("Connected to Temporal at %s", hostPort)

	opts := client.StartWorkflowOptions{
		ID:        "hello-world-workflow",
		TaskQueue: "hello-world-task-queue",
	}

	// Retry starting the workflow to handle the case where the worker hasn't
	// registered on the task queue yet.
	var we client.WorkflowRun
	for attempt := 1; attempt <= 30; attempt++ {
		we, err = c.ExecuteWorkflow(context.Background(), opts, helloworld.HelloWorldWorkflow, "World")
		if err == nil {
			break
		}
		log.Printf("Attempt %d/30 failed: %v — retrying in 5s...", attempt, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatalf("Unable to start workflow after 30 attempts: %v", err)
	}

	log.Printf("Workflow started (WorkflowID: %s, RunID: %s)", we.GetID(), we.GetRunID())

	var result string
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow returned an error: %v", err)
	}

	fmt.Printf("\n✓ Workflow result: %s\n\n", result)
}
