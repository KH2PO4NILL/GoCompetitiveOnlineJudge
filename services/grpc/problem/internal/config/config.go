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

	AuthRPC        zrpc.RpcClientConf
	AuthRPCAuth    sharedauth.ServiceClientAuthConf
	ContestRPC     zrpc.RpcClientConf
	ContestRPCAuth sharedauth.ServiceClientAuthConf

	Mysql         MysqlConf
	RedisDB       RedisConf
	InternalAuth  sharedauth.InternalAuthConf
	Upload        UploadConf
	ObjectStorage ObjectStorageConf
}
