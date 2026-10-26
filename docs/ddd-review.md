# DDD 架构审查与改进

审查日期：2026-10-21。基线：`1011c4b`。工作分支：`refactor/ddd-boundaries-and-invariants`。

## 结论

这个项目已经有实质性的 DDD 实践，适合称为**按业务上下文划分、采用 DDD 的分层单体**。它并非仅把传统代码改成四个目录：已有领域规则、读写模型分离、消费方端口、PO 转换和组合根。但是，改动前仍有业务写入绕过领域行为、技术实现藏在应用层、上下文直接耦合等问题，不能称为所有聚合都严格封装的富领域模型。

DDD 没有“纯正认证”或必须全部使用的模式清单。是否拆成微服务、是否使用事件溯源、是否每个结构体都有方法、是否存在 JSON tag，都不能单独作为合格标准。更有意义的判断是：业务语言是否一致、模型由谁拥有、规则能否保护不变量、事务和集成边界是否明确。

本次审查覆盖 domain/application 的生产依赖，沿改稿、投稿、审核、定时发布、投币/三连、验证码和 AI 菜单生成追踪了主要写入路径。没有用代码审查替代与业务人员确认限界上下文和审核政策。

## 已有基础

| 方面 | 代码依据 | 判断 |
|---|---|---|
| 按业务划分模块 | `internal/{domain,application}/{video,media,identity,rbac,...}` | 有清晰的上下文候选边界，部署形态仍是单体 |
| 纯领域规则 | `domain/video/status_resolve.go`、`domain/identity/exp.go` | 状态与经验规则可以独立测试，不需要数据库 |
| 依赖倒置 | 各 application 端口、`bootstrap/providers_cross_context.go` | 技术实现由组合根选择 |
| 读写模型分离 | `application/video/dto.go`、`infrastructure/persistence/video/converter.go` | 查询可以使用投影，不必加载整个写侧聚合 |
| 事务端口 | `domain/shared/tx`、`infrastructure/persistence/gormx` | 用例表达事务范围，GORM 句柄由适配器管理 |

## 本次已修复

### 1. 让改稿由领域行为约束

原先 `UpdateService` 直接拼接 `map[string]any` 数据库列更新，再逐字段修改响应快照。`Video` 没有决定“谁可以在什么状态下改稿”。作者提交 `isPrivate=false` 时，已删除的视频也可能被改成公开或待审。

- 新增 [`Video.Revise` 与 `Revision`](../internal/domain/video/revision.go)，检查作者和可编辑状态，再修改领域快照。已删除视频沿用“视频不存在或无权限”的错误契约，恢复只留给管理端。
- 仓储以 `SaveRevision(ctx, *Video)` 接收类型明确的写模型，列名映射只留在基础设施；显式限定改稿字段，不覆盖播放量、点赞量、时长或媒体资源。
- 分区复用 `Partition.EnsureSubmittable()`，应用层只转换原有接口错误文案。

这改善了改稿入口，但没有把所有 `Video` 字段改成私有，也没有宣称所有状态写入都只能经过这一方法。

### 2. 修复事务范围和改稿语义

原先在事务外读视频，事务内无条件更新；并发删除或审核后，改稿可能按旧快照覆盖新状态。

- [`UpdateService`](../internal/application/video/update_service.go) 在同一事务中完成锁定读取、领域行为、主体保存及标签替换。
- MySQL 仓储使用 `SELECT ... FOR UPDATE`，并保留作者条件。视频、标签和发布时间在同一事务内提交或回滚，缓存操作在提交成功后执行。
- 仅传 `publishTime` 也会重新计算状态，修复“公开状态不变，却新增未来调度”的不一致。
- 未传 `isPrivate` 时继承现有私密状态，私密视频不会因只修改时间而加入公开调度。
- 未传 `tags` / `tags:null` 保留标签；显式 `tags:[]` 清空标签。原先空列表被当成不修改。
- 元数据修改不调整调度；显式修改可见性或时间时，按新状态添加或移除调度。

行锁解决数据库写入的旧快照问题；定时计划保存在 `videos.publish_at`，与状态一起提交。到期发布也在视频行锁内重新检查最新时间。

### 3. 技术实现移回基础设施

- DASH 清单的 HTTP 客户端、请求、超时和读取实现从 application 移到 [`infrastructure/external/video`](../internal/infrastructure/external/video/dash_resolver.go)。应用层只保留 `DASHVariantResolver` 端口，保留原有超时、读取上限、解析和错误行为。
- 邮箱验证码服务通过 `EmailCaptchaGenerator` 生成标识和验证码，由 [`captcha.EmailGenerator`](../internal/infrastructure/captcha/email_generator.go) 封装原来的 base64Captcha 算法。验证码长度和字符集仍由用例策略提供。

### 4. 恢复上下文所有权

| 原问题 | 修复后的边界 |
|---|---|
| video 上传额度直接 import identity 应用层并读取其角色模型 | video 声明 `UploadQuotaExemption`，组合根适配器调用 identity 的管理员判定，video 只消费豁免结论 |
| AI 返回 RBAC 的领域类型并执行菜单约束 | AI 返回生成文本，RBAC 在自己的应用层解析成 `MenuSuggestion`，复用自己的 SVG 安全规则 |
| 视频仓储直接锁 users 表 | `CoinLedger.LockAccounts` 经适配器进入 identity，由账户仓储按 ID 升序去重加锁，并翻译不存在错误 |
| 视频仓储保留未调用的 `SoftDeleteWithComments`，可直接删除评论上下文的数据 | 删除该领域接口及实现，现有删除用例继续通过 `CommentPurger` 端口清理评论 |

投币、收藏和三连仍使用原有同库事务，账户锁排序移动到 identity 实现中；没有将这些操作改为最终一致性。

### 5. 增强自动架构约束

[`arch_deps_test.go`](../internal/application/shared/arch_deps_test.go) 现在检查：

- domain 不能反向导入 application、interfaces、bootstrap、配置机制或列出的技术框架。
- application 不能导入 infrastructure、interfaces、bootstrap 或列出的数据库、消息、验证码实现库。
- domain/application 不能直接导入其它业务上下文的 domain/application；共享内核 `shared` 是明确例外。
- 应用层不能直接构造 HTTP 请求或客户端，HTTP `Header/Response` 等现有 AI 端口数据类型仍允许。
- 扫描带 build tag 的生产文件；测试文件可以装配真实适配器。包边界按完整路径匹配。

这些检查保护已明确的依赖约束，不证明业务归属正确，也不检查所有间接依赖。Wire 配置及生成文件已同步。

## 仍然需要演进的地方

| 优先级 | 现状与证据 | 建议与本次未直接实施的原因 |
|---|---|---|
| 高 | `PublishService` 在数据库落库后派发转码；向量同步也在提交后执行 | 为转码与索引引入持久化投递意图及幂等重试。定时发布已在下述 2026-10-22 修复中改为数据库持久化，不再依赖提交后写 Redis |
| 中 | `Video`、`User` 字段仍公开，视频仓储仍有通用状态更新和较宽的方法集 | 按审核、恢复、删除等具体不变量逐步引入领域行为和更窄端口；直接把整个项目字段私有化会影响现有转换、查询与调用方，本次只完整落实改稿链路 |
| 中 | `application/identity/coin_service.go` 仍计算余额并校验负值；`User.CoinCount` 使用 float64 | 可引入按最小单位存储的硬币值对象，集中舍入和余额约束；需要验证历史金额、数据库精度及接口兼容，不能直接切换类型 |
| 中 | AI 应用端口仍暴露 Eino `schema.Message/StreamReader` 和 HTTP 响应 | 若需要独立更换 SDK 或传输协议，应引入自有消息、流和资源响应契约；当前仍是明确的技术耦合，移除会涉及流式接口链路的完整回归 |
| 中 | 只改标题/描述是否需要重新审核，代码尚未把该政策统一建模 | 应先明确哪些内容变更需要复审，再形成领域策略。本次保持原有内容审核政策，未擅自规定所有文字编辑必须重审 |
| 持续 | `shared/enum` 共享了多个业务枚举；多个上下文使用共享数据库投影和跨上下文事务 | 记录实际共享内核及合作关系。读侧跨表不是必然违规；如果将来拆部署，需要重新设计一致性与数据所有权，不能直接用网络调用替换同库事务 |

本次不引入微服务、CQRS 独立数据库或事件溯源；这些是具体业务与运行需求下的选项，不是 DDD 必选项。

## 验证与复现

### 2026-10-22：可靠定时发布修复

- 在现有 `videos` 表增加 `publish_at`（计划时间）、`approved_at`（人工审核通过时间）、`is_scheduled`（是否定时作品）及到期查询索引，沿用现有 GORM `AutoMigrate`，不新增调度表或独立 SQL 脚本。
- 投稿时把状态与发布时间写入同一条视频记录；改稿、审核、删稿和到期发布在同一事务中更新状态与该字段，失败一起回滚。`publish_at` 为空表示没有定时发布计划。
- 调度器默认每 5 秒通过索引读取最多 100 个符合发布状态的到期视频；发布失败供下一轮重试。成功后保留计划时间和定时标识，已发布状态不会再次被轮询；开启审核时待审视频保留计划，等待管理员处理。
- 到期发布、作者改期与管理员审核共用视频行锁，并在锁内重读发布时间。过期的轮询结果不能提前发布或清除新时间；审核读取时间失败时拒绝提前发布。
- 定时发布链路完全移除 Redis 读写和旧任务迁移逻辑；本项目没有需要迁移的旧 Redis 任务。
- 管理员点击通过时原子写入 `approved_at`；免审作品保持空值，重新进入待审状态时清除旧的审核通过时间。管理端列表、详情、审核与回收站展示发布方式和两类时间，并支持发布方式筛选。
- 回归测试覆盖现有视频加字段迁移、投稿原子写入、改稿与删稿失败回滚、到期发布失败重试、重启、改期及审核时机。

**升级步骤**：通过现有入口执行 `go run ./cmd/server -f <config.yaml> -db`，由 GORM 为 `videos` 增加字段和索引，然后启动新版服务。此变更只修改模型和迁移代码，不自动连接业务数据库执行迁移。

以下是原 DDD 分支的验证记录；本节新增测试与本机复测结果以本次提交的测试日志为准。

修复验证环境为 Windows / Go 1.26.4：全量测试 60 个包通过，2 个包失败，51 个包无测试；两项失败仍是下表中的 OpenAPI 元数据与管理员登录验证码基线问题，已在独立 master 副本复现。`go test -race ./internal/infrastructure/persistence/video ./internal/application/video`、`go build ./cmd/...` 与 Wire 重新生成通过。未执行生产迁移或真实 MySQL 并发压力测试。

- 领域测试覆盖身份、删除状态保护、元数据保持状态、私密/定时/审核组合。
- SQLite 集成测试覆盖真实改稿事务、空值与 false 保存、清空标签、标签失败回滚、不覆盖独立统计，以及提交前不触发缓存/调度。
- MySQL 方言 DryRun 检查改稿的 `FOR UPDATE`，以及账户锁的升序、去重、忽略零 ID、不修改调用方输入。**这不是实际 MySQL 并发压力测试。**
- HTTP 适配器用本地测试服务验证成功解析、上游错误、无效 XML 和请求取消。
- 原有验证码发送、管理员额度豁免和 AI/RBAC 菜单测试随接口调整继续验证，未删减安全校验。竞态检测发现上传测试桩的闲置调用计数器存在并发写入，已移除该计数器，未删除任何断言。

开发环境需先执行 `source /workspace/.cloud-setup/env.sh` 使用准备好的 Go 1.24.9；通用开发机器使用 `go.mod` 要求的 Go 版本即可。本地 HTTP/miniredis 测试需要允许回环 socket。

```sh
go test ./...
go test -race ./internal/domain/video ./internal/application/shared ./internal/application/identity ./internal/application/ai ./internal/application/rbac ./internal/application/video ./internal/bootstrap/... ./internal/infrastructure/persistence/identity ./internal/infrastructure/persistence/video ./internal/infrastructure/external/video ./internal/infrastructure/captcha ./internal/interfaces/http/handler/identity
go build ./cmd/...
```

全量测试有三组基线失败，已从 `1011c4b` 导出独立副本复现，并非本次新增：

| 包 | 失败测试 | 原因 |
|---|---|---|
| `internal/infrastructure/storage` | `TestLocalResolveSource` | `/uploads/...` 被当作操作系统绝对路径，未按存储根目录解析 |
| `internal/infrastructure/persistence/rbac` | `TestOpenAPIMetadataCoversRegisteredAPIRoutes` | `captcha/type` 和 `video/danmu/month-dates` 缺少 OpenAPI summary/tags |
| `internal/interfaces/http/router` | `TestAdminLoginRouteRequiresSlideCaptcha` | 验证码路由测试的设置或预期消息与当前实现不一致 |

这些问题与本次 DDD 改造分开记录，未通过跳过测试或放宽断言制造全绿结果。最终验证结果：

| 检查 | 结果 |
|---|---|
| 全量 `go test -json ./...` | 723 个顶层测试通过，3 个顶层测试失败；59 个包通过，3 个包失败，51 个包没有测试文件；失败均与独立基线副本一致 |
| 上述相关包 `go test -race` | 全部通过（修复测试桩计数器后复测） |
| `go build ./cmd/...` | 通过 |
| Wire 重新生成、Go 格式及 `git diff --check` | 通过 |

全量结果产生后仅移除了上传测试桩的闲置计数器，随后该包普通测试与相关包竞态测试均重新通过。未验证真实 Kafka/SMTP/OAuth/AI 服务集成，也未执行真实 MySQL 并发压力测试。
