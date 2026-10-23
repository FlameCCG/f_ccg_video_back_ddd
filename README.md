# CCG 视频后端：DDD 源码阅读入口

本项目是按业务上下文组织的 Go 分层单体。HTTP、CLI、WebSocket 和后台任务复用应用用例，MySQL、MongoDB、Redis、Elasticsearch、Kafka 与存储驱动通过端口接入。Wire 在构建前生成依赖装配代码。

第一次阅读建议先看这份导航，再沿一条业务链进入源码。每个 Go 包的包注释说明职责、关键文件和边界；复杂函数的注释解释事务、并发、失败和生命周期。

## 从哪里开始

| 阅读目标 | 入口 | 接下来追踪 |
| --- | --- | --- |
| 进程如何启动和退出 | [服务入口](cmd/server/main.go) | [注入器声明](internal/bootstrap/wire.go) → [App](internal/bootstrap/app.go) → [后台组件](internal/bootstrap/runtime.go) |
| Wire 如何把接口接到实现 | [Provider 集合](internal/bootstrap/providers.go) | [上下文绑定](internal/bootstrap/providers_contexts.go)、[跨上下文绑定](internal/bootstrap/providers_cross_context.go) → [生成结果](internal/bootstrap/wire_gen.go) |
| 一次投稿如何完成 | [HTTP 投稿入口](internal/interfaces/http/handler/video/publish.go) | [投稿用例](internal/application/video/publish_service.go) → [领域规则](internal/domain/video/status_resolve.go) → [单库保存](internal/infrastructure/persistence/video/publish_repo.go) |
| 上传如何续传和清理 | [上传用例](internal/application/video/upload_service.go) | [配额](internal/application/video/upload_quota.go)、[哈希锁](internal/infrastructure/cache/video/upload_guard.go)、[清理用例](internal/application/video/upload_cleanup.go) |
| 改稿、审核与定时如何协调 | [改稿用例](internal/application/video/update_service.go) | [实体改稿行为](internal/domain/video/revision.go)、[审核用例](internal/application/video/admin_video_service.go)、[到期发布](internal/application/video/scheduler.go) |
| 媒体如何从源片变成可播放资源 | [任务消费](internal/infrastructure/mq/transcode_consumer.go) | [转码用例](internal/application/media/transcode_service.go) → [领域计划](internal/domain/media/plan.go) → [管线协调](internal/application/media/pipeline_coordinator.go) |
| 写模型与页面数据为何分开 | [详情用例](internal/application/video/detail_service.go) | [读模型](internal/application/video/dto.go)、[SQL 投影](internal/infrastructure/persistence/video/detail_query.go)、[缓存回填](internal/infrastructure/cache/video/detail_meta_cache.go) |
| 跨上下文如何保持本地事务 | [投币用例](internal/application/video/interact_service.go) | [硬币适配器](internal/bootstrap/adapters/identity.go) → [账本用例](internal/application/identity/coin_service.go) → [事务实现](internal/infrastructure/persistence/gormx/tx.go) |
| 权限如何形成与生效 | [角色继承查询](internal/application/rbac/role_graph.go) | [领域权限规则](internal/domain/rbac/permission.go)、[模型配置](conf/rbac_model.conf)、[请求鉴权](internal/interfaces/http/middlewares/casbin_auth.go) |
| 搜索数据从哪里来 | [搜索用例](internal/application/search/search_service.go) | [River 同步](internal/infrastructure/river/sync.go)、[向量用例](internal/application/search/vector_service.go)、[SQL 回退](internal/infrastructure/persistence/search/search_fallback.go) |

## 看懂目录，也看懂依赖方向

| 目录 | 负责什么 | 新增代码时先问什么 |
| --- | --- | --- |
| `internal/interfaces` | 请求、响应、参数绑定、身份提取、协议连接 | 变化是否只涉及外部协议？ |
| `internal/application/<上下文>` | 编排一次业务目标，声明查询与协作端口 | 需要取得哪些事实、按什么顺序完成哪些操作？ |
| `internal/domain/<上下文>` | 实体、值、允许条件、状态规则与业务端口 | 不考虑数据库和 HTTP 时，业务结论是什么？ |
| `internal/infrastructure` | SQL、缓存、消息、外部协议、存储和命令执行 | 如何用具体技术履行端口的行为契约？ |
| `internal/bootstrap` | 选择实现、翻译跨上下文模型、连接生命周期 | 如何把既有能力构造成完整运行对象？ |
| `internal/config` | 配置树与纯校验的数据契约 | 字段形状、单位、缺省值和读取策略是否一致？ |
| `pkg` | 通用协议/技术辅助 | 它是否真的通用，还是应属于某个业务上下文？ |

编译时，应用依赖自己或领域声明的接口，基础设施实现这些接口，组合根认识两边。运行时，请求仍会从应用调用注入的仓储并访问数据库。接口倒置改变的是源码依赖关系，不会消除数据库调用。

这里使用共用存储的轻量 CQRS：写侧模型维护业务规则，读侧查询直接组装页面 DTO。PO 描述数据库形状，DTO 描述用例输入输出，领域实体表达业务概念，三者不必逐字段一致。上下文之间通过消费方端口协作，并不意味着每个上下文都有独立进程或数据库。

## 用一个 Wire 绑定理解装配

以 `PublishService` 为例：

1. [领域仓储接口](internal/domain/video/repository.go)声明 `VideoRepository`，投稿用例的构造器接收这个接口。
2. [基础设施仓储](internal/infrastructure/persistence/video/video_repo.go)提供具体实现及构造器。
3. [组合根](internal/bootstrap/providers_contexts.go)登记构造器，并用 `wire.Bind` 声明接口对应哪个具体类型。
4. Wire 从 `InitializeApp` 的返回类型反向解析可达依赖，在 `wire_gen.go` 生成普通 Go 构造调用。
5. 程序启动执行生成函数，收到请求后才调用 `PublishVideo` 等用例；请求期间不运行 Wire 的图解析。

`wire.NewSet` 汇总候选 provider，不按声明列表逐项启动服务。某个后台组件没有被最终对象引用时可能被裁剪，所以 `RuntimeComponents` 显式持有并启停它们。provider 的 `cleanup` 负责资源释放，后台组件的 `Stop` 负责停止工作，两者要配合使用。

修改装配声明后，可在项目根目录重新生成：

```sh
go run -mod=mod github.com/google/wire/cmd/wire ./internal/bootstrap
```

`wire.go` 带 `wireinject` 标签，只供生成；普通构建使用 `wire_gen.go`。生成文件应由工具管理，学习注释放在声明、provider 和生命周期代码中。

## 沿调用链辨认完成点

- 投稿的 MySQL 事务保存稿件、关联资源和定时信息；Kafka 投递与向量同步是提交后的进程内任务，存在独立失败窗口。
- 定时意图、审核通过和转码就绪是不同事实。待审稿件到达计划时间，不会因此绕过人工审核。
- `tx.Manager` 只提供本地数据库事务；各参与仓储需要保留回调的 `context`，它不覆盖 Redis、对象存储或 Kafka。
- 消息 `MarkMessage` 是可提交标记，实际 offset 提交和故障重投另有时机；转码检查点和资源探测帮助恢复，不能推导为恰好执行一次。
- 缓存命中、失效和回填分别有并发条件。generation/版本比较保护特定竞争窗口，不能代替数据库与缓存之间的整体一致性设计。
- 配置快照指针原子发布后，嵌套对象仍须保持只读；管理端先发布内存、再写文件，写文件失败不自动回滚内存。

这些边界也是阅读错误分支的线索：先确定哪一步已经完成，再判断本次失败是否允许重试、需要补偿或只影响派生展示。

## 更多资料与验证

- [DDD 从零到读懂本项目](docs/DDD-从零到读懂本项目.md)：概念、实例与学习练习。
- [架构说明](docs/architecture.md)：上下文、依赖图、配置和生命周期。
- [架构审查记录](docs/ddd-review.md)：现有约束和待改进项。
- [接口导航](docs/openapi-md/index.md)与 [OpenAPI](docs/openapi.yaml)：协议字段和入口约定。
- [独立转码消费者](cmd/transcode-consumer/README.md)：媒体进程运行方式。
- [架构依赖测试](internal/application/shared/arch_deps_test.go)：分层与跨上下文导入约束。

Go 工具链版本见 `go.mod`（当前为 Go 1.24.9）。已有测试可在根目录执行：

```sh
go test ./...
```

阅读或修改注释时，优先解释规则理由、输入输出契约、失败边界与生命周期；避免逐行复述赋值和循环。业务行为变动时同步更新注释，并以实现和验证结果校对说明，尤其不要把迁移历史或未来演进方案写成当前已经具备的保证。
