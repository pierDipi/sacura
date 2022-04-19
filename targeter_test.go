package sacura

import (
	"net/http"
	"testing"

	ce "github.com/cloudevents/sdk-go/v2"
	ceformat "github.com/cloudevents/sdk-go/v2/binding/format"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestNewTargeterGenerator(t *testing.T) {

	id := uuid.MustParse("8da9496e-f4a1-4103-b6dd-3183c9d6e5ee")

	tests := []struct {
		name      string
		targetURL string
		want      vegeta.Target
	}{
		{
			name:      "happy case",
			targetURL: "http://localhost:9090",
			want: vegeta.Target{
				Method: "POST",
				URL:    "http://localhost:9090",
				Header: http.Header{
					cehttp.ContentType: []string{ceformat.JSON.MediaType()},
					CloudEventIdHeader: []string{id.String()},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			out := make(chan ce.Event, 1)
			f := NewTargeterGenerator(Config{Sender: SenderConfig{Target: tt.targetURL}}, func() uuid.UUID { return id }, out)

			target := &vegeta.Target{}
			if err := f(target); err != nil {
				t.Fatal(err)
			}

			filter := func(path cmp.Path) bool {
				return path.String() == "Body"
			}

			if diff := cmp.Diff(tt.want, *target, cmpopts.SortMaps(func(x, y string) bool { return x < y }), cmp.FilterPath(filter, cmp.Ignore())); diff != "" {
				t.Fatal("(-want, +got)", diff)
			}
		})
	}
}
