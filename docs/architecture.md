# 架构说明（f_ccg_video_back_ddd）

本文件描述本仓库的 DDD 分层、限界上下文映射、依赖方向、PO↔domain 转换约定与 wire 装配图。
执行规范见 [refactor-spec.md](./refactor-spec.md)，进度见 [migration-progress.md](./migration-progress.md)，
迁移期发现的问题台账见仓库根目录 [report.md](../report.md)，裁量决策见 [decisions.md](./decisions.md)。

## 1. 目录骨架

```
f_ccg_video_back_ddd/
├── cmd/server/main.go              # 仅：解析 flags → 调 bootstrap 注入器 → 运行/等退出信号
├── internal/
│   ├── bootstrap/                  # 组合根：wire.go(wireinject) + wire_gen.go + App 生命周期
│   ├── domain/
│   │   ├── shared/enum/            # 共享内核：跨上下文枚举（原 models/enum）
│   │   └── <ctx>/                  # 实体、值对象、聚合、领域服务、领域事件、仓储接口（纯 Go）
│   ├── application/<ctx>/          # 应用服务/用例、DTO（含读模型）、查询端口、事务编排、ProviderSet
│   ├── infrastructure/
│   │   ├── config/                 # 配置结构体 + Provider（热更新）+ 窄化 provider
│   │   ├── logger/                 # zap 日志器
│   │   ├── persistence/
│   │   │   ├── mysql/              # GORM 连接池
│   │   │   ├── mongodb/            # Mongo 客户端
│   │   │   ├── po/                 # GORM 持久化对象（表/列/tag 与原 models/mysqlEntity 完全一致）
│   │   │   ├── mongopo/            # Mongo 文档结构（与原 models/mongoEntity 完全一致）
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
│       ├── http/router/            # 路由装配（method+path 与原项目 1:1）
│       ├── http/middlewares/       # gin 中间件
│       ├── ws/                     # WebSocket 端点与 Origin 校验
│       └── cli/                    # 命令行选项与运维任务（原 flags）
├── pkg/                            # 与业务无关的通用工具（res、utils、validate、ctxutil 等）
├── conf/rbac_model.conf            # Casbin 模型（运行期按相对路径读取，位置不变）
├── init_resource/                  # 初始化资产（分区 SQL、横幅图片）
└── docs/、config.example.yaml、config_dev.yaml、docker-compose.yaml
```

## 2. 限界上下文映射

| 上下文 | 职责 | 原代码来源 |
|---|---|---|
| identity | 用户、登录（密码/5 家 OAuth）、注册、邮箱验证、验证码、封禁、经验、硬币、登录记录 | `service/user_service`、`service/login_service`、`service/email_service`、`service/captcha_service`、`api/user_api`、`api/captcha_api` |
| rbac | 角色、菜单、权限、角色继承、接口资源同步 | `service/rbac_service`、`pkg/recursion`、`api/rbac_api` |
| video | 投稿、审核、分区、播放、点赞/投币/收藏、举报、回收站、弹幕 | `service/video_service`、`api/video_api` |
| media | 存储驱动 + 全部转码管线（分片、断点、看门狗、取消） | `service/mq_service`、`service/storage_service`、`service/transcode_cancel_service` |
| comment | 评论、评论点赞、置顶、创作者视角列表 | `service/comment_service`、`api/comment_api` |
| social | 关注/粉丝关系 | `service/user_service/focus_service.go`、`api/social_api` |
| dynamic | 动态、动态点赞、关注流、已读 | `service/dynamic_service`、`api/dynamic_api` |
| chat | 会话、消息、WebSocket 聊天 | `service/chat_service`、`api/chat_api` |
| notification | 全站通知与用户已读 | `service/notification_service`、`api/notification_api` |
| search | 搜索、建议、热词、向量、River 同步 | `service/search_service`、`service/es_service`、`service/river_service` |
| content-ops | 站点配置、站点统计、轮播图 | `service/site_service`、`service/banner_service`、`api/site_api`、`api/banner_api` |
| ai | AI 对话/图片/视频生成 | `service/ai_service`、`api/ai_api` |
| platform | cron 调度、CLI 运维任务 | `service/cron_service`、`flags` |

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

## 4. 写模型与读模型（CQRS 轻量拆分）

外部契约要求响应 JSON 字段与原项目完全一致，而原项目大量接口直接序列化 GORM 实体
或由服务层拼装的嵌套结构。为同时满足「1:1 响应」与「领域层纯净」，本仓库区分两条路径：

- **写侧（命令）**：`interfaces → application → domain 实体/领域服务 → domain 仓储接口`，
  由 `infrastructure/persistence/<ctx>` 用 PO 落库，转换函数放在 infrastructure。
- **读侧（查询）**：`interfaces → application 查询用例 → application 查询端口`，
  由 `infrastructure/persistence/<ctx>` 直接把 PO 组装成 application 读模型 DTO；
  读模型的 `json` tag 与原响应结构逐字段一致。

读模型不进入领域层，领域层只承载业务规则；纯 CRUD 上下文因此保持薄领域层（规范第 1 节允许）。

## 5. PO ↔ domain 转换约定

- PO 位于 `internal/infrastructure/persistence/po`（MySQL）与 `mongopo`（Mongo），
  结构体字段、`gorm` tag、`json` tag、`TableName()`、索引定义与原 `models/*` 完全一致。
- 每个上下文的仓储实现文件内提供 `toDomainXxx(po) *domain.Xxx` 与 `toPOXxx(*domain.Xxx) po.Xxx`，
  不引入反射，字段逐一显式映射，便于 diff 校对。
- PO 的 GORM 钩子保持原有语义；需要配置参与的钩子（如 `User.AfterCreate` 解析默认横幅 ID）
  改由仓储层经 `context` 注入具体值，PO 不再反向依赖全局配置。

## 6. wire 装配图

注入器位于 `internal/bootstrap`：`wire.go`（`//go:build wireinject`）声明注入器，
`wire_gen.go` 由 `go run github.com/google/wire/cmd/wire ./internal/bootstrap` 生成并提交进仓库。

```
cli.Options ─▶ cli.ProvideConfigPath ─▶ config.Load ─▶ *config.Provider
                                                          │
        ┌─────────────────────────────────────────────────┴──────────────────────────────┐
        │ 窄化 provider：ProvideServerConf / ProvideLoggerConf / ProvideMysqlConf /       │
        │ ProvideRedisConf / ProvideMongoConf / ProvideESConf / ProvideKafkaConf /        │
        │ ProvideRiverConf / ProvideTranscodeConf / ProvideJwtConf                        │
        └────────────────────────────────────────────────────────────────────────────────┘
                                       │
   logger.Provide ─▶ mysql.Provide ─▶ search.Provide ─▶ cache.Provide ─▶ mongodb.Provide
        ─▶ ipregion.Provide ─▶ auth.NewEnforcer ─▶ NewInfrastructure
                                       │
        infrastructure/persistence/<ctx> 仓储实现 ──(wire.Bind)──▶ domain 仓储接口
                                       │
        application/<ctx>.ProviderSet（应用服务 + wire.Bind 端口绑定）
                                       │
        interfaces/http/handler/<ctx> ─▶ interfaces/http/router.New ─▶ NewApp ─▶ *App + cleanup
```

`NewInfrastructure` 的形参顺序刻意等于原 `core.InitCore` 的初始化次序
（logger → MySQL → ES → Redis → Mongo → IPDB → Casbin），使 wire 生成的构建顺序与原项目一致；
`NewApp` 把 `*Infrastructure` 放在首位，保证基础设施先于仓储/处理器构建。

## 7. 生命周期与停序

`*App` 聚合 HTTP 服务、后台组件（Kafka 转码消费者、视频调度器、River、cron）与基础设施句柄。

- `App.Start(ctx)`：按原 `core.InitCore` 尾部顺序启动后台组件
  （River → 视频调度器 → 转码消费者 → cron），单个失败只记录日志、不阻断 HTTP。
- `App.Serve()`：等价原 `router.Run()`——监听、等待 SIGINT/SIGTERM、HTTP 优雅关闭；
  `ListenAndServe` 异常时 `zap.Fatal` 直接退出（与原项目一致）。
- `App.Stop(ctx)`：等价原 `core.GracefulStop()`——
  转码消费者 → 视频调度器 → River → cron → detached 任务 → Mongo → MySQL → Redis → ES → IPDB → Logger。
- wire 的 `cleanup` 只做纯资源关闭（注入中途失败时回收），不承载停序逻辑。

## 8. 测试策略

- 测试不走 wire：单测手写 fake/stub 实现 domain 接口或 application 查询端口，直接注入构造函数。
- 离线策略与原项目一致：`glebarez/sqlite` 内存/临时库、`miniredis`、`httptest`，不依赖真实中间件。
- 原先依赖 `global` 全局状态的测试，一律改造为注入式。
