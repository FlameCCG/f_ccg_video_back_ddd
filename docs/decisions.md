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
- 决策：七个一次性任务各有独立 Wire 注入器，且只构造该任务实际需要的依赖：
  - `InitializeMigrateDBTask`（`-db`）：config、logger、MySQL；
  - `InitializeVideoPartitionsTask`（`-vp`）：config、logger、MySQL；
  - `InitializeBannerTask`（`-br`）：config、logger、MySQL、当前配置选中的存储驱动；
  - `InitializeSuperUserTask`（`-su`）：config、logger、MySQL、Casbin；
  - `InitializeESIndexTask`（`-es`）：config、logger、MySQL、Elasticsearch；
  - `InitializeResourceSyncTask`（`-rs`）：原命令执行点之前的完整基础设施，外加与服务启动
    共用注册逻辑的真实 HTTP Router；只用空 handler 壳枚举路由，不构造 App、Kafka 或后台组件；
  - `InitializeRBACInitTask`（`-rbac`）：原命令执行点之前的完整基础设施与 RBAC 初始化用例。
  `cmd/server/main.go` 严格按 `db > vp > br > su > es > rs > rbac` 选择唯一任务；
  任务用例只返回 `error`，最终退出由 `main` 统一映射为成功 0、失败 1，并保证 cleanup 可执行。
- 影响：各命令不会因无关中间件离线而失败；`-rs` 同步的是实际 189 条 Gin 路由，
  依赖范围、任务优先级、输出与退出语义均与原项目当前工作树一致。

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

## D-013 `infrastructure/config` 视为横切关注点，允许外层读取

- 背景：站点配置在运行期可热更新，被 40+ 处应用层与接口层逻辑读取
  （验证码开关、CORS 白名单、存储类型、三方登录、AI/邮件参数等）。
  若严格禁止 application 读 infrastructure，需要为每个上下文声明一大堆窄接口，
  收益极低而改动面极大。
- 决策：`internal/infrastructure/config` 作为**配置横切关注点**，
  允许 `application`、`interfaces`、`infrastructure` 三层读取（注入 `*config.Provider` 或窄配置结构体）；
  **`internal/domain/**` 一律禁止 import 它**（领域层零框架依赖，规则不放松）。
  规范第 7 节的 DI 验收项是「application/domain 层零 gin 依赖」，本决策不违反该项。
- 影响：无外部行为变化；领域层保持纯净可测。

## D-014 路由奇偶校验采用双侧 Gin 引擎实测

- 背景：验收要求「分别从新旧两边路由代码提取 method+path 全量清单并 diff，差异为零」。
  原项目目录只读，不能在其中新增测试程序来 dump `engine.Routes()`。
- 决策：原侧在仓外临时模块中 import 原当前工作树的 public router，构造真实 Gin engine，
  从 `engine.Routes()` 导出清单；该模块不在原目录生成任何文件，审计后立即删除。
  新侧在 `internal/interfaces/http/router` 内用 nil handler 构造 `*Router` 并调 `BuildEngine()`，
  同样读取 `engine.Routes()`，再与 `testdata/routes_golden.txt` 逐条比对。
  静态扫描仅作为数量交叉检查：187 处显式注册 + `/uploads` 静态目录的 GET/HEAD 共 189 条；
  其中本轮原工作树增量包含 RBAC 菜单 `generate`/`move` 与视频向量索引 `status`/`rebuild`
  四条管理端路由。最终清单与散列写入 report.md 附录。
- 影响：2026-08-01 原磁盘当前工作树与目标仓均为 189 条；两侧清单均来自 Gin 引擎实测，
  method/path 差异为零，目标侧任何后续漂移都会在 `go test` 中直接失败。

## D-015 撤销旧 HEAD 冻结，原磁盘当前工作树是唯一权威基线

- 背景：旧决策曾把迁移源冻结到开工时的 git HEAD，并把迁移期间出现的修改与未跟踪文件
  排除在交付范围外。这与规范「以原项目磁盘当前工作区状态为基线」冲突，也会漏掉已落盘的
  AI、多模态搜索、RBAC、封禁、Feed、转码、River 等有效增量。
- 决策：旧的 HEAD/`git archive` 冻结与“增量另行迁移”结论自 2026-08-01 起正式撤销。
  唯一权威输入是原仓 `E:\f_ccg_video_final\f_ccg_video_back` 当日磁盘当前工作树：
  - 原仓共 43 个 modified，其中 `AGENTS.md` 不在迁移范围；范围内为 **42 modified + 29 untracked**；
  - 范围内共 **561 个 Go 文件（其中 85 个测试）+ 45 个资产/文档 = 606 项**；
  - 所有已修改与未跟踪增量均须迁入目标仓并纳入行为、契约和测试验收，不再设置漂移例外。
  原仓继续绝对只读；只读审计可在隔离副本运行，不得在原目录生成、格式化、提交或清理文件。
- 影响：目标仓复刻完整的 2026-08-01 当前工作树，而不是旧 HEAD；迁移清单、路由金丝雀、
  报告和依赖对比都以 606 项基线为准。

## D-016 继承文档保留原文 + 集中式路径映射表

- 背景：规范要求迁移「docs/ 中仍适用的文档（路径引用更新为新结构）」。
  `docs/project-architecture.md`（40KB）、`transcode-dash-architecture.md`（27KB）、
  `ai_text_image_video_docs_2026-04-10.md`（42KB）含大量 `file.go:line` 级引用与叙述性上下文。
- 决策：
  - 接口契约类文档（`openapi.yaml` 334KB、`openapi-md/`）**原样复制**——
    本次重构保证 HTTP method+path 与请求/响应 JSON 完全一致，故其内容无需任何改动。
  - 三份架构/功能文档**保留原文**，在标题后插入统一提示，并新增
    `docs/legacy-path-mapping.md` 作为权威的「旧路径 → 新结构」换算表
    （覆盖顶层布局、全部 handler 包、全部 service 包、以及 `global.*` / `ctxutil.GoDetached` /
    `mysqlEntity.*` 等常见符号换算）。
  - 不做逐处机械改写：10 万字规模的机械替换会产生错误路径（如把
    `service/video_service/detail_service.go:761` 改成不存在的新文件行号）并破坏行文，
    可靠性低于一张集中映射表。
- 影响：文档所述设计/行为与新仓库一致；任何旧路径都能一步换算到新位置。

## D-017 领域层测试需要真实基础设施适配器时，测试上移到 infrastructure

- 背景：rbac 的「权限覆盖判定」（`PermissionEvaluator`）依赖路径匹配器端口，
  其语义必须与 Casbin 模型里的 `globMatch` 完全一致。用自制 stub 测会出现
  「测试通过而线上鉴权结论不同」的假绿，因此该测试需要注入
  `infrastructure/persistence/rbac.NewGlobPathMatcher()`（真实 casbin `util.GlobMatch`）。
- 决策：把这类「领域规则 + 真实适配器」的集成测试放到 **infrastructure 层**
  （`internal/infrastructure/persistence/rbac/permission_evaluator_test.go`），
  使 `internal/domain/**` 目录保持对 `internal/infrastructure` 的**零引用**，
  验收扫描无需为测试文件开例外。
- 影响：断言与覆盖范围不变（4 个用例全部保留），领域层依赖方向更干净。

## D-018 `pkg/validate` 使用注入式 Translator，取消 `init()` 与包级翻译器

- 背景：旧平移方案保留了包级 `ut.Translator` 与 `init()`，虽然错误文案正确，仍违反
  「无包级可变单例、无隐式装配」硬约束，也让翻译器的创建与请求使用点不可见。
- 决策：`validate.New()` 显式创建 `*validate.Translator`，注册既有中文翻译与
  `json---label` tag 解析；Wire 把同一实例注入 `router.Router`，Router 挂载其 middleware，
  middleware 再把实例放入请求上下文，handler 的统一错误响应从当前请求显式取得翻译器。
  包内不再声明可变 translator，也不再用 `init()` 装配。
- 影响：中文校验文案、字段 label 拆分、分号拼接及非校验错误的“请求参数错误”回退契约不变；
  初始化顺序可见、翻译器实例可注入测试，生产代码扫描中 `func init()` 与全局 translator 均为零。

## D-019 ProviderSet 按依赖方向划分，接口绑定留在组合根

- 背景：每个限界上下文需要可复用的 Wire provider 集，但若 application 为了声明
  `wire.Bind(domain.Port, infrastructure.Impl)` 而 import infrastructure，会形成
  `application → infrastructure` 反向依赖，破坏 `interfaces → application → domain`
  与 `infrastructure → domain/application port` 的硬方向。
- 决策：
  1. 每个 `internal/application/<ctx>` 导出自己的 `ProviderSet`，**只包含该应用包的构造器**；
  2. logger、MySQL、Mongo、Redis、ES、MQ、storage、auth、IPDB 等基础设施技术包各自导出
     `ProviderSet`，只聚合本技术包的 provider；
  3. `domain/application port → infrastructure implementation` 的 `wire.Bind`、仓储/缓存/外部适配器
     构造器，以及跨上下文 DTO 转换适配器，统一留在 `internal/bootstrap` 的 adapter/context sets；
  4. `wire.go` 只声明完整 App 与七个 CLI 注入器，生成的 `wire_gen.go` 提交但禁止手改。
- 影响：ProviderSet 可按上下文与技术能力复用，同时 application/domain 不反向依赖 infrastructure；
  所有具体实现选择集中在组合根，依赖图可由 `wire check` 在编译期验证。
