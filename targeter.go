package sacura

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	ceformat "github.com/cloudevents/sdk-go/v2/binding/format"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const CloudEventIdHeader = "Cloudevent-Id"

func NewTargeterGenerator(config Config, newUIID func() uuid.UUID, out chan<- ce.Event) vegeta.Targeter {

	return func(target *vegeta.Target) error {

		id := newUIID().String()

		event := cetest.FullEvent()
		event.SetID(id)
		event.SetExtension(BenchmarkTimestampAttribute, fmt.Sprint(time.Now().UnixMilli()))

		if config.Ordered != nil {
			event.SetExtension("partitionkey", fmt.Sprint(rand.Int()%int(config.Ordered.NumPartitionKeys)))
		}

		hdr := http.Header{}
		hdr.Set(cehttp.ContentType, ceformat.JSON.MediaType())
		hdr.Set(CloudEventIdHeader, id)

		body, err := ceformat.JSON.Marshal(&event)
		if err != nil {
			return fmt.Errorf("failed to marshal event %v: %w", event, err)
		}

		*target = vegeta.Target{
			Method: "POST",
			URL:    config.Sender.Target,
			Body:   body,
			Header: hdr,
		}

		out <- event

		return nil
	}
}
