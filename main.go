package sacura

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
)

func Main(config Config) error {

	c, _ := json.Marshal(&config)
	log.Println("config", string(c))

	ctx, cancel := context.WithCancel(NewContext())

	log.Println("Creating channels")
	buffer := int(math.Min(float64(int(config.ParsedDuration)*config.Sender.FrequencyPerSecond), math.MaxInt8))

	sent := make(chan ce.Event, buffer)
	received := make(chan ce.Event, buffer)
	var metrics Metrics

	go func() {
		defer cancel()
		defer close(sent)

		log.Println("Starting attacker ...")

		time.Sleep(time.Second * 10) // Waiting for receiver to start

		metrics = StartSender(config, sent)
	}()

	log.Println("Creating state manager ...")
	sm := NewStateManager(StateManagerConfigFromConfig(config))
	receivedSignal := sm.ReadReceived(received)
	sentSignal := sm.ReadSent(sent)

	log.Println("Starting receiver ...")
	if err := StartReceiver(ctx, config.Receiver, received); err != nil {
		return fmt.Errorf("failed to start receiver: %w", err)
	}

	log.Println("Waiting for attacker to finish ...")
	<-ctx.Done()

	log.Println("Attacker finished sending events - waiting for events")
	<-time.After(config.ParsedTimeout)

	log.Println("Waiting for received channel signal")
	<-receivedSignal

	log.Println("Waiting for sent channel signal")
	<-sentSignal

	sm.Terminated(metrics)
	report := sm.GenerateReport()
	logReport(report)

	if report.Metrics.AcceptedCount == 0 {
		return fmt.Errorf("no events were accepted: %+v", report.Metrics)
	}

	if lost := report.Metrics.AcceptedCount - report.ReceivedCount; lost != 0 {
		return fmt.Errorf("lost count (accepted but not received): %d - %d = %d", report.Metrics.AcceptedCount, sm.ReceivedCount(), lost)
	}

	duplicatesPercentage := (((report.DuplicateCount + report.ReceivedCount) / report.Metrics.AcceptedCount) - 1) * 100
	log.Printf("Duplicates percentage %d", duplicatesPercentage)

	if config.Receiver.MaxDuplicatesPercentage != nil && duplicatesPercentage > *config.Receiver.MaxDuplicatesPercentage {
		return fmt.Errorf("too many duplicates detected %d, expected at most %d, listing duplicates:\n%+v",
			duplicatesPercentage,
			config.Receiver.MaxDuplicatesPercentage,
			report.DuplicateEventsByPartitionKey,
		)
	}

	return nil
}

func logReport(report Report) {
	jsonReport, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		log.Println("failed to marshal report", err)
		return
	}

	log.Println("report", string(jsonReport))
}

// NewContext creates a new context with signal handling.
func NewContext() context.Context {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	return ctx
}
