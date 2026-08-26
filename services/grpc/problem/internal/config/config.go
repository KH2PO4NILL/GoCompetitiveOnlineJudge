package config

import "github.com/zeromicro/go-zero/zrpc"

type MysqlConf struct {
	DSN string
}

type RedisConf struct {
	Addr     string
	Password string
	DB       int
}

type Config struct {
	zrpc.RpcServerConf

	Auth zrpc.RpcClientConf

	Mysql   MysqlConf
	RedisDB RedisConf
}
