package sacura

import (
	"time"

	vegeta "github.com/tsenart/vegeta/v12/lib"
	ce "github.com/cloudevents/sdk-go/v2"
)

func StartSender(config Config, sentOut chan<- ce.Event) (vegeta.Metrics, int) {

	rate := vegeta.Rate{
		Freq: config.Sender.FrequencyPerSecond,
		Per:  time.Second,
	}

	targeter := NewTargeterGenerator(config.Sender.Target, sentOut)

	attacker := vegeta.NewAttacker(
		vegeta.Workers(config.Sender.Workers),
		vegeta.KeepAlive(config.Sender.KeepAlive),
		vegeta.MaxWorkers(config.Sender.Workers),
	)

	var metrics vegeta.Metrics
	var acceptedCount int
	for res := range attacker.Attack(targeter, rate, config.ParsedDuration, "Sacura") {
		metrics.Add(res)
		if res.Error == "" && res.Code >= 200 && res.Code < 300 {
			acceptedCount++
		}
	}
	metrics.Close()

	return metrics, acceptedCount
}
