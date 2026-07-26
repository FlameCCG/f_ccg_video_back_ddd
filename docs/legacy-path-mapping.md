# 旧路径 → 新结构 映射表

本仓库是 `f_ccg_video_back` 的 DDD 重构版。`docs/` 下从原项目继承的文档
（[project-architecture.md](./project-architecture.md)、
[transcode-dash-architecture.md](./transcode-dash-architecture.md)、
[ai_text_image_video_docs_2026-04-10.md](./ai_text_image_video_docs_2026-04-10.md)）
描述的**设计与行为在本仓库 1:1 保留**，但其中的**代码路径引用指向重构前的目录布局**。
按下表换算即可定位到新位置。

> 为什么不逐处改写：这三份文档共约 10 万字，含大量 `file.go:line` 级引用与叙述性上下文，
> 机械改写会产生错误路径并破坏行文。集中一张权威映射表更可靠。
> 接口契约类文档（[openapi.yaml](./openapi.yaml)、[openapi-md/](./openapi-md/)）
> 描述的是 HTTP 外部契约，本次重构保证 method+path 与请求/响应 JSON 完全一致，
> 因此**原样有效，无需换算**。

## 顶层布局

| 原路径 | 新位置 | 说明 |
|---|---|---|
| `main.go` | `cmd/server/main.go` | 仅解析 flags → 调注入器 → 运行 |
| `core/enter.go`、`core/graceful_stop.go` | `internal/bootstrap/app.go` | App 生命周期（Start / Serve / Stop） |
| `core/init_conf.go` | `internal/infrastructure/config/provider.go` | 配置装载 + 热更新 + 校验 |
| `core/init_zap.go` | `internal/infrastructure/logger/logger.go`（日志器）<br>`internal/interfaces/http/middlewares/zap_gin.go`（gin 中间件） | |
| `core/init_db.go` | `internal/infrastructure/persistence/mysql/mysql.go` | |
| `core/init_redis.go` | `internal/infrastructure/cache/redis.go` | |
| `core/init_mongo.go` | `internal/infrastructure/persistence/mongodb/mongo.go` | 聊天索引初始化下沉到 chat 上下文组件 |
| `core/init_es.go` | `internal/infrastructure/search/es.go` | |
| `core/init_casbin.go` | `internal/infrastructure/auth/casbin.go` | |
| `core/init_ip_db.go` | `internal/infrastructure/ipregion/{searcher.go,resolver.go}` | |
| `core/init_river.go` | `internal/infrastructure/river/`（River 组件） | |
| `core/init_transcode_consumer.go` | `internal/infrastructure/mq/`（转码消费者组件） | |
| `core/init_video_scheduler.go` | `internal/application/video/`（视频调度器组件） | |
| `global/` | **已消除**：配置改注入 `*config.Provider`，其余客户端由 wire 注入 | 见 docs/decisions.md D-003 |
| `conf/` | `internal/infrastructure/config/` | 包名 `conf` → `config`（D-001） |
| `conf/site/` | `internal/infrastructure/config/site/` | |
| `conf/third_login/` | `internal/infrastructure/config/thirdlogin/` | |
| `models/enum/` | `internal/domain/shared/enum/` | 共享内核（D-009） |
| `models/mysqlEntity/` | `internal/infrastructure/persistence/po/` | 表名/列名/tag 完全一致 |
| `models/mongoEntity/` | `internal/infrastructure/persistence/mongopo/` | |
| `router/` | `internal/interfaces/http/router/` | method+path 与原项目 1:1 |
| `pkg/middlewares/` | `internal/interfaces/http/middlewares/` | 有依赖的中间件改为 `*Middlewares` 方法 |
| `pkg/ratelimit/` | `internal/infrastructure/cache/ratelimit/` | |
| `pkg/mq/` | `internal/infrastructure/mq/` | |
| `pkg/recursion/` | `internal/domain/rbac/` | 菜单树构建归 rbac 领域 |
| `pkg/wsorigin/` | `internal/interfaces/ws/wsorigin/` | |
| `pkg/enter.go`（`CheckStorageTyped`） | `internal/infrastructure/storage/` | |
| `pkg/`（其余通用件） | `pkg/`（原样平移） | res / utils / validate / ctxutil / jwt / encrypt / casbins / constants / uploadcheck / coinutil / emailStore |
| `flags/` | `internal/interfaces/cli/`（选项与任务分派）<br>`internal/infrastructure/persistence/migration/`（迁移与种子导入） | |
| `flags/mappings/` | `internal/interfaces/cli/mappings/` | `go:embed` 随包移动 |

## HTTP 处理器（`api/*_api` → `internal/interfaces/http/handler/*`）

| 原包 | 新包 |
|---|---|
| `api/user_api`、`api/captcha_api` | `internal/interfaces/http/handler/identity` |
| `api/rbac_api` | `internal/interfaces/http/handler/rbac` |
| `api/video_api` | `internal/interfaces/http/handler/video` |
| `api/comment_api` | `internal/interfaces/http/handler/comment` |
| `api/dynamic_api` | `internal/interfaces/http/handler/dynamic` |
| `api/social_api` | `internal/interfaces/http/handler/social` |
| `api/chat_api` | `internal/interfaces/http/handler/chat` |
| `api/notification_api` | `internal/interfaces/http/handler/notification` |
| `api/banner_api`、`api/site_api` | `internal/interfaces/http/handler/contentops` |
| `api/ai_api` | `internal/interfaces/http/handler/ai` |
| `api/enter.go`（`api.App` 聚合器） | **已消除**：处理器由 wire 注入 `internal/interfaces/http/router.Router` |

## 业务服务（`service/*` → 三层）

每个原 service 包按职责拆到三层：**领域规则**进 `internal/domain/<ctx>`，
**用例编排与 DTO** 进 `internal/application/<ctx>`，
**数据访问与外部系统调用**进 `internal/infrastructure/**`。

| 原包 | 新位置 |
|---|---|
| `service/user_service` | `internal/{domain,application}/identity` + `internal/infrastructure/{persistence,cache}/identity` |
| `service/user_service/focus_service.go` | `internal/{domain,application}/social` + `internal/infrastructure/persistence/social` |
| `service/login_service`（含 5 家 OAuth 子包） | `internal/application/identity` + `internal/infrastructure/external/oauth` |
| `service/email_service` | `internal/infrastructure/external/email` |
| `service/captcha_service` | `internal/infrastructure/captcha` + `internal/application/identity/captcha_service.go` |
| `service/rbac_service` | `internal/{domain,application}/rbac` + `internal/infrastructure/persistence/rbac` |
| `service/video_service` | `internal/{domain,application}/video` + `internal/infrastructure/{persistence,cache}/video` + `internal/interfaces/ws/danmu` |
| `service/mq_service` | `internal/{domain,application}/media` + `internal/infrastructure/{mq,persistence/media,cache/media}` |
| `service/storage_service` | `internal/infrastructure/storage` |
| `service/transcode_cancel_service` | `internal/{domain,application}/media` + `internal/infrastructure/mq` |
| `service/comment_service` | `internal/{domain,application}/comment` + `internal/infrastructure/{persistence,cache}/comment` |
| `service/dynamic_service` | `internal/{domain,application}/dynamic` + `internal/infrastructure/{persistence,cache}/dynamic` |
| `service/notification_service` | `internal/{domain,application}/notification` + `internal/infrastructure/{persistence,cache}/notification` |
| `service/chat_service` | `internal/{domain,application}/chat` + `internal/infrastructure/persistence/chat` + `internal/interfaces/ws/chat` |
| `service/search_service`、`service/es_service` | `internal/{domain,application}/search` + `internal/infrastructure/{search,cache/search,persistence/search}` |
| `service/river_service` | `internal/infrastructure/river`（`rule` 子包路径不变） |
| `service/banner_service`、`service/site_service` | `internal/{domain,application}/contentops` + `internal/infrastructure/{persistence,cache}/contentops` |
| `service/ai_service` | `internal/{domain,application}/ai` + `internal/infrastructure/external/ark` |
| `service/cron_service` | `internal/infrastructure/scheduler` + `internal/application/platform` |

## 常见符号换算

| 原写法 | 新写法 |
|---|---|
| `global.Config()` | 注入 `*config.Provider` 后 `provider.Get()` |
| `global.SetConfig(c)` | `provider.Set(c)`（配合 `config.Clone`） |
| `global.ConfigPath` | `provider.Path()` |
| `global.DB` | 注入 `*gorm.DB`；仓储内用 `gormx.DB(ctx, r.db)` 以复用事务 |
| `global.RedisClient` | 注入 `*redis.Client` |
| `global.MongoDB` | 注入 `*mongo.Database` |
| `global.ESClient` / `global.ES8Client` | 注入 `*elastic.Client` / `*elasticsearch8.Client` |
| `global.CasbinEnforcer` | 注入 `*casbin.SyncedEnforcer` |
| `global.IPSearcher` | 注入 `ipUtils.RegionResolver`（实现 `ipregion.Resolver`） |
| `global.CaptchaStore` | 注入 `identity.GraphicCaptchaStore` |
| `ctxutil.GoDetached(...)` | 注入 `*ctxutil.DetachedTaskGroup` 后 `tasks.Go(...)` |
| `db.Transaction(func(tx *gorm.DB) …)` 跨模块传 tx | 注入 `tx.Manager` 后 `txm.Do(ctx, func(ctx) error …)` |
| `mysqlEntity.Xxx` | `po.Xxx`（infrastructure 内）；领域实体见 `internal/domain/<ctx>` |
