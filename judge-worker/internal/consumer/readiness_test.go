package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/judge-worker/internal/processor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
)

type receiveOutcome struct {
	messages []*judgemq.Delivery
	err      error
}

type controlledReceiver struct {
	judgemq.Receiver
	outcomes chan receiveOutcome
}

func (receiver *controlledReceiver) Receive(ctx context.Context, _ int32, _ time.Duration) ([]*judgemq.Delivery, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-receiver.outcomes:
		return result.messages, result.err
	}
}

func TestReadinessTracksIdleSuccessFailureAndStop(t *testing.T) {
	driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result {
		return processor.Result{Action: processor.ACK}
	})
	receiver := &controlledReceiver{Receiver: broker, outcomes: make(chan receiveOutcome)}
	driver.receiver = receiver
	if driver.Ready() {
		t.Fatal("unstarted consumer is ready")
	}
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	if driver.Ready() {
		t.Fatal("consumer became ready before a successful receive")
	}
	receiver.outcomes <- receiveOutcome{}
	awaitReadiness(t, driver, true)
	if driver.Metrics.Get("judge_consumer_receive_failures_total") != 0 {
		t.Fatal("empty poll counted as a failure")
	}
	receiver.outcomes <- receiveOutcome{err: errors.New("broker unavailable")}
	awaitReadiness(t, driver, false)
	receiver.outcomes <- receiveOutcome{}
	awaitReadiness(t, driver, true)
	if err := driver.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if driver.Ready() {
		t.Fatal("stopped consumer is ready")
	}
}

func TestReadinessRemainsHealthyDuringBackpressureAndStopsBeforeDrain(t *testing.T) {
	entered, release := make(chan struct{}, 2), make(chan struct{})
	defer close(release)
	driver, broker := fixture(t, func(context.Context, *pb.JudgeTaskEvent) processor.Result {
		entered <- struct{}{}
		<-release
		return processor.Result{Action: processor.ACK}
	})
	if err := driver.Start(); err != nil {
		t.Fatal(err)
	}
	broker.messages <- delivery(t)
	broker.messages <- delivery(t)
	for count := 0; count < 2; count++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("tasks did not fill the bounded pool")
		}
	}
	awaitReadiness(t, driver, true)
	finished := make(chan error, 1)
	go func() { finished <- driver.Stop(context.Background()) }()
	awaitReadiness(t, driver, false)
	select {
	case <-finished:
		t.Fatal("stop returned before active tasks drained")
	default:
	}
	release <- struct{}{}
	release <- struct{}{}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not finish")
	}
}

func awaitReadiness(t *testing.T, driver *Driver, want bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if driver.Ready() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("readiness=%t, want %t", driver.Ready(), want)
}
