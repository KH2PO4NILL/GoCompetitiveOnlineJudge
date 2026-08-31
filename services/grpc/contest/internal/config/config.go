package config

import (
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"

	"github.com/zeromicro/go-zero/zrpc"
)

type MysqlConf struct {
	DSN string
}

type ScoreboardFreezeConf struct {
	IntervalSeconds         int64
	BatchSize               int
	OperationTimeoutSeconds int64
}

type Config struct {
	zrpc.RpcServerConf

	AuthRPC          zrpc.RpcClientConf
	AuthRPCAuth      sharedauth.ServiceClientAuthConf
	ProblemRPC       zrpc.RpcClientConf
	ProblemRPCAuth   sharedauth.ServiceClientAuthConf
	InternalAuth     sharedauth.InternalAuthConf
	Mysql            MysqlConf
	ScoreboardFreeze ScoreboardFreezeConf
}
