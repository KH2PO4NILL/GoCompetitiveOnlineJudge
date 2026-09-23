# App HTTP API

面向用户端的 HTTP 网关，默认监听 `0.0.0.0:8002`，业务前缀为 `/api/v1`。
这里只负责 HTTP 协议、参数与身份预检、RPC 调用和响应映射；浏览器工程在 [web/](../../web/README.md)，
题目、比赛和提交业务规则仍由对应 RPC 服务执行。

## 路由与契约

契约源文件：[desc/app.api](desc/app.api)。完整方法、路径、DTO 和中间件清单见
[HTTP 契约索引](../../docs/audit/http-contracts.md)。以下业务路径均相对于 `/api/v1`：

| 分组 | 路径与操作 | HTTP 身份要求 |
| --- | --- | --- |
| 认证 | `POST /auth/login/username`、`/auth/login/email-password`、`/auth/login/email-code`、`/auth/register` | 按登录或注册凭据校验 |
| 会话 / 邮件 | `POST /auth/email-codes`、`/auth/token/refresh`、`/auth/logout`、`/auth/password/reset` | 无统一身份中间件；Auth 按用途和请求凭据检查 |
| 个人资料 | `GET/PATCH /users/me`、`PUT /users/me/email`、`PUT /users/me/password` | `UserIdentity` |
| 题库 | `GET /problems`、`GET /problems/:problem_id` | 可匿名，RPC 继续检查可见性 |
| 比赛读取 | `GET /contests`、`/contests/:contest_id`、`/contests/:contest_id/problems`、`/contests/:contest_id/scoreboard` | 可匿名，RPC 继续检查比赛权限 |
| 报名 | `POST /contests/:contest_id/registrations`、`POST .../registrations/cancel`、`GET .../registrations/me` | `UserIdentity` |
| 提交 | `POST/GET /submissions`、`GET /submissions/:submission_id` | `UserIdentity` |

另有不带业务前缀的 `GET /healthz`，返回 `{"status":"ok"}`。
它只证明 HTTP Handler 可响应，**不检查 RPC、数据库、RocketMQ 或 Worker 就绪**。

## 鉴权、响应与安全

- 登录后通过 `Authorization: Bearer <access_token>` 调用受保护路由。
  `UserIdentity` 调用 Auth `GetCurrentUser`，要求有效用户及正常账号状态。
- 网关只将用户 Authorization 转发到 RPC；不透传或伪造 `x-service-name` / `x-service-token`。
  内部 `ValidateToken`、领取租约和结果回报均不是用户 HTTP 接口。
- 未挂身份中间件不等于无权限校验。公开题目 / 比赛读取、验证码用途、退出凭据和成绩可见性仍由 RPC 判断。
- 成功响应直接使用 `.api` 中的 DTO，不额外包装统一 `data` 层；经统一错误处理器返回的错误形如
  `{"code":"unauthenticated","message":"..."}`。常见状态为 400 参数错误、401 未认证、403 禁止、
  404 不存在、409 冲突、412 前置条件失败、429 限流、503 下游不可用、504 超时。
- 枚举、可选字段、分页和时间格式以 `.api` 与 Mapper 为准，不能直接向 HTTP 发送 protobuf 枚举数字。
- 响应默认 `Cache-Control: no-store` 和 `X-Content-Type-Options: nosniff`。
  安全服务构造器禁止框架请求 / 响应内容转储，避免 Token、源码、编译输出或预签名 URL 进入日志。

公共实现见 [httpbridge.go](../internal/httpbridge/httpbridge.go)与 [server.go](../internal/httpbridge/server.go)。

## 依赖与本地启动

配置入口：[etc/app-api.yaml](etc/app-api.yaml)。网关通过 etcd 发现 Auth、Problem、Contest、Submit，
不直接连接 MySQL、Redis、MinIO 或 RocketMQ。当前 YAML 最大请求体为 8 MiB、HTTP 超时 25 秒；
Submit 的源码上限等更严格业务限制仍然生效。

先按[项目总 README](../../README.md)启动基础设施、迁移和四个 RPC，再从项目根目录运行：

```bash
go run ./cmd/app -f cmd/app/etc/app-api.yaml
curl --fail http://127.0.0.1:8002/healthz
```

当前网关 YAML 不需要数据库密钥；下游 RPC 的环境变量由各服务加载。
`make up` 可管理六个 HTTP/RPC 进程，但不包含浏览器前端和 Worker。
前端开发代理见 Web README；生产反向代理应区分 App 与 Admin 前缀，不能把 API 错误回退成 SPA HTML。

## 目录、测试与生成

- `app.go`、`etc/`：启动及配置。
- `desc/`：HTTP 契约；`internal/handler/`、`internal/types/`：路由适配及 DTO。
- `internal/logic/`：按业务分组调用 RPC；`internal/mapper/`：HTTP / protobuf 字段转换。
- `internal/middleware/`：用户身份预检；`internal/svc/`、`internal/config/`：客户端与配置装配。

```bash
go test -race ./cmd/app/... ./cmd/internal/httpbridge/...
go build -o .run/bin/app ./cmd/app
make api-app
```

修改接口先修改 `.api`，再运行生成命令并审查路由 / DTO 差异，不只修改生成文件。
HTTP Logic 和 Mapper 测试含 RPC fixture；通过不等于浏览器完整评测已验收。
真实隔离认证验收入口为 `bash scripts/test-web-auth.sh`，依赖与当前覆盖范围见 Web README，且不涵盖所有业务流程。

下游文档：[Auth](../../services/grpc/auth/README.md) · [Problem](../../services/grpc/problem/README.md) ·
[Contest](../../services/grpc/contest/README.md) · [Submit](../../services/grpc/submit/README.md)。
