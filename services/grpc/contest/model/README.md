# Contest 数据模型

`ContestModel` 是 Contest RPC 唯一公开的持久化边界。Logic 不直接使用 GORM，也不跨表组合写操作。

## 生命周期和 revision

- `contests.status` 只保存 Draft、Published、Cancelled、Archived、Deleted 等稳定状态。
- Running 和 Finished 由 Published、`start_at`、`end_at` 与服务端时间即时计算，不依赖定时任务更新数据库。
- 配置、生命周期、题目编排、Freeze 和 Unfreeze 共享比赛聚合 revision；所有写操作锁定聚合根并检查 `expected_revision`。
- Delete 为软删除并保存 `deleted_from_status`，Restore 只恢复 Model 已记录的状态。

## 报名与参赛

- 报名记录和参赛者是不同实体；Pending 报名不产生参赛资格。
- Open 和 Invite-only 报名在同一事务中批准并创建 Normal 参赛者。
- Approval-required 报名由审核事务同时更新报名和参赛者，失败时全部回滚。
- Pending/Approved 报名拒绝重复注册；Cancelled/Rejected 记录可以复用同一 registration ID 重新报名并递增独立 revision。
- 参赛者只允许 Normal → Disqualified/Withdrawn，以及 Disqualified/Withdrawn → Normal；状态切换与 participant_count 同事务提交。
- 邀请码使用 bcrypt 摘要保存，读取模型从不返回摘要或原始邀请码。

## 排行榜和封榜

- MySQL 保存当前真实排行榜投影；`source_version` 保证旧事件不能覆盖新投影，同版本重试保持幂等。
- Freeze 在锁定比赛和排行榜版本后复制完整行及每题结果，随后原子切换活动快照。
- 封榜后的真实投影仍可更新，但用户读取只能选择不可变冻结快照。
- Unfreeze 只清除活动快照指针，不删除历史快照。
- 排行榜、报名状态和带 total 的分页读取使用一致性只读事务，避免响应混合两个提交版本。
- Redis 可在后续作为可重建缓存接入，不是本 Model 的一致性来源。

## 数据归属

Contest 仅保存 `creator_id`、`user_id` 和 `problem_id` 等跨服务标识，不建立 Auth、Problem、Submit 的跨库外键。所有网络校验必须在数据库事务外完成。

## 测试

默认测试不连接数据库。设置 `CONTEST_MODEL_TEST_DSN` 后，集成测试会创建随机命名的 `contest_model_test_*` 隔离数据库、执行 migration，并在测试结束后删除该数据库。测试不会修改 DSN 中已有的业务表。
