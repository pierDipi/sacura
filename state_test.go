package sacura

import (
	"fmt"
	"testing"
)

func TestStateManager(t *testing.T) {

	n := 1000

	received := make(chan string, n)
	sent := make(chan string, n)

	sm := NewStateManager()
	sm.ReadReceived(received)
	sm.ReadSent(sent)

	go func() {
		for i := 0; i < n; i++ {
			sent <- fmt.Sprintf("%d", i)
		}
	}()

	go func() {
		for i := 0; i < n; i++ {
			received <- fmt.Sprintf("%d", i)
		}
	}()

	if diff := sm.Diff(); diff != "" {
		t.Errorf("want not diff, got %s", diff)
	}
}
