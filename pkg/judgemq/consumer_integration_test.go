package judgemq

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"github.com/google/uuid"
	"go.uber.org/goleak"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSimpleConsumerIntegration(t *testing.T) {
	if os.Getenv("JUDGE_MQ_INTEGRATION") != "1" {
		t.Skip("set JUDGE_MQ_INTEGRATION=1 with isolated broker")
	}
	baseline := goleak.IgnoreCurrent()
	t.Cleanup(func() { goleak.VerifyNone(t, baseline) })
	endpoint := os.Getenv("ROCKETMQ_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:18081"
	}
	group := os.Getenv("JUDGE_MQ_TEST_GROUP")
	if group == "" {
		group = "judge-acceptance-" + uuid.NewString()
	}
	settings := Config{Endpoint: endpoint, Timeout: 5 * time.Second}
	receiver, err := NewConsumer(settings, group)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.Start(); err != nil {
		t.Fatal("start consumer:", err)
	}
	t.Cleanup(func() { _ = receiver.Close() })
	sender, err := NewProducer(settings)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	event := &pb.JudgeTaskEvent{SchemaVersion: 1, EventId: uuid.NewString(), SubmissionId: 9223372036854775806, AttemptId: 9223372036854775806, EnqueuedAt: timestamppb.Now()}
	body, err := judgeevent.Encode(event)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := sender.Publish(ctx, judgeevent.Topic, "9223372036854775806", body); err != nil {
		t.Fatal(err)
	}
	for ctx.Err() == nil {
		messages, err := receiver.Receive(ctx, 1, 30*time.Second)
		if err != nil {
			t.Logf("receive status: %v", err)
			if ctx.Err() != nil {
				break
			}
			continue
		}
		for _, message := range messages {
			if len(message.Keys) != 1 {
				continue
			}
			decoded, err := judgeevent.Decode(message.Body, message.Keys[0])
			if err != nil || decoded.EventId != event.EventId {
				continue
			}
			if err := receiver.Visibility(ctx, message, 30*time.Second); err != nil {
				t.Fatal("visibility:", err)
			}
			if err := receiver.Ack(ctx, message); err != nil {
				t.Fatal("ack:", err)
			}
			return
		}
	}
	t.Fatal("test event not received before deadline")
}
