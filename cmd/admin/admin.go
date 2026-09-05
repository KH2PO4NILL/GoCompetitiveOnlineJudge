// Code scaffolded by goctl. No recover, Safe to edit.

package main

import (
	"flag"
	"fmt"

	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/config"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/handler"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/admin/internal/svc"
	"github.com/KH2PO4NILL/GoCompetitiveOnlineJudge/cmd/internal/httpbridge"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "cmd/admin/etc/admin-api.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	logx.AddGlobalFields(logx.Field("service_name", c.Log.ServiceName))

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	httpbridge.ConfigureErrorHandler()
	server.Use(httpbridge.ProtectResponse)
	server.Use(httpbridge.ForwardAuthorization)

	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)

	fmt.Printf("Starting server at %s:%d...\n", c.Host, c.Port)
	server.Start()
}
