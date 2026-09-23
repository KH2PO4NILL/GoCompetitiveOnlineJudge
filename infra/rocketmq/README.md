# Judge Task 消息队列

本地服务使用固定 RocketMQ 5.3.2、嵌入式 gRPC Proxy 与 Go v5 SDK。
仅监听 localhost，不启用 ACL，不适用于公网部署；生产环境应使用 TLS/ACL。

```sh
docker compose --env-file .env up -d --wait rocketmq-namesrv rocketmq-broker
docker compose --env-file .env --profile init run --rm rocketmq-init
```

Proxy 默认地址 `127.0.0.1:18081`，避开 API 常用的 8081。
同步修改 `proxy.json` 和端口映射才能改变实际监听端口。
初始化建立 `judge-task`、`judge-task-dead` Topic 和 `judge-workers` Group。
Broker 必须开启 `timerWheelEnable=true`，否则 v5 SimpleConsumer 的 POP 消费不可用。
本地 Proxy 同时使用 `transferMsgByHeap=true`，不走外部 Netty Channel 的文件零拷贝路径。
Broker 存储卷初始化只调整专用卷根目录所有者，不以 root 运行 Broker。

Submit 配置 `SUBMIT_OUTBOX_ENABLED=true`、`ROCKETMQ_ENDPOINT=127.0.0.1:18081`。
加载 `.env` 后启动 Submit；未启用时不会连接 Broker。
Publisher 参数见 `services/grpc/submit/etc/submit.yaml` 的 Outbox 部分。
每批最多 16 条、4 路并发；数据库 `SKIP LOCKED` Claim 60 秒。
收到 Broker ACK 才 Mark Published；失败指数退避，12 次后 Dead。
Broker ACK 后数据库写失败保留 Claim，超时可由另一实例重复发布。
不要手动把 Published 当作已判题；Submission/Attempt 才是业务事实。
SIGTERM 停止新 Claim，宽限期后取消在途请求。
`127.0.0.1:9101/metrics` 提供 backlog、延迟及成功/失败/Dead 指标。

## 协议兼容

消息为 protobuf `JudgeTaskEvent`，schema_version=1，Key 为十进制 submission_id。
事件只有 UUID、Submission/Attempt ID、入队时间及版本，最多 16 KiB，不含源码或密钥。
新增兼容字段可以沿用 schema 1，禁止复用字段编号或改变含义；未知 protobuf 字段保留兼容。
语义不兼容更改必须升级 schema，先部署支持新版的 Consumer；不支持版本进入 DLQ。
旧数据库 JSON Outbox 在 Publisher 边界转换为版本化 protobuf，不接受旧版无版本的 MQ 消息。

快速验收：`go test -race ./services/grpc/submit/internal/outbox ./pkg/judgeevent ./pkg/judgemetrics`。
真实 Broker：设置 `JUDGE_MQ_INTEGRATION=1` 后运行 Outbox 包测试；只发布唯一测试事件，不消费业务队列。
MySQL：`SUBMIT_MODEL_TEST_DSN` 指向具备建库权限的测试实例，模型测试自动建/删隔离数据库。
