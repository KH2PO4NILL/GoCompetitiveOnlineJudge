# Submit 数据模型

`SubmitModel` 是 Submit RPC 唯一公开的持久化边界。Logic 负责在事务外调用 Auth、Problem 和 Contest，Model 负责提交、Attempt、逐测试点结果与 Outbox 的本地原子性。

## 聚合与状态

- `submissions` 保存当前对外状态；`submission_attempts` 保存首次派发、租约重试和管理员重判历史。
- `current_attempt_id` 是并发隔离边界。旧或已取代 Attempt 的心跳与结果不能修改当前 Submission。
- 重判和租约恢复创建新 Attempt，保留原 `problem_revision`、旧结果与内部诊断。
- Verdict 仅在 FINISHED 状态持久化；QUEUED/JUDGING 使用 SQL NULL，不把 UNSPECIFIED 当作业务状态保存。

## 幂等与租约

- `(user_id, client_request_id)` 唯一；稳定请求指纹相同返回原提交，不同则返回 `ErrIdempotencyConflict`。
- Worker 领取时数据库只保存随机 lease token 的 SHA-256 摘要，明文只返回一次。
- Result 使用 `attempt_id + report_id + report_fingerprint` 判重；同 key 不同内容是冲突。

## Outbox

- 创建提交/重判与 JudgeTaskEvent 在同一事务提交。
- 判题结果、逐测试点结果、聚合终态与 SubmissionJudgedEvent 在同一事务提交。
- Outbox claim 使用 MySQL 8 `FOR UPDATE SKIP LOCKED`，支持多个 publisher 并行工作。
- RocketMQ publisher、退避调度和部署不属于 Model；它们只能通过 Outbox 方法访问数据。

## 数据归属与安全

Auth、Problem 和 Contest ID 不建立跨库外键。Submit 自有表之间使用外键和 CHECK 约束。源码、编译输出、lease 摘要、Outbox payload 与 `judge_message` 均排除在通用 JSON 序列化之外；生产 GORM 使用 parameterized query 日志。

## 测试

默认测试不连接数据库。设置 `SUBMIT_MODEL_TEST_DSN` 后，集成测试创建随机命名的 `submit_model_test_*` 隔离数据库、执行 migration，并在测试结束后删除数据库。测试不会修改 DSN 中已有业务表。
