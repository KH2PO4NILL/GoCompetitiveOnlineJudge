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

type TokenConf struct {
	Secret               string
	Issuer               string
	Audience             string
	AccessExpireSeconds  int64
	RefreshExpireSeconds int64
	LeewaySeconds        int64
}

type MailConf struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Security string // ssl 或 starttls
}

type SessionConf struct {
	CleanupIntervalSeconds  int64
	RevokedRetentionSeconds int64
}

type Config struct {
	zrpc.RpcServerConf

	Mysql   MysqlConf
	RedisDB RedisConf
	Token   TokenConf
	Mail    MailConf
	Session SessionConf
}
