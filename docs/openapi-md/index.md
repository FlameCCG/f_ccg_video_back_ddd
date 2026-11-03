# CCG Video Backend API

- Base URL: /v1
- 说明:
  - 视频平台后端 API 文档，包含用户、视频、评论、社交、通知、动态、RBAC 等模块。
  - 普通 JSON 业务响应使用 HTTP 200，通过响应体 code 判断成功/失败（0=成功，1=失败），错误信息在 msg。仍须处理真实 HTTP 状态：登录会话签发、刷新及受保护请求的鉴权依赖故障返回 503、超时返回 504；AI 额度耗尽返回 429（带 Retry-After），WebSocket 升级返回 101，CORS 拒绝可返回 403、预检返回 204；SSE、静态文件和 AI 资源代理使用各自协议。
  - 管理后台接口除登录鉴权外还会经过 RBAC（Casbin）权限校验。

## 文档入口

1. [普通用户](user/index.md)
2. [管理员](admin/index.md)

当前 OpenAPI 覆盖 191 条 `/v1` 操作；另有 `/uploads/*filepath` GET/HEAD 静态路由。完整注册清单见[路由金丝雀](../../internal/interfaces/http/router/testdata/routes_golden.txt)。字段与行为以请求类型、响应 DTO 和 Handler 为准；[OpenAPI](../openapi.yaml)与各模块说明应同步维护。

## 登录会话与错误处理

密码、管理员与 OAuth 登录成功均返回 `data.accessToken`、`refreshToken`、`accessExp`、`refreshExp`；到期时间为 Unix 秒数。受保护请求使用 `Authorization: Bearer <accessToken>`；支持 query token 的 WebSocket/SSE 路由见对应模块说明。每次登录创建独立设备会话，新登录不替换其他设备的凭据。

刷新成功后保存新令牌对及到期时间。已消费的刷新令牌有固定 30 秒重试宽限，重试不延长宽限；超过宽限重放会撤销该设备会话。修改或重置密码使旧凭据失效；Redis 会话丢失或升级前不含 `sid` 的令牌需要重新登录。

| 结果 | 客户端处理 |
|---|---|
| HTTP 200、`code=0` | 处理业务成功数据 |
| HTTP 200、`code=1`，明确提示凭据无效、过期或账号不可用 | 进入登录或账号恢复流程 |
| HTTP 503 / 504、`code=1` | 暂时无法验证身份；保留令牌，稍后重试，不据此清除登录态 |
| 验证码失败，`data` 含 `captchaType` / `captchaReason` | 按[验证码恢复流程](user/02-captcha-management.md#验证失败与客户端恢复)换题或回到表单 |

上表的 503/504 也适用于模块文档中标为“需要登录”的接口在鉴权阶段失败的情况。普通业务失败仍按具体接口处理。WebSocket 建立后的会话校验失败使用关闭码 1008（凭据失效）或 1013（临时校验故障）；协议细节见[私信](user/09-chat-management.md#websocket-消息协议)与[弹幕](user/05-video-management.md)。
