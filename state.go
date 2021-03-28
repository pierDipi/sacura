package sacura

import (
	"fmt"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
)

const (
	unknownPartitionKey = "unknown"
)

type StateManager struct {
	received map[string][]string
	sent     map[string][]string
	config   StateManagerConfig
}

type StateManagerConfig struct {
	Ordered bool
	OrderedConfig
}

func StateManagerConfigFromConfig(config Config) StateManagerConfig {
	if config.Ordered != nil {
		return StateManagerConfig{
			OrderedConfig: *config.Ordered,
			Ordered:       true,
		}
	}
	return StateManagerConfig{Ordered: false}
}

func NewStateManager(config StateManagerConfig) *StateManager {
	return &StateManager{
		received: make(map[string][]string),
		sent:     make(map[string][]string),
		config:   config,
	}
}

func (s *StateManager) ReadSent(sent <-chan ce.Event) <-chan struct{} {
	sg := make(chan struct{})
	go func(set *StateManager) {
		for e := range sent {
			insert(&e, s.sent, &s.config)
		}
		sg <- struct{}{}
	}(s)
	return sg
}

func (s *StateManager) ReadReceived(received <-chan ce.Event) <-chan struct{} {
	sg := make(chan struct{})
	go func(set *StateManager) {
		for e := range received {
			insert(&e, s.received, &s.config)
		}
		sg <- struct{}{}
	}(s)
	return sg
}

func insert(e *ce.Event, store map[string][]string, config *StateManagerConfig) {
	pk := unknownPartitionKey
	if config.Ordered {
		extenstions := e.Extensions()
		if v, ok := extenstions["partitionkey"]; ok {
			pk = v.(string)
		}
	}
	if _, ok := store[pk]; !ok {
		store[pk] = make([]string, 0, 100)
	}
	store[pk] = append(store[pk], e.ID())
}

func (s *StateManager) ReceivedCount() int {
	count := 0
	for _, v := range s.received {
		count += len(v)
	}
	return count
}

func (s *StateManager) Diff() string {

	hasDiff := false
	fullDiff := "Diff by partition key\n"

	for k, v := range s.sent {
		sent := v
		var received []string
		if v, ok := s.received[k]; ok {
			received = removeDuplicates(v) // at least once TODO configurable delivery guarantee
		}

		diff := cmp.Diff(received, sent)
		if diff != "" {
			hasDiff = true
		}
		fullDiff += fmt.Sprintf("partitionkey: '%s' (-want, +got)\n%s", k, diff)
	}

	if !hasDiff {
		return ""
	}
	return fullDiff
}

func removeDuplicates(a []string) []string {
	t := make(map[string]struct{})
	result := make([]string, 0, len(a))
	for _, v := range a {
		if _, ok := t[v]; !ok {
			result = append(result, v)
		}
		t[v] = struct{}{}
	}
	return result
}
