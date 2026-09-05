# Judge Worker：运行与验收

已支持 C++20、Go，固定 profile `cpp20-v3`、`go-v3`。标准判题支持
Exact Bytes、Ignore Trailing Whitespace、Token Compare；不支持 Python/Java、SPJ 或交互题。

## 本地启动

1. 按根 README 启动 MySQL/Redis/etcd/MinIO、执行迁移并启动 Problem/Submit RPC。
2. `bash scripts/build-language-images.sh` 构建四个版本化语言镜像；运行时不自动拉镜像。
3. 按 `infra/rocketmq/README.md` 启动 Broker/Proxy，初始化 Topic/Group。
4. 加载环境变量；设置 `SUBMIT_OUTBOX_ENABLED=true`、`JUDGE_CONSUMER_ENABLED=true`、
   `ROCKETMQ_ENDPOINT=127.0.0.1:18081`，启动 Submit 和 Worker。

Worker 必填环境变量见 `.env.example`：唯一 `JUDGE_WORKER_ID`、Submit/Problem RPC 地址、
各自的内部 Token、只读测试数据 Bucket 凭据、缓存/工作目录与 Docker Endpoint。
对象存储身份在生产只授予 GetObject；不要给 Worker MinIO 管理员权限。
启用 Worker YAML 的 `Metrics.Enabled` 后监听 `127.0.0.1:9100/metrics`；
Submit Outbox 指标默认 `127.0.0.1:9101/metrics`。均不应直接暴露到公网。

### Worker 存活与消费就绪

启用 `Metrics.Enabled` 后，同一个内部监听地址额外提供 `GET /healthz` 与 `GET /readyz`（也支持 HEAD）。
`/healthz` 返回 200 只表示监控 HTTP 服务存活；`/readyz` 仅在 Worker 启动完成、Consumer 启用，
且最近一次 Receive 成功后返回 200，否则为 503。收到合法的“空队列”结果同样视为消费通道正常，
不会增加 receive failure 计数；网络错误、鉴权失败或 Topic 不存在仍会撤销就绪状态。
工作槽位全满属于正常背压，不因暂时没有发起新 Receive 而撤销就绪。停机开始即撤销就绪，
不等待长任务排空；存活探针可在排空期间继续返回 200，最终监听器随进程资源一起关闭。

探针不返回凭据、具体依赖地址或错误详情，响应禁止缓存；Metrics.Path 不能占用 `/healthz`、
`/readyz`，也不能使用 ServeMux 通配符或非法请求路径。
**消费就绪不是端到端可评测保证**：该探针不主动执行 Submit / Problem 业务 RPC，也不下载私有测试数据。
RPC 权限、对象存储和镜像实际执行仍由真实评测验收验证，不能用这个 200 替代全栈验收。

MQ 只通知，Submission/Attempt/Outbox 数据库状态才是事实来源。
重复发布、ACK 丢失、Report 响应丢失不会重复应用最终结果。
已 Acquire 任务失败时不复用过期租约；Submit Lease recovery 创建新的 Attempt 和 Outbox。
SIGTERM 停止拉取/Claim，活跃任务继续心跳，宽限期结束才取消并清理容器。

## 升级与工作目录

Submit 与内置 Profile 的源码上限统一为 **1MiB（1,048,576 字节）**；部署时同步更新
Submit 的 `Limits.MaxSourceCodeBytes`（允许更小，不允许超过全局上限）。已入库的旧超大源码
仍可通过有界的 Acquire RPC 拉取，但直接上报 CE，不再通过新建 Attempt 永久重试。

每个 Worker 在工作目录内使用 `worker-<WorkerID 的 SHA-256>` 子目录及独占文件锁。
同一目录/WorkerID 的第二个进程会在清容器前拒绝启动；不同 WorkerID 的目录互不清理。
重启时先清理归属本 Worker 的容器，再回收其目录内的 `attempt-<id>-<随机数>`；
锁持有到任务全部退出，SIGKILL 会由内核释放锁。工作目录须位于支持 flock 的本地文件系统，
WorkerID 在重启之间保持稳定，不要手动删除 `.lock` 文件或把活跃目录迁给其他实例。

旧版直接位于工作目录根部的 `attempt-*` 没有归属信息，新版不会猜测并删除；
首次升级应先停止所有旧 Worker、确认其容器与任务已退出，再由管理员清理这些旧目录。
缓存 TTL 是闲置淘汰条件；活跃 Package 租约期间不会因重装或清理而删除测试数据。
测试数据缓存目录须由各 Worker 独占；Package 租约是进程内机制，不用于多个 Worker 共享缓存目录。

## 一键隔离验收

前提：Linux Docker daemon/cgroup v2、Docker Compose v2、Go（版本见 go.mod）、curl、openssl、C 编译器，
以及已构建的官方语言镜像。Docker daemon 需要访问 Worker 使用的本地临时目录。

```sh
bash scripts/build-language-images.sh
bash scripts/test-judge-e2e.sh
JUDGE_E2E_SLOW=1 bash scripts/test-judge-e2e.sh
```

脚本不会读取开发 `.env`，不会重启/消费已有业务服务；每次新建独立 Compose project、
随机凭据、MySQL 数据库、MinIO Bucket/对象以及 MQ 消费状态。
默认独立端口为 MQ 28081、MySQL 23316、MinIO 29000；有冲突时可设置
`JUDGE_E2E_MQ_PORT`、`JUDGE_E2E_MYSQL_PORT`、`JUDGE_E2E_MINIO_PORT`。
退出（包括失败、SIGINT/SIGTERM）删除该 project 的容器/网络/卷；测试自身释放对象、数据库、
缓存与沙箱容器。SIGKILL/断电无法执行 shell trap，可按 `.run/judge-e2e` 中的 project 名称
使用对应 Compose 文件手动清理，禁止使用全局 `docker system prune`。

诊断只保存测试结果和有限 Broker 状态到 `.run/judge-e2e/<project>`，不导出环境变量、
源码、Token、对象存储凭据、数据库转储或容器文件系统。

脚本同时对 Problem、Contest、Submit 三个 Model 执行真实 MySQL 集成测试，
三个 `*_MODEL_TEST_DSN` 仅指向本次隔离 MySQL；结果单独写入 `models.log`。
它们覆盖题目 revision / 发布快照、上传事务回滚、比赛报名 / 冻结榜单、提交租约 / 幂等事务，
仍不能代替浏览器、完整 HTTP 鉴权或真实题目上传验收。运行期间不要编辑该脚本。

## 验收矩阵

| 范围 | 测试及真实边界 |
| --- | --- |
| 全链路 C++20 / Go AC | `internal/e2e`：真实 MySQL Outbox → RocketMQ → Consumer → 真实 Submit RPC/事务 → 固定 revision Problem RPC fixture → 真实签名 MinIO GET/cache → Docker 编译/执行 → 真实 Report RPC → MQ ACK |
| 各语言 AC/WA/CE/RE/TLE/MLE/OLE | `internal/engine` 的官方镜像矩阵，`JUDGE_E2E_SLOW=1`；每用例独立容器、非 CE 返回全部用例 |
| 三种标准 Checker | `internal/checker` 的流式比较、超长 token、取消和大小边界测试 |
| 网络/文件/PID/内存/输出隔离 | `internal/sandbox`：无非 loopback 网卡、网络访问失败、宿主已知敏感文件不可读、Docker Socket 不存在、只读根目录、PID/FD 耗尽、CPU/Wall/MLE/OLE、取消后容器不存在 |
| ZIP 攻击 | `internal/testdatacache`、`pkg/testdatamanifest`：路径穿越、绝对路径、符号链接、设备文件、压缩炸弹、数量/单文件/总量上限，失败无原子缓存命中或临时残留 |
| 结果恰好应用一次 | E2E 丢失 Report 响应、丢失 MQ ACK、Outbox Broker ACK 后 Mark 失败；验证数据库只有一个 Finished Attempt，Applied 次数为 1 |
| Lease 崩溃恢复 | E2E 通过真实 RPC Acquire 后停止心跳，真实 recovery worker 创建后继 Attempt，旧消息只能 ACK/拒绝 |
| 源码永久错误收敛 | E2E 预置旧 4MiB 源码，经真实 Acquire/Report RPC 返回 CE，只有一个 Attempt，最终结果只应用一次 |
| 可信 CPU/进程隔离 | 100ms CPU 限制下的 200ms 短程序及自动回收子孙进程均为 TLE；伪造计量帧、访问 PID 1 控制 fd、跨容器同 UID 的有界 fork 均有真实 Docker 回归 |
| 崩溃工作目录回收 | `internal/workspace` 真正 SIGKILL 持锁子进程，验证重启回收源码/产物，保留其他活跃 Worker 和无归属目录 |
| 临时依赖故障 | E2E 注入 Submit/Problem RPC Unavailable、真实 MinIO GET 503、Docker Execute 失败；均恢复为 AC；临时心跳失败及明确 Lease 失效另有 Processor 竞态测试 |
| 有界并发/停机 | `signal_test.go` 通过子进程验证真实 SIGTERM 进入有界、未提前取消的退出宽限期；`internal/consumer`/`worker`/`app` 验证硬预取上限、停止拉取、活跃心跳、超时取消及 join；Consumer 在确认 C++ 容器运行后强制关闭，并检查容器、目录及 goroutine 无残留 |
| 可观测性/脱敏 | E2E 与 Consumer 测试校验指标 scrape、零 inflight、关联 ID/阶段，且不包含故障注入的秘密标记或源码全文 |

Problem RPC 在 E2E 使用带鉴权的固定数据 fixture，不启动完整题目 CRUD 服务；
测试覆盖 Worker 获取固定 revision 的 RPC 边界，而不是题目上传业务。
Docker 暂时故障在 Execute 接口注入，避免停止同机 Docker daemon 影响其他项目。

## CI 与单独运行

`.github/workflows/judge.yml` 在 push/PR 运行普通竞态测试、vet 和完整 Go build；
昂贵、需 Docker 权限的语言/安全/E2E Job 仅手动 `workflow_dispatch` 触发。
本地可独立运行 `go test -race ./...`，默认跳过需要基础设施的测试。
真实 MQ 单包测试必须使用隔离 Broker；不要把 `JUDGE_MQ_INTEGRATION=1` 指向业务队列。

RocketMQ 固定服务端 5.3.2。Go SDK 固定 5.1.4，并在 `third_party/rocketmq-clients`
保留必要的配置快照与关闭修复（包括真实 Broker race 回归），详细差异见 `LOCAL_PATCHES.md`。
升级时必须重新跑真实 MQ 竞态、关闭以及本页完整验收，不能只验证 Producer 能发消息。
无网络可单独运行 `go test -race github.com/apache/rocketmq-clients/golang/v5 -run TestLocal`，
检查 SDK 配置防御性快照、维护线程关闭、消息大小边界与 TLS 安全默认值。
