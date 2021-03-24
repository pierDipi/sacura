package sacura

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	cetest "github.com/cloudevents/sdk-go/v2/test"
	ce "github.com/cloudevents/sdk-go/v2"
)

func TestStateManager(t *testing.T) {

	n := 1000

	received := make(chan ce.Event, n)

	sent := make(chan ce.Event, n)

	sm := NewStateManager()
	receivedSignal := sm.ReadReceived(received)
	sentSignal := sm.ReadSent(sent)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for i := 0; i < n; i++ {
			e := cetest.FullEvent()
			e.SetID(fmt.Sprintf("%d", i))
			sent <- e
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < n; i++ {
			e := cetest.FullEvent()
			e.SetID(fmt.Sprintf("%d", i))
			received <- e
		}
		wg.Done()
	}()
	wg.Wait()
	close(sent)
	close(received)
	<-receivedSignal
	<-sentSignal

	wg.Wait()

	_ = wait.PollInfinite(time.Second, func() (done bool, err error) {
		return len(received) == 0 && len(sent) == 0, nil
	})

	if diff := sm.Diff(); diff != "" {
		t.Errorf("want not diff, got %s", diff)
	}
}
