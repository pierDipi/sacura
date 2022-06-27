package sacura

import (
	"os"
	"testing"
)

func Test_Main(t *testing.T) {
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "k=v")
	t.Setenv("OTEL_SERVICE_NAME", "sacura")

	tt := []struct {
		name string
		path string
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

			if err := Main(config); err != nil {
				t.Error(err)
			}
		})
	}
}
