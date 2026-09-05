package judgemq

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	rmq "github.com/apache/rocketmq-clients/golang/v5"
	"github.com/apache/rocketmq-clients/golang/v5/credentials"
)

type Sender interface {
	Publish(context.Context, string, string, []byte) error
}
type Config struct {
	Endpoint             string
	TLS                  bool
	AccessKey, SecretKey string
	Timeout              time.Duration
}

var tlsOnce sync.Once
var tlsEnabled bool

func configure(config Config) error {
	if config.Endpoint == "" || config.Timeout <= 0 || config.Timeout > time.Minute {
		return errors.New("invalid RocketMQ endpoint or timeout")
	}
	tlsOnce.Do(func() {
		tlsEnabled = config.TLS
		rmq.EnableSsl = config.TLS
		_ = os.Setenv(rmq.ENABLE_CONSOLE_APPENDER, "true")
		_ = os.Setenv(rmq.CLIENT_LOG_LEVEL, "error")
		rmq.ResetLogger()
	})
	if tlsEnabled != config.TLS {
		return errors.New("RocketMQ TLS mode must be process-wide consistent")
	}
	return nil
}
func sdkConfig(config Config) *rmq.Config {
	return &rmq.Config{Endpoint: config.Endpoint, Credentials: &credentials.SessionCredentials{AccessKey: config.AccessKey, AccessSecret: config.SecretKey}}
}

type Producer struct {
	mu        sync.Mutex
	config    Config
	producers map[string]rmq.Producer
	closed    bool
}

func NewProducer(config Config) (*Producer, error) {
	if err := configure(config); err != nil {
		return nil, err
	}
	return &Producer{config: config, producers: make(map[string]rmq.Producer)}, nil
}
func (producer *Producer) client(topic string) (rmq.Producer, error) {
	producer.mu.Lock()
	defer producer.mu.Unlock()
	if producer.closed {
		return nil, errors.New("RocketMQ producer closed")
	}
	if topic != judgeevent.Topic && topic != judgeevent.DeadTopic {
		return nil, errors.New("unsupported judge topic")
	}
	if producer.producers[topic] == nil {
		client, err := rmq.NewProducer(sdkConfig(producer.config), rmq.WithTopics(topic))
		if err != nil {
			return nil, err
		}
		client.SetRequestTimeout(producer.config.Timeout)
		if err := client.Start(); err != nil {
			_ = client.GracefulStop()
			return nil, err
		}
		producer.producers[topic] = client
	}
	return producer.producers[topic], nil
}
func (producer *Producer) Publish(ctx context.Context, topic, key string, content []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	client, err := producer.client(topic)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, producer.config.Timeout)
	defer cancel()
	message := &rmq.Message{Topic: topic, Body: content}
	message.SetKeys(key)
	receipts, err := client.Send(ctx, message)
	if err != nil {
		return err
	}
	if len(receipts) != 1 {
		return errors.New("broker did not acknowledge exactly one message")
	}
	return nil
}
func (producer *Producer) Close() error {
	producer.mu.Lock()
	defer producer.mu.Unlock()
	producer.closed = true
	var failures []error
	for _, client := range producer.producers {
		failures = append(failures, client.GracefulStop())
	}
	return errors.Join(failures...)
}
