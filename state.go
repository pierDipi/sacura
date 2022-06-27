package sacura

import (
	"encoding/json"
	"sort"
	"sync"

	ce "github.com/cloudevents/sdk-go/v2"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	unknownPartitionKey = "unknown"
)

type StateManager struct {
	lock     sync.RWMutex
	received map[string][]string
	sent     map[string][]string

	config             Config
	stateManagerConfig StateManagerConfig

	terminated bool
	metrics    Metrics
}

type StateManagerConfig struct {
	Ordered bool
	OrderedConfig
}

func stateManagerConfigFromConfig(config Config) StateManagerConfig {
	if config.Ordered != nil {
		return StateManagerConfig{
			OrderedConfig: *config.Ordered,
			Ordered:       true,
		}
	}
	return StateManagerConfig{Ordered: false}
}

func NewStateManager(config Config) *StateManager {
	return &StateManager{
		received:           make(map[string][]string),
		sent:               make(map[string][]string),
		config:             config,
		stateManagerConfig: stateManagerConfigFromConfig(config),
	}
}

func (s *StateManager) ReadSent(sent <-chan ce.Event) <-chan struct{} {
	sg := make(chan struct{})
	go func(set *StateManager) {
		for e := range sent {
			func() {
				s.lock.RLock()
				defer s.lock.RUnlock()
				insert(&e, s.sent, &s.stateManagerConfig)
			}()
		}
		sg <- struct{}{}
	}(s)
	return sg
}

func (s *StateManager) ReadReceived(received <-chan ce.Event) <-chan struct{} {
	sg := make(chan struct{})
	go func(set *StateManager) {
		for e := range received {
			func() {
				s.lock.RLock()
				defer s.lock.RUnlock()
				insert(&e, s.received, &s.stateManagerConfig)
			}()
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
	s.lock.RLock()
	defer s.lock.RUnlock()

	count := 0
	for _, v := range s.received {
		count += len(v)
	}
	return count
}

func (s *StateManager) Diff() string {
	report := s.GenerateReport()
	if len(report.LostEventsByPartitionKey) > 0 {
		b, _ := json.MarshalIndent(report.LostEventsByPartitionKey, "", " ")
		return "lost events by partition key:\n" + string(b)
	}
	return ""
}

func (s *StateManager) GenerateReport() Report {
	s.lock.RLock()
	defer s.lock.RUnlock()

	r := Report{
		LostCount:                     0,
		Metrics:                       s.metrics,
		LostEventsByPartitionKey:      make(map[string][]string, 8),
		DuplicateEventsByPartitionKey: make(map[string][]string, 8),
		ReceivedEventsByPartitionKey:  make(map[string][]string, 8),
		Terminated:                    s.terminated,
	}

	for k, v := range s.sent {
		var sent []string
		copy(sent, v)
		var received []string
		var duplicates []string
		if v, ok := s.received[k]; ok {
			received, duplicates = removeDuplicates(v) // at least once TODO configurable delivery guarantee
		}

		if !s.stateManagerConfig.Ordered {
			sort.Strings(sent)
			sort.Strings(received)
			sort.Strings(duplicates)
		}

		diff := sets.NewString(sent...).Difference(sets.NewString(received...)).List()
		if len(diff) > 0 {
			r.LostEventsByPartitionKey[k] = diff
			r.LostCount += len(r.LostEventsByPartitionKey[k])
		}
		if len(duplicates) > 0 {
			r.DuplicateEventsByPartitionKey[k] = duplicates
			r.DuplicateCount += len(duplicates)
		}
		if len(received) > 0 {
			r.ReceivedEventsByPartitionKey[k] = received
			r.ReceivedCount += len(received)
		}
	}

	return r
}

func (s *StateManager) Terminated(metrics Metrics) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.terminated = true
	s.metrics = metrics
}

func removeDuplicates(a []string) ([]string, []string) {
	t := make(map[string]struct{})
	result := make([]string, 0, len(a))
	duplicates := make([]string, 0, len(a))
	for _, v := range a {
		if _, ok := t[v]; !ok {
			result = append(result, v)
		} else {
			duplicates = append(duplicates, v)
		}
		t[v] = struct{}{}
	}
	return result, duplicates
}
