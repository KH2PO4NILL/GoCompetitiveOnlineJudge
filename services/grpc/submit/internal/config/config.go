package config

import (
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"

	"github.com/zeromicro/go-zero/zrpc"
)

type MysqlConf struct {
	DSN string
}

type RedisConf struct {
	Addr     string
	Password string
	DB       int
}

type LimitsConf struct {
	MaxSourceCodeBytes    int
	MaxClientRequestBytes int
	MaxPageSize           int
	MaxBatchRejudge       int
	MaxCaseResults        int
	MaxCompileOutputBytes int
	MaxJudgeMessageBytes  int
}

type LeaseConf struct {
	DurationSeconds         int64
	RecoveryIntervalSeconds int64
	RecoveryBatchSize       int
	RecoveryTimeoutSeconds  int64
}

type RateLimitConf struct {
	Enabled        bool
	WindowSeconds  int64
	MaxSubmissions int64
}

type Config struct {
	zrpc.RpcServerConf
	Outbox OutboxConf `json:",optional"`

	AuthRPC        zrpc.RpcClientConf
	AuthRPCAuth    sharedauth.ServiceClientAuthConf
	ProblemRPC     zrpc.RpcClientConf
	ProblemRPCAuth sharedauth.ServiceClientAuthConf
	ContestRPC     zrpc.RpcClientConf
	ContestRPCAuth sharedauth.ServiceClientAuthConf
	InternalAuth   sharedauth.InternalAuthConf

	Mysql     MysqlConf
	RedisDB   RedisConf
	Limits    LimitsConf
	Lease     LeaseConf
	RateLimit RateLimitConf
}
