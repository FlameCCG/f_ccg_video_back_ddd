# 应用日志与采集契约

本仓库继续使用显式注入的 Zap、Wire、`res` 和现有分层架构。HTTP 响应仍为 `code/msg/data`，成功 `0`、失败 `1`。日志以 `success` 表示已知业务结果，不记录 `biz_code`；`error_code/error_kind` 仅供内部检索。

## 新代码固定模式

同步调用保留错误原因，向最终 HTTP、WebSocket、Kafka 或进程边界返回；已处理的降级、清理和独立任务异常在当地记录。包装原因、迁移记录位置、删除原内层日志应在同一改动中完成。

```go
// Domain 可以使用只依赖标准库的 apperr；不依赖 Gin、Zap 或日志级别。
return apperr.Wrap("video.save", domain.ErrCreateVideoFailed, cause, apperr.Dependency)

// HTTP：登记原始错误，保持旧响应文案与协议。
res.FailWithCause(err, clienterror.Message(err), c)

// Application：对不再返回的降级事件使用构造时注入的 Logger。
logctx.With(s.logger, ctx).Warn("video.cache.degraded",
    zap.String("event", "video.cache.degraded"),
    zap.String("operation", "video.load"),
    zap.Error(err))
```

使用 `errors.Is/As`，不直接比较被包装的错误。业务错误和底层 cause 均可被识别。按上下文维护 `internal/interfaces/clienterror/*_descriptions.go`：稳定代码、语义类别、安全文案分开声明。`Message()` 仅做文案转换。

| 情况 | 记录位置与级别 |
| --- | --- |
| 参数、正常业务拒绝、不存在、冲突、普通限流 | 访问摘要，记录失败结果；无额外 Error |
| 有请求上下文证据的客户端取消 | Info |
| 已处理降级、可恢复异常、安全事件 | Warn |
| 依赖故障、内部或未知异常 | 边界 Error |
| HTTP/WS/独立任务 panic | Error，独立 stack 字段 |

外层明确语义优先于错误描述，未知错误视为内部异常。可展示安全文案与是否需要告警无关。包装中的未知 cause、或合并错误中的系统故障，不会被外层安全业务文案遮蔽。不要用错误字符串推断业务语义。

## 请求与其他入口

HTTP 链为 `请求上下文 → 日志汇总 → Recovery → 校验/鉴权/业务`，使用 `gin.New()`。每个请求一条 `http.request` 摘要；系统异常另有一条 `http.request.failed` 详情。摘要不重复异常正文和堆栈，汇总中间件不写响应。服务端重新生成 `X-Request-ID`，不复用客户端传入值。

`res.Response.JSON()` 登记 `success`，原有 `Json()` 方法保留兼容；未经过 res 且没有明确结果时省略该字段。`FailWithMsg` 仅输出安全文案；`FailWithError` 保留校验翻译并登记参数类别；`FailWithCause(nil, ...)` 登记内部调用错误。SSE 中断用 `res.RecordFailure`，正常结束用 `res.SetSuccess`，已经开始流式响应后不追加 JSON 响应。

WebSocket 在 Hub 消息入口记录结果，分别提供 `connection_id/message_id`，保留请求和用户关联。消息 panic 在入口恢复并记录。握手已由升级器写出错误时不重复写 HTTP JSON。

`logmeta` 仅通过 `context.Context` 携带元数据，`logctx.With(injectedLogger, ctx)` 给显式 Logger 补字段，不从 context 取隐式 Logger。Application 不接收 Gin Context。detached 任务解除父取消但保留元数据，并获得任务、执行 ID；受进程停机控制，panic 由任务边界记录。

Kafka Header 携带 `request_id/user_id/task_id/attempt_id`，有上级任务时增加 `parent_task_id`。新投递获得新的 task ID，每次消费执行获得新的 attempt ID；重试保留 task ID，死信保留关联 Header，任务 JSON 不变。旧消息使用原始 topic、partition、offset 的确定性摘要作 task ID。消费者组每次重建均安装新的 `Errors()` 监听，并在关闭旧组后结束旧监听。重试为 Warn，最终处理失败为 Error，死信投递结果单独记录。删除、重平衡及正常关机取消为 Info。

GORM 使用注入的 Zap 适配器：默认慢查询阈值 200ms，可用 `mysql.slowQueryThresholdMs` 修改。Trace 不求值 SQL 文本、不展开参数；`db.query.failed/db.query.slow` 是 Warn 诊断，最终业务边界负责 Error 告警。River 自有代码使用注入 Logger；旧 canal 库只能通过 `infrastructure/legacylog` 的显式桥接接口输出。定时任务使用各自的任务关联信息与注入的库适配器。

## 字段、脱敏与数量

公共字段为 `time/level/msg/service/env/instance/component`；时间为 UTC RFC3339，耗时用 `duration_ms`，其他 Duration 字段明确以 `_ms` 结尾。字段用 snake_case。按入口补充 `event/operation/request_id/user_id/http_status/success/error_code/error_kind/task_id/attempt_id/video_id/part_id/topic/partition/offset`。

事件名称应稳定，例如 `http.request.failed`、`transcode.task.started/completed/failed/canceled`、`kafka.message.retry`、`kafka.message.dead_lettered`。重试、诊断和任务最终失败是不同事件。未来加入真实追踪时新增 `trace_id/span_id`，不要把 request ID 当 trace ID。

默认仅记录路由模板，不记录完整 Query、请求头、请求体、Kafka payload 或 AI 对话。邮箱掩码；密码、验证码、Bearer、Cookie、签名 URL 参数和配置凭据脱敏。输出端同时处理字符串、错误链及嵌套结构。已知会回显输入的 AI/OAuth 响应使用 `apperr.Private` 保留原始 cause、输出安全诊断；SQL 驱动错误保留错误号、隐藏参数值。更新凭据后仍保留旧凭据的脱敏规则，以覆盖未重建的旧客户端。

普通文本限制 2 KiB，错误文本 4 KiB，截断标记为 `[truncated]`，堆栈单独保存。不得对系统最终异常或安全事件采样。诊断进度需在 Debug 局部限频，避免把终态事件混入进度采样器。

## 配置与生效

```yaml
logger:
  level: info
  output: both          # files / stdout / both
  consoleFormat: json   # json / console
  service: video-backend
  env: production
  instance: ""          # 默认 hostname-PID；每个并行实例应具有不同身份
  debugFileName: ./runtime/{service}/{instance}/debug.log
  infoFileName: ./runtime/{service}/{instance}/info.log
  warnFileName: ./runtime/{service}/{instance}/warn.log
  errorFileName: ./runtime/{service}/{instance}/error.log
  maxSize: 500
  maxAge: 28
  maxBackups: 3
mysql:
  slowQueryThresholdMs: 200
```

未配置 output 时继续沿用文件加 enableConsole；未配置 level 默认为 Info，开发示例显式为 Debug。格式未配置时，development 使用可读 console，否则使用 JSON。文件始终为 JSON。四个级别文件必须为不同路径；Error 文件包含 Error、DPanic、Panic、Fatal。stdout 模式无需配置文件路径。

level 通过同一个 `zap.AtomicLevel` 即时作用于既有子 Logger。输出、编码、身份、路径、轮转需重启；管理端读取和更新 logger 响应保留原字段并补充 `data.restartRequired`。已变更输出设置而尚未重启时，后续仅改 level 仍会报告待重启。

管理更新先校验、原子写临时文件、同步并重命名成功，再发布快照。管理更新和文件监听串行发布，合并最新配置段，使用同一订阅通知；失败保留旧值。配置文件和 MySQL/Redis/Mongo/ES 的可预期初始化故障向入口返回错误，日志器建立前由 stderr 输出安全诊断并返回非零退出码。

日志写入保持同步。启动时验证路径，持有独占 `.lock` 文件，拒绝并行实例共享同一轮转文件；支持 `{service}/{instance}` 路径隔离。旧进程也必须升级并遵循锁约定。停止 HTTP/后台组件、detached、消息生产者、基础设施和配置监听/订阅后，最后显式关闭 Lumberjack Writer。`Sync()` 不代表 Lumberjack 已持久化，关闭不会删除锁文件，锁随文件句柄释放。

文件或 stdout 写入故障走独立 stderr 诊断，首次失败立即可见，重复故障最多每分钟一条，恢复时立即可见，不递归调用故障 Logger。

## 采集、排障与检查

采集器以独立日志数据集保存 JSON，按 service、instance、event 和关联 ID 检索。both 模式只采集文件或 stdout 中的一种，避免重复入库。集中平台部署、保留周期和完整分布式追踪不在本轮部署范围。

排障从请求 ID 找摘要和边界详情，再通过 task ID、attempt ID 及 Kafka 的 topic/partition/offset 串联重试与死信。先查 stderr 的 `logger.write.failed/recovered` 确认采集源是否健康，再查进程和配置事件。不要通过临时开启请求体、SQL 参数或对话正文日志排障。

```sh
go test ./internal/architecture ./pkg/...
go test -race ./pkg/ctxutil ./pkg/logsafe ./internal/infrastructure/logger ./internal/infrastructure/config ./internal/infrastructure/mq ./internal/interfaces/http/middlewares ./internal/interfaces/ws/...
go test ./...
go vet ./...
golangci-lint run ./...
```

架构测试禁止自有代码重新引入 Logrus、slog、标准库全局日志、`zap.L/S/ReplaceGlobals`、Gin 默认日志和完整请求 Dump；第三方全局日志桥接仅允许在专用基础设施适配器中出现。
