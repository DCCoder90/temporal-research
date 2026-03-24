package main

import (
	"log"
	"os"

	scheduled "temporal-scheduled/workflow"

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
	log.Println("Starting scheduled worker on task queue 'scheduled-task-queue'...")

	w := worker.New(c, "scheduled-task-queue", worker.Options{})
	w.RegisterWorkflow(scheduled.ScheduledReportWorkflow)
	w.RegisterActivity(scheduled.GenerateReportActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
