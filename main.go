package sacura

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
)

func Main(ctx context.Context, config Config) error {

	c, _ := json.Marshal(&config)
	log.Println("config", string(c))

	ctx, cancel := context.WithCancel(ctx)

	log.Println("Creating channels")
	buffer := int(math.Min(float64(int(config.ParsedDuration)*config.Sender.FrequencyPerSecond), math.MaxInt8))

	sent := make(chan ce.Event, buffer)
	received := make(chan ce.Event, buffer)
	var metrics Metrics

	go func() {
		defer close(sent)

		if !config.Sender.Disabled {
			defer cancel()
			log.Println("Starting attacker ...")
			time.Sleep(time.Second * 10) // Waiting for receiver to start
			metrics = StartSender(config, sent)
		}
	}()

	log.Println("Creating state manager ...")
	sm := NewStateManager(config)
	receivedSignal := sm.ReadReceived(received)
	sentSignal := sm.ReadSent(sent)

	log.Println("Starting receiver ...")
	if err := StartReceiver(ctx, config.Receiver, received); err != nil {
		return fmt.Errorf("failed to start receiver: %w", err)
	}

	if !config.Sender.Disabled {
		log.Println("Waiting for attacker to finish ...")
	} else {
		log.Println("Waiting for term signals")
	}
	<-ctx.Done()

	log.Println("Waiting for received channel signal")
	<-receivedSignal

	log.Println("Waiting for sent channel signal")
	<-sentSignal

	sm.Terminated(metrics)
	report := sm.GenerateReport()
	logReport(report)

	if !config.Sender.Disabled && report.Metrics.AcceptedCount == 0 {
		return fmt.Errorf("no events were accepted: %+v", report.Metrics)
	}

	if lost := report.Metrics.AcceptedCount - report.ReceivedCount; !config.Sender.Disabled && lost != 0 {
		return fmt.Errorf("lost count (accepted but not received): %d - %d = %d", report.Metrics.AcceptedCount, sm.ReceivedCount(), lost)
	}

	if report.ReceivedCount > 0 {

		// x: 100 =  duplicateCount : (duplicateCount +  receivedCount)
		// x = 100 * duplicateCount / (duplicateCount + receivedCount)
		duplicatesPercentage := 100 * report.DuplicateCount / (report.DuplicateCount + report.ReceivedCount)

		log.Printf("Duplicates percentage %d", duplicatesPercentage)

		if config.Receiver.MaxDuplicatesPercentage != nil && duplicatesPercentage > *config.Receiver.MaxDuplicatesPercentage {
			return fmt.Errorf("too many duplicates detected %d, expected at most %d, listing duplicates:\n%+v",
				duplicatesPercentage,
				*config.Receiver.MaxDuplicatesPercentage,
				report.DuplicateEventsByPartitionKey,
			)
		}
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
