package svc

import "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/services/grpc/submit/internal/config"

type ServiceContext struct {
	Config config.Config
}

func NewServiceContext(c config.Config) *ServiceContext {
	return &ServiceContext{
		Config: c,
	}
}
