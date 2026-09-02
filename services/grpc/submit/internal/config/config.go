package config

import "github.com/zeromicro/go-zero/zrpc"

type MysqlConf struct {
	DSN string
}

type Config struct {
	zrpc.RpcServerConf

	Mysql MysqlConf
}
