package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"knative.dev/pkg/signals"

	"github.com/pierdipi/sacura"
)

const (
	filePathFlag = "config"
)

func main() {

	path := flag.String(filePathFlag, "", "Path to the configuration file")
	flag.Parse()

	if path == nil || *path == "" {
		log.Printf("invalid flag %s", filePathFlag)
		usage()
		return
	}

	if err := run(*path); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	log.Printf(`
sacura --%s <absolute_path_to_config_file>
`, filePathFlag)
}

func run(path string) error {

	log.Println("Reading configuration ...")

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", path, err)
	}

	config, err := sacura.FileConfig(f)
	if err != nil {
		return fmt.Errorf("failef to read config from file %s: %w", path, err)
	}

	ctx, cancel := context.WithCancel(signals.NewContext())

	buffer := int(math.Min(float64(int(config.ParsedDuration)*config.Sender.FrequencyPerSecond), math.MaxInt32))

	sent := make(chan string, buffer)
	received := make(chan string, buffer)

	go func() {
		defer cancel()
		defer close(received)
		defer close(sent)

		log.Println("Starting attacker ...")

		time.Sleep(time.Second * 10) // Waiting for receiver to start

		metrics := sacura.StartSender(config, sent)
		logMetrics(metrics)
	}()

	sm := sacura.NewStateManager()
	sm.ReadReceived(received)
	sm.ReadSent(sent)

	protocol, err := cehttp.New(cehttp.WithPort(config.Receiver.Port))
	if err != nil {
		return fmt.Errorf("failed to create protocol: %w", err)
	}

	client, err := ceclient.New(protocol)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	log.Println("Starting receiver ...")
	err = client.StartReceiver(ctx, func(ctx context.Context, event ce.Event) {
		received <- event.ID()
	})
	if err != nil {
		return fmt.Errorf("failed to start receiver: %w", err)
	}

	log.Println("Waiting for attacker to finish ...")
	<-ctx.Done()
	<-time.After(config.ParsedTimeout)

	if diff := sm.Diff(); diff != "" {
		return fmt.Errorf("set state is not correct: %s", diff)
	}

	return nil
}

func logMetrics(metrics vegeta.Metrics) {
	jsonMetrics, err := json.MarshalIndent(metrics, " ", " ")
	if err != nil {
		log.Println("failed to marshal metrics", err)
		return
	}

	log.Println("metrics", string(jsonMetrics))
}
