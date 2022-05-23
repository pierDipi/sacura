package sacura

import (
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

type Metrics struct {
	ProposedCount int            `json:"proposedCount"`
	AcceptedCount int            `json:"acceptedCount"`
	Metrics       vegeta.Metrics `json:"metrics"`
}

type Report struct {
	LostCount                int                 `json:"lostCount"`
	LostEventsByPartitionKey map[string][]string `json:"lostEvents"`
	DuplicateCount           int                 `json:"duplicateCount"`
	// DuplicateEventsByPartitionKey collects duplicate events by
	DuplicateEventsByPartitionKey map[string][]string `json:"duplicateEvents"`
	// ReceivedCount is the number of events received, including duplicates
	ReceivedCount int `json:"receivedCount"`
	// ReceivedEventsByPartitionKey collects all events by partition, including duplicates
	ReceivedEventsByPartitionKey map[string][]string `json:"-"`
	Terminated                   bool                `json:"terminated"`
	Metrics                      Metrics             `json:"metrics"`
}
