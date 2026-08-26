# Problem 数据模型

`ProblemModel` 是 Problem RPC 唯一公开的数据持久化边界。Logic 不应直接使用
GORM，也不应组合 `*_model_gen.go` 中的表级 CRUD。

## 当前状态与不可变快照

- `problems` 保存聚合根、当前 revision、生命周期状态以及当前编辑值，便于管理端读取。
- `problem_revisions` 保存每次成功写操作产生的完整不可变快照，包括题面、元数据、
  判题设置、测试数据版本和当时的状态。
- `problem_tag_definitions` 保存规范化标签；`problem_revision_tags` 使标签集合属于具体
  revision，而不是可变的题目根记录。
- `latest_published_revision` 指向最近一次真正执行 Publish 产生的快照。编辑已发布题目
  不会改变公开页面或历史 `JudgeProblemSpec`，再次 Publish 后才会移动该指针。

## 事务和并发

- Update、Publish、Archive、Delete、Restore 和 Finalize 都锁定题目根，并检查
  `expected_revision`。
- 当前记录更新、revision 插入、revision 标签写入和发布指针更新在同一事务内完成。
- Finalize 先锁上传会话，再写入测试数据版本、测试点、新 revision 和幂等结果；失败时
  全部回滚。
- 已 finalize 的 upload 保存版本 ID 和 revision，重复请求返回第一次提交的结果。

## 查询边界

- `GetPublic`、`ListPublic` 和 `BatchGetPublicSummaries` 只读取当前生命周期为 Published、
  未软删除且已发布快照为 Public 的题目。
- 公开查询读取 `latest_published_revision`，不会泄露尚未再次发布的编辑内容。
- 列表只选择摘要列，标签使用一次批量查询加载；批量查询保持输入顺序，以 `nil` 表示
  不存在或不可公开的条目。
- 关键字查询当前只对标题使用参数化 `LIKE`；数据规模增长后可在不改变 Model filter
  语义的前提下迁移到 MySQL FULLTEXT 或独立搜索服务。

## 测试

默认测试不会连接数据库。设置 `PROBLEM_MODEL_TEST_DSN` 后，集成测试会在指定 MySQL
服务器创建随机命名的 `problem_model_test_*` 隔离数据库，执行 migration，并在测试结束
后删除该数据库。测试不会清理或修改 DSN 中原有的业务数据库。
