# Auth RPC

认证、会话和用户管理服务。默认监听 `0.0.0.0:8003`，etcd 注册键为 `auth.rpc`。
本模块提供 gRPC，不直接提供浏览器页面或 HTTP 路由。

## 职责与接口

契约入口：[proto/auth.proto](proto/auth.proto)，包含两个服务：

| 服务 | 主要接口 | 职责 |
| --- | --- | --- |
| `AuthService` | `EmailRegister`、`UsernameLogin`、`EmailPasswordLogin`、`EmailCodeLogin` | 注册与登录 |
| `AuthService` | `SendEmailCode`、`ChangePassword`、`ResetPassword` | 验证码与密码流程 |
| `AuthService` | `ValidateToken`、`RefreshToken`、`Logout` | Access Token 校验、Refresh Token 轮换与会话撤销 |
| `UserService` | `GetCurrentUser`、`GetUserById`、`ListUsers` | 当前用户与用户查询 |
| `UserService` | `UpdateProfile`、`ChangeEmail`、`AdminUpdateUser`、`DeleteUser` | 资料、邮箱和管理操作 |

数据库保存用户、学校及会话记录；Redis 用于验证码和限流等短期状态。
用户角色、状态及管理操作权限由服务端判断，不能把客户端传入的用户 ID 或角色当作可信身份。

## 依赖与配置

主配置：[etc/auth.yaml](etc/auth.yaml)。从项目根目录启动时，必须通过 `-f` 指定该路径。

| 依赖 / 配置 | 环境变量或配置项 |
| --- | --- |
| MySQL | `AUTH_MYSQL_DSN`；迁移位于 [migrations/](migrations/) |
| Redis | `RedisDB.Addr`、`RedisDB.DB`、`AUTH_REDIS_PASSWORD` |
| etcd | `Etcd.Hosts`、`Etcd.Key` |
| JWT | `AUTH_TOKEN_SECRET`；`Token.Issuer`、`Audience`、过期时间与时钟宽限 |
| SMTP | `AUTH_MAIL_USERNAME`、`AUTH_MAIL_PASSWORD`、`AUTH_MAIL_FROM`；主机、端口及 TLS 模式在 `Mail` 中配置 |
| 内部调用 | `AUTH_INTERNAL_PROBLEM_TOKEN`、`AUTH_INTERNAL_CONTEST_TOKEN`、`AUTH_INTERNAL_SUBMIT_TOKEN` |

当前 YAML 的 Access / Refresh 有效期分别为 900 / 2592000 秒；会话清理参数见 `Session`。
`SendEmailCode` 的 RPC 超时单独设为 20 秒，以覆盖 SMTP 握手；HTTP 网关配置与此保持一致。
本地配置模板不是生产密钥或真实邮件服务验收证据。

## 鉴权边界

- 用户态请求通过 gRPC metadata 的 `authorization: Bearer <access_token>` 提供身份。
- `ValidateToken` 只允许配置中授权的内部服务调用；服务凭据使用 `x-service-name` / `x-service-token`，
  不能发给浏览器。HTTP 网关通过 `GetCurrentUser` 验证用户身份，不冒充内部服务调用 `ValidateToken`。
- 注册、登录、刷新和重置密码有各自的凭据校验，不统一要求已有登录态。
  `SendEmailCode` 是否要求登录取决于用途；修改邮箱需要身份验证。
- `Logout` 支持由请求体 Access / Refresh Token 完成替代校验，不能把“HTTP 路由未挂身份中间件”解释为任意撤销会话。
- 用户鉴权策略见 [auth_policy.go](auth_policy.go)，内部方法白名单见配置，细粒度权限见 Logic。
  密码、验证码、Token 和修改邮箱请求已纳入 RPC 内容日志排除名单，不应恢复请求体转储。

## 本地启动

先按[项目总说明](../../../README.md)准备依赖、环境文件和迁移。以下命令均在项目根目录执行；
仅加载可信的本地 shell 环境文件，不将其内容提交到 Git：

```bash
set -a
. ./.env
set +a
go run ./services/grpc/auth -f services/grpc/auth/etc/auth.yaml
```

此命令不会自动创建数据库、执行迁移或配置 SMTP。仅监听端口不代表登录和邮件链路已就绪。

## 目录与开发

- `auth.go`、`auth_policy.go`：启动、服务注册与鉴权策略。
- `proto/`、`pb/`、`client/authservice/`、`client/userservice/`：协议及生成的服务端 / 客户端代码。
- `internal/server/`、`internal/logic/`：RPC 适配层与业务实现；Logic 按两个服务分组。
- `internal/token/`、`internal/authenticator/`、`internal/mailer/`：令牌、身份验证及邮件实现。
- `internal/svc/`、`internal/config/`、`model/`、`migrations/`：依赖装配、配置和持久化。

```bash
go test -race ./services/grpc/auth/... ./infra/interceptor/...
go build -o .run/bin/auth ./services/grpc/auth
make rpc-auth
```

`make rpc-auth` 会重新生成代码，要求已安装总 README 中的生成工具；先修改 `.proto`，再审查生成差异，
不要用手改 `pb/` 代替契约变更。普通测试不能代替真实 SMTP 或 HTTP 登录验收；已有验证范围见
[审查记录](../../../docs/audit/2026-09-06-results.md)。

相关入口：[App HTTP](../../../cmd/app/README.md) · [Admin HTTP](../../../cmd/admin/README.md)。
