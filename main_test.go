package sacura

import (
	"os"
	"testing"
)

func Test_Main(t *testing.T) {
	tt := []struct {
		name string
		path string
	}{
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
