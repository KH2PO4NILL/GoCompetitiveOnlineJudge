package judgemq

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/pkg/judgeevent"
	protocol "github.com/apache/rocketmq-clients/golang/v5/protocol/v2"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
)

type silentBroker struct {
	protocol.UnimplementedMessagingServiceServer
	port  int32
	ready chan struct{}
}

func (broker silentBroker) QueryRoute(_ context.Context, request *protocol.QueryRouteRequest) (*protocol.QueryRouteResponse, error) {
	return &protocol.QueryRouteResponse{Status: &protocol.Status{Code: protocol.Code_OK}, MessageQueues: []*protocol.MessageQueue{{Topic: request.Topic, Id: 0, Permission: protocol.Permission_READ_WRITE, AcceptMessageTypes: []protocol.MessageType{protocol.MessageType_NORMAL}, Broker: &protocol.Broker{Name: "silent-broker", Endpoints: &protocol.Endpoints{Scheme: protocol.AddressScheme_IPv4, Addresses: []*protocol.Address{{Host: "127.0.0.1", Port: broker.port}}}}}}}, nil
}
func (broker silentBroker) Telemetry(stream protocol.MessagingService_TelemetryServer) error {
	close(broker.ready)
	<-stream.Context().Done()
	return stream.Context().Err()
}

func TestStartupTimeoutIntegration(t *testing.T) {
	if os.Getenv("JUDGE_MQ_INTEGRATION") != "1" {
		t.Skip("requires local TCP sockets")
	}
	baseline := goleak.IgnoreCurrent()
	t.Cleanup(func() { goleak.VerifyNone(t, baseline) })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portText, _ := net.SplitHostPort(listener.Addr().String())
	port, _ := strconv.Atoi(portText)
	server := grpc.NewServer()
	ready := make(chan struct{})
	protocol.RegisterMessagingServiceServer(server, silentBroker{port: int32(port), ready: ready})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	sender, err := NewProducer(Config{Endpoint: listener.Addr().String(), Timeout: 100 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sender.Close() })
	started := time.Now()
	if err := sender.Publish(context.Background(), judgeevent.Topic, "1", []byte("test")); err == nil || !strings.Contains(err.Error(), "settings handshake timed out") {
		t.Fatal("expected bounded settings handshake error", err)
	}
	select {
	case <-ready:
	default:
		t.Fatal("silent telemetry handshake was not exercised")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("startup handshake was not bounded")
	}
}
