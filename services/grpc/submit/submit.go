package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/server"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/submit.yaml", "the config file")

func submitMethodAuthentication(fullMethod string) interceptor.AuthenticationMode {
	switch fullMethod {
	case pb.SubmitService_AcquireJudgeTask_FullMethodName,
		pb.SubmitService_HeartbeatJudgeTask_FullMethodName,
		pb.SubmitService_ReportJudgeResult_FullMethodName:
		return interceptor.AuthenticationSkipped
	default:
		return interceptor.AuthenticationRequired
	}
}

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	ctx, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("initialize service context: %v", err)
	}
	defer func() {
		if err := ctx.Close(); err != nil {
			log.Printf("close service context: %v", err)
		}
	}()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterSubmitServiceServer(grpcServer, server.NewSubmitServiceServer(ctx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	authenticator := interceptor.NewRPCAuthenticator(ctx.AuthClient, c.AuthRPCAuth)
	servicePolicy := func(fullMethod string) []string { return ctx.InternalCallers[fullMethod] }
	authInterceptors := interceptor.NewServerAuthInterceptors(
		authenticator,
		ctx.InternalCredentials,
		servicePolicy,
		interceptor.WithMethodAuthentication(submitMethodAuthentication),
	)
	s.AddUnaryInterceptors(authInterceptors.Unary...)
	s.AddStreamInterceptors(authInterceptors.Stream...)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
