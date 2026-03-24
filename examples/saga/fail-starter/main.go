package main

import (
	"context"
	"fmt"
	"log"
	"os"

	saga "temporal-saga/workflow"

	"go.temporal.io/sdk/client"
)

// Failure-path starter: hotel booking fails, triggering saga compensation
// (the already-booked flight is automatically cancelled).
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
		CustomerID: "customer-2",
		FlightID:   "BA-200",
		HotelID:    "MARRIOTT-LON",
		CarID:      "AVIS-COM",
		FailHotel:  true, // hotel booking will fail → compensation runs
	}

	we, err := c.ExecuteWorkflow(context.Background(), client.StartWorkflowOptions{
		ID:        "booking-workflow-fail",
		TaskQueue: "saga-task-queue",
	}, saga.BookingWorkflow, req)
	if err != nil {
		log.Fatalf("Unable to start workflow: %v", err)
	}

	log.Printf("BookingWorkflow (fail scenario) started (WorkflowID: %s, RunID: %s)", we.GetID(), we.GetRunID())
	log.Println("Hotel booking will fail — watch compensation (CancelFlight) execute automatically...")

	var result string
	if err := we.Get(context.Background(), &result); err != nil {
		fmt.Printf("\n✓ Workflow ended with expected error (saga compensated): %v\n\n", err)
		return
	}

	log.Fatalf("Expected failure but workflow succeeded: %s", result)
}
