# 迁移问题台账

本文件记录 DDD 重构过程中在原项目 `f_ccg_video_back` 发现的每一个 bug、设计不合理、
安全隐患、性能问题与隐性不一致，以及新仓库的处理方式。

条目格式见 [docs/refactor-spec.md](./docs/refactor-spec.md) 第 4 节。
修复原则：默认保持外部行为 1:1；只有当行为本身就是缺陷时才改变行为，并以 **【行为变更】** 醒目标注。

> 说明：权威基线本身含 42 个 modified + 29 个 untracked 的在途修复。下文的【行为变更】描述
> “缺陷版本 → 修复语义”的产品差异；若该修复已存在于权威工作树，DDD 版本是原样承接，并不构成
> 新旧当前工作树之间的额外漂移。

## 权威基线（原项目磁盘当前工作树）

- 冻结时间：2026-08-01（Asia/Shanghai）；环境为 Windows、Go 1.26.4，`go.mod` 声明 `go 1.24.9`。
- 原仓 HEAD：`481e4e35a3f2c3f6e4c6b617aa76a8cfe1152362`。
- 权威源码不是 HEAD 快照，而是原目录当时的**磁盘当前工作树**：43 个 tracked modified
  （其中 `AGENTS.md` 不在迁移范围）+ 29 个 untracked；迁移范围内合计 42 个 modified + 29 个 untracked。
- 当前范围共有 561 个 `.go` 文件（含 85 个 `_test.go`）与 45 个资产/文档条目，共 606 项；
  完整逐文件清单及目标落点见 `docs/migration-progress.md`。
- 旧版 D-015 曾把迁移源错误固定为干净 HEAD，并排除后续工作树改动；该决定已被撤销。
  AI/多模态向量、RBAC 菜单、封禁、Feed、River、站点统计、转码等全部当前改动均属本次交付范围。

为避免测试副作用触碰原目录，基线验证在原当前工作树的独立临时镜像中执行，完成后已删除镜像：

- `go build ./...`：通过，零错误。
- `go test ./...`：除下列一项外全部通过；最新新增的 AI、RBAC、社交、动态、MQ、River、搜索、
  封禁与 Feed 测试均通过。

| 包 | 用例 | 结果 | 归属 |
|---|---|---|---|
| `service/video_service` | `TestTripleAtomicSuccess` | FAIL：`triple: SQL logic error: no such column: open_id (1)` | **原项目既有失败**，不计入新项目账（见 FIND-001/FIND-005） |

原项目目录全程零写入：未修改、创建或删除文件，未 commit/stash；所有 Git 审计命令均使用
`--no-optional-locks`，构建与测试只在可删除镜像中运行。

---

## FIND-001 | 类别: bug

- 原位置: `service/video_service/triple_service_test.go:191` `TestTripleAtomicSuccess`
  （被测代码 `service/user_service/coin_service.go:104` `CoinService.ApplyChangeTx`）
- 问题: 用例在 SQLite 上执行三连（点赞+投币+收藏）事务时报
  `SQL logic error: no such column: open_id`。测试建表的 `users` 表按被测字段裁剪、不含 `open_id`，
  而硬币扣减语句 `Update("coin_count", …)` 竟然会连带写 `open_id` 列——
  这不是测试装置问题，而是 FIND-005 描述的生产缺陷在测试里露头。
- 触发场景/影响: 开工前在原项目执行 `go test ./service/video_service/` 即稳定复现，
  是唯一的基线失败项。它实际上是一个未被识别的生产 bug 信号。
- 新代码处理: 不动测试装置；由 FIND-005 修正
  `internal/infrastructure/persistence/po/user.go` 的 `normalizeOpenIDColumn` 后，
  硬币更新不再触碰 `open_id`，用例自然转绿，并作为 FIND-005 的回归守卫保留。
- 改变外部可观测行为: 否（本条为测试结论；行为变更见 FIND-005）。

## FIND-002 | 类别: 设计

- 原位置: `models/mysqlEntity/user_banner_default.go:1` `DefaultUserBannerID` /
  `DefaultUserBannerIDs` / `SystemDefaultUserBannerIDs`，被 `models/mysqlEntity/user.go:78`
  `User.AfterCreate` 调用
- 问题: 持久化实体包反向依赖全局配置（`global.Config()`），使数据模型层与配置装载耦合，
  单测必须先摆全局状态才能创建用户；也让「默认横幅 ID」这一业务策略藏在 GORM 钩子里不可见。
- 触发场景/影响: 任何直接构造 `mysqlEntity.User` 并落库的路径（含 CLI `-su`、测试）
  都隐式依赖全局配置是否已装载；未装载时静默回落常量 19。
- 新代码处理: `internal/infrastructure/persistence/po/user_banner_default.go` 改为
  `WithDefaultUserBannerID(ctx, id)` / `DefaultUserBannerIDFromContext(ctx)`，
  由仓储层从注入的 `*config.Provider` 解析后写入 context；PO 不再 import 配置。
  回落值仍为 19，与原实现一致。
- 改变外部可观测行为: 否。

## FIND-003 | 类别: 设计

- 原位置: `core/init_zap.go:161` `GinLogger`、`core/init_zap.go:195` `GinRecovery`
- 问题: 两个 gin 中间件定义完整但从未挂载——`router.buildEngine` 使用 `gin.Default()`
  （即 gin 自带 Logger/Recovery），自定义的 zap 版本是死代码；
  这导致「请求日志走 zap 落盘」的设计意图实际未生效。
- 触发场景/影响: 运行期请求日志只输出到 gin 默认 writer，不进入 zap 的分级日志文件；
  panic 由 gin 自带 Recovery 处理，不产生结构化 zap 记录。
- 新代码处理: 完整迁移到 `internal/interfaces/http/middlewares/zap_gin.go`（日志器改显式注入），
  **同样不挂载**，以保持引擎中间件链与原项目 1:1。是否启用留给后续产品决策。
- 改变外部可观测行为: 否。

## FIND-004 | 类别: 设计

- 原位置: `pkg/ctxutil/detached.go:8` `processDetachedTasks`（包级单例）
- 问题: 进程级 detached 任务组以包级可变单例实现，无法在测试中隔离；
  且 `StopDetachedTasks` 一旦调用即进程内不可恢复，任何跑过一次优雅退出的测试会污染同包后续用例。
- 触发场景/影响: 同包内先执行停止用例再执行提交任务的用例时，后者的任务会被静默拒绝。
- 新代码处理: `pkg/ctxutil/detached.go` 导出 `DetachedTaskGroup` 类型与 `NewDetachedTaskGroup()`，
  由 bootstrap 注入唯一实例；测试可各自构造独立实例。
- 改变外部可观测行为: 否。

## FIND-005 | 类别: bug（数据损坏）| **【行为变更】**

- 原位置: `models/mysqlEntity/user.go:60` `(*User).normalizeOpenIDColumn`
  （经 `BeforeSave` 触发；典型调用方 `service/user_service/coin_service.go:104`）
- 问题: `normalizeOpenIDColumn` 在 `BeforeSave` 中**无条件**执行
  `tx.Statement.SetColumn("open_id", nil)`。GORM 的 `SetColumn` 在 `Statement.Dest`
  为 `map[string]interface{}` 时是直接写入该 map，而 `db.Update("col", v)` /
  `db.Updates(map{...})` 的 `Dest` 正是这种 map。
  于是任何与 `open_id` 毫不相关的部分更新，都会被追加一列 `open_id = NULL`。
  钩子接收者是 `Model(&User{})` 传入的零值结构体，`u.OpenID` 恒为 `""`，
  因此提前 return 的守卫永远不生效。
- 触发场景/影响: 具体链路——用户任意一次硬币变动
  （`CoinService.ApplyChangeTx` → `tx.Model(&mysqlEntity.User{}).Where("id = ?", userID).Update("coin_count", afterCoins)`）
  生成的 SQL 变为 `UPDATE users SET coin_count=?, open_id=NULL WHERE id=?`。
  硬币变动发生在每日登录奖励、收藏奖励、投币/三连消费等高频路径上。
  后果：**OAuth 用户（QQ/Google/GitHub/LinuxDo/X）的 `open_id` 在首次硬币变动后被静默清空**，
  之后按 `open_id` 查找账号失效，第三方登录会走到「新用户」分支并可能创建重复账号，
  原账号（含等级、经验、硬币、投稿）与第三方身份脱钩。
  该缺陷在原项目中已经以 `TestTripleAtomicSuccess` 失败的形式暴露，但被当作测试问题。
- 新代码处理: `internal/infrastructure/persistence/po/user.go:58`
  `normalizeOpenIDColumn` 增加目标类型判定：
  - `Dest` 为 map 时，仅当 map 中**确实存在** `open_id` 键且其值为空串时，才改写为 `nil`；
    map 中没有该键则完全不介入（部分更新不再污染无关列）；
  - `Dest` 为结构体（`Create` / `Save` 全量写入）时保持原逻辑：
    空 `OpenID` 规范为 SQL `NULL`，以维持 `uk_users_open_id_source` 的「多行无 open_id」语义。
- 改变外部可观测行为: **是**。部分更新语句不再包含 `open_id = NULL`；
  OAuth 绑定不再被硬币变动清空。这是修正数据损坏，原行为本身即缺陷，
  符合规范第 4 节「仅行为本身是缺陷时才可改变行为」。

## FIND-006 | 类别: 设计

- 原位置: `service/user_service/enter.go:16` `UserService.isLogin` / `IsLogin`
- 问题: 该函数解析 Authorization 头判断登录态，但全项目除本文件外无任何调用方（死代码）；
  同时它把 HTTP 关注点（读取请求头）放进了 service 层。
- 触发场景/影响: 无运行期影响；属于分层污染与死代码。
- 新代码处理: 迁移为 `internal/interfaces/http/middlewares/is_login.go` 的
  `IsLogin(c *gin.Context, jwtConf *config.Jwt) bool`，
  JWT 密钥改显式注入；保持导出但同样无调用方（不丢弃任何原有能力）。
- 改变外部可观测行为: 否。

## FIND-007 | 类别: 设计

- 原位置: `service/user_service/creator_analytics_service.go:922`
  `CreatorAnalyticsService.Overview` → `UserService{}.GetFansCount(creatorID)`
- 问题: `GetFansCount` / `GetFocusCount` / `IsFocus` 内部固定使用 `context.TODO()`，
  丢弃了请求上下文；请求超时或客户端断开时这些查询无法被取消。
- 触发场景/影响: 创作中心总览与关注状态查询在请求已超时后仍会占用数据库连接。
- 新代码处理: `internal/application/identity/creator_analytics_service.go` 与
  `internal/infrastructure/persistence/identity/creator_query.go` 全部改为透传调用方 ctx。
- 改变外部可观测行为: 否（成功路径结果一致；超时路径提前释放资源）。

## FIND-008 | 类别: 设计

- 原位置: `service/user_service/creator_analytics_service.go:924`
  `Overview` 中 `if username == "" { return overview, nil }`
- 问题: 不可达分支——上一行的 `fetchCreatorUsername` 已在 `username == ""` 时返回
  `errors.New("用户不存在")`，函数早已 return。
- 触发场景/影响: 无运行期影响，属于误导性死代码。
- 新代码处理: 新实现只保留可达语义（用户名为空即返回「用户不存在」），不再保留死分支。
- 改变外部可观测行为: 否。

## FIND-009 | 类别: 设计

- 原位置: `pkg/middlewares/user_ban_check.go:57` `UserBanCheck`
- 问题: HTTP 中间件直接使用 `global.DB` 执行 `UPDATE users SET status = ...`
  完成「限时封禁到期自动解封」，业务写操作下沉到了 interfaces 层。
- 触发场景/影响: 无法单测；解封逻辑与封禁逻辑分散在两处，容易不一致。
- 新代码处理: 新增应用层用例 `BanService.ReleaseExpired(ctx, userID)`
  （更新状态 + write-through 状态缓存），中间件改为调用它。
- 改变外部可观测行为: 否。

## FIND-010 | 类别: 安全 | **【行为变更】**

- 原位置: `service/captcha_service/click_captcha.go:191` `buildClickTargets`
- 问题: 函数末尾遗留调试语句 `fmt.Println(targets)`，把点击验证码的**正确答案坐标**
  （每个目标的 index/x/y/width/height）打印到标准输出。
- 触发场景/影响: 每次生成点击验证码都会输出答案。标准输出通常被容器日志、
  systemd journal、日志采集链路收集，任何能读到服务日志的人（运维、日志平台使用者、
  日志泄露场景）都可以直接拿到验证码答案并绕过点击验证码，
  使「登录点击验证码」防护形同虚设。
- 新代码处理: `internal/infrastructure/captcha/click.go` 的 `buildClickTargets`
  删除该打印语句（其余逻辑逐行保持一致）。
- 改变外部可观测行为: **是**。标准输出不再出现验证码答案。
  原行为本身即安全缺陷，符合规范第 4 节允许改变行为的条件。

## FIND-011 | 类别: 设计

- 原位置: `service/captcha_service/click_captcha.go` `fetchCaptchaPayload` /
  `deleteCaptchaPayload`
- 问题: 两个函数在改用原子 `GETDEL`（`consumeCaptchaPayload`）后已无任何调用方，属死代码；
  其中 `deleteCaptchaPayload` 忽略 `Del` 的返回错误。
- 触发场景/影响: 无运行期影响。
- 新代码处理: `internal/infrastructure/captcha/store.go` 只保留实际在用的
  `store` / `consume` 两个方法，不再保留失效路径。
- 改变外部可观测行为: 否。

## FIND-012 | 类别: bug（潜在 panic）

- 原位置: `service/captcha_service/click_captcha.go:206` `getClickCaptchaTTL`、
  `getClickCaptchaPadding`、`service/captcha_service/slide_captcha.go` 的同类函数、
  `pkg/middlewares/click_captcha_verify.go:26`、`slide_captcha_verify.go:27`、
  `graphics_text_captcha_verify.go:27`、`email_captcha_verify.go:41`
- 问题: 这些位置读取 `global.Config().Site.Login.X` / `Site.Register.X` 时，
  只判空到 `Site` 一层，未判 `Site.Login` / `Site.Register` 子结构；
  而 `site.Site` 中 `Login`、`Register` 均为指针字段。
- 触发场景/影响: 配置文件缺少 `site.login` 或 `site.register` 段时，
  首次访问登录/注册/验证码接口即 nil 解引用 panic（被 gin Recovery 兜成 500）。
- 新代码处理: `internal/application/identity/captcha_service.go` 与
  `internal/infrastructure/captcha/*.go` 的开关与参数读取补齐指针判空，
  缺失时回落到与原实现相同的默认值（TTL 2 分钟 / padding 20 或 10 / 开关关闭）。
- 改变外部可观测行为: 否（正常配置下结果一致；错误配置下由 500 panic 变为按默认值放行）。

## FIND-013 | 类别: 设计

- 原位置: `global/global.go`、`core/init_zap.go` 以及生产代码中的 `zap.L()` 调用
- 问题: 配置、数据库、缓存、搜索客户端和日志器均由包级全局状态提供；业务代码可在任意位置取用，
  构造依赖不可见，测试之间还会互相污染。`zap.ReplaceGlobals` 又把日志生命周期藏在进程全局。
- 触发场景/影响: 任一用例脱离完整 `InitCore` 运行时都可能 nil 解引用；并行测试替换全局客户端或日志器会竞态，
  关闭日志器后仍调用 `zap.L()` 也无法表达资源所有权。
- 新代码处理: `internal/bootstrap` 用 Wire 组合根显式构造；application、infrastructure、interfaces
  的构造器均注入 `*zap.Logger`，nil 归一化为 Nop；全仓生产代码不再使用 `zap.L/S` 或
  `zap.ReplaceGlobals`。
- 改变外部可观测行为: 否。日志字段与级别保持；只是依赖和生命周期显式化。

## FIND-014 | 类别: 设计

- 原位置: `pkg/validate/enter.go` 的包级 `ut.Translator` 与 `init()`
- 问题: 校验翻译器在包加载时修改 Gin validator 的全局状态，无法控制初始化错误，也无法为测试创建隔离实例。
- 触发场景/影响: import 顺序决定翻译是否就绪；同进程内替换 Gin validator 会让包级 translator 与实际校验器不匹配。
- 新代码处理: `pkg/validate.New()` 构造 `*validate.Translator`，由 Wire 注入 middleware/handler；
  绑定错误只从 Gin Context 读取已注入实例，全仓无 `init()` 业务装配和包级 translator。
- 改变外部可观测行为: 否。`json---label` 拆分和中文错误文案有契约测试守卫。

## FIND-015 | 类别: 设计

- 原位置: `service/captcha_service/graphics_text_captcha.go` 对
  `base64Captcha.DefaultMemStore` 的直接使用
- 问题: 图形验证码共享第三方包级内存库，实例不可隔离，容量和过期状态也不属于应用生命周期。
- 触发场景/影响: 同一进程内的测试或多套应用实例会共享验证码 ID；关闭/重建应用不能回收该状态。
- 新代码处理: `internal/infrastructure/captcha.Graphic` 持有独立 `base64Captcha.Store`，由构造器创建并注入。
- 改变外部可观测行为: 否。验证码 ID、校验与过期语义保持。

## FIND-016 | 类别: 设计

- 原位置: `core/enter.go`、`flags/enter.go`、`main.go`
- 问题: 七类 CLI 任务夹在基础设施初始化过程里并从深层直接 `os.Exit`；任务依赖和退出码无法单测，
  迁移中还曾出现 flags 被解析但始终构造完整 HTTP App 的一致性缺口。
- 触发场景/影响: `-db` 等只需 MySQL 的命令可能被无关中间件阻塞；深层退出会跳过调用方 defer。
- 新代码处理: `internal/interfaces/cli.Executor` + 七个独立 Wire injector，按
  `db > vp > br > su > es > rs > rbac` 分派；仅 `cmd/server/main.go` 末端调用一次 `os.Exit`，
  每个任务只构造其真实前置依赖。
- 改变外部可观测行为: 否。flag、优先级、成功 0/失败 1 与初始化先后保持。

## FIND-017 | 类别: 设计（资源泄漏） | **【行为变更】**

- 原位置: `router/enter.go` `Run`
- 问题: `ListenAndServe` 异常时调用 `zap.Fatal`，直接终止进程，因而不会执行
  `core.GracefulStop`，Kafka、River、数据库和日志器来不及显式关闭。
- 触发场景/影响: 端口占用或监听地址非法会跳过全部停机逻辑，正在处理的后台任务被硬切断。
- 新代码处理: `internal/bootstrap.App.Serve()` 返回监听/关闭错误；main 记录非零退出码后仍调用
  `App.Stop`，并有“监听失败不会退出测试进程”的回归测试。
- 改变外部可观测行为: **是**。仍以退出码 1 结束，但会先完成显式停机，不再由 zap 直接 `os.Exit`。

## FIND-018 | 类别: bug（并发/一致性） | **【行为变更】**

- 原位置: `api/rbac_api/create_menu.go`、`update_menu.go` 的直接 GORM 写入路径
- 问题: 菜单存在性校验、父级校验、插入/移动排序和角色授权不在同一事务；只禁止“父级是自己”，
  没有禁止把节点挂到后代；并发写会产生重复或断裂的 `sort_order`，创建者也可能立即看不到新菜单。
- 触发场景/影响: 两个管理员同时插入同一位置、跨层拖动或把祖先挂到后代时，菜单树可能成环、排序冲突，
  且部分写成功后无法回滚。
- 新代码处理: `internal/application/rbac.MenuService` 与 persistence repositories 在事务中使用
  `FOR UPDATE`，检测完整祖先链，重建连续同层顺序，并原子直连创建者当前角色；新增同层 move/generate 路由与回滚测试。
- 改变外部可观测行为: **是**。非法环/跨层移动被拒绝，并发排序保持连续；原行为本身会破坏 RBAC 数据。

## FIND-019 | 类别: 安全 | **【行为变更】**

- 原位置: 新增能力 `service/ai_service/menu_generation.go` `parseMenuSuggestion`
- 问题: AI 返回内容属于不可信输入；若把任意 icon 字符串直接存入菜单并由前端渲染，可注入
  `<script>`、事件属性、`javascript:` 或 `foreignObject`。
- 触发场景/影响: 模型被提示注入或上游响应遭篡改时，管理员打开菜单即可触发存储型 XSS。
- 新代码处理: `internal/application/ai/menu_generation.go` 对 JSON 字段完整性、数据库长度、SVG 根、
  script/事件/外链/foreignObject 做白名单式校验，失败不落库。
- 改变外部可观测行为: **是**。不安全或超长的 AI 建议返回业务错误；这是必要的输入边界修复。

## FIND-020 | 类别: bug（并发/审计） | **【行为变更】**

- 原位置: `service/user_service/ban_service.go`、`service/cron_service/auto_unban_user.go`、
  `pkg/middlewares/user_ban_check.go`
- 问题: 手动/到期解封会新建“active”记录而不关闭原封禁记录；middleware 与 cron 可并发重复解封，
  最新封禁查询仍可能取到已结束记录。
- 触发场景/影响: 请求恰逢 cron 扫描时可写出重复审计记录，缓存与用户状态也可能被较旧操作覆盖。
- 新代码处理: ban aggregate 增加 `IsUnbanned`；应用服务在事务内锁定最新活动记录，条件更新用户状态并标记原记录；
  cron 与 middleware 共用幂等语义，成功后 write-through 状态缓存。
- 改变外部可观测行为: **是**。解封不再制造伪造的 active ban 记录，重复执行成为幂等。

## FIND-021 | 类别: bug（隐私） | **【行为变更】**

- 原位置: `api/social_api/focus_list.go`、`fans_list.go`
- 问题: 隐私开关对所有查看者一视同仁，用户关闭公开后连本人也无法查看自己的关注/粉丝；
  ID 解析还接受 0/负数，深页限制依赖字符串 context 值而可能误判本人。
- 触发场景/影响: 本人在隐私设置关闭时被锁在自己的关系列表之外；非法 ID 会落到后续查询并返回误导错误。
- 新代码处理: `internal/application/social.RelationService` 显式接收 viewer/owner，所有者绕过公开开关与前五页限制；
  非所有者仍严格受隐私与深页约束，handler 使用注入式身份值并校验正 ID。
- 改变外部可观测行为: **是**。本人可查看私有列表；他人权限不放宽。

## FIND-022 | 类别: 性能/设计

- 原位置: `api/social_api/focus_user.go`、`unfocus_user.go`
- 问题: 关注写事务成功后同步刷新两名用户的 ES 文档，把搜索系统可用性耦合进社交请求热路径；
  River 又会消费相同 binlog，形成重复写和显式 refresh。
- 触发场景/影响: ES 抖动会拉长关注接口延迟并产生噪声日志；每次关系变化至少重复索引两次。
- 新代码处理: 社交用例只提交关系与统计；搜索投影由 River/binlog 单一路径最终一致更新，
  `UserIndexer` 不再逐文档强制 refresh。
- 改变外部可观测行为: 否（以当前工作树为基线）；搜索关系字段仍最终一致，HTTP 成功不依赖 ES。

## FIND-023 | 类别: 一致性 | **【行为变更】**

- 原位置: `api/dynamic_api/list.go`、`service/dynamic_service/feed_service.go`
- 问题: handler 强制要求 `userId`，导致服务中“当前用户本人 + 关注作者”的聚合流能力无法访问；
  类型过滤、本人未发布视频与关注者仅已发布视频的边界也未形成统一查询契约。
- 触发场景/影响: 不指定作者时本应返回 following feed，却在参数绑定阶段直接失败。
- 新代码处理: `internal/application/dynamic.FeedService` 把 `authorID=0` 定义为关注聚合流，
  查询端口按 workType 仅视频/仅动态过滤，并用 owner 规则控制未发布视频；真实 SQLite 集成测试覆盖分页与总数。
- 改变外部可观测行为: **是**。省略 `userId` 的已登录请求现在返回聚合流；这是当前 API 设计意图的修复。

## FIND-024 | 类别: bug/性能 | **【行为变更】**

- 原位置: `service/video_service/feed_service.go`
- 问题: 观看历史与偏好查询错误被丢弃成空结果；两者串行执行；列表先 COUNT 再查数据，空第一页也多一次往返；
  加入作者 JOIN 后裸列名和热度表达式还会发生歧义。
- 触发场景/影响: context 已取消或 DB 失败时接口可能伪装成正常空推荐；首页延迟包含无必要串行查询和 COUNT。
- 新代码处理: `internal/application/video.FeedService` 并发读取历史/偏好并传播错误；
  persistence query 使用限定列名和作者 JOIN，第一页空集跳过 COUNT，其余页保持精确 total。
- 改变外部可观测行为: **是**。数据库/取消错误不再静默变成成功空列表；正常结果与 JSON 不变。

## FIND-025 | 类别: bug | **【行为变更】**

- 原位置: `service/video_service/detail_service.go` `dedupeResourceBriefs`
- 问题: 去重 key 同时包含 resolution 与 `isSource`，同一清晰度的源文件和派生文件会各返回一条。
- 触发场景/影响: 非标准源被归入标准档位时，详情响应出现两个同分辨率选项，播放器选择不确定。
- 新代码处理: `internal/domain/video.DedupeResourceBriefs` 只按 resolution 去重，并优先源文件、再按文件大小择优。
- 改变外部可观测行为: **是**。同一清晰度只返回一个确定性最佳资源。

## FIND-026 | 类别: bug（数据丢失） | **【行为变更】**

- 原位置: `service/river_service/sync.go` `doBulk`
- 问题: ES Bulk 响应含逐项错误时仅打印日志却返回 nil，随后保存 MySQL binlog 位点；
  注释中的 retry 也未实现。
- 触发场景/影响: mapping 冲突、429 或短暂 5xx 会永久漏索引文档，因为位点已经越过失败事件。
- 新代码处理: `internal/infrastructure/river` 使用独立 olivere/v7 客户端检查每个 item；
  408/425/429/5xx 指数退避重试，永久错误立即失败并禁止推进 checkpoint，关闭前强制 flush。
- 改变外部可观测行为: **是**。短暂错误会重试，永久错误会停同步而不是静默丢文档。

## FIND-027 | 类别: 安全/bug | **【行为变更】**

- 原位置: `service/mq_service/transcode_checkpoint.go`、`transcode_consumer.go` 的 DASH 复用/完成校验
- 问题: 只对 MPD 做一次 ffprobe/时长检查，没有确认 static manifest、全部 representation 文件存在且非空，
  也没有约束 BaseURL 必须留在输出目录内。
- 触发场景/影响: 截断上传或缺失分片仍可能被 checkpoint 当作可复用成功产物；恶意/损坏 MPD 的 `../` 或外链可越界探测。
- 新代码处理: `internal/infrastructure/mq/dash_validation.go` 限制 XML 大小与 15 秒预算，
  解析 ISO-8601 时长，拒绝外链/绝对路径/目录穿越，逐个验证 representation，并 probe 视频轨。
- 改变外部可观测行为: **是**。不完整或不安全的 DASH 产物会重编码/失败，不再作为成功资源发布。

## FIND-028 | 类别: bug | **【行为变更】**

- 原位置: `service/mq_service/transcode_progress.go` 的 pipeline 百分比聚合
- 问题: 一旦存在 pipeline，成功状态仍取各 pipeline 的旧平均值；最后一次 pipeline flush 丢失时，
  状态已经 succeeded 而 percent 仍小于 100。
- 触发场景/影响: 前端轮询看到“转码成功 96%”并可能一直保留进度 UI。
- 新代码处理: `internal/domain/media.AggregateProgressPercent` 以终态优先：succeeded 恒为 100，
  非终态才使用 pipeline 平均或旧算法 fallback，所有 snapshot/flush/fail 路径共用。
- 改变外部可观测行为: **是**。成功响应的 percent 保证为 100。

## FIND-029 | 类别: 一致性 | **【行为变更】**

- 原位置: `service/site_service/admin_stats_service.go` `AdminStats`
- 问题: “今日登录”卡片按登录记录去重，而日趋势只有站点 UV；两者口径不同却被前端并列展示，
  无法计算今日登录的真实环比。
- 触发场景/影响: 匿名流量增长会被误读为登录增长，运营判断偏差。
- 新代码处理: content-ops 查询端口按日 `COUNT(DISTINCT user_id)` 构造七天 `daily.loginUsers`，
  overview 今日值直接复用序列末位；OpenAPI 与管理文档同步。
- 改变外部可观测行为: **是**。响应新增当前基线要求的 `daily.loginUsers` 字段，原字段不变。

## FIND-030 | 类别: 性能

- 原位置: `api/user_api/user_info.go` `UserInfoView`
- 问题: 用户与 `UserConf` 使用 Preload 两次查询，随后关系计数和横幅解析继续串行；高频资料页有多次可并行往返。
- 触发场景/影响: 数据库/对象存储延迟叠加，用户资料接口尾延迟升高。
- 新代码处理: identity repository 用 GORM `Joins("UserConf")` 单查询装载，应用服务并发读取关系计数与 banner，
  并传播核心查询错误；查询次数测试固定为 1。
- 改变外部可观测行为: 否。

## FIND-031 | 类别: 性能/可用性 | **【行为变更】**

- 原位置: `conf/conf_mysql.go` `Mysql.GetDSN`
- 问题: MySQL DSN 没有连接超时，短 SQL 还要 prepare/execute/close 三次往返。
- 触发场景/影响: 数据库地址不可达时初始化可长时间挂起；大量简单查询增加网络往返。
- 新代码处理: `internal/infrastructure/config.Mysql.GetDSN` 保留全部原参数并追加
  `interpolateParams=true&timeout=3s`，测试锁定精确字符串。
- 改变外部可观测行为: **是**。不可达连接约 3 秒失败；参数化语义保持，utf8mb4 不触发驱动危险字符集限制。

## FIND-032 | 类别: 设计

- 原位置: `service/search_service/video_vector.go` 的 `vectorIndexMu/vectorIndexChecked` 与
  `video_vector_rebuild.go` 的包级 `videoVectorRebuild`
- 问题: 向量索引状态和重建任务是包级可变单例，测试无法隔离，也不能被 App 生命周期取消等待。
- 触发场景/影响: 测试或同进程多实例共享 running/failed/cache 状态；停机可能留下脱离所有者的 goroutine。
- 新代码处理: `VideoVectorIndex` 与 `VideoVectorRebuildService` 持有实例状态，DetachedTaskGroup 管理 6 小时任务；
  rebuild 捕获固定 VectorConfig，按 ID 游标每页 200 条预载全部关联数据，4 workers（上限 16）。
- 改变外部可观测行为: 否。status/rebuild HTTP 契约和索引 mapping 保持。

## FIND-033 | 类别: 性能（一致性守卫）

- 原位置: `service/search_service/video_vector_rebuild.go` `loadRebuildPage`
- 问题: 原当前实现刻意按页预载标签、分区、分 P；DDD 迁移中一度只分页 ID 后逐视频 `Load`，
  会把每页固定数次查询退化为约 `1 + 3N` 次，是迁移引入的 N+1 风险。
- 触发场景/影响: 200 条一页时产生数百次 SQL，长库重建可能耗时数小时并压垮连接池。
- 新代码处理: `VideoVectorRebuildSource.PublishedAfter` 一次返回预载后的完整 source，worker 不再回查 DB；
  SQLite 集成测试断言分页结果包含关联数据。
- 改变外部可观测行为: 否；恢复原当前工作树的批量语义。

## FIND-034 | 类别: 一致性（热更新） | **【行为变更】**

- 原位置: `service/ai_service` 的一次请求配置副本语义；迁移中涉及
  `internal/application/ai` 与 `internal/infrastructure/external/ark`
- 问题: DDD 初版在 application 校验后，Ark adapter 再次读取热更新 Provider；更新恰好夹在两步间时，
  会出现旧模型白名单/思考策略搭配新 endpoint、API key 或 timeout 的跨版本请求。
- 触发场景/影响: 同一合法请求可能偶发鉴权失败、发往意外端点，或使用与已校验模型不配套的配置。
- 新代码处理: application 在公开用例入口复制一次 AI 配置为自身 `RuntimeConfig`，随 text/image/video/asset invocation
  传给无 Provider 的 Ark adapter；提示词分析、补图循环和正式调用共用该快照，并有热更新夹点测试。
- 改变外部可观测行为: **是**。热更新只影响后续请求，正在执行的请求保持原实现的一致快照。
