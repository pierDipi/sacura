package sacura

import (
	"fmt"
	"math/rand"
	"net/http"

	ce "github.com/cloudevents/sdk-go/v2"
	ceformat "github.com/cloudevents/sdk-go/v2/binding/format"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func NewTargeterGenerator(config Config, out chan<- ce.Event) vegeta.Targeter {

	return func(target *vegeta.Target) error {

		id := uuid.New().String()

		event := cetest.FullEvent()
		event.SetID(id)

		if config.Ordered != nil {
			event.SetExtension("partitionkey", fmt.Sprint(rand.Int()%int(config.Ordered.NumPartitionKeys)))
		}

		hdr := http.Header{}
		hdr.Set(cehttp.ContentType, ceformat.JSON.MediaType())

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
