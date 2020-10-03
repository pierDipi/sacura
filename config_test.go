package sacura

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestFileConfig(t *testing.T) {

	tests := []struct {
		name    string
		r       io.Reader
		want    Config
		wantErr bool
	}{
		{
			name: "happy case",
			r: strings.NewReader(`
sender:
  target: http://localhost:8080
  frequency: 1000
  workers: 100
  keepAlive: true
receiver:
  port: 8080
  timeout: 1m
duration: 1m
timeout: 1m
`),
			want: Config{
				Sender: SenderConfig{
					Target:             "http://localhost:8080",
					FrequencyPerSecond: 1000,
					Workers:            100,
					KeepAlive:          true,
				},
				Receiver: ReceiverConfig{
					Port:          8080,
					Timeout:       "1m",
					ParsedTimeout: time.Minute,
				},
				Duration:       "1m",
				ParsedDuration: time.Minute,
				Timeout:        "1m",
				ParsedTimeout:  time.Minute,
			},
			wantErr: false,
		},
		{
			name: "invalid target",
			r: strings.NewReader(`
sender:
  target: /target
  frequency: 1000
  workers: 100
  keepAlive: true
receiver:
  port: 8080
  timeout: 1m
duration: 1m
timeout: 1m
`),
			want: Config{
				Sender: SenderConfig{
					Target:             "/target",
					FrequencyPerSecond: 1000,
					Workers:            100,
					KeepAlive:          true,
				},
				Receiver: ReceiverConfig{
					Port:          8080,
					Timeout:       "1m",
					ParsedTimeout: 0, // it isn't set since we check the invalid field before checking timeout
				},
				Duration:       "1m",
				ParsedDuration: time.Minute,
				Timeout:        "1m",
				ParsedTimeout:  time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid frequency",
			r: strings.NewReader(`
sender:
  target: http://localhost:8080
  frequency: -1
  workers: 100
  keepAlive: true
receiver:
  port: 8080
  timeout: 1m
duration: 1m
timeout: 1m
`),
			want: Config{
				Sender: SenderConfig{
					Target:             "http://localhost:8080",
					FrequencyPerSecond: -1,
					Workers:            100,
					KeepAlive:          true,
				},
				Receiver: ReceiverConfig{
					Port:          8080,
					Timeout:       "1m",
					ParsedTimeout: 0, // it isn't set since we check the invalid field before checking timeout
				},
				Duration:       "1m",
				ParsedDuration: time.Minute,
				Timeout:        "1m",
				ParsedTimeout:  time.Minute,
			},
			wantErr: true,
		},
		{
			name: "invalid duration",
			r: strings.NewReader(`
sender:
  target: http://localhost:8080
  frequency: 1000
  workers: 100
  keepAlive: true
receiver:
  port: 8080
  timeout: 1m
duration: 1H
timeout: 1m
`),
			want: Config{
				Sender: SenderConfig{
					Target:             "http://localhost:8080",
					FrequencyPerSecond: 1000,
					Workers:            100,
					KeepAlive:          true,
				},
				Receiver: ReceiverConfig{
					Port:          8080,
					Timeout:       "1m",
					ParsedTimeout: 0, // it isn't set since we check the invalid field before checking timeout
				},
				Duration:       "1H",
				ParsedDuration: 0,
				Timeout:        "1m",
				ParsedTimeout:  0, // it isn't set since we check the invalid field before checking timeout
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			r: strings.NewReader(`
sender:
  target: http://localhost:8080
  frequency: 1000
  workers: 100
  keepAlive: true
receiver:
  port: 8080
  timeout: 1m
duration: 1h
timeout: 1H
`),
			want: Config{
				Sender: SenderConfig{
					Target:             "http://localhost:8080",
					FrequencyPerSecond: 1000,
					Workers:            100,
					KeepAlive:          true,
				},
				Receiver: ReceiverConfig{
					Port:          8080,
					Timeout:       "1m",
					ParsedTimeout: 0, // it isn't set since we check the invalid field before checking timeout
				},
				Duration:       "1h",
				ParsedDuration: time.Hour,
				Timeout:        "1H",
				ParsedTimeout:  0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FileConfig(tt.r)
			if (err != nil) != tt.wantErr {
				t.Errorf("FileConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t.Log(err)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("FileConfig() got = %v, want %v - (-want, +got) %s", got, tt.want, diff)
			}
		})
	}
}
