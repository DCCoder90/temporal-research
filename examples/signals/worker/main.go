package main

import (
	"log"
	"os"

	signals "temporal-signals/workflow"

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
	log.Println("Starting signals worker on task queue 'signals-task-queue'...")

	w := worker.New(c, "signals-task-queue", worker.Options{})
	w.RegisterWorkflow(signals.ApprovalWorkflow)
	w.RegisterActivity(signals.ProcessOrderActivity)
	w.RegisterActivity(signals.CancelOrderActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
