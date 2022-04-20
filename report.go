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
	Terminated               bool                `json:"terminated"`
	Metrics                  Metrics             `json:"metrics"`
}
