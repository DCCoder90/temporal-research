package main

import (
	"log"
	"os"

	childworkflows "temporal-child-workflows/workflow"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
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
	log.Println("Starting child-workflows worker on task queue 'child-workflows-task-queue'...")

	w := worker.New(c, "child-workflows-task-queue", worker.Options{})
	w.RegisterWorkflow(childworkflows.DataPipelineWorkflow)
	w.RegisterWorkflow(childworkflows.ProcessItemWorkflow)
	w.RegisterActivity(childworkflows.ProcessItemActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
