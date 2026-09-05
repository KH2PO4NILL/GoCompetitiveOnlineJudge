package processor

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	submitpb "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
)

type authServer struct {
	submitpb.UnimplementedSubmitServiceServer
	metadata chan metadata.MD
}

func (server *authServer) AcquireJudgeTask(ctx context.Context, _ *submitpb.AcquireJudgeTaskReq) (*submitpb.AcquireJudgeTaskResp, error) {
	values, _ := metadata.FromIncomingContext(ctx)
	server.metadata <- values
	return &submitpb.AcquireJudgeTaskResp{}, nil
}
func TestRPCGatewayAttachesOnlyInternalCredentials(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	defer server.Stop()
	handler := &authServer{metadata: make(chan metadata.MD, 1)}
	submitpb.RegisterSubmitServiceServer(server, handler)
	go func() { _ = server.Serve(listener) }()
	connection, err := grpc.NewClient("passthrough:///test", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	gateway := &RPCSubmit{Client: submitpb.NewSubmitServiceClient(connection), Token: "service-secret", Timeout: time.Second}
	if _, err := gateway.Acquire(context.Background(), &submitpb.AcquireJudgeTaskReq{}); err != nil {
		t.Fatal(err)
	}
	values := <-handler.metadata
	if values.Get(interceptor.ServiceNameMetadataKey)[0] != "judge.worker" || values.Get(interceptor.ServiceTokenMetadataKey)[0] != "service-secret" {
		t.Fatal("missing internal auth")
	}
}
