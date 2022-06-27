package sacura

import (
	"context"
	"os"
	"testing"
	"time"
)

func Test_Main(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "k=v")
	t.Setenv("OTEL_SERVICE_NAME", "sacura")

	tt := []struct {
		name        string
		path        string
		cancelAfter *time.Duration
	}{
		{
			name: "receiver sleep",
			path: "test/config-receiver-sleep.yaml",
		},
		{
			name: "unordered",
			path: "test/config.yaml",
		},
		{
			name: "ordered",
			path: "test/config-ordered.yaml",
		},
		{
			name: "receiver only",
			path: "test/config-receiver-only.yaml",
			cancelAfter: func() *time.Duration {
				c := 5 * time.Second
				return &c
			}(),
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			f, err := os.Open(tc.path)
			if err != nil {
				t.Fatalf("failed to open file %s: %v", tc.path, err)
			}
			defer f.Close()

			config, err := FileConfig(f)
			if err != nil {
				t.Fatalf("failed to read config from file %s: %v", tc.path, err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			errChan := make(chan error, 1)
			defer close(errChan)

			go func() {
				errChan <- Main(ctx, config)
				t.Logf("Main returned")
			}()

			if tc.cancelAfter != nil {
				go func() {
					<-time.After(*tc.cancelAfter)
					cancel()
				}()
			}

			if err := <-errChan; err != nil {
				t.Fatal(err)
			}

			t.Logf("No errors")
		})
	}
}
