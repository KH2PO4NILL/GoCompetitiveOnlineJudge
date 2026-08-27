package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/server"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/problem.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c, conf.UseEnv())
	svcCtx, err := svc.NewServiceContext(c)
	if err != nil {
		log.Fatalf("initialize service context: %v", err)
	}
	defer func() {
		if err := svcCtx.Close(); err != nil {
			log.Printf("close service context: %v", err)
		}
	}()

	s := zrpc.MustNewServer(c.RpcServerConf, func(grpcServer *grpc.Server) {
		pb.RegisterProblemServiceServer(grpcServer, server.NewProblemServiceServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})

	s.AddUnaryInterceptors(
		interceptor.NewServiceAuthInterceptor(
			svcCtx.InternalCredentials,
			func(fullMethod string) []string { return svcCtx.InternalCallers[fullMethod] },
		),
		interceptor.NewAuthInterceptor(
			svcCtx.AuthClient,
			interceptor.WithMethodAuthentication(
				func(fullMethod string) interceptor.AuthenticationMode {
					switch fullMethod {
					case pb.ProblemService_BatchGetProblems_FullMethodName,
						pb.ProblemService_GetJudgeProblemSpec_FullMethodName:
						return interceptor.AuthenticationSkipped
					case pb.ProblemService_GetProblem_FullMethodName,
						pb.ProblemService_ListProblems_FullMethodName:
						return interceptor.AuthenticationOptional
					default:
						return interceptor.AuthenticationRequired
					}
				},
			),
		),
	)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
