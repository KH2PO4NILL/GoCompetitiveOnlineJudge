# Language Profile 与镜像协议（Schema v1）

Worker 启动时严格校验全部语言 Profile（包括禁用项），然后把只读 Registry 和通用
`engine.Engine` 注入 `ServiceContext`。Engine 不识别具体语言名称，不包含语言 switch。
本层只负责一次 Attempt 的编译、重复运行和执行事实，不实现队列消费、RPC 编排、Checker
或 AC/WA 等最终 verdict，也不新增 Python、PyPy3、Java。

## 配置与部署

`judge-worker/etc/judge-worker.yaml` 的 `LanguageRegistryFile` 接受注册表 YAML 路径，
相对路径相对于 Worker 启动目录。环境变量 `JUDGE_LANGUAGE_REGISTRY_FILE` 为空时使用
编译进 Worker 的 `internal/language/profiles.yaml`，包含 `cpp20-v2` 和 `go-v2`。
指定外部文件时完整替换默认表，不合并、不热更新；文件内部不展开环境变量。

先构建并在每台 Worker 的 Docker daemon 中预置镜像：

```bash
bash scripts/build-language-images.sh
```

Dockerfile 使用固定版本的 gcc / golang 基础镜像；可用 `--build-arg BASE_IMAGE=仓库@sha256:摘要`
进一步固定构建输入。Compiler 和 Runtime 分别发布为 `1.1.0-compiler` / `1.1.0-runtime`。
Runtime 基于 scratch，只包含静态入口，不包含编译器、包管理器或 shell；两个语言均生成静态产物。
C++ 使用 `-std=c++20 -O2 -static`、固定随机种子、路径映射和 SOURCE_DATE_EPOCH；
Go 禁用 CGO，使用 `-trimpath -buildvcs=false -mod=readonly -ldflags=-buildid=`。
相同工具链、架构、Profile 与源码的两次构建必须通过产物 SHA-256 一致性验收。
v2 是相对 #32 原型 v1 的新策略版本，不覆盖已部署的 v1 定义；仍需要 v1 的部署应保留其外部注册表。
生产发布应把发布后的 `repository@sha256:...` 写入外部 Profile。
不要重新绑定已发布的版本标签，不要给已有 Profile ID 更换语义；策略、脚本、编译器变化时，
发布新的镜像版本和新的 `-vN` Profile ID。

所有镜像必须使用 SHA-256 digest、Docker 内容寻址 image ID，或明确的
`major.minor.patch` / `vmajor.minor.patch[-后缀]` 标签。
无标签、`latest`（即使同时指定 digest）、`stable`、`14`、`1.26` 等浮动引用被拒绝。
Worker 不拉取镜像。启动时验证引用格式；首次创建 Attempt 时通过 Docker ImageInspect
解析本地镜像，缺失镜像明确报错，之后 Build/Run 只使用解析后的不可变 image ID。
因此标签在 Attempt 执行期间移动不会改变该 Attempt 的镜像。

## Profile Schema

完整可运行示例是 `../internal/language/profiles.yaml`；Go 类型和严格校验在
`../internal/language/profile.go`。顶层只允许 `profiles` 列表（1–256 项）。
注册表最多 1 MiB，只能包含一个 YAML 文档，未知字段、重复字段、重复 ID、非法配置均拒绝。

| 字段 | 约定 |
| --- | --- |
| `id` | 最多 128 字节，小写字母开头、字母数字和连字符组成，必须以 `-vN` 结束；注册后不可修改，变更需新 ID |
| `schema_version` | 当前只接受 `1` |
| `enabled` | `false`（省略时默认禁用）不可创建 Attempt；禁用不豁免配置校验 |
| `source.entry_file` | 必须存在的入口文件，如 `main.cpp`、`main.go` |
| `source.extensions` | 允许的扩展名，入口也必须匹配；不可重复 |
| `source.max_files` / `max_bytes` | 源文件数和总大小上限，最多 64 个 / 16 MiB |
| `build.image` / `run.image` | 固定 Compiler / Runtime 镜像引用 |
| `build.program` / `run.program` | 必须分别为 `/opt/judge/build` 与 `/opt/judge/run` |
| `build.args` / `run.args` | 独立 argv 字符串数组，不进行插值、拼接或 shell 展开 |
| `artifact.directory` | 必须为 `/sandbox/artifact` |
| `artifact.files` / `max_bytes` | 成功编译后精确输出清单和总大小上限，最多 64 个 / 256 MiB |
| `build_limits` / `run_limits` | `cpu_time_ms`、`wall_time_ms`、`memory_bytes`、`output_bytes`、`pids`、`open_files` |
| `build_limits.tmpfs_bytes` / `run_limits.tmpfs_bytes` | 可选容器 `/tmp` 容量，不超过内存限制或 1 GiB；0 使用沙箱原有默认值 |
| `time_multiplier` / `memory_multiplier` | 有限正数，最多 100；应用于题目运行限制，向上取整 |
| `environment_allowlist` | 允许由 Profile 显式注入的环境变量名，不可重复 |
| `environment` | 显式环境变量键值；必须全部位于白名单内，不继承 Worker 进程环境 |

v1 源文件和产物只允许扁平 ASCII 文件名，不接受目录、绝对路径、`..` 或路径分隔符。
成功产物必须是非空、所有用户可读的普通文件，拒绝符号链接、目录、未声明文件和超量产物。
`run_limits` 的时间与内存是上限；实际运行时间/内存是题目限制乘语言倍率，超出上限则拒绝
Attempt，不静默截断。PID、FD、stdout+stderr 上限直接使用各阶段配置。编译限制不乘倍率。

## 标准镜像协议

两个入口是镜像内由运维审查的可信可执行文件，而不是提交代码的一部分。可以是固定脚本；
Engine 永远用独立 Program/Args 调用，禁止 `/bin/sh -c` 和由提交内容拼接的命令。
现有脚本通过固定编译器路径及 `"$@"` 传递参数，不使用 `eval`。

| 阶段 | 源文件 | 产物 | 工作目录 |
| --- | --- | --- | --- |
| Build | `/sandbox/source` 只读 | `/sandbox/artifact` 可写 | `/sandbox/artifact` |
| Run | 不挂载 | `/sandbox/artifact` 只读 | `/sandbox/artifact` |

两个阶段均使用现有 Docker 沙箱的无网络、非 root 用户、只读根文件系统、能力剥夺和资源限制。
Build 唯一可写的宿主机 bind mount 是 Artifact，只向这里写入声明的最终产物。
编译中间文件和缓存放在受限的、容器私有的 `/tmp` tmpfs，不能持久化回宿主机；
Go 编译配置 512 MiB 临时空间。临时空间计入容器内存，容器销毁时连同中间文件清理，
即使编译被强制终止，也不会在宿主机遗留属于沙箱 UID 的不可清理缓存目录。
Run 入口把 `TMPDIR` 设为 `/tmp`，不修改 Artifact；stdin/stdout/stderr 用于测试输入和输出。
镜像自带环境是可信镜像协议的一部分；白名单控制的是 Worker 显式注入的环境，不清除基础镜像
内置的 PATH 等变量。不得把凭据或其他秘密烘焙进语言镜像。

宿主机每个 Attempt 使用独立随机目录；父目录仅 Worker 可访问，Artifact bind 源目录允许
沙箱 UID 写入。调用方必须在成功或失败后 `Close()` 清理；Run 不接受外部产物路径，且只能在
一次成功 Build 后调用，可重复运行多个测试点。编译失败、协议不符、Close 后均禁止 Run，
失败的 Build 不得在同一 Attempt 重试，应创建新 Attempt。

Artifact 大小是在 Build 结束后验证的产物约束，不是磁盘写入硬配额。与现有 Docker 沙箱一样，
生产环境仍须给工作目录配置文件系统配额、容量监控和异常退出后的工作目录清理；本 Issue
不新增磁盘配额或 Worker 崩溃后的宿主机目录回收器。

## 策略冻结与审计

Registry 不提供修改接口，复制配置及每次返回的策略，防止调用方修改共享 map/slice。
`Registry.Snapshot` 解析 Compiler/Runtime 本地 image ID，将完整 Profile 和解析后的
两个镜像 ID 规范化 JSON 编码后计算 SHA-256。Engine 再把实际生效的运行限制纳入 Attempt
Policy Digest，仅在 `Prepare` 时冻结一次；后续 Build 和所有 Run 不重新查询 Registry。

审计上下文包括 Profile ID、Schema Version、原始 Compiler/Runtime 引用、两个已解析的
SHA-256 image ID 和 Policy Digest。注意 Docker image ID 是镜像配置内容摘要，不是 registry
manifest digest；原始引用单独保留，用于同时追踪部署 manifest 引用和实际本地执行镜像。
上下文进入策略冻结日志、成功编译/运行日志、`Attempt.Audit()` 和每次 `engine.Result.Policy`，
执行报错时也保留 Result 的策略上下文。未来的结果持久化/RPC 编排应存储这份上下文。

未知 Profile 返回可用 `errors.Is` 判断的 `language.ErrUnknownProfile`，禁用项返回
`language.ErrDisabledProfile`；执行顺序错误返回 `engine.ErrAttemptState`。

## 新增语言与测试

新增语言只需要：新增版本化 Profile；提供固定镜像与标准 Build/Run 入口；新增语言测试数据和
语言包集成测试用例。不修改 Engine 的 Prepare/Build/Run 主流程。

```bash
go test ./judge-worker/...
go test -race ./judge-worker/internal/language ./judge-worker/internal/engine
JUDGE_LANGUAGE_INTEGRATION=1 go test ./judge-worker/internal/engine -run TestLanguageImagesIntegration -v
JUDGE_LANGUAGE_INTEGRATION=1 go test ./judge-worker/internal/engine -run TestOfficialLanguagePacks -v
```

前两组不依赖 Docker，覆盖严格 Schema、镜像约束、策略快照、倍率边界、启动拒绝、
Fake Profile、只读挂载、异常产物、编译失败和策略审计。第三组需预置镜像并有 Docker socket
访问权限，对 C++20 / Go 用同一执行流程验证输入输出、复用 Artifact 和编译失败。
沙箱底层超时、内存、进程、FD 和网络隔离测试继续使用 `internal/sandbox` 的测试套件。
