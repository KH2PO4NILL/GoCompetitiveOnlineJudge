// Code scaffolded by goctl. No recover, Safe to edit.

package svc

import (
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/app/internal/middleware"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/authservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/auth/client/userservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/contest/contestservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/problem/problemservice"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/submitservice"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"

	"github.com/go-playground/validator/v10"
)

type ServiceContext struct {
	Config       config.Config
	Validator    *validator.Validate
	Auth         authservice.AuthService
	Users        userservice.UserService
	Problems     problemservice.ProblemService
	Contests     contestservice.ContestService
	Submissions  submitservice.SubmitService
	UserIdentity rest.Middleware
}

func NewServiceContext(c config.Config) *ServiceContext {
	valid := validator.New(validator.WithRequiredStructEnabled())
	httpbridge.RegisterSensitiveRPCMethods()
	authClient := zrpc.MustNewClient(c.AuthRPC)
	authRPC := authservice.NewAuthService(authClient)
	usersRPC := userservice.NewUserService(authClient)

	return &ServiceContext{
		Config:       c,
		Validator:    valid,
		Auth:         authRPC,
		Users:        usersRPC,
		Problems:     problemservice.NewProblemService(zrpc.MustNewClient(c.ProblemRPC)),
		Contests:     contestservice.NewContestService(zrpc.MustNewClient(c.ContestRPC)),
		Submissions:  submitservice.NewSubmitService(zrpc.MustNewClient(c.SubmitRPC)),
		UserIdentity: middleware.NewUserIdentityMiddleware(usersRPC).Handle,
	}
}
