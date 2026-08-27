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

type InternalCallerConf struct {
	Name    string
	Token   string
	Methods []string
}

type InternalAuthConf struct {
	Callers []InternalCallerConf
}

type UploadConf struct {
	URLTTLSeconds        int64
	MaxUploadBytes       int64
	AllowedExtensions    []string
	MaxUncompressedBytes int64
	MaxFileCount         int
	MaxManifestBytes     int64
}

type ObjectStorageConf struct {
	Endpoint              string
	Region                string
	Bucket                string
	AccessKey             string
	SecretKey             string
	UsePathStyle          bool
	RequestTimeoutSeconds int64
}

type Config struct {
	zrpc.RpcServerConf

	AuthRPC    zrpc.RpcClientConf
	ContestRPC zrpc.RpcClientConf

	Mysql         MysqlConf
	RedisDB       RedisConf
	InternalAuth  InternalAuthConf
	Upload        UploadConf
	ObjectStorage ObjectStorageConf
}
