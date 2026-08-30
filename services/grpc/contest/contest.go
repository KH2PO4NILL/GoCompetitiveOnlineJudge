package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/server"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/contest.yaml", "the config file")

func contestMethodAuthentication(fullMethod string) interceptor.AuthenticationMode {
	switch fullMethod {
	case pb.ContestService_CanAccessProblem_FullMethodName,
		pb.ContestService_ValidateContestSubmission_FullMethodName:
		return interceptor.AuthenticationSkipped
	case pb.ContestService_ListContests_FullMethodName,
		pb.ContestService_GetContest_FullMethodName,
		pb.ContestService_ListContestProblems_FullMethodName,
		pb.ContestService_GetScoreboard_FullMethodName:
		return interceptor.AuthenticationOptional
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
		pb.RegisterContestServiceServer(grpcServer, server.NewContestServiceServer(ctx))

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
		interceptor.WithMethodAuthentication(contestMethodAuthentication),
	)
	s.AddUnaryInterceptors(authInterceptors.Unary...)
	s.AddStreamInterceptors(authInterceptors.Stream...)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
