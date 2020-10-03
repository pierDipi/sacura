package sacura

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func TestStateManager(t *testing.T) {

	n := 1000

	received := make(chan string, n)

	sent := make(chan string, n)

	sm := NewStateManager()
	sm.ReadReceived(received)
	sm.ReadSent(sent)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		for i := 0; i < n; i++ {
			sent <- fmt.Sprintf("%d", i)
		}
		wg.Done()
	}()

	go func() {
		for i := 0; i < n; i++ {
			received <- fmt.Sprintf("%d", i)
		}
		wg.Done()
	}()

	wg.Wait()

	_ = wait.PollInfinite(time.Second, func() (done bool, err error) {
		return len(received) == 0 && len(sent) == 0, nil
	})

	if diff := sm.Diff(); diff != "" {
		t.Errorf("want not diff, got %s", diff)
	}
}
