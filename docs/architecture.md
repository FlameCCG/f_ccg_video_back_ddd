# 架构说明：从请求到后台任务

本文以**当前工作树代码**为准，解释这个 Go 视频站后端如何组织代码、保存数据、处理投稿与转码，以及运行时有哪些边界。它是按业务上下文拆包的**单体应用**：默认由 `cmd/server` 在同一进程内运行 HTTP、WebSocket 和后台组件，转码也可由 `cmd/transcode-consumer` 单独运行；MySQL、Redis、MongoDB、Kafka、Elasticsearch 和存储服务是外部依赖。目录分层不等于已拆成微服务。

| 你想先弄清什么 | 阅读入口 |
|---|---|
| 代码按哪些业务拆，四层分别做什么 | 第 1～3 节 |
| 为什么有实体、PO、DTO 三种形状 | 第 4～5 节 |
| 接口如何得到实现，后台如何启动/关闭 | 第 6～7 节 |
| 上传之后到底发生了什么 | 第 9 节 |
| 需求变化应该改哪里，如何解释设计取舍 | 第 10～12 节 |

建议先看[交互架构图](./diagrams/system-architecture.html)和下面的运行图，再沿[上传到可播放的链路](#9-从上传到可播放的链路)追一条请求。

```text
浏览器 / 管理端
    │ HTTP、WebSocket
    ▼
Gin 路由与中间件 → handler → application 用例 → domain 规则/端口
                                     │                 ▲
                                     └─ bootstrap 注入的基础设施实现 ─┘
                                        │
                   ┌────────┬─────────┼──────────┬─────────────┐
                   ▼        ▼         ▼          ▼             ▼
                 MySQL    Redis    MongoDB      Kafka      本地/MinIO
                   │                            │
                   └─ binlog → River → ES       └→ 转码消费者（同进程或独立进程）→ FFmpeg
```

`bootstrap` 是构造时的连接点，不是每个请求经过的网络服务。MySQL 承载主要业务记录，Redis 支持缓存、计数、调度、转码进度和上传协调，MongoDB 保存聊天会话与消息；Kafka 承载转码任务，River 从 MySQL binlog 同步部分数据到 Elasticsearch。搜索是异步读侧，可能落后于 MySQL。证据入口：[进程入口](../cmd/server/main.go)、[组合根](../internal/bootstrap/app.go)、[River](../internal/infrastructure/river/river.go)。

当前 [路由金丝雀](../internal/interfaces/http/router/testdata/routes_golden.txt) 有 **193 条** `method + path`，包含 `POST /v1/common/image/upload` 与 `/uploads` GET/HEAD。历史迁移时记录的 189 条是另一时点的快照；联调应以现行路由代码和金丝雀为准。[路由测试](../internal/interfaces/http/router/routes_test.go)校验注册清单，不能代替请求和响应的端到端验证。

## 1. 目录骨架

```
f_ccg_video_back_ddd/
├── cmd/server/main.go              # 解析 flags → 默认服务或七个 CLI 注入器 → 退出码
├── cmd/transcode-consumer/main.go  # 独立转码消费者入口，只启动转码及必要依赖
├── internal/
│   ├── bootstrap/                  # 组合根：wire.go(wireinject) + wire_gen.go + App 生命周期
│   ├── domain/
│   │   ├── shared/enum/            # 共享内核：跨上下文枚举（原 models/enum）
│   │   └── <ctx>/                  # 实体、值对象、业务规则、仓储接口
│   ├── application/<ctx>/          # 应用服务/用例、DTO（含读模型）、查询端口、事务编排、ProviderSet
│   ├── infrastructure/
│   │   ├── config/                 # Viper 加载/热更新与 Provider；数据结构在 internal/config
│   │   ├── logger/                 # zap 日志器
│   │   ├── persistence/
│   │   │   ├── mysql/              # GORM 连接池
│   │   │   ├── mongodb/            # Mongo 客户端
│   │   │   ├── po/                 # MySQL GORM 持久化对象
│   │   │   ├── mongopo/            # Mongo 文档结构
│   │   │   └── <ctx>/              # 各上下文仓储实现 + PO↔domain 转换
│   │   ├── cache/                  # Redis 客户端与限流
│   │   ├── search/                 # Elasticsearch 客户端与索引
│   │   ├── mq/                     # Kafka 驱动与消息契约
│   │   ├── storage/                # 本地/MinIO 存储驱动
│   │   ├── auth/                   # Casbin enforcer、JWT 适配
│   │   ├── external/               # 邮件、三方登录、火山引擎、eino 等外部服务
│   │   ├── river/                  # MySQL binlog → ES 同步引擎
│   │   ├── ipregion/               # ip2region 归属地解析
│   │   └── scheduler/              # cron 与视频调度
│   └── interfaces/
│       ├── http/handler/<ctx>/     # gin 处理器 + 请求结构体
│       ├── http/router/            # 路由装配（以当前金丝雀为准）
│       ├── http/middlewares/       # gin 中间件
│       ├── ws/                     # WebSocket 端点与 Origin 校验
│       └── cli/                    # 命令行选项、任务优先级与可测试 Executor（原 flags）
├── pkg/                            # 通用工具（res、utils、validate、ctxutil 等）
├── conf/rbac_model.conf            # Casbin 模型（运行期按相对路径读取，位置不变）
├── init_resource/                  # 初始化资产（分区 SQL、横幅图片）
└── docs/、config.example.yaml、docker-compose.yaml
```

`internal/config` 是配置**数据契约**，`internal/infrastructure/config` 是读取、监听和发布配置的**机制**；应用层经 `shared.ConfigReader/ConfigStore` 端口访问。`config_dev.yaml` 是可按示例创建的本地文件，不是仓库要求存在的固定资产。`gin.Context` 停在 HTTP 适配层，向用例传 `context.Context` 与显式参数。

## 2. 限界上下文映射

| 上下文 | 当前职责与代码入口 |
|---|---|
| `identity`、`rbac` | 账户、登录、封禁、积分；角色、菜单、权限与接口资源。见 `application/identity`、`application/rbac`。 |
| `video` | 投稿、审核、分区、详情、互动、弹幕、举报，以及分片上传用例、每日额度和过期上传清理。见 `application/video`。 |
| `media` | 转码管线编排、存储与取消的端口；FFmpeg、Kafka、存储驱动的具体实现仍在 `infrastructure`。见 `application/media`、`domain/media`。 |
| `comment`、`social`、`dynamic` | 评论及点赞、关注关系、动态及关注流。 |
| `chat`、`notification` | 聊天会话/消息、站内通知；聊天和弹幕的 WebSocket Hub 位于接口层。 |
| `search` | 搜索查询、索引和向量用例；River 是单独的基础设施组件。 |
| `contentops`、`ai` | 站点配置/统计/Banner；AI 对话、图片和视频生成。 |
| `platform`、`provisioning` | 定时维护用例；初始化和运维用例。Cron 调度器本身在 `infrastructure/scheduler`。 |

边界不等于“一个业务动作只进一个包”。例如投稿在 `video.PublishService` 中完成 DB 写入，然后经跨上下文端口派发到 `media.TranscodeDispatchService`；分片上传的 HTTP 用例在 `video.UploadService`，底层存储驱动在 `infrastructure/storage`。可顺着[投稿用例](../internal/application/video/publish_service.go)、[上传用例](../internal/application/video/upload_service.go)、[转码投递](../internal/application/media/dispatch_service.go)追踪。

## 3. 依赖方向与能验证的约束

```
interfaces ──▶ application ──▶ domain（规则与写侧仓储端口）
                  │                ▲
                  └─▶ 读侧/跨上下文端口
                               ▲  │
                               │  └── infrastructure 实现
                               └────── bootstrap 绑定与注入
```

- 领域层不应依赖 Gin/GORM/Redis/Sarama/Viper/Casbin 等实现；应用层不直接 import `internal/infrastructure`。现有[架构测试](../internal/application/shared/arch_deps_test.go)还检查反向分层依赖、领域/应用层直接导入外部业务上下文、列出的技术框架，以及应用层构造 HTTP 请求/客户端。这不是对所有间接依赖或业务归属的完整静态证明。
- 跨上下文通过应用端口协调，必要的 DTO 转换放在 `internal/bootstrap/adapters`；具体绑定见[跨上下文 ProviderSet](../internal/bootstrap/providers_cross_context.go)。
- `gin.Context` 止步 interfaces 层：向 application/domain 只传 `context.Context` 与 DTO；
  用户身份、客户端 IP 等在 interfaces 层提取后显式传参。
- 接口所有权归消费方：写侧仓储接口定义在 domain；读侧查询端口定义在 application。
- 构造与运行状态尽量显式注入，避免用包级可变单例隐藏生命周期；例如上传合并锁和完成标记现在是 `UploadService` 实例字段。
- 每个 `application/<ctx>.ProviderSet` 只列应用构造器；应用层不得为了 `wire.Bind`
  反向 import infrastructure。具体适配器选择与 interface binding 只出现在 bootstrap 组合根。
- 日志器经构造函数注入，HTTP 监听错误返回 `cmd/server` 决定退出码。**配置加载是例外**：当前 `infrastructure/config.Load` 对启动时读取、解析和必填项校验失败使用 `panic`；不能宣称所有启动错误都经 `error` 返回。
- `validate.New()` 生成注入式 Translator；Router middleware 把它传入请求上下文，
  handler 的统一错误响应从当前请求读取，不存在包级 translator 或校验器 `init()` 装配。

这里的 DDD 主要体现为业务边界、规则与端口的分离，并非每个上下文都拥有复杂聚合。CRUD 与查询可保持薄用例；遇到实际跨库一致性问题，也不能只靠分层图解释。

## 4. 写模型与读模型（CQRS 轻量拆分）

接口需要稳定的 JSON 契约，而视频详情等响应包含作者、标签、资源与统计数据，不适合直接把 GORM PO 暴露出去。本仓库主要区分两条路径：

- **写侧（命令）**：`interfaces → application → domain 实体/领域服务 → domain 仓储接口`，
  由 `infrastructure/persistence/<ctx>` 用 PO 落库，转换函数放在 infrastructure。
- **读侧（查询）**：`interfaces → application 查询用例 → application 查询端口`，
  由 `infrastructure/persistence/<ctx>` 直接把 PO 组装成 application 读模型 DTO；
  读模型定义对外 `json` 字段，具体字段以 DTO 与接口契约为准。

这叫**轻量 CQRS**：读写使用不同的模型和端口，但并没有为所有读请求建立独立数据库，也不是事件溯源。详情读取还会合并 Redis metadata 与计数；缓存未命中时回源 DB，并用 generation 防止失效后旧快照写回，见[DetailService](../internal/application/video/detail_service.go)。纯 CRUD 上下文可以保持薄领域层。

## 5. PO ↔ domain 转换约定

- PO 位于 `internal/infrastructure/persistence/po`（MySQL）与 `mongopo`（Mongo）；它们描述落库形状，不是领域对象，也不应被当成所有 API 的响应类型。
- 需要从写侧进入领域层时，由仓储显式转换。例如[视频转换器](../internal/infrastructure/persistence/video/converter.go)的 `toDomainVideo` / `toPOVideo`；读侧可直接从查询结果组装[应用读模型](../internal/application/video/dto.go)。转换是按具体用例做的，并非每个 PO 都有完全对称的双向映射。
- 旧代码兼容性要对表名、列名、索引、JSON 契约分别验证；不要仅凭“PO 与旧实体一致”推论所有行为兼容。

## 6. wire 装配图

注入器位于 `internal/bootstrap`：`wire.go`（`//go:build wireinject`）声明注入器，
`wire_gen.go` 是已提交的生成代码；修改 provider 或绑定后可用 `go run github.com/google/wire/cmd/wire ./internal/bootstrap` 重新生成。

```text
cli.Options
    │
    ▼
ConfigProviderSet
    ├── cli.ProvideConfigPath ─▶ config.Load ─▶ *config.Provider
    └── ProvideServer/Logger/MySQL/Redis/Mongo/ES/Kafka/River/Transcode/JWTConf
    │
    ▼
InfrastructureProviderSet
    └── 各技术包 ProviderSet：logger / MySQL / Redis / Mongo / ES / IPDB / auth / ...
    │
    ├─────────────────────────────────────────────────────────────────────────┐
    │                                                                         │
    ▼                                                                         ▼
application/<ctx>.ProviderSet                                  bootstrap context/adapter sets
（仅应用服务构造器）                               （仓储、缓存、外部适配器构造器 + wire.Bind）
    │                                                                         │
    └──────────────────────────────┬──────────────────────────────────────────┘
                                   ▼
                         CrossContextProviderSet
                    （跨上下文薄适配器与端口绑定）
                                   │
                                   ▼
                handler / middleware / Router / RuntimeComponents
                                   │
                                   ▼
                          NewApp ─▶ *App + cleanup
```

职责边界如下：

- application ProviderSet 只复用本上下文应用构造器，不知道任何基础设施实现；
- 基础设施技术包 ProviderSet 只导出本技术能力的 provider；
- `domain/application port → infrastructure implementation` 的 `wire.Bind` 与跨上下文适配器
  集中在 bootstrap，因此 domain/application 不会反向依赖 infrastructure；
- `NewInfrastructure` 强制完整服务按 logger → MySQL → ES → Redis → Mongo → IPDB → Casbin
  完成基础设施构建，`NewApp` 再聚合路由、后台组件和关闭句柄。

### 6.1 CLI 注入器

默认无任务 flag 时走 `InitializeApp`。单项任务按
`db > vp > br > su > es > rs > rbac` 选出一个注入器，成功返回 0、失败返回 1。
`-all` 优先于单项 flag，按 `db → vp → br → es → rs → rbac → su` 依次初始化全部资源及超级管理员，
每步完成后执行 cleanup，再构造下一步依赖；任一步失败立即停止并返回 1，全部成功返回 0，不启动 HTTP 服务。
`-all` 未追加账号密码时，默认超级管理员为 `ccg / 123456`；也可追加用户名和密码覆盖默认值。
只提供用户名时会在初始化任何资源前报错退出。单独使用 `-su` 时仍须显式提供账号密码。
不指定 `-f` 时默认读取 `config_dev.yaml`。项目根目录执行：

```sh
go run ./cmd/server -all
# 自定义超级管理员账号密码
go run ./cmd/server -all myadmin mypassword
```

`-all` 的 ES 步骤沿用 `-es` 的删除并重建搜索索引行为。

| flag | 注入器 | 最小依赖范围 |
|---|---|---|
| `-db` | `InitializeMigrateDBTask` | config + logger + MySQL |
| `-vp` | `InitializeVideoPartitionsTask` | config + logger + MySQL |
| `-br` | `InitializeBannerTask` | config + logger + MySQL + storage |
| `-su` | `InitializeSuperUserTask` | config + logger + MySQL + Casbin |
| `-es` | `InitializeESIndexTask` | config + logger + ES |
| `-rs` | `InitializeResourceSyncTask` | config + logger + MySQL + 真实 Router 路由清单 |
| `-rbac` | `InitializeRBACInitTask` | config + logger + MySQL + Casbin + RBAC 初始化用例 |

`-rs` 复用生产 Router 的注册代码并由 Gin 实测当前全部路由（金丝雀现为 193 条），但使用不会处理请求的空 handler 壳，
因此不构造完整 App、Kafka、对象存储或后台组件。各 Executor 只返回 `error`，不在用例内调用
`os.Exit`，从而保证 Wire cleanup 和日志刷盘可以执行。

### 6.2 配置快照与热更新

`internal/config` 定义配置树，`internal/infrastructure/config.Provider` 使用 Viper 读取 YAML/环境变量、fsnotify 监听配置所在目录，并以 `atomic.Pointer` 发布当前快照。配置加载分为面向长期服务的 `Load`（开启文件监听与完整必填校验）、面向 CLI 运维任务的 `LoadOnce`（加载静态快照、不启 fsnotify 监听、仅校验维护用基础项）以及面向转码节点的 `LoadTranscodeWorker`（松绑 Web 端 JWT 密钥要求）。加载或解析失败向上层返回 `error`，由入口决定退出策略，不再直接 panic。应用层通过 `shared.ConfigReader/ConfigStore` 端口访问；一次用例操作先 `Get()` 一份快照再读字段，避免同次操作跨版本读取。热更新合并短时间内的文件事件，配置文件被原子替换或删除后重建时继续监听；读取或校验失败保留上一份有效配置。管理端和默认横幅配置更新与热更新共用 Provider 锁：基于最新快照修改、校验 JWT 等配置、原子替换文件，成功后发布内存快照；写文件失败时保留旧快照和旧文件。见[Provider](../internal/infrastructure/config/provider.go)、[管理端配置用例](../internal/application/contentops/site_config_service.go)及[配置示例](../config.example.yaml)。

### 6.3 独立转码入口与节点角色

默认 `cmd/server` 仍装配转码消费者。设置 `transcode.localConsumerDisabled: true` 后，当前 HTTP 实例启动时跳过本机消费，但保留 Kafka 生产者，投稿仍会投递任务；修改此开关需重启当前实例。没有其他消费者时任务会排队。管理端更新时省略或传 `null` 保留当前开关值。

独立消费者在项目根目录执行：

```sh
go run ./cmd/transcode-consumer -f config_dev.yaml
```

它通过 `InitializeTranscodeWorker` 只构造转码需要的 MySQL、Redis、Kafka、本地/MinIO 存储和附属资源，不启动 HTTP、WebSocket、River、视频调度或 Cron，也不运行初始化任务。此入口始终承担转码角色，不受 `localConsumerDisabled` 影响。消费启动失败返回非零退出码；Ctrl+C / SIGTERM 触发停止消费、关闭生产者池并等待 detached 任务，再执行基础设施 cleanup。详见[组合根](../internal/bootstrap/providers_transcode.go)和[生命周期](../internal/bootstrap/transcode_worker.go)。

独立节点复用完整配置文件及其必填项，需启用 Kafka、安装 FFmpeg/ffprobe，并能访问 MySQL、Redis 和源文件存储。同一 `kafka.groupID` 内按 topic 分区分摊任务，并行度还受分区数与各实例 worker 数限制。跨机器时优先使用双方可访问的 MinIO；若用本地存储，必须让任务中的路径在消费节点也指向同一份文件。

Kafka 的 bootstrap 地址和 broker 宣告的对外地址都必须能从消费节点访问。当前 Compose 的对外 listener 示例为 `192.168.1.3:9094`，部署时替换为实际宿主机地址并重建 Kafka 容器；远程节点的 `kafka.brokers` 也应填写可达地址，不能直接照搬配置示例的 `localhost:9094`。

## 7. 生命周期与停序

默认 HTTP 服务的 `*App` 聚合 HTTP 服务、后台组件、detached 任务组与基础设施句柄。下列顺序针对 `cmd/server`；本机消费关闭时转码组件的启动会跳过消费，独立入口的生命周期见第 6.3 节。启动和关闭均由组合根显式控制：

1. `App.Start(ctx)` 先初始化聊天索引（`IndexInitializer`），再按序启动 River → 视频调度器 → 转码消费者 → cron。WebSocket Hub（Chat / Danmu）按首个连接懒启动，不在此阶段初始化；单个后台组件启动失败记录到注入日志器并继续，保持 HTTP 仍可提供服务的既有语义。
2. `App.Serve()` 启动 HTTP，并等待 SIGINT/SIGTERM 或监听错误；停止阶段首先执行 HTTP
   graceful shutdown，确保不再接收会产生新后台工作的请求。
3. HTTP 停止后，`App.Stop(ctx)` 保持原 `core.GracefulStop` 的相对停序：转码消费者 →
   视频调度器 → River → cron；随后关闭 Chat Hub → Danmu Hub，再停止并等待 detached 任务。
4. 后台工作全部退出后关闭 Mongo → MySQL → Redis → ES → IPDB，最后 Sync/关闭 Logger。

HTTP 监听/关闭错误返回入口并影响退出码；后台组件启动失败或停止失败仅记日志，`App.Start` 与 `App.Stop` 为无返回值方法。因此“HTTP 仍启动”只表示服务监听策略，**不保证转码、River、搜索等依赖能力可用**。入口保留 Wire `cleanup` 的 defer，正常停序仍由 `App.Stop` 显式执行。配置加载错误会返回到入口退出，见[配置 Provider](../internal/infrastructure/config/provider.go)。

## 8. 测试策略

- 用例层单测可手写 fake/stub 实现端口，直接注入构造函数；组合根另有专门测试。
- 已有测试使用 `glebarez/sqlite`、`miniredis`、`httptest` 等离线替身，不等于真实 MySQL、Redis、Kafka、MinIO 的集成行为已被覆盖。
- 依赖边界测试检查 application 不导入 infrastructure、domain 不导入 infrastructure/config，见[架构测试](../internal/application/shared/arch_deps_test.go)。
- 路由契约测试从 Gin `engine.Routes()` 取得全部 method+path，与
  `testdata/routes_golden.txt` 逐条比较；金丝雀同时覆盖 RBAC `generate`/`move`、
  视频向量 `status`/`rebuild` 与 `/uploads` GET/HEAD。

## 9. 从上传到可播放的链路

下面按时间顺序追一条投稿。它横跨 Redis、对象存储、MySQL 和 Kafka，**没有覆盖全部系统的分布式事务**；“投稿成功”只说明同步阶段成功，不能直接推论“已转码、已入搜索索引、可播放”。

1. **分片与秒传**：`video.UploadService` 校验文件哈希、分片编号和大小。普通用户在首片写入前、确认秒传前或完成上传前，按“北京时间自然日 + 不同源文件哈希”占用额度；同哈希重复请求不重复计数。Redis Lua 脚本负责原子判定，管理员免额度。Redis 故障时额度校验失败，服务不会默认放行。见[上传服务](../internal/application/video/upload_service.go)、[配额用例](../internal/application/video/upload_quota.go)、[Redis 脚本](../internal/infrastructure/cache/video/upload_quota.go)。
2. **完成上传**：先询问存储驱动最终对象是否存在，已存在就复用；否则核对分片集合并合并。进程内的“已完成”缓存不是对象存在的权威证据，状态查询仍探测驱动。这解释了为什么删除源对象后旧缓存不能继续构成秒传成功。见[上传服务](../internal/application/video/upload_service.go)。
3. **投稿入库**：`PublishService` 验证已完成源文件、分区、标签，并调用 `VideoRepository.Persist` 写视频、分 P 和初始资源。验证至入库期间持有按哈希排序的 Redis 锁，清理任务用同一把锁协调；多哈希排序避免相互等待。见[投稿用例](../internal/application/video/publish_service.go)、[Redis 哈希锁](../internal/infrastructure/cache/video/upload_guard.go)、[持久化实现](../internal/infrastructure/persistence/video/publish_repo.go)。
4. **异步派发与处理**：定时发布计划与视频状态在同一数据库事务内提交；入库后，detached 任务处理向量同步和 Kafka 转码投递；失败记日志，不会回滚已完成的投稿事务。消费者以有限 worker 池执行 `media.TranscodeService`，由 FFmpeg/ffprobe、断点、进度、产物上传和资源落库适配器配合完成。见[投稿用例](../internal/application/video/publish_service.go)、[消费组件](../internal/infrastructure/mq/transcode_consumer.go)、[转码服务](../internal/application/media/transcode_service.go)。
5. **重试与清理**：成功或视频已删除时会标记 Kafka 消息；永久错误成功写入 DLQ 后也会标记。可重试错误耗尽本轮尝试、停机取消或 rebalance 可能导致再投递，断点恢复不等于恰好一次。Cron 每 5 分钟清理过期分片和未投稿源文件，删源文件前查询 MySQL 资源引用，并尝试取得相同哈希锁。默认保留时间是未合并分片 2 小时、未投稿源文件 1 小时，可由站点配置覆盖。见[Kafka 消费循环](../internal/infrastructure/mq/kafka_consumer.go)、[清理用例](../internal/application/video/upload_cleanup.go)、[Cron](../internal/infrastructure/scheduler/cron.go)。

排障时按阶段看：存储里是否有完整源文件 → MySQL 是否有视频/资源引用 → 异步投递是否成功 → Kafka 消费进度/死信 → 转码产物与资源记录是否一致。详情接口还会合并 Redis 缓存与数据库读模型；DASH 资源可解析 MPD 展开 Representation。见[详情用例](../internal/application/video/detail_service.go)、[解析端口](../internal/application/video/dash_resolver.go)和 [HTTP 适配器](../internal/infrastructure/external/video/dash_resolver.go)。

## 10. 用三个方向把这套架构讲明白

### 10.1 按业务看：谁拥有规则？

`video` 负责稿件的投稿、状态与可见性，`media` 负责媒体任务和处理管线。一次投稿会用到两个上下文，并不说明应该合并它们：修改审核政策主要影响稿件业务；改变 FFmpeg 编码方式主要影响媒体处理。两种变化需要不同知识，也有不同失败边界。

限界上下文定义的是概念的责任范围，不是进程数。这里各上下文默认在同一服务进程中通过端口与薄适配器协作，转码还支持独立消费者入口；部分持久化适配器还访问共同业务表。因此应说“业务边界显式化”，不能宣称已经物理隔离数据库。具体交叉点见[跨上下文绑定](../internal/bootstrap/providers_cross_context.go)和[video/media 适配器](../internal/bootstrap/adapters/video.go)。

### 10.2 按代码看：谁依赖接口，谁提供实现？

以投稿为例，`PublishService` 的保存能力字段是 `domain.VideoRepository`。用例声明自己需要保存稿件，不需要知道数据库连接池、表名或 GORM 关联写法。基础设施仓储实现 `Persist`，bootstrap 选择该对象并传给应用构造器。

```text
编译时：PublishService → domain.VideoRepository
        GORM 仓储 → domain.VideoRepository / PublishDraft
        bootstrap → 构造器与具体实现

运行时：Handler → PublishService → 注入的 GORM 仓储 → MySQL
```

两条路径能同时成立。Go 接口由方法集满足，构造函数负责拿到对象，Wire 生成的是普通构造代码。依赖倒置不阻止访问数据库，它让业务流程不直接绑定数据库实现。换成 fake 可以验证流程；换真实实现则还需验证事务、错误和 ID 回填等契约。

### 10.3 按时间看：哪些工作完成了，哪些仍待执行？

```text
源对象已上传
    → 投稿校验
    → MySQL 同步事务提交
         ├─ 返回稿件 ID、业务状态
         └─ 进程内后台任务：向量同步 / Kafka 投递
                                              → 消费者处理
                                              → 产物与资源逐步保存
```

这是提交后的分叉。后台任务在应用方法返回前被安排，实际可能与响应并发；请求不等待它成功。MySQL 提交是一个明确完成点，启动 goroutine 不是持久化任务的完成点。

另外两条读链路也要独立判断：关键词索引由 MySQL binlog 经 River 派生，向量由应用侧维护；详情通过查询 DTO 与 Redis 元数据/计数组合。稿件已保存，不代表这些读模型在同一瞬间更新。源码入口：[投稿](../internal/application/video/publish_service.go)、[向量同步](../internal/application/video/vector_sync.go)、[关键词搜索](../internal/application/search/search_service.go)。

## 11. 新需求应该落在哪一层？

下面是定位方法，表中的“新需求”属于教学假设，不表示已经改动项目行为。先判断变化的是规则、流程、协议、查询形状还是外部技术。

| 新需求 | 应先检查哪里 | 为什么 | 还需连带检查什么 |
|---|---|---|---|
| 改私密、审核、定时的状态政策 | domain 的状态函数，再查 application 调用点 | 核心变化是业务条件与结论 | 历史状态、管理员审核、到期调度、缓存与索引失效 |
| 投稿多返回一个展示字段 | application DTO、查询端口与实现、HTTP 响应 | 展示形状不必进入写侧聚合 | 公开/预览/管理路径和缓存命中结果 |
| 用另一种对象存储 | 存储端口、基础设施实现、bootstrap 绑定 | 是能力实现变化 | 路径、合并、探测、删除和浏览器访问语义 |
| 普通用户上传额度改成按另一时区计算 | 额度用例与 Redis store 契约 | 时间窗口与原子去重共同决定正确性 | 跨日、多哈希并发、管理员豁免、旧 key 迁移 |
| 投稿保存成功必须能恢复未投递任务 | 投稿事务、持久任务意图、投递器与消费者 | 单纯增加 Kafka 重试无法覆盖进程退出 | 多实例领取、重复投递、故障状态、积压与人工处理 |

判断规则归属时，拿掉“Gin、GORM、Redis、Kafka”这些名字再读需求：剩下的允许条件可能是领域规则；为了拿到事实和完成目标的步骤是应用编排；SQL、Lua、协议序列化和命令参数是具体实现。这个判断法比按文件名猜层更有用。

## 12. 架构解决了什么，还有什么要另外设计？

| 问题 | 当前方案 | 仍需认识的边界 |
|---|---|---|
| Handler 难以阅读和验证 | 协议翻译留在接口层，流程移到应用服务 | 目录搬迁本身不能保证所有业务规则归属合理 |
| 业务流程绑定具体存储 | 消费方声明端口，基础设施实现，组合根接线 | 接口兼容不自动证明实现的行为兼容 |
| 写模型被页面字段撑大 | 写侧实体与读侧 DTO 分开 | 读侧仍必须有身份、可见性和缓存失效规则 |
| 投稿相关表只写成功一部分 | MySQL 本地事务保存稿件与关联数据 | 不覆盖上传对象、Redis、Kafka、ES |
| 耗时转码堵住请求 | 提交后异步派发，消费者受 worker 名额约束 | 默认同进程转码仍竞争资源，可独立部署消费者；入队前仍有漏投窗口 |
| 已完成任务被重新消费 | 对象/资源核验、检查点、资源唯一约束 | 重复内部步骤仍可能发生，未入队任务不能靠消费者恢复 |

面试可以先说：“这是一套按业务上下文组织的 DDD 分层单体，我用投稿说明边界：HTTP 负责入口，应用编排流程，领域表达状态与分区规则，仓储实现单库事务，bootstrap 接入具体依赖。转码与索引异步完成，所以我会分别解释它们的成功条件。”然后选一个失败窗口展开，避免把所有保证压成“用了事务，保证最终一致”。

如果讨论后续演进，可以提出持久化投递意图与 Outbox、基于测量拆出转码 worker、进一步收窄仓储端口等方案，但要说明它们需要新增机制与验证。当前实现、教学示例和演进建议应分别表达。
