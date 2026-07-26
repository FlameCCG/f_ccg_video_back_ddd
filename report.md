# 迁移问题台账

本文件记录 DDD 重构过程中在原项目 `f_ccg_video_back` 发现的每一个 bug、设计不合理、
安全隐患、性能问题与隐性不一致，以及新仓库的处理方式。

条目格式见 [docs/refactor-spec.md](./docs/refactor-spec.md) 第 4 节。
修复原则：默认保持外部行为 1:1；只有当行为本身就是缺陷时才改变行为，并以 **【行为变更】** 醒目标注。

## 基线（开工前在原项目实测）

- 环境：Windows 10、Go 1.26.4（`go.mod` 声明 `go 1.24.9`）、工作区干净（`git status --porcelain` 为空）。
- `go build ./...`：通过，零错误。
- `go test ./...`：除下列一项外全部通过。

| 包 | 用例 | 结果 | 归属 |
|---|---|---|---|
| `service/video_service` | `TestTripleAtomicSuccess` | FAIL：`triple: SQL logic error: no such column: open_id (1)` | **原项目既有失败**，不计入新项目账（见 FIND-001） |

其余包结果：`api/captcha_api`、`api/rbac_api`、`api/site_api`、`api/user_api`、`api/video_api`、
`conf`、`core`、`flags`、`global`、`pkg/ctxutil`、`pkg/jwt`、`pkg/middlewares`、`router`、
`service/chat_service`、`service/cron_service`、`service/email_service`、`service/login_service`、
`service/login_service/x_service`、`service/mq_service`、`service/river_service`、
`service/river_service/rule`、`service/search_service`、`service/storage_service`、
`service/transcode_cancel_service`、`service/user_service` 全部 ok；其余包无测试文件。

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
