package outbox

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgemq"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/model"
)

func TestRocketMQPublisherIntegration(t *testing.T) {
	if os.Getenv("JUDGE_MQ_INTEGRATION") != "1" {
		t.Skip("set JUDGE_MQ_INTEGRATION=1 with an isolated RocketMQ broker")
	}
	endpoint := os.Getenv("ROCKETMQ_ENDPOINT")
	if endpoint == "" {
		endpoint = "127.0.0.1:18081"
	}
	sender, err := judgemq.NewProducer(judgemq.Config{Endpoint: endpoint, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	publisher, repository, _ := fixture(t)
	publisher.sender = sender
	publisher.options.Timeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := publisher.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if repository.rows[0].Status != model.OutboxStatusPublished {
		t.Fatal("broker did not acknowledge publication")
	}
}
