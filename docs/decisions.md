# 裁量决策记录

规范要求全程不向用户提问，所有裁量决策自主拍板并记录于此。按决策时间顺序追加。

---

## D-001 配置包命名为 `config`（`package config`）

- 背景：原 `conf` 包目录名与包名均为 `conf`，落到 `internal/infrastructure/config` 后
  出现「目录名 ≠ 包名」，会触发 lint 与阅读歧义。
- 决策：目录与包名统一为 `config`，同步把 16 个导出标识符引用（`conf.Config`、`conf.Jwt` 等）
  重写为 `config.*`。子包 `conf/site` → `config/site`（包名 `site` 不变），
  `conf/third_login` → `config/thirdlogin`（包名 `third_login` 保持不变以减少改动面）。
- 影响：仅编译期符号，配置文件 key（`mapstructure`/`json` tag）零改动，
  同一份 `config_dev.yaml` 免改动可启动。

## D-002 `pkg/` 内工具包沿用原包名（含 camelCase）

- 背景：原 `pkg/emailStore`、`pkg/utils/ipUtils`、`pkg/utils/dateUtils`、
  `pkg/utils/computerUtils`、`pkg/utils/markdownUtils`、`pkg/jwt`（`package jwts`）
  使用非惯用命名。
- 决策：保持原目录名与包名。规范要求「与业务无关的工具代码直接平移，把精力花在分层与领域建模上」，
  重命名会带来大面积无收益改动与回归风险。
- 影响：无外部可观测影响。

## D-003 全局配置改为注入式 `*config.Provider`

- 背景：原 `global.Config()` 为包级 `atomic.Pointer[conf.Config]`，被 60+ 处运行期读取，
  且管理端接口会在运行期写回配置（热更新）。规范禁止 global 包与包级可变单例。
- 决策：新增 `infrastructure/config.Provider`（内部持 `atomic.Pointer[Config]` + 配置文件路径），
  由 wire 创建唯一实例并注入。
  - 需要读运行期热更新配置的组件注入 `*config.Provider`；
  - 启动期一次性读取的基础客户端注入窄配置结构体（`ProvideMysqlConf` 等，见规范第 2 节）。
- 影响：热更新语义、校验逻辑、panic 行为与原 `core.InitConf` 完全一致。

## D-004 detached 任务组改为可注入类型

- 背景：`pkg/ctxutil` 原有包级单例 `processDetachedTasks` + 三个包级函数
  （`GoDetached`/`WaitDetachedTasks`/`StopDetachedTasks`），违反「无包级可变单例」。
- 决策：导出 `ctxutil.DetachedTaskGroup` 类型与 `NewDetachedTaskGroup()`，
  方法为 `Go` / `Wait` / `Stop`；由 bootstrap 创建唯一实例注入需要 fire-and-forget 的应用服务，
  `App.Stop` 负责取消并等待在途任务。
- 影响：行为与原实现逐行一致（同一套 mu/active/idle/rootCtx 语义）。

## D-005 归属地解析改为端口 + 适配器

- 背景：`pkg/utils/ipUtils.GetIpAddr` 直接读 `global.IPSearcher`。
- 决策：在 `pkg/utils/ipUtils` 声明 `RegionResolver` 端口（`Region(ip string) string`），
  实现落在 `infrastructure/ipregion.Resolver`；
  `GetIPAndAddr` / `GetClientIPAndAddr` 增加 `resolver` 形参。
  「内网」判定上移到 `GetClientIPAndAddr` 与 `Resolver.Region` 两处（两处判定一致），
  保证与原 `GetIpAddr` 的返回值序列完全相同。
- 影响：`resolver` 为 nil 时公网 IP 返回「未知地址」，与原 searcher 为 nil 时一致。

## D-006 `User.AfterCreate` 的默认横幅 ID 经 context 注入

- 背景：原 `models/mysqlEntity/user_banner_default.go` 让持久化对象反向依赖 `global.Config()`；
  `User.AfterCreate` 钩子据此创建 `UserConf` 与默认收藏夹。
- 决策：保留 GORM 钩子（用户创建路径有 8 条：邮箱注册、5 家 OAuth、CLI `-su`、测试），
  但默认横幅 ID 改为从 `tx.Statement.Context` 读取，由仓储层用注入的配置写入
  （`po.WithDefaultUserBannerID`）；缺失时回落 19，与原实现「配置为 nil 时回落 19」一致。
- 备选方案（未采用）：把钩子副作用上移到仓储 `Create`。会改变副作用触发点，
  且任何绕过仓储的创建路径都会静默丢失默认配置与默认收藏夹，风险更高。
- 影响：外部可观测行为不变。见 report.md FIND-002。

## D-007 保留未挂载的 `GinLogger` / `GinRecovery`

- 背景：原 `core/init_zap.go` 定义了这两个 gin 中间件，但 `router.buildEngine` 使用 `gin.Default()`，
  两者从未被挂载（死代码）。
- 决策：完整迁移到 `interfaces/http/middlewares/zap_gin.go`，日志器改为显式形参注入，
  同样不挂载——保持引擎中间件链与原项目 1:1。规范禁止悄悄丢弃任何代码。
- 影响：无。见 report.md FIND-003。

## D-008 CLI 运维任务使用独立注入器

- 背景：原 `core.InitCore` 把 CLI 运维任务夹在基础设施初始化中间
  （`InitDB` → `RunDBTasks` → `InitES` → `RunESTasks` → … → Casbin → `-rs`/`-rbac`），
  任务执行后 `os.Exit`。这意味着 `-db` 不需要 Redis/Mongo/ES 在线。
- 决策：为运维任务提供三个独立注入器，覆盖范围严格等于原执行点之前已初始化的组件：
  - `InitializeDBTasks`（config、logger、MySQL）——服务 `-db`/`-vp`/`-br`/`-su`；
  - `InitializeESTasks`（+ ES）——服务 `-es`；
  - `InitializeRBACTasks`（+ Redis、Mongo、IPDB、Casbin）——服务 `-rs`/`-rbac`。
  `cmd/server/main.go` 按原优先级 `db > vp > br > su > es > rs > rbac` 分派，退出码逐一对齐。
- 影响：CLI 对中间件的依赖范围与退出码与原项目一致。

## D-009 `enum` 作为共享内核置于 `domain/shared/enum`

- 背景：原 `models/enum` 被 domain / infrastructure / interfaces 三层共用。
- 决策：置于 `internal/domain/shared/enum` 作为共享内核（Shared Kernel）。
  规范禁止的是「领域层之间相互 import」，共享内核是 DDD 标准做法，不构成上下文耦合。
- 影响：无。

## D-010 跨仓储事务改用 tx.Manager + context 传递

- 背景：原项目通过在服务签名里直接传 `*gorm.DB` 实现跨模块共享事务
  （`CoinService.ApplyChangeTx(tx *gorm.DB, ...)` 被 `video_service` 三连流程调用），
  迫使调用方依赖 GORM。
- 决策：领域层声明 `internal/domain/shared/tx.Manager`；
  基础设施 `persistence/gormx.TxManager` 实现之，事务句柄随 context 传递
  （`gormx.WithTx` / `gormx.DB(ctx, root)`）。
  `Do` 检测 context 中已有事务时**直接复用、不开保存点**，语义等于原 `XxxTx` 方法。
- 影响：应用层与领域层零 GORM 依赖；三连等跨上下文原子性与原实现一致。

## D-011 读侧投影允许直读其它上下文的表

- 背景：创作中心总览需要「作者已发布视频的统计汇总」与「粉丝数」，
  原实现在 `user_service` 里直接查 `videos` 表与 `user_focus` 表并叠加 Redis 视频统计缓存。
- 决策：按 CQRS 约定，**读侧投影**允许直读其它上下文的表，实现放在消费方上下文的
  infrastructure（`persistence/identity/creator_query.go`）；
  **写侧**一律经拥有该聚合的上下文（视频统计写侧仍归 video，关注关系写侧仍归 social）。
  这样各上下文可独立编译、独立演进，也避免为一次 COUNT 引入跨上下文调用链。
- 影响：SQL 与 Redis key 与原实现完全一致；无外部行为变化。

## D-012 迁移顺序按依赖关系微调：notification 先于 identity 接线

- 背景：规范第 6.3 节建议顺序为 identity → … → notification，
  但 `identity` 的封禁流程需要向用户发系统通知（原 `user_service` 直接 import `notification_service`）。
- 决策：identity 声明 `BanNotifier` 端口后，紧接着迁移 notification 上下文，
  使 `*notification.Service` 直接满足该端口（`NotifyBanStatus` 方法），
  由 bootstrap 用 `wire.Bind` 绑定。其余顺序不变。
- 影响：无外部行为变化；避免为跨上下文依赖引入占位实现（规范禁止 stub）。
