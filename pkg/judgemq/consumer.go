package judgemq

import (
	"context"
	"errors"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	rmq "github.com/apache/rocketmq-clients/golang/v5"
	v2 "github.com/apache/rocketmq-clients/golang/v5/protocol/v2"
)

type Delivery struct {
	ID      string
	Keys    []string
	Body    []byte
	Attempt int32
	Born    *time.Time
	receipt *rmq.MessageView
}

type Receiver interface {
	Receive(context.Context, int32, time.Duration) ([]*Delivery, error)
	Ack(context.Context, *Delivery) error
	Visibility(context.Context, *Delivery, time.Duration) error
}

type Consumer struct{ client rmq.SimpleConsumer }

func NewConsumer(config Config, group string) (*Consumer, error) {
	if err := configure(config); err != nil {
		return nil, err
	}
	if group == "" {
		group = judgeevent.Group
	}
	settings := sdkConfig(config)
	settings.ConsumerGroup = group
	client, err := rmq.NewSimpleConsumer(settings,
		rmq.WithSimpleSubscriptionExpressions(map[string]*rmq.FilterExpression{judgeevent.Topic: rmq.SUB_ALL}),
		rmq.WithSimpleAwaitDuration(5*time.Second), rmq.WithSimpleReceiveRateLimit(1))
	if err != nil {
		return nil, err
	}
	client.SetRequestTimeout(config.Timeout)
	return &Consumer{client: client}, nil
}

func (consumer *Consumer) Start() error { return consumer.client.Start() }
func (consumer *Consumer) Close() error { return consumer.client.GracefulStop() }
func (consumer *Consumer) Receive(ctx context.Context, maximum int32, invisible time.Duration) ([]*Delivery, error) {
	views, err := consumer.client.Receive(ctx, maximum, invisible)
	if err != nil {
		var rpcError *rmq.ErrRpcStatus
		if errors.As(err, &rpcError) && rpcError.GetCode() == int32(v2.Code_MESSAGE_NOT_FOUND) {
			return nil, nil
		}
		return nil, err
	}
	if len(views) > int(maximum) {
		return nil, errors.New("broker exceeded receive bound")
	}
	deliveries := make([]*Delivery, 0, len(views))
	for _, view := range views {
		if view == nil {
			return nil, errors.New("broker returned an invalid message view")
		}
		deliveries = append(deliveries, &Delivery{ID: view.GetMessageId(), Keys: view.GetKeys(), Body: view.GetBody(), Attempt: view.GetDeliveryAttempt(), Born: view.GetBornTimestamp(), receipt: view})
	}
	return deliveries, nil
}
func (consumer *Consumer) Ack(ctx context.Context, delivery *Delivery) error {
	return consumer.client.Ack(ctx, delivery.receipt)
}
func (consumer *Consumer) Visibility(ctx context.Context, delivery *Delivery, duration time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return consumer.client.ChangeInvisibleDuration(delivery.receipt, duration)
}
