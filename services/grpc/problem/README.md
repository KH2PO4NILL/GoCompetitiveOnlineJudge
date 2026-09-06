# Problem RPC

题目、判题配置和不可变测试数据版本的业务服务。默认监听 `0.0.0.0:8006`，etcd 注册键为 `problem.rpc`。
公开题面、管理编辑数据和 Worker 判题规格是不同的读取边界，不能互相替代。

## 职责与接口

完整协议：[proto/problem.proto](proto/problem.proto)。

| 分组 | 主要接口 |
| --- | --- |
| 题库读取 | `ListProblems`、`GetProblem` |
| 管理读取 | `ListProblemsForManagement`、`GetProblemForEdit` |
| 题目维护 | `CreateProblem`、`UpdateProblem`、`PublishProblem`、`ArchiveProblem`、`DeleteProblem`、`RestoreProblem` |
| 测试数据 | `CreateTestDataUpload`、`FinalizeTestDataUpload` |
| 内部读取 | `BatchGetProblems`、`GetJudgeProblemSpec` |

题目生命周期和并发写通过 revision 约束；修改、发布及完成上传等请求必须携带所需的 `expected_revision`。
冲突后应重新读取当前版本，不要用旧请求反复覆盖。持久化语义见 [model/README.md](model/README.md)。

## 测试数据流程

1. 管理端创建题目并取得当前 revision。
2. `CreateTestDataUpload` 创建上传记录，返回 `upload_id`、`upload_url` 和 `expires_at`。
3. 客户端直接向对象存储上传 ZIP；不是把 ZIP 作为普通 JSON 传给 HTTP 网关。
4. `FinalizeTestDataUpload` 校验所属题目、revision、有效期、长度、SHA-256、清单与 ZIP 安全边界，
   将临时对象复制到服务端拥有的不可变版本 Key 后提交题目新版本。
5. 满足发布条件后调用 `PublishProblem`；Submit / Worker 按固定 revision 获取判题规格。

数据包必须在根目录提供 `manifest.json`，完整格式见 [TEST_DATA_FORMAT.md](TEST_DATA_FORMAT.md)。
上传 URL 对浏览器可达、签名请求头一致及对象存储 CORS 需要单独验证；后端单元测试不代表浏览器上传已验收。
协议保留的 SPJ / 交互题字段不等于 Worker 能执行，当前执行支持以 [Worker 文档](../../../judge-worker/README.md)为准。

## 依赖、配置与权限

主配置：[etc/problem.yaml](etc/problem.yaml)。

| 依赖 | 配置入口 |
| --- | --- |
| MySQL / Redis | `PROBLEM_MYSQL_DSN`、`PROBLEM_REDIS_PASSWORD`、`RedisDB.Addr`；迁移位于 [migrations/](migrations/) |
| etcd | `Etcd`、`AuthRPC.Etcd`、`ContestRPC.Etcd` |
| Auth / Contest | `AUTH_INTERNAL_PROBLEM_TOKEN`、`CONTEST_INTERNAL_PROBLEM_TOKEN` |
| 对象存储 | `PROBLEM_OBJECT_STORAGE_ENDPOINT`、`REGION`、`BUCKET`、`ACCESS_KEY`、`SECRET_KEY`（后四项也使用 `PROBLEM_OBJECT_STORAGE_` 前缀） |
| 内部调用方 | `PROBLEM_INTERNAL_CONTEST_TOKEN`、`PROBLEM_INTERNAL_SUBMIT_TOKEN`、`PROBLEM_INTERNAL_JUDGE_TOKEN` |
| 上传限制 | `Upload`：URL 有效期、包大小、解压总量、单文件大小、文件数量及压缩比 |

- 题库读取允许匿名，但仍检查题目状态、可见性和比赛上下文；提供了无效 Token 不等于匿名访问。
- 管理接口要求用户身份并由业务层检查角色、归属和状态，不能只依赖管理页面隐藏按钮。
- `BatchGetProblems` 只开放给配置授权的 Contest / Submit；`GetJudgeProblemSpec` 只开放给 Submit / Worker。
  内部接口跳过的是用户 Token 检查，不是服务身份认证。
- Problem 的对象存储身份需要完成上传和版本化操作；Worker 应使用独立的只读身份，不能复用写入或管理员凭据。

## 本地启动与验证

按[总 README](../../../README.md)准备依赖与迁移，再在项目根目录执行：

```bash
set -a
. ./.env
set +a
go run ./services/grpc/problem -f services/grpc/problem/etc/problem.yaml
```

仅加载可信环境文件。实际读取需要 Auth；比赛可见性校验还需要 Contest，上传流程需要对象存储。

```bash
go test -race ./services/grpc/problem/... ./pkg/testdatamanifest/...
go build -o .run/bin/problem ./services/grpc/problem
make rpc-problem
```

普通测试未设置 `PROBLEM_MODEL_TEST_DSN` 时跳过 MySQL 集成部分；设置后会创建并删除随机隔离数据库，
需要专用测试账号具备建库 / 删库权限，不能将业务 DSN 当作测试配置。规则见 Model README。
完整评测基础设施验收入口为 `JUDGE_E2E_SLOW=1 bash scripts/test-judge-e2e.sh`，该脚本也执行隔离模型测试，
但其 Worker 场景的 Problem RPC 仍是固定 revision fixture，不代替本模块真实建题 / 上传链路。

## 目录导航

- `problem.go`：启动、服务注册及方法级鉴权策略。
- `proto/`、`pb/`、`problemservice/`：协议和生成的服务端 / 客户端代码；协议修改后用 `make rpc-problem` 生成并审查差异。
- `internal/server/`、`internal/logic/`：RPC 转发、业务权限与状态校验。
- `internal/svc/`、`internal/config/`：数据库、下游 RPC、对象存储和上传校验依赖。
- `model/`、`migrations/`：事务、revision、不可变数据版本和表结构。

相关入口：[App HTTP](../../../cmd/app/README.md) · [Admin HTTP](../../../cmd/admin/README.md)。
