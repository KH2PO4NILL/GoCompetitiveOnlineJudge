package config

import (
	sharedauth "github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/infra/interceptor"

	"github.com/zeromicro/go-zero/zrpc"
)

type MysqlConf struct {
	DSN string
}

type Config struct {
	zrpc.RpcServerConf

	AuthRPC      zrpc.RpcClientConf
	AuthRPCAuth  sharedauth.ServiceClientAuthConf
	InternalAuth sharedauth.InternalAuthConf
	Mysql        MysqlConf
}
