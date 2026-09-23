# Contest RPC

比赛生命周期、题目编排、报名资格、榜单读取与封榜服务。默认监听 `0.0.0.0:8007`，etcd 注册键为 `contest.rpc`。
契约包含 ICPC、IOI、OI 三种模式；不能由接口或模型存在推断全部赛制业务链路已经交付。

## 职责与接口

完整协议：[proto/contest.proto](proto/contest.proto)。

| 分组 | 主要接口 |
| --- | --- |
| 比赛读取 | `ListContests`、`GetContest`、`ListContestProblems`、`GetScoreboard` |
| 用户报名 | `RegisterContest`、`CancelRegistration`、`GetMyContestStatus` |
| 管理与生命周期 | `ListContestsForManagement`、`CreateContest`、`GetContestForEdit`、`UpdateContest`、`PublishContest`、`CancelContest`、`ArchiveContest`、`DeleteContest`、`RestoreContest` |
| 编排与审核 | `SetContestProblems`、`ListRegistrations`、`ReviewRegistration`、`ListParticipants` |
| 封榜 | `FreezeScoreboard`、`UnfreezeScoreboard` |
| 内部策略 | `CanAccessProblem`、`ValidateContestSubmission`、`ResolveSubmissionResultVisibility` |

## 状态、一致性与交付边界

- Draft / Published / Cancelled / Archived / Deleted 是持久化状态；Running / Finished 由比赛时间和服务端时间推导。
- 配置、生命周期、题目编排和封榜写入检查比赛 revision；报名记录有独立 revision。
  冲突时先读取新版本，不以客户端时间或旧 revision 绕过状态限制。
- Pending 报名不等于已取得参赛资格；审核通过与创建参赛者在同一事务完成。
  邀请码以 bcrypt 摘要保存，邀请信息和审核备注不应进入请求内容日志。
- MySQL 保存榜单投影、版本及不可变封榜快照；封榜后的公开读取不会直接读取更新中的真实榜单。
  事务和版本规则见 [model/README.md](model/README.md)。
- 当前服务有自动封榜任务，但该任务只冻结已有投影，**不负责从提交事件实时计分**。
  `ReplaceScoreboard` 是 Model 写入边界；当前模块没有从 Submit 已判题事件到榜单的生产消费链路，
  不应把投影模型测试描述为完整的实时比赛榜单验收。
- 模型中的参赛者状态变更能力不等于已暴露同名 RPC / HTTP 管理动作；可调用动作以 `.proto` 为准。

## 依赖与配置

主配置：[etc/contest.yaml](etc/contest.yaml)。

| 依赖 / 任务 | 配置入口 |
| --- | --- |
| MySQL | `CONTEST_MYSQL_DSN`；迁移位于 [migrations/](migrations/) |
| etcd | `Etcd`、`AuthRPC.Etcd`、`ProblemRPC.Etcd` |
| Auth / Problem | `AUTH_INTERNAL_CONTEST_TOKEN`、`PROBLEM_INTERNAL_CONTEST_TOKEN` |
| 内部调用方 | `CONTEST_INTERNAL_PROBLEM_TOKEN`、`CONTEST_INTERNAL_SUBMIT_TOKEN` |
| 自动封榜 | `ScoreboardFreeze.IntervalSeconds`、`BatchSize`、`OperationTimeoutSeconds` |

默认自动封榜每 10 秒扫描，批量 100 场，单场操作超时 5 秒。多实例通过事务和 revision 竞争封榜，
不是靠单机内存定时器保证唯一性。当前 Contest YAML 不配置 Redis，榜单一致性来源是 MySQL。

## 鉴权边界

- 公开读取允许可选用户身份，实际可见性仍由比赛状态、公开范围和参赛资格控制。
- 报名等用户操作要求登录；管理接口继续校验管理员角色、比赛归属及允许的生命周期操作。
- `CanAccessProblem` 只允许授权的 Problem 服务调用；两个提交策略接口只允许授权的 Submit 服务调用。
  使用 `x-service-name` / `x-service-token` 的内部白名单，不将这些凭据交给 HTTP 客户端。
- 用户 Authorization 与服务身份各有职责；方法策略见 [contest.go](contest.go)，业务授权见 `internal/logic/`。

## 本地启动与测试

按[总 README](../../../README.md)准备依赖及迁移，然后在项目根目录加载可信环境文件：

```bash
set -a
. ./.env
set +a
go run ./services/grpc/contest -f services/grpc/contest/etc/contest.yaml
```

Auth 提供用户身份校验，Problem 提供题目编排所需的内部读取；端口启动不代表依赖和榜单数据已齐备。

```bash
go test -race ./services/grpc/contest/...
go build -o .run/bin/contest ./services/grpc/contest
make rpc-contest
```

未设置 `CONTEST_MODEL_TEST_DSN` 时跳过 MySQL 集成部分；专用测试服务器账号需要创建 / 删除随机隔离数据库的权限。
测试规则见 Model README，不能将业务 DSN 当作测试配置。
上述测试不等于真实比赛提交、事件计分、浏览器封榜或多赛制闭环验收。

## 目录导航

- `contest.go`：启动、RPC 注册和方法鉴权。
- `proto/`、`pb/`、`contestservice/`：协议及生成代码；修改契约后运行 `make rpc-contest` 并审查差异。
- `internal/server/`、`internal/logic/`：RPC 适配、业务验证和响应裁剪。
- `internal/svc/`：依赖装配、题目查询及自动封榜任务。
- `internal/config/`、`model/`、`migrations/`：配置、事务模型及表结构。

相关入口：[App HTTP](../../../cmd/app/README.md) · [Admin HTTP](../../../cmd/admin/README.md) · [Submit RPC](../submit/README.md)。
