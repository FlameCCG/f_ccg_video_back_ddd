# 普通用户 API 文档

- Base URL: /v1
- 说明:
  - 视频平台后端 API 文档，包含用户、视频、评论、社交、通知、动态等模块。
  - 所有接口统一返回 HTTP 200，业务成功/失败请以响应体 code 判断（0=成功，1=失败），错误信息在 msg。

## 模块目录
1. [用户管理](01-user-management.md) - 注册登录、用户信息、设置、记录
2. [验证码管理](02-captcha-management.md) - 图形/滑块/点击/邮箱验证码获取与校验
3. [站点管理](03-site-management.md) - 站点配置与统计、心跳/打点
4. [轮播图管理](04-banner-management.md) - 轮播图获取与展示
5. [视频管理](05-video-management.md) - 视频浏览、搜索、互动、投稿、上传、弹幕、收藏/历史
6. [评论管理](06-comment-management.md) - 评论、回复与互动
7. [社交管理](07-social-management.md) - 关注/粉丝/关系链
8. [通知管理](08-notification-management.md) - 通知与消息
9. [私信管理](09-chat-management.md) - 私信/聊天相关
10. [动态管理](10-dynamic-management.md) - 动态发布与列表
11. [AI 管理](11-ai-management.md) - 文本生成、图像理解、图片生成/编辑、视频生成与查询、资源代理
