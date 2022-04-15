package sacura

import (
	"net/http"
	"testing"

	ce "github.com/cloudevents/sdk-go/v2"
	ceformat "github.com/cloudevents/sdk-go/v2/binding/format"
	cehttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"github.com/google/go-cmp/cmp"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestNewTargeterGenerator(t *testing.T) {

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
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			out := make(chan ce.Event, 1)
			f := NewTargeterGenerator(Config{Sender: SenderConfig{Target: tt.targetURL}}, out)

			target := &vegeta.Target{}
			if err := f(target); err != nil {
				t.Fatal(err)
			}

			filter := func(path cmp.Path) bool {
				return path.String() == "Body"
			}

			if diff := cmp.Diff(tt.want, *target, cmp.FilterPath(filter, cmp.Ignore())); diff != "" {
				t.Fatal("(-want, +got)", diff)
			}
		})
	}
}
