# CCG Video Backend API

- Base URL: /v1
- 说明:
  - 视频平台后端 API 文档，包含用户、视频、评论、社交、通知、动态、RBAC 等模块。
  - 普通 JSON 业务响应使用 HTTP 200，通过响应体 code 判断成功/失败（0=成功，1=失败），错误信息在 msg。仍须处理真实 HTTP 状态：AI 额度耗尽返回 429（带 Retry-After），WebSocket 升级返回 101，CORS 拒绝可返回 403、预检返回 204；SSE、静态文件和 AI 资源代理使用各自协议。
  - 管理后台接口除登录鉴权外还会经过 RBAC（Casbin）权限校验。

## 文档入口

1. [普通用户](user/index.md)
2. [管理员](admin/index.md)

当前 OpenAPI 覆盖 191 条 `/v1` 操作；另有 `/uploads/*filepath` GET/HEAD 静态路由。完整注册清单见[路由金丝雀](../../internal/interfaces/http/router/testdata/routes_golden.txt)。字段与行为以请求类型、响应 DTO 和 Handler 为准；[OpenAPI](../openapi.yaml)与各模块说明应同步维护。
