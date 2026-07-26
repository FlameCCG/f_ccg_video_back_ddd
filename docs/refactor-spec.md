# DDD 重构执行规范（/goal 引用文件，具完全约束力）

> 目标：将 `E:\f_ccg_video_final\f_ccg_video_back`（Gin + GORM/MySQL + Redis + MongoDB + Elasticsearch + Kafka + MinIO + Casbin + WebSocket + eino 的视频平台后端）完整重构为 DDD 架构，产出到同级新目录 `E:\f_ccg_video_final\f_ccg_video_back_ddd`。最终交付物 = 功能 1:1 等价的 DDD 版本 + 迁移中发现的所有问题的修复 + 新仓库根目录 `report.md` 问题台账 + 原子化 git 提交历史。
>
> 本文件是唯一详细规范：开工第一件事通读全文，并复制为新仓库 `docs/refactor-spec.md` 随首个 commit 提交；每次上下文压缩恢复后必须重读本文件。

## 0. 基线与红线
- 以原项目**磁盘当前工作区状态**为基线（包含未提交改动与未跟踪文件，如 service/mq_service/transcode_checkpoint.go、transcode_chunked.go），不是最后一次 commit 的状态。
- 原项目目录**绝对只读**：不写入任何文件、不 commit、不 stash；允许在其中运行 go build / go test 获取基线。
- 迁移范围（约 532 个生产 .go 文件，其中 68 个 _test.go）：main.go、api/、conf/、core/、flags/、global/、models/、pkg/、router/、service/，以及资产：init_resource/、conf/rbac_model.conf、conf/site/、conf/third_login/、flags/mappings/、config.example.yaml、config_dev.yaml、docker-compose.yaml、.golangci.yml、.gitignore、docs/ 中仍适用的文档（路径引用更新为新结构）。注意：global/ 与 core/init_*.go 属于"职责必须被吸收"的范围（见第 2 节），不允许同形态平移。
- 排除（不迁移）：.claude、.agents、.grok、.roo、.kiro、.idea、.vscode、.serena、.tmp、.gocache、runtime、artifacts、terminals、agent-tools、components/（docker 运行时数据）、scripts/__pycache__、.git。
- 全程**不向用户提问、不暂停等待确认**；所有裁量决策自主拍板并记入新仓库 docs/decisions.md。

## 1. 目标架构（骨架固定，直接照此落地）

```
f_ccg_video_back_ddd/
├── cmd/server/main.go       # 仅：解析 flags → 调 bootstrap 注入器 → 等退出信号
├── internal/
│   ├── bootstrap/           # 组合根：wire.go(//go:build wireinject) + wire_gen.go + App 生命周期
│   ├── domain/<ctx>/        # 实体、值对象、聚合、领域服务、领域事件、仓储接口（纯 Go，零框架依赖）
│   ├── application/<ctx>/   # 应用服务/用例、DTO、事务编排、跨上下文协调、ProviderSet
│   ├── infrastructure/      # config、logger、persistence(GORM PO+仓储实现)、cache、search、
│   │                        # mq、storage、auth(jwt+casbin)、external(email/三方登录/volcengine/eino)、scheduler
│   └── interfaces/          # http/handler/<ctx>、http/router、http/middleware、ws
├── pkg/                     # 与业务无关的通用工具（res 响应、utils、validate 等，可近似平移）
├── docs/、init_resource/、config.example.yaml 等资产
```

限界上下文默认映射（除非有强理由并记入 decisions.md，否则照此执行）：identity（user/login/oauth/email 验证/captcha）、rbac、video（投稿/审核/分区/播放/点赞投币收藏/举报/回收站/弹幕 danmu）、media（storage_service + 全部转码管线 + transcode_cancel）、comment、social、dynamic、chat、notification、search（search/es/river 同步）、content-ops（banner/site）、ai、platform（cron 调度与 flags CLI 归接口/基础设施层）。

战术尺度：有真实业务规则处（视频状态机、转码管线、审核流、投币/收藏规则、rbac）做充血模型；纯 CRUD 上下文允许薄领域层，但分层与依赖方向（interfaces→application→domain←infrastructure）必须严格一致。领域层之间不得相互 import；跨上下文在应用层协调或经领域事件。与业务无关的工具代码不要为了 DDD 而重写，直接平移，把精力花在分层与领域建模上。首个脚手架 commit 需附 docs/architecture.md（上下文映射 + 分层依赖规则 + PO↔domain 转换约定 + wire 装配图）。

## 2. 依赖注入：google/wire（强约束）
1. **用 github.com/google/wire v0.6.0 做编译期依赖注入**。注入器集中在 internal/bootstrap：wire.go（带 `//go:build wireinject` 标签）声明 `InitializeApp(...)`，生成的 wire_gen.go **必须提交进仓库**。除测试外禁止手写散装装配。
2. **Provider 组织**：每个限界上下文在 application 层导出 `ProviderSet = wire.NewSet(...)`，内含该上下文的应用服务构造函数与 `wire.Bind`（domain 仓储接口 → infrastructure 实现）；infrastructure 按技术导出基础客户端 ProviderSet（zap/mysql/redis/mongo/es/kafka/minio/casbin/ip2region）；根配置经窄化 provider 函数拆出各组件所需的**窄配置结构体**（如 ProvideKafkaConf(cfg) conf.Kafka），禁止组件直接吞整个大配置。bootstrap 聚合全部 ProviderSet 生成注入器。
3. **本机专属生成命令（已验证，严格照做）**：
   - go.mod 直接依赖 `github.com/google/wire v0.6.0`，并执行一次 `go get github.com/google/wire/cmd/wire@v0.6.0` 补全 go.sum（subcommands/x/mod/x/sync 条目）；x/tools 保持继承自原项目的 v0.38.0，**严禁降级**。
   - 重新生成：`go run github.com/google/wire/cmd/wire ./internal/bootstrap`（模块内运行、版本由 go.mod 锁定、依赖已缓存可离线）；备用二进制 `C:\Users\Administrator\go\bin\wire.exe`（不在 PATH，须绝对路径）。
   - **严禁 `go install github.com/google/wire/cmd/wire@v0.6.0`**：其自带 x/tools v0.17.0 在 Go 1.24 下编译失败（tokeninternal invalid array length），已实测。
   - wire 生成前提是 providers 所在包能编译：先把新增 provider 编译通过，再跑 wire，再整体 `go build ./...`。任何改动 provider/接口绑定的 commit 都必须同 commit 内重新生成 wire_gen.go。
4. **依然禁止**：新仓库出现 global 包、service locator（global.XXX() 取用）、包级可变单例、`init()` 里做业务装配。原 global/ 与 core/init_*.go 的职责全部由 wire 注入与 bootstrap 吸收。接口所有权归消费方：仓储接口定义在 domain；跨上下文依赖对方应用服务接口或领域事件，不得 import 对方 domain 内部。
5. **生命周期**：注入器最终产出 `*App`（聚合 HTTP server、Kafka 消费者、cron、river、视频调度器等后台组件，均实现 Start(ctx)/Stop(ctx)）+ cleanup。运行期停序由 App.Stop 显式控制，语义与原 core.GracefulStop 等价（先停 HTTP、再停后台消费者、最后关基础设施连接）；wire 的 cleanup 只用于纯资源关闭，不承载停序逻辑。
6. **gin.Context 止步于 interfaces 层**：向 application/domain 只传 `context.Context` 与 DTO；用户身份、客户端 IP 等在 interfaces 层提取后显式传参。**测试不走 wire**：单测手写 fake/stub 实现 domain 接口直接注入构造；原测试依赖 global 全局状态的写法一律改造为注入式。

## 3. 外部契约 1:1（这就是"复刻"的验收定义，逐项对齐）
1. HTTP 路由：method + path 完全一致（/v1/common/*、/v1/admin/*、/uploads 静态目录），请求/响应 JSON 字段、状态码、错误码结构一致。
2. DB：GORM PO（表名、列名、tag、索引）与原 models/mysqlEntity、mongoEntity 完全一致；domain 实体另建，PO↔domain 转换放 infrastructure。
3. Kafka topic 名与消息 JSON 结构、Redis key 格式与 TTL、ES 索引名与 mapping、MinIO bucket 与对象路径、JWT claims、cookie 名全部保持一致。
4. 配置兼容：同一份 config_dev.yaml 不做任何修改即可启动新项目；CLI flags 与原 flags/ 等价；cron 任务与优雅退出顺序等价。
5. go.mod：module 名取 `f_ccg_video_back_ddd`；go 版本与全部依赖版本从原 go.mod/go.sum 原样继承（复用本地模块缓存）；**唯一允许新增的依赖是 github.com/google/wire v0.6.0**（及其 go.sum 条目），此外不新增任何第三方依赖。

## 4. 问题台账 report.md（新仓库根目录）
每发现一个 bug、设计不合理、安全隐患、性能问题或隐性不一致，立即追加一条，并随所属功能的 commit 一起提交：

```
## FIND-001 | 类别: bug|设计|安全|性能|一致性
- 原位置: service/xxx/yyy.go:123 `FuncName`
- 问题: <一句话说清缺陷>
- 触发场景/影响: <具体输入或时序 → 错误结果>
- 新代码处理: internal/xxx/yyy.go:45 已修复 / 保留原行为（原因）
- 改变外部可观测行为: 否 / 是（差异说明）
```

修复原则：默认保持外部行为 1:1；只有当行为本身就是缺陷（逻辑错误、并发竞态、资源泄漏、安全漏洞）时才允许改变行为，且必须在该条目醒目标注【行为变更】。

## 5. Git 纪律
- 在新目录 `git init`；首 commit 为脚手架（go.mod、目录骨架、architecture.md、refactor-spec.md 副本、wire 注入器首次生成、progress 与 report 模板）。
- 每完成一个小功能单元（一组路由 + 应用服务 + 领域模型 + 仓储 + ProviderSet 接线 + 对应测试）就 commit 一次，禁止大杂烩提交。
- 每个 commit 都必须 `go build ./...` 通过、wire_gen.go 与 wire.go 同步；测试随功能同一个 commit 迁移，不欠账、不最后补。
- 提交信息沿用原仓库风格的中文 conventional commits（如 `feat(video): xxx`、`fix(transcode): xxx`）；涉及 FIND 条目的在提交正文引用编号。

## 6. 过程纪律（长任务一次跑完的关键）
1. 第一步：在新仓库建 docs/migration-progress.md —— 在原项目用 `git ls-files` + 未跟踪文件列表（按第 0 节范围过滤）生成**逐文件核对表**，每项标 pending/done。它是唯一进度事实来源，每个 commit 同步更新。
2. 开工前在原项目跑一次 `go test ./...` 记录基线；原本就失败/跳过的用例记入 report.md，不算新项目的账。
3. 迁移顺序：pkg 通用件 + infrastructure（config/zap/mysql/redis/mongo/es/kafka/minio/casbin/ip2region）+ bootstrap 骨架（含 wire 首次成功生成）→ PO 模型 → identity → rbac → content-ops → social → video → media（转码管线，注意 checkpoint/chunked/watchdog 等最新未提交代码）→ comment → dynamic → notification → chat(ws) → search/river → ai → cron → flags → router 汇总 + main + 优雅退出 → 资产与文档收尾。
4. 严禁缩水：不允许"示例性/代表性迁移"，不允许 TODO/stub/`panic("not implemented")` 占位，不允许悄悄丢弃任何 handler、service 方法或测试文件。进度表里存在任何 pending 即视为任务未完成。
5. 上下文被压缩/恢复后：先读本规范（新仓库 docs/refactor-spec.md）+ docs/migration-progress.md + `git log --oneline -20` + report.md 再继续下一个未完成项，绝不重做已完成项、绝不从头开始。
6. 编译或测试一旦失败，当场修好才能进入下一个条目；不得跳过、不得注释掉失败用例。

## 7. 验收标准（全部满足才算完成，缺一不可）
- [ ] docs/migration-progress.md 全部 done，0 pending
- [ ] 新仓库 `go build ./...`、`go vet ./...` 零错误
- [ ] `go test ./...` 全绿（离线策略与原项目一致：sqlite/miniredis/httptest，不依赖真实中间件），结果不差于原项目基线
- [ ] wire 校验：`go run github.com/google/wire/cmd/wire check ./internal/bootstrap` 无错误，`go run github.com/google/wire/cmd/wire diff ./internal/bootstrap` 零差异（wire_gen.go 与声明同步）
- [ ] DI 校验：新仓库不存在 global 包；除 bootstrap 外无包级可变单例；application/domain 层零 gin 依赖（grep 可证）
- [ ] 路由奇偶校验：分别从新旧两边路由代码提取 method+path 全量清单并 diff，差异为零，比对结果作为附录写入 report.md
- [ ] report.md 台账完整，所有【行为变更】条目醒目标注
- [ ] git 历史原子化，任取一个 commit 均可编译通过
- [ ] 原项目目录零写入（保持与开工前完全一致）
