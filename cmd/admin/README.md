# Admin HTTP API

管理端 HTTP 网关，默认监听 `0.0.0.0:8001`，业务前缀为 `/api/v1/admin`。
本模块不是管理前端页面；浏览器工程见 [web/README.md](../../web/README.md)。
网关负责协议适配和管理员身份预检，业务授权与事务仍由 Auth、Problem、Contest、Submit 执行。

## 路由与契约

契约源文件：[desc/admin.api](desc/admin.api)。完整方法、路径和 DTO 见
[HTTP 契约索引](../../docs/audit/http-contracts.md)。下表业务路径均相对于 `/api/v1/admin`：

| 分组 | 路径与操作 |
| --- | --- |
| 登录 / 会话 | `POST /auth/login/username`、`/auth/login/email-password`、`/auth/login/email-code`、`/auth/email-codes`、`/auth/token/refresh`、`/auth/logout` |
| 当前账号 | `GET/PATCH /users/me`、`PUT /users/me/email`、`PUT /users/me/password` |
| 用户管理 | `GET /users`、`GET/PATCH/DELETE /users/:user_id` |
| 题目管理 | `GET/POST /problems`、`GET/PATCH/DELETE /problems/:problem_id`，以及 `POST .../publish`、`.../archive`、`.../restore` |
| 测试数据 | `POST /problems/:problem_id/test-data/uploads`、`POST .../uploads/:upload_id/finalize` |
| 比赛管理 | `GET/POST /contests`、`GET/PATCH/DELETE /contests/:contest_id`，以及 `POST .../publish`、`.../cancel`、`.../archive`、`.../restore` |
| 编排 / 报名 | `PUT /contests/:contest_id/problems`、`GET .../registrations`、`POST .../registrations/:registration_id/review`、`GET .../participants` |
| 封榜 | `POST /contests/:contest_id/scoreboard/freeze`、`POST .../scoreboard/unfreeze` |
| 提交 / 重判 | `GET /submissions`、`GET /submissions/:submission_id`、`POST .../:submission_id/rejudge`、`POST /submissions/rejudge` |

另有独立的 `GET /healthz`，响应 `{"status":"ok"}`，只表示 HTTP 存活，不检查四个 RPC 或评测链路。
接口清单不等于所有对应前端页面已经实现。

## 管理员权限边界

- 登录 / 会话路由不统一挂 `AdminIdentity`；它们复用 Auth 的认证流程。
  **从管理端登录 URL 得到 Token，不代表获得管理员权限**，Token 对应角色不会因此升级。
- 当前账号、用户、题目、比赛和提交等管理路由统一使用 `AdminIdentity`：通过 `GetCurrentUser`
  要求正常账号状态，且角色必须是 `admin` 或 `super_admin`。普通用户访问这些路由被拒绝。
- 通过网关身份检查后，下游 RPC 仍检查操作归属、角色层级、revision 和业务状态。
  管理员身份不等于可以修改所有超级管理员账号或绕过比赛 / 题目限制。
- 网关仅透传用户 Authorization，不将外部 `x-service-name` / `x-service-token` 转换成内部权限。
  Worker 判题规格、租约领取与回报没有对应的公开管理 HTTP 路由。

## 写入、上传与响应约定

- `PATCH` 的 optional 字段用于区分“不修改”和“明确设置”，不要把省略字段自动填成零值。
- 题目 / 比赛写入、上传 Finalize、报名审核等操作按 DTO 要求携带对应 revision；
  遇到冲突先重新读取编辑详情或报名记录，再决定是否重试。
- 上传接口返回 `upload_id`、`upload_url`、`expires_at`，客户端向对象存储上传 ZIP 后再 Finalize。
  URL 可达性、签名和 CORS 需单独配置；数据包规范见 [Problem 文档](../../services/grpc/problem/TEST_DATA_FORMAT.md)。
- 重判会创建新的 Attempt，不是直接覆盖历史评测结果；源码、内部诊断与比赛成绩继续遵守 Submit 权限规则。
- 成功返回 `.api` DTO JSON；统一错误处理器返回 `code` / `message`，常见状态包括 400、401、403、404、
  409（冲突）、412（前置条件）、429（限流）、503（下游不可用）、504（超时）。
- 响应默认禁缓存和类型嗅探；使用安全 HTTP 服务构造器，禁止把 Token、源码、邀请码、预签名 URL
  或下游敏感内容写入请求 / 响应转储日志。公共实现见 [httpbridge](../internal/httpbridge/httpbridge.go)。

## 配置与本地启动

主配置：[etc/admin-api.yaml](etc/admin-api.yaml)。通过 etcd 发现四个 RPC，不直接持有业务数据库或对象存储凭据。
默认最大请求体 8 MiB、HTTP 超时 25 秒；业务服务的源码、上传及批量操作限制仍有效。

先按[总 README](../../README.md)准备基础设施、迁移和 RPC，然后在项目根目录执行：

```bash
go run ./cmd/admin -f cmd/admin/etc/admin-api.yaml
curl --fail http://127.0.0.1:8001/healthz
```

健康请求成功不等于管理员账号已准备好。不要提交默认管理员密码或通过削弱鉴权创建演示账号。
同源反向代理必须优先匹配 `/api/v1/admin` 到 Admin，再匹配 `/api/v1` 到 App；管理 API 错误不能回退为前端 HTML。

## 目录、测试与生成

- `admin.go`、`etc/`：启动和配置；`desc/`：HTTP 契约。
- `internal/handler/`、`internal/types/`：路由、解析及 DTO；`internal/logic/`：按领域调用 RPC。
- `internal/mapper/`：协议字段转换；`internal/middleware/`：管理员身份校验。
- `internal/svc/`、`internal/config/`：服务客户端及依赖装配。

```bash
go test -race ./cmd/admin/... ./cmd/internal/httpbridge/...
go build -o .run/bin/admin ./cmd/admin
make api-admin
```

先修改 `.api` 再生成，审查路由、中间件与 DTO 差异；不要用手改生成文件代替契约变更。
普通 HTTP 测试使用 RPC fixture，不能代替真实对象存储上传、完整管理员操作或比赛验收。
隔离认证脚本 `bash scripts/test-web-auth.sh` 的覆盖与依赖见 Web README；完整交付边界见 [审查记录](../../docs/audit/)。

下游文档：[Auth](../../services/grpc/auth/README.md) · [Problem](../../services/grpc/problem/README.md) ·
[Contest](../../services/grpc/contest/README.md) · [Submit](../../services/grpc/submit/README.md)。
