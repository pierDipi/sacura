package sacura

import (
	"sync"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	vegeta "github.com/tsenart/vegeta/v12/lib"
	"k8s.io/apimachinery/pkg/util/sets"
)

func StartSender(config Config, sentOut chan<- ce.Event) Metrics {

	rate := vegeta.Rate{
		Freq: config.Sender.FrequencyPerSecond,
		Per:  time.Second,
	}

	proposedCount := 0
	proposed := make(chan ce.Event, cap(sentOut))
	accepted := make(chan string, cap(sentOut))
	var m sync.Mutex
	var wg sync.WaitGroup

	go func() {
		proposedArr := make(map[string]ce.Event, 100)
		acceptedArr := sets.NewString()
		wg.Add(1)
		go func() {
			defer wg.Done()

			for e := range proposed {
				func() {
					m.Lock()
					defer m.Unlock()

					proposedCount++
					proposedArr[e.ID()] = e
					if acceptedArr.Has(e.ID()) {
						sentOut <- e
						delete(proposedArr, e.ID())
						acceptedArr.Delete(e.ID())
					}

				}()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			for id := range accepted {
				func() {
					m.Lock()
					defer m.Unlock()

					acceptedArr.Insert(id)
					if v, ok := proposedArr[id]; ok {
						sentOut <- v
						delete(proposedArr, id)
						acceptedArr.Delete(id)
					}
				}()
			}
		}()
	}()

	targeter := NewTargeterGenerator(config, uuid.New, proposed)

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
			id := res.RequestHeaders.Get(CloudEventIdHeader)
			if id != "" {
				accepted <- id
			}
		}
	}
	metrics.Close()
	close(proposed)
	close(accepted)
	wg.Wait()

	return Metrics{
		ProposedCount: proposedCount,
		AcceptedCount: acceptedCount,
		Metrics:       metrics,
	}
}
