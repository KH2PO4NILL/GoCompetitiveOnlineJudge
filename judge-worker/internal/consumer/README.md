# RocketMQ 消费生命周期

设置 `JUDGE_CONSUMER_ENABLED=true` 和 `ROCKETMQ_ENDPOINT=127.0.0.1:18081`，
其余参数见 Worker YAML 的 Consumer 部分。未启用时保持无 MQ 的离线 Worker 启动行为。
默认 Topic `judge-task`、Group `judge-workers`，不使用 SDK PushConsumer 的预取队列。

SimpleConsumer 每次只 Receive 一条，先保留并发槽位才拉取；运行中、待执行和正在拉取的
总数不超过 Worker.Concurrency，复用有界 Pool，本地无额外无界缓冲。
默认消息不可见期 60 秒，每 20 秒同步续期，长任务不会因默认可见期到期而堆积重复投递。
续期失败会取消本次 Processor，停止租约心跳并清理 Sandbox，等待重投或数据库 Lease recovery。
SDK ChangeInvisibleDuration 不提供 context 参数，因此额外受已配置的 SDK request timeout 限制。

ACK 对应已应用、重复、过期、完成或其他 Worker 持有。临时失败用 broker visibility
实现 5 秒起、300 秒封顶的指数退避，最多 16 次后进入审计 DLQ（可配置）。
永久无效事件直接发布 `judge-task-dead`，收到 DLQ Broker ACK 后才 ACK 原消息。
DLQ 保存 Message ID、Key、重投次数和安全原因码，不保存原始消息体；超长元数据以 SHA256 替代。
ACK 丢失允许重复消息，由数据库租约及 Report ID 保证结果幂等，不依赖 MQ 去重。

SIGTERM 先停止 Receive，不再把排队消息交给 Processor；已开始任务继续心跳及结果上报。
宽限期耗尽才取消 Pool Context，等待清理完成后关闭 MQ/RPC/Sandbox。
已拉取但未开始、未 ACK 的消息会在可见期到期后重投。

启用 `Metrics.Enabled` 后 `/metrics` 暴露 `judge_consumer_*`：
inflight、pool_saturation（0–1）、ack/retry/dlq 及失败计数、端到端延迟 sum/count。
日志只包含受控阶段/原因及 submission_id、attempt_id、worker_id，不输出依赖错误正文或源码。

```sh
go test -race ./judge-worker/internal/consumer ./judge-worker/internal/processor ./judge-worker/internal/worker
JUDGE_MQ_INTEGRATION=1 go test ./pkg/judgemq -run TestSimpleConsumerIntegration -count=1 -v
```

真实测试仅用于隔离 Broker，使用独立消费组，不 ACK 其他事件。完整隔离与清理见 E2E 脚本。
