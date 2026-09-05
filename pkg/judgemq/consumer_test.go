package judgemq

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	rmq "github.com/apache/rocketmq-clients/golang/v5"
	v2 "github.com/apache/rocketmq-clients/golang/v5/protocol/v2"
)

type receiveClient struct {
	rmq.SimpleConsumer
	views []*rmq.MessageView
	err   error
}

func (client *receiveClient) Receive(context.Context, int32, time.Duration) ([]*rmq.MessageView, error) {
	return client.views, client.err
}

func TestConsumerNormalizesOnlyEmptyQueueStatus(t *testing.T) {
	for _, test := range []struct {
		name  string
		err   error
		empty bool
	}{
		{name: "empty success", empty: true},
		{name: "message not found", err: &rmq.ErrRpcStatus{Code: int32(v2.Code_MESSAGE_NOT_FOUND)}, empty: true},
		{name: "wrapped empty", err: fmt.Errorf("receive: %w", &rmq.ErrRpcStatus{Code: int32(v2.Code_MESSAGE_NOT_FOUND)}), empty: true},
		{name: "topic unavailable", err: &rmq.ErrRpcStatus{Code: int32(v2.Code_TOPIC_NOT_FOUND)}},
		{name: "permission denied", err: &rmq.ErrRpcStatus{Code: int32(v2.Code_FORBIDDEN)}},
		{name: "cancelled", err: context.Canceled},
		{name: "transport", err: errors.New("transport unavailable")},
	} {
		t.Run(test.name, func(t *testing.T) {
			consumer := &Consumer{client: &receiveClient{err: test.err}}
			messages, err := consumer.Receive(context.Background(), 1, time.Minute)
			if len(messages) != 0 || test.empty && err != nil || !test.empty && !errors.Is(err, test.err) {
				t.Fatalf("unexpected receive result: count=%d error=%v", len(messages), err)
			}
		})
	}
}

func TestConsumerRejectsInvalidViewsWithoutPanic(t *testing.T) {
	for _, views := range [][]*rmq.MessageView{{nil}, {nil, nil}} {
		consumer := &Consumer{client: &receiveClient{views: views}}
		if messages, err := consumer.Receive(context.Background(), 1, time.Minute); err == nil || messages != nil {
			t.Fatal("invalid or unbounded SDK response was accepted")
		}
	}
}
