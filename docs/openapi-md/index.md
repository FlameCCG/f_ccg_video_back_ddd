# CCG Video Backend API

- Base URL: /v1
- 说明:
  - 视频平台后端 API 文档，包含用户、视频、评论、社交、通知、动态、RBAC 等模块。
  - 所有接口统一返回 HTTP 200，业务成功/失败请以响应体 code 判断（0=成功，1=失败），错误信息在 msg。
  - 管理后台接口除登录鉴权外还会经过 RBAC（Casbin）权限校验。

## 文档入口
1. [普通用户](user/index.md)
2. [管理员](admin/index.md)
