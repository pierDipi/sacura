package sacura

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
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
	"go.uber.org/atomic"
)

var (
	latencyHistogram       syncint64.Histogram  = nil
	latencyHistogramLabels []attribute.KeyValue = nil

	inFlightRequestsHistogram       syncint64.Histogram  = nil
	inFlightRequestsHistogramLabels []attribute.KeyValue = nil
)

const (
	BenchmarkTimestampAttribute = "benchmarktimestamp"
)

func StartReceiver(ctx context.Context, config ReceiverConfig, received chan<- ce.Event) error {
	defer close(received)

	protocol, err := cehttp.New(
		cehttp.WithPort(config.Port),
		cehttp.WithRequestDataAtContextMiddleware(),
	)
	if err != nil {
		return fmt.Errorf("failed to create protocol: %w", err)
	}

	client, err := ceclient.New(protocol,
		ceclient.WithPollGoroutines(100),
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	innerCtx, cancel := context.WithCancel(context.Background())
	wait := exportMetrics(innerCtx)
	defer wait()

	go func() {
		defer cancel()

		<-ctx.Done()
		if err := ctx.Err(); err != nil {
			log.Println(err)
		}
		<-time.After(config.ParsedTimeout)

		log.Println("Receiver timeout reached")
	}()

	inFlightRequests := atomic.NewInt64(0)

	err = client.StartReceiver(innerCtx, func(ctx context.Context, event ce.Event) {
		req := cehttp.RequestDataFromContext(ctx)

		inFlightRequests.Inc()
		inFlightRequestsHistogramReqLabels := addRequestLabels(req, inFlightRequestsHistogramLabels)
		inFlightRequestsHistogram.Record(ctx, inFlightRequests.Load(), inFlightRequestsHistogramReqLabels...)
		defer func() {
			inFlightRequests.Dec()
			inFlightRequestsHistogram.Record(ctx, inFlightRequests.Load(), inFlightRequestsHistogramReqLabels...)
		}()

		exstensions := event.Extensions()
		if v, ok := exstensions[BenchmarkTimestampAttribute]; ok {
			t, err := strconv.ParseInt(v.(string), 10, 64)
			if err != nil {
				panic(err)
			}
			start := time.UnixMilli(t)
			latency := time.Since(start)
			if latency.Milliseconds() < 0 {
				log.Printf("Negative latency %d\n", latency.Milliseconds())
			} else {
				latencyHistogram.Record(ctx, latency.Milliseconds(), addRequestLabels(req, latencyHistogramLabels)...)
			}
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

func addRequestLabels(req *cehttp.RequestData, latencyHistogramLabels []attribute.KeyValue) []attribute.KeyValue {
	labels := make([]attribute.KeyValue, 0, len(latencyHistogramLabels)+2)
	copy(labels, latencyHistogramLabels)
	path := "/"
	if req.URL.Path != "" {
		path = req.URL.Path
	}
	labels = append(labels, attribute.String("request_path", path))
	labels = append(labels, attribute.String("remote_addr", req.RemoteAddr))
	return labels
}

func maybeSleep(config ReceiverConfig) {
	if config.ReceiverFaultConfig == nil || config.ReceiverFaultConfig.MinSleepDuration == nil {
		return
	}

	max := *config.ReceiverFaultConfig.MaxSleepDuration
	min := *config.ReceiverFaultConfig.MinSleepDuration

	time.Sleep(min + time.Duration(rand.Int63n(int64(max-min))))
}

func exportMetrics(ctx context.Context) (wait func()) {
	config := prometheus.Config{
		DefaultHistogramBoundaries: []float64{
			10, 20, 50, 100, 500, 1000, // < 1s
			5 * 1000, 10 * 1000, 30 * 1000, 60 * 1000, // < 60s
			5 * 60 * 1000, 10 * 60 * 1000, 20 * 60 * 1000, // < 20m
		},
	}

	ctrl := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)

	promExporter, err := prometheus.New(config, ctrl)
	if err != nil {
		panic(err)
	}

	global.SetMeterProvider(ctrl)

	meter := global.Meter("sacura")

	latencyHistogram, err = meter.SyncInt64().Histogram("latency_e2e_ms",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Histogram for E2E latency since "+BenchmarkTimestampAttribute),
	)
	if err != nil {
		panic(err)
	}
	inFlightRequestsHistogram, err = meter.SyncInt64().Histogram("in_flight_requests",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Histogram for in-flight requests"),
	)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		s := http.Server{
			Handler: http.HandlerFunc(promExporter.ServeHTTP),
			Addr:    ":9090",
		}
		defer s.Close()

		go func() {
			if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}()

		<-ctx.Done()
		scrapeMetrics()

		log.Println("Metrics server closed")
	}()

	return wg.Wait
}

func scrapeMetrics() {
	resp, err := http.DefaultClient.Get("http://localhost:9090")
	if err != nil {
		panic(err)
	}
	if resp.StatusCode >= 300 {
		panic("expected status code 2xx, got " + fmt.Sprint(resp.StatusCode))
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	log.Println("Metrics\n", string(body))
}
