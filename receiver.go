package sacura

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	ceclient "github.com/cloudevents/sdk-go/v2/client"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/metric/unit"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var (
	latencyHistogram       syncint64.Histogram  = nil
	latencyHistogramLabels []attribute.KeyValue = nil
)

const (
	BenchmarkTimestampAttribute = "ce-benchmarktimestamp"
)

func init() {
	meter := global.MeterProvider().Meter("sacura")

	keyValueLabels := strings.Split(os.Getenv("LATENCY_E2E_METRIC_LABELS"), ",")
	labels := make([]attribute.KeyValue, 0, len(keyValueLabels)*2)
	for _, kv := range keyValueLabels {
		parts := strings.Split(kv, "=")
		labels = append(labels, attribute.String(parts[0], parts[1]))
	}
	latencyHistogramLabels = labels

	var err error
	latencyHistogram, err = meter.SyncInt64().Histogram("latency_e2e",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Histogram for E2E latency since "+BenchmarkTimestampAttribute),
	)
	if err != nil {
		panic(err)
	}
}

func StartReceiver(ctx context.Context, config ReceiverConfig, received chan<- ce.Event) error {
	exportMetrics(ctx)

	defer close(received)

	protocol, err := cehttp.New(cehttp.WithPort(config.Port))
	if err != nil {
		return fmt.Errorf("failed to create protocol: %w", err)
	}

	client, err := ceclient.New(protocol)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	innerCtx, cancel := context.WithCancel(context.Background())

	go func() {
		defer cancel()

		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			log.Println(err)
		}
		<-time.After(config.ParsedTimeout)
	}()

	err = client.StartReceiver(innerCtx, func(ctx context.Context, event ce.Event) {
		exstensions := event.Extensions()
		if v, ok := exstensions[BenchmarkTimestampAttribute]; ok {
			t, err := strconv.ParseInt(v.(string), 10, 64)
			if err != nil {
				panic(err)
			}
			start := time.Unix(t, 0)
			latency := time.Since(start)
			latencyHistogram.Record(ctx, int64(latency), latencyHistogramLabels...)
		}

		maybeSleep(config)
		received <- event
	})
	if err != nil {
		select {
		case <-innerCtx.Done():
			// There is not way ATM to know whether the receiver has been terminated because of the cancelled context or
			// because there was an error, so if context is done suppress the error.
			log.Println(err)
		default:
			return fmt.Errorf("failed to start receiver: %w", err)
		}
	}

	return nil
}

func maybeSleep(config ReceiverConfig) {
	if config.ReceiverFaultConfig == nil || config.ReceiverFaultConfig.MinSleepDuration == nil {
		return
	}

	max := *config.ReceiverFaultConfig.MaxSleepDuration
	min := *config.ReceiverFaultConfig.MinSleepDuration

	time.Sleep(min + time.Duration(rand.Int63n(int64(max-min))))
}

func exportMetrics(ctx context.Context) {
	config := prometheus.Config{}

	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)

	exporter, err := prometheus.New(config, ctrl)
	if err != nil {
		panic(err)
	}

	global.SetMeterProvider(exporter.MeterProvider())

	go func() {
		s := http.Server{
			Handler: http.HandlerFunc(exporter.ServeHTTP),
		}
		go func() {
			if err := s.ListenAndServe(); err != nil {
				log.Fatal(err)
			}
		}()

		<-ctx.Done()
		_ = s.Close()
	}()
}
