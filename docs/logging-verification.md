# 日志治理验证记录

本次完成错误包装与描述表、响应结果登记、HTTP 汇总与恢复、SSE 中断、WebSocket 连接/消息、Kafka Header 与重试/死信、转码/后台任务、GORM/River 适配，以及日志输出、级别热更新、文件锁、Writer 关闭和配置发布通知的改造。字段、调用示例、配置和采集规范见 [日志与采集契约](logging.md)。

对外 `code/msg/data` 契约保持成功 0、失败 1。日志不增加业务响应码字段；安全文案和错误分类分别处理，未知 cause 不会被可展示的业务错误遮蔽。

## 验证结果

| 检查 | 结果 |
| --- | --- |
| HTTP、SSE、WebSocket、Kafka、错误描述与应用相关包测试 | 通过 |
| 错误双分支识别、原始类型、进程 stderr 与脱敏测试 | 通过 |
| UTC、毫秒单位、文本上限、SQL 参数与敏感来源保护测试 | 通过 |
| 严重级别文件、输出模式、重复关闭、独占锁、写入失败及恢复测试 | 通过 |
| 配置失败保留旧值、实际文件重载、订阅及动态级别测试 | 通过 |
| 日志架构检查 | 通过 |
| 全项目并发测试（race） | 通过，无竞态报告 |
| 两个进程入口构建、Wire 注入声明编译 | 通过 |
| `go vet ./...` | 通过 |
| `go test -timeout 5m -p 2 ./...` | 通过，包含 `TestLocalResolveSource` |
| 现有严格 `golangci-lint run --max-same-issues 0 ./...`（不带 `--fix`） | 通过，0 项问题 |

## 严格 lint 修复范围

保留原有严格检查项与复杂度阈值，修复错误处理、资源管理、命名、重复代码、函数复杂度、格式和测试断言。生命周期上下文、兼容 API、非认证用途的旧摘要格式等确需保留的写法使用有明确理由的局部注释，不关闭整项规则。

同时修复以下可观测的行为问题：

- `LocalStorage.ResolveSource` 优先识别公开的 `/uploads/` 前缀，按存储根目录解析，仍支持实际文件绝对路径。
- 用户查询、Redis 统计解析和 River 字段转换保留底层错误；业务边界或已处理的降级事件负责记录。
- 配置中的过期时间按明确的分钟或天数转换，防止时长乘法和溢出；搜索数值转换拒绝越界和非有限数值。
- 移除非指针结构体上无效的 `omitempty`，保持这些字段原本始终输出的 JSON 行为；保留 `Response.Json()` 兼容入口。

格式器的废弃 `extra-rules` 配置迁移为等价的 `group-params` 与 `clothe-returns`，严格 lint 的最终复核不带 `--fix`。

验证使用 Go 1.24.9；严格 lint 使用支持当前配置中 modernize 等规则的 golangci-lint v2.15.0。没有部署集中采集平台，也没有声称完成真实 Kafka 集群、GPU 和集中采集平台的部署验收。

## 复核命令

```sh
golangci-lint run --max-same-issues 0 ./...
go test -timeout 5m -p 2 ./...
go test -race -timeout 5m -p 2 ./...
go build ./cmd/server ./cmd/transcode-consumer
go vet ./...
go test -tags=wireinject ./internal/bootstrap
git diff --check
```

测试覆盖实际 HTTP/WebSocket 握手与摘要结果、正常拒绝与系统故障的区分、panic、客户端取消证据、Kafka 旧消息及重试/死信关联、独立任务异常、配置通知和敏感文本输出。库或环境相关测试仍以其实际跳过与执行结果为准。
