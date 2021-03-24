package sacura

import (
	"context"
	"testing"
	"time"

	ce "github.com/cloudevents/sdk-go/v2"
)

func TestStartReceiverContextCancelled(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	received := make(chan ce.Event)

	go func() {
		defer cancel()

		<-time.After(time.Second)
	}()

	err := StartReceiver(ctx, ReceiverConfig{Port: 9201}, received)

	if err != nil {
		t.Fatal("expected nil, got", err)
	}

	<-ctx.Done()
}
