package saga

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BookingRequest is the input to BookingWorkflow.
// Set FailHotel to true to simulate a hotel booking failure and trigger
// saga compensation (cancelling the already-booked flight).
type BookingRequest struct {
	CustomerID string
	FlightID   string
	HotelID    string
	CarID      string
	FailHotel  bool // inject failure to demonstrate compensation
}

// BookingWorkflow books a flight, hotel, and car rental in sequence.
// Each successful booking registers a compensation (cancellation) action.
// If any step fails, all previously completed bookings are rolled back in
// reverse order — the saga pattern.
//
// Observe in Wireshark with FailHotel=true: you will see the flight booking
// RPC succeed, the hotel booking RPC fail, then two compensation RPCs
// (CancelFlight) before the workflow errors out.
func BookingWorkflow(ctx workflow.Context, req BookingRequest) (string, error) {
	logger := workflow.GetLogger(ctx)

	// Disable retries for booking activities so failures are immediate.
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 1},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// compensations holds rollback functions in registration order.
	// On failure we execute them in reverse (LIFO).
	var compensations []func(workflow.Context)

	compensate := func(ctx workflow.Context) {
		for i := len(compensations) - 1; i >= 0; i-- {
			compensations[i](ctx)
		}
	}

	// ── Step 1: Book flight ──────────────────────────────────────────────────
	var flightConf string
	if err := workflow.ExecuteActivity(ctx, BookFlightActivity, req.FlightID).Get(ctx, &flightConf); err != nil {
		return "", fmt.Errorf("flight booking failed: %w", err)
	}
	logger.Info("Flight booked", "confirmation", flightConf)
	compensations = append(compensations, func(ctx workflow.Context) {
		_ = workflow.ExecuteActivity(ctx, CancelFlightActivity, req.FlightID).Get(ctx, nil)
	})

	// ── Step 2: Book hotel ───────────────────────────────────────────────────
	var hotelConf string
	if err := workflow.ExecuteActivity(ctx, BookHotelActivity, req.HotelID, req.FailHotel).Get(ctx, &hotelConf); err != nil {
		logger.Info("Hotel booking failed — running compensation")
		compensate(ctx)
		return "", fmt.Errorf("hotel booking failed, saga compensated: %w", err)
	}
	logger.Info("Hotel booked", "confirmation", hotelConf)
	compensations = append(compensations, func(ctx workflow.Context) {
		_ = workflow.ExecuteActivity(ctx, CancelHotelActivity, req.HotelID).Get(ctx, nil)
	})

	// ── Step 3: Book car ─────────────────────────────────────────────────────
	var carConf string
	if err := workflow.ExecuteActivity(ctx, BookCarActivity, req.CarID).Get(ctx, &carConf); err != nil {
		logger.Info("Car booking failed — running compensation")
		compensate(ctx)
		return "", fmt.Errorf("car booking failed, saga compensated: %w", err)
	}
	logger.Info("Car booked", "confirmation", carConf)

	return fmt.Sprintf("booking complete — flight: %s  hotel: %s  car: %s",
		flightConf, hotelConf, carConf), nil
}

// ── Activities ────────────────────────────────────────────────────────────────

func BookFlightActivity(ctx context.Context, flightID string) (string, error) {
	activity.GetLogger(ctx).Info("Booking flight", "flightID", flightID)
	return fmt.Sprintf("FL-%s-CONF", flightID), nil
}

func BookHotelActivity(ctx context.Context, hotelID string, fail bool) (string, error) {
	logger := activity.GetLogger(ctx)
	if fail {
		logger.Info("Hotel booking failed (injected failure)", "hotelID", hotelID)
		return "", fmt.Errorf("hotel %s unavailable (injected failure)", hotelID)
	}
	logger.Info("Booking hotel", "hotelID", hotelID)
	return fmt.Sprintf("HT-%s-CONF", hotelID), nil
}

func BookCarActivity(ctx context.Context, carID string) (string, error) {
	activity.GetLogger(ctx).Info("Booking car", "carID", carID)
	return fmt.Sprintf("CR-%s-CONF", carID), nil
}

func CancelFlightActivity(ctx context.Context, flightID string) error {
	activity.GetLogger(ctx).Info("Cancelling flight (compensation)", "flightID", flightID)
	return nil
}

func CancelHotelActivity(ctx context.Context, hotelID string) error {
	activity.GetLogger(ctx).Info("Cancelling hotel (compensation)", "hotelID", hotelID)
	return nil
}
