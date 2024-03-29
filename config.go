package sacura

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"time"

	"github.com/go-yaml/yaml"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type Config struct {
	Sender SenderConfig `json:"sender" yaml:"sender"`

	Receiver ReceiverConfig `json:"receiver" yaml:"receiver"`

	Duration string         `json:"duration" yaml:"duration"`
	Ordered  *OrderedConfig `json:"ordered" yaml:"ordered"`

	ParsedDuration time.Duration
}

type OrderedConfig struct {
	NumPartitionKeys uint8 `json:"numPartitionKeys" yaml:"numPartitionKeys"`
}

type SenderConfig struct {
	Disabled           bool   `json:"disabled" yaml:"disabled"`
	Target             string `json:"target" yaml:"target"`
	FrequencyPerSecond int    `json:"frequency" yaml:"frequency"`
	Workers            uint64 `json:"workers" yaml:"workers"`
	KeepAlive          bool   `json:"keepAlive" yaml:"keepAlive"`
}

type ReceiverConfig struct {
	Port                      int    `json:"port" yaml:"port"`
	Timeout                   string `json:"timeout" yaml:"timeout"`
	MaxDuplicatesPercentage   *int   `json:"maxDuplicatesPercentage" yaml:"maxDuplicatesPercentage"`
	IncludeRemoteAddressLabel *bool  `json:"includeRemoteAddressLabel" yaml:"includeRemoteAddressLabel"`

	ReceiverFaultConfig *ReceiverFaultConfig `json:"fault" yaml:"fault"`

	ParsedTimeout time.Duration
}

type ReceiverFaultConfig struct {
	// MinSleepDuration is the minimum duration to sleep before sending the response.
	//
	// When MinSleepDuration is specified, MaxSleepDuration must be specified.
	MinSleepDuration *time.Duration `json:"minSleepDuration" yaml:"minSleepDuration"`
	// MaxSleepDuration is the maximum duration to sleep before sending the response.
	MaxSleepDuration *time.Duration `json:"maxSleepDuration" yaml:"maxSleepDuration"`
}

func FileConfig(r io.Reader) (Config, error) {

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read: %w", err)
	}

	config := &Config{}
	if err := yaml.Unmarshal(b, config); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal file content %s: %w", string(b), err)
	}

	return *config, config.validate()
}

func (c *Config) validate() error {
	var err error

	c.ParsedDuration, err = time.ParseDuration(c.Duration)
	if err != nil {
		return invalidErr("duration", err)
	}

	if !c.Sender.Disabled && c.Sender.FrequencyPerSecond <= 0 {
		return invalidErr("sender.frequency", errors.New("frequency cannot be less or equal to 0"))
	}

	if !c.Sender.Disabled && c.Sender.Target == "" {
		return invalidErr("sender.target", errors.New("target cannot be empty"))
	}

	if c.Receiver.MaxDuplicatesPercentage != nil && *c.Receiver.MaxDuplicatesPercentage < 0 {
		return invalidErr("receiver.maxDuplicatesPercentage", errors.New("cannot be negative"))
	}

	if c.Receiver.ReceiverFaultConfig != nil && c.Receiver.ReceiverFaultConfig.MinSleepDuration != nil {
		if c.Receiver.ReceiverFaultConfig.MaxSleepDuration == nil {
			return invalidErr(
				"receiver.fault.maxSleepDuration",
				fmt.Errorf(
					"maxSleepDuration must be specified when minSleepDuration (%v) is configured",
					c.Receiver.ReceiverFaultConfig.MinSleepDuration,
				),
			)
		}
	}

	if u, err := url.Parse(c.Sender.Target); !c.Sender.Disabled && err != nil {
		return invalidErr("sender.target", err)
	} else if !c.Sender.Disabled && !u.IsAbs() {
		return invalidErr("sender.target", errors.New("target must be an absolute URL"))
	}

	if c.Sender.Workers == 0 {
		c.Sender.Workers = vegeta.DefaultWorkers
	}

	c.Receiver.ParsedTimeout, err = time.ParseDuration(c.Receiver.Timeout)
	if err != nil {
		return invalidErr("receiver.timeout", err)
	}

	return err
}

func invalidErr(field string, err error) error {
	return fmt.Errorf("invalid %s: %w", field, err)
}
