package main

import (
	"context"
	"fmt"
	"log"
	"os"

	saga "temporal-saga/workflow"

	"go.temporal.io/sdk/client"
)

// Happy-path starter: all three bookings succeed.
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

	req := saga.BookingRequest{
		CustomerID: "customer-1",
		FlightID:   "AA-100",
		HotelID:    "HILTON-NYC",
		CarID:      "HERTZ-MID",
		FailHotel:  false,
	}

	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        "booking-workflow",
		TaskQueue: "saga-task-queue",
	}, saga.BookingWorkflow, req)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("BookingWorkflow started (WorkflowID: %s, RunID: %s)", we.GetID(), we.GetRunID())

	var result string
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow returned an error: %v", err)
	}

	fmt.Printf("\n✓ %s\n\n", result)
}
