# GoCompetitiveOnlineJudge

GoCompetitiveOnlineJudge 是一个以 Go 为主要开发语言的
分布式程序设计竞赛在线评测系统。

## 核心功能：

- 多语言在线代码评测
- 分布式 Judge Worker
- CPU / 内存 / 时间资源限制
- 创建与管理比赛
- 支持 ICPC / IOI 比赛模式
- 实时排行榜
- ICPC 封榜与解封榜

---

**技术栈**：Go+GoZero+gRPC...

## HTTP 服务

| 服务 | 端口 | 描述 |
|---|---:|---|
| admin | 8001 | 管理端 |
| app | 8002 | 客户端 |

## gRPC 服务

| 服务 | 端口 | 描述 |
|---|---:|---|
| auth-rpc | 8003 | 鉴权模块 |
| submit-rpc | 8004 | 提交模块 |
| problem-rpc | 8006 | 题面模块 |
| contest-rpc | 8007 | 比赛模块 |


异步服务

judge-worker 评测服务
```
judge-worker
│
├── consumer
├── worker pool
├── executor
│   ├── cpp
│   └── go
├── docker sandbox
├── testcase
├── checker
└── reporter
```