// Code scaffolded by goctl. No recover, Safe to edit.

package config

import (
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	rest.RestConf
	AuthRPC    zrpc.RpcClientConf
	ProblemRPC zrpc.RpcClientConf
	ContestRPC zrpc.RpcClientConf
	SubmitRPC  zrpc.RpcClientConf
}
