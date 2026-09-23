# Submit RPC

提交记录、Attempt 租约、结果持久化、重判和评测任务 Outbox 的业务服务。
默认监听 `0.0.0.0:8004`，etcd 注册键为 `submit.rpc`；实际编译运行由独立 Judge Worker 完成。

## 职责与接口

完整协议：[proto/submit.proto](proto/submit.proto)。

| 分组 | 接口 | 调用方 |
| --- | --- | --- |
| 提交与读取 | `CreateSubmission`、`GetSubmission`、`ListSubmissions` | 已登录用户；结果按权限裁剪 |
| 重判 | `RejudgeSubmission`、`BatchRejudgeSubmissions` | 经业务权限校验的管理操作 |
| 评测租约 | `AcquireJudgeTask`、`HeartbeatJudgeTask` | 授权的 `judge.worker` |
| 结果回报 | `ReportJudgeResult` | 授权的 `judge.worker`，同时校验 Attempt 和租约 |

## 提交到结果的流程

1. 校验用户身份、源码及语言，向 Problem 获取固定 revision 的判题规格；比赛提交还调用 Contest 校验资格与时间窗口。
2. 同一事务创建 Submission、Attempt 和 JudgeTask Outbox 事件。
3. 启用的 Outbox publisher 将任务投递到 RocketMQ `judge-task` Topic，Worker 使用 `judge-workers` Consumer Group 消费。
4. Worker 通过 `AcquireJudgeTask` 取得租约并持续心跳，在隔离沙箱执行后回报结果。
5. Submit 在事务内保存逐测试点结果、终态及已判题事件；Worker 在处理成功后确认消息。

数据库是状态事实来源，MQ 至少一次投递不等于任务恰好执行一次：

- 同一用户的 `client_request_id` 与请求指纹用于提交幂等；同 key 不同请求内容属于冲突。
- `current_attempt_id` 隔离重判和租约恢复；旧 Attempt 不能覆盖当前结果。
- Lease Token 仅保存摘要；回报使用 Attempt、Report ID 和内容指纹去重。
- 租约恢复与重判创建新 Attempt，保留历史结果和固定题目版本。
- 已判题 Outbox 记录不等于榜单已经更新。当前 publisher 只发布 JudgeTask 事件，比赛实时计分与榜单消费链路仍待补齐。

事务、不变量与测试数据库规则见 [model/README.md](model/README.md)。

## 依赖与配置

主配置：[etc/submit.yaml](etc/submit.yaml)。

| 依赖 / 规则 | 配置入口 |
| --- | --- |
| MySQL / Redis | `SUBMIT_MYSQL_DSN`、`SUBMIT_REDIS_PASSWORD`、`RedisDB`；迁移位于 [migrations/](migrations/) |
| etcd | `Etcd` 及 `AuthRPC`、`ProblemRPC`、`ContestRPC` 的服务发现配置 |
| 下游内部身份 | `AUTH_INTERNAL_SUBMIT_TOKEN`、`PROBLEM_INTERNAL_SUBMIT_TOKEN`、`CONTEST_INTERNAL_SUBMIT_TOKEN` |
| Worker 调用身份 | `SUBMIT_INTERNAL_JUDGE_TOKEN` |
| RocketMQ | `ROCKETMQ_ENDPOINT`；TLS、发布并发、重试与退出参数见 `Outbox` |
| 启用发布 | YAML `Outbox.Enabled`，或环境覆盖 `SUBMIT_OUTBOX_ENABLED=true` |
| 源码与响应上限 | `Limits`；默认源码上限 1 MiB，最大分页 / 批量重判均为 100 |
| 租约 / 限流 | `Lease`、`RateLimit`；默认租约 30 秒、恢复扫描 5 秒、提交频率每 60 秒 30 次 |

YAML 默认不启用 Outbox；只启动 HTTP 和 RPC 并不能完成评测。启用发布后还必须准备 Broker/Proxy、Topic/Group、
语言镜像及 Worker，见 [RocketMQ 文档](../../../infra/rocketmq/README.md)与 [Worker 文档](../../../judge-worker/README.md)。
Outbox 指标默认监听 `127.0.0.1:9101`，不是公网业务接口或完整依赖就绪证明。

## 权限与敏感数据

用户请求通过 Authorization 传递身份；Worker 内部方法使用 `x-service-name` / `x-service-token` 方法白名单，
仍须校验业务租约，不能仅凭一个合法内部 Token 任意覆盖结果。
源码、编译输出、逐测试点结果及比赛成绩可见性由业务层裁剪；比赛封榜不能仅由前端遮挡分数。
配置中的 RPC 内容日志排除名单覆盖提交、读取、领取、心跳和回报，不能恢复源码或租约凭据转储。

## 本地启动与测试

按[总 README](../../../README.md)准备依赖及迁移，在项目根目录加载可信环境文件：

```bash
set -a
. ./.env
set +a
go run ./services/grpc/submit -f services/grpc/submit/etc/submit.yaml
```

仅准备模型或开启监听并不证明 Auth / Problem / Contest 或队列可用。

```bash
go test -race ./services/grpc/submit/... ./pkg/judgemq/...
go build -o .run/bin/submit ./services/grpc/submit
make rpc-submit
```

未设置 `SUBMIT_MODEL_TEST_DSN` 时跳过 MySQL 模型集成测试；专用测试账号需能创建和删除随机隔离数据库，
不能将业务 DSN 当作测试配置。MQ 测试也有基础设施启用条件。
完整隔离验收使用 `JUDGE_E2E_SLOW=1 bash scripts/test-judge-e2e.sh`，覆盖真实 Worker 评测与依赖故障恢复；
它不代表真实浏览器上传、全部 HTTP 授权或比赛榜单闭环已验收。

## 目录导航

- `submit.go`：启动、注册与方法鉴权；`proto/`、`pb/`、`submitservice/`：协议及生成代码。
- `internal/server/`、`internal/logic/`：RPC 适配、权限、请求校验与结果映射。
- `internal/svc/`：下游 RPC、限流及租约恢复；`internal/outbox/`：有界发布、重试与指标。
- `internal/config/`、`model/`、`migrations/`：配置、原子持久化及表结构；`testsupport/`：测试辅助。

修改契约后运行 `make rpc-submit` 并审查生成差异；不直接手改 `pb/` 代替协议变更。
相关入口：[App HTTP](../../../cmd/app/README.md) · [Admin HTTP](../../../cmd/admin/README.md) · [Contest RPC](../contest/README.md)。
