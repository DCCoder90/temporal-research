package main

import (
	"log"
	"os"

	retries "temporal-retries/workflow"

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
	log.Println("Starting retries worker on task queue 'retries-task-queue'...")

	w := worker.New(c, "retries-task-queue", worker.Options{})
	w.RegisterWorkflow(retries.UnreliableWorkflow)
	w.RegisterActivity(retries.UnreliableActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
