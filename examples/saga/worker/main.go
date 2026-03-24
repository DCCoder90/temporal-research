package main

import (
	"log"
	"os"

	saga "temporal-saga/workflow"

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
	log.Println("Starting saga worker on task queue 'saga-task-queue'...")

	w := worker.New(c, "saga-task-queue", worker.Options{})
	w.RegisterWorkflow(saga.BookingWorkflow)
	w.RegisterActivity(saga.BookFlightActivity)
	w.RegisterActivity(saga.BookHotelActivity)
	w.RegisterActivity(saga.BookCarActivity)
	w.RegisterActivity(saga.CancelFlightActivity)
	w.RegisterActivity(saga.CancelHotelActivity)

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker stopped with error: %v", err)
	}
}
