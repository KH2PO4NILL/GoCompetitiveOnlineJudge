package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/interceptor"
	authserviceServer "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/server/authservice"
	userserviceServer "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/server/userservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/pb"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/service"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var configFile = flag.String("f", "etc/auth.yaml", "the config file")

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
		pb.RegisterAuthServiceServer(grpcServer, authserviceServer.NewAuthServiceServer(svcCtx))
		pb.RegisterUserServiceServer(grpcServer, userserviceServer.NewUserServiceServer(svcCtx))

		if c.Mode == service.DevMode || c.Mode == service.TestMode {
			reflection.Register(grpcServer)
		}
	})
	s.AddUnaryInterceptors(
		interceptor.NewAuthInterceptor(svcCtx),
	)
	defer s.Stop()

	fmt.Printf("Starting rpc server at %s...\n", c.ListenOn)
	s.Start()
}
