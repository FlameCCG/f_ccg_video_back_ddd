# 架构说明（f_ccg_video_back_ddd）

本文件描述本仓库的 DDD 分层、限界上下文映射、依赖方向、PO↔domain 转换约定与 wire 装配图。

HTTP 契约包含 189 条路由（187 条显式注册 + `/uploads` 的 GET/HEAD）。

## 1. 目录骨架

```
f_ccg_video_back_ddd/
├── cmd/server/main.go              # 解析 flags → 默认服务或七个 CLI 注入器 → 统一退出码
├── internal/
│   ├── bootstrap/                  # 组合根：wire.go(wireinject) + wire_gen.go + App 生命周期
│   ├── domain/
│   │   ├── shared/enum/            # 共享内核：跨上下文枚举
│   │   └── <ctx>/                  # 实体、值对象、聚合、领域服务、领域事件、仓储接口（纯 Go）
│   ├── application/<ctx>/          # 应用服务/用例、DTO（含读模型）、查询端口、事务编排、ProviderSet
│   ├── infrastructure/
│   │   ├── config/                 # 配置结构体 + Provider（热更新）+ 窄化 provider
│   │   ├── logger/                 # zap 日志器
│   │   ├── persistence/
│   │   │   ├── mysql/              # GORM 连接池
│   │   │   ├── mongodb/            # Mongo 客户端
│   │   │   ├── po/                 # GORM 持久化对象
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
│       ├── http/router/            # 路由装配（method + path 由路由测试验证）
│       ├── http/middlewares/       # gin 中间件
│       ├── ws/                     # WebSocket 端点与 Origin 校验
│       └── cli/                    # 命令行选项、任务优先级与可测试 Executor
├── pkg/                            # 与业务无关的通用工具（res、utils、validate、ctxutil 等）
├── conf/rbac_model.conf            # Casbin 模型（运行期按相对路径读取，位置不变）
├── init_resource/                  # 初始化资产（分区 SQL、横幅图片）
└── docs/、config.example.yaml、config_dev.yaml、docker-compose.yaml
```

## 2. 限界上下文映射

| 上下文 | 职责 |
|---|---|
| identity | 用户、登录（密码/5 家 OAuth）、注册、邮箱验证、验证码、封禁、经验、硬币、登录记录 |
| rbac | 角色、菜单、权限、角色继承、接口资源同步 |
| video | 投稿、审核、分区、播放、点赞/投币/收藏、举报、回收站、弹幕 |
| media | 存储驱动 + 全部转码管线（分片、断点、看门狗、取消） |
| comment | 评论、评论点赞、置顶、创作者视角列表 |
| social | 关注/粉丝关系 |
| dynamic | 动态、动态点赞、关注流、已读 |
| chat | 会话、消息、WebSocket 聊天 |
| notification | 全站通知与用户已读 |
| search | 搜索、建议、热词、向量、River 同步 |
| content-ops | 站点配置、站点统计、轮播图 |
| ai | AI 对话/图片/视频生成 |
| platform | cron 调度、CLI 运维任务 |

## 3. 依赖方向（硬约束）

```
interfaces ──▶ application ──▶ domain ◀── infrastructure
     │                                        ▲
     └────────────────────────────────────────┘（仅通过 bootstrap 注入的适配器/端口）
```

- 领域层零框架依赖：不 import gin / gorm / redis / sarama / viper / casbin。
- 领域层之间不得相互 import；跨上下文在应用层协调，或依赖对方应用服务接口 / 领域事件。
- `gin.Context` 止步 interfaces 层：向 application/domain 只传 `context.Context` 与 DTO；
  用户身份、客户端 IP 等在 interfaces 层提取后显式传参。
- 接口所有权归消费方：写侧仓储接口定义在 domain；读侧查询端口定义在 application。
- 禁止 global 包、service locator、包级可变单例、`init()` 里做业务装配。
- 每个 `application/<ctx>.ProviderSet` 只列应用构造器；应用层不得为了 `wire.Bind`
  反向 import infrastructure。具体适配器选择与 interface binding 只出现在 bootstrap 组合根。
- 日志器经构造函数显式注入；禁止 `zap.L()`/`zap.S()`、`zap.ReplaceGlobals` 与 `zap.Fatal`。
  启动/运行错误逐层返回到 `cmd/server`，由入口统一写 stderr、执行 cleanup 并决定退出码。
- `validate.New()` 生成注入式 Translator；Router middleware 把它传入请求上下文，
  handler 的统一错误响应从当前请求读取，不存在包级 translator 或校验器 `init()` 装配。

## 4. 写模型与读模型（CQRS 轻量拆分）

响应包含 GORM 实体字段及服务层组装的嵌套结构。为保证接口契约稳定并保持领域层的依赖边界，本仓库区分两条路径：

- **写侧（命令）**：`interfaces → application → domain 实体/领域服务 → domain 仓储接口`，
  由 `infrastructure/persistence/<ctx>` 用 PO 落库，转换函数放在 infrastructure。
- **读侧（查询）**：`interfaces → application 查询用例 → application 查询端口`，
  由 `infrastructure/persistence/<ctx>` 直接把 PO 组装成 application 读模型 DTO；
  读模型的 `json` tag 定义响应字段契约。

读模型不进入领域层，领域层只承载业务规则；纯 CRUD 上下文因此保持薄领域层。

## 5. PO ↔ domain 转换约定

- PO 位于 `internal/infrastructure/persistence/po`（MySQL）与 `mongopo`（Mongo），
  结构体字段、`gorm` tag、`json` tag、`TableName()` 与索引共同定义持久化及序列化契约。
- 每个上下文的仓储实现文件内提供 `toDomainXxx(po) *domain.Xxx` 与 `toPOXxx(*domain.Xxx) po.Xxx`，
  不引入反射，字段逐一显式映射，便于核对读写行为。
- PO 的 GORM 钩子参与持久化操作；需要配置参与的钩子（如 `User.AfterCreate` 解析默认横幅 ID）
  由仓储层经 `context` 注入具体值，PO 不反向依赖全局配置。

## 6. wire 装配图

注入器位于 `internal/bootstrap`：`wire.go`（`//go:build wireinject`）声明注入器，
`wire_gen.go` 由 `go run github.com/google/wire/cmd/wire ./internal/bootstrap` 生成并提交进仓库。

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

默认无任务 flag 时走 `InitializeApp`。一次性任务严格按
`db > vp > br > su > es > rs > rbac` 选出一个注入器，成功返回 0、失败返回 1：

| flag | 注入器 | 最小依赖范围 |
|---|---|---|
| `-db` | `InitializeMigrateDBTask` | config + logger + MySQL |
| `-vp` | `InitializeVideoPartitionsTask` | config + logger + MySQL |
| `-br` | `InitializeBannerTask` | config + logger + MySQL + storage |
| `-su` | `InitializeSuperUserTask` | config + logger + MySQL + Casbin |
| `-es` | `InitializeESIndexTask` | config + logger + MySQL + ES |
| `-rs` | `InitializeResourceSyncTask` | 完整基础设施 + 真实 Router 路由清单 |
| `-rbac` | `InitializeRBACInitTask` | 完整基础设施 + RBAC 初始化用例 |

`-rs` 复用生产 Router 的注册代码并由 Gin 实测 189 条路由，但使用不会处理请求的空 handler 壳，
因此不构造完整 App、Kafka、对象存储或后台组件。各 Executor 只返回 `error`，不在用例内调用
`os.Exit`，从而保证 Wire cleanup 和日志刷盘可以执行。

## 7. 生命周期与停序

`*App` 聚合 HTTP 服务、后台组件、detached 任务组与基础设施句柄。启动和关闭均由组合根显式控制：

1. `App.Start(ctx)` 启动 River → 视频调度器 → 转码消费者 → cron → Chat Hub → Danmu Hub；
   单个后台组件启动失败记录到注入日志器并继续，保持 HTTP 仍可提供服务的既有语义。
2. `App.Serve()` 启动 HTTP，并等待 SIGINT/SIGTERM 或监听错误；停止阶段首先执行 HTTP
   graceful shutdown，确保不再接收会产生新后台工作的请求。
3. HTTP 停止后，`App.Stop(ctx)` 保持原 `core.GracefulStop` 的相对停序：转码消费者 →
   视频调度器 → River → cron；随后关闭 Chat Hub → Danmu Hub，再停止并等待 detached 任务。
4. 后台工作全部退出后关闭 Mongo → MySQL → Redis → ES → IPDB，最后 Sync/关闭 Logger。

监听或关闭错误不调用 `zap.Fatal`，而是返回组合根统一决定非零退出码；全程使用构造函数注入的
logger，不读写 Zap 全局 logger。Wire `cleanup` 只负责注入中途失败和入口 defer 的幂等资源回收，
正常停序仍以 `App.Stop` 为权威。

## 8. 测试策略

- 测试不走 wire：单测手写 fake/stub 实现 domain 接口或 application 查询端口，直接注入构造函数。
- 离线测试使用：`glebarez/sqlite` 内存/临时库、`miniredis`、`httptest`，不依赖真实中间件。
- 测试通过构造函数注入配置和依赖。
- 路由契约测试从 Gin `engine.Routes()` 取得目标侧 189 条 method+path，与
  `testdata/routes_golden.txt` 逐条比较；金丝雀同时覆盖 RBAC `generate`/`move`、
  视频向量 `status`/`rebuild` 与 `/uploads` GET/HEAD。
