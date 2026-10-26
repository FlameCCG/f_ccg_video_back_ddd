# 站点管理

分类说明：站点配置与统计、心跳/打点

Base URL：/v1

## [GET] 获取QQ登录URL

- 接口路径: GET /common/site/qq-url
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取QQ OAuth登录跳转URL
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | string | QQ登录跳转URL |

响应示例:

```json
{
  "code": 0,
  "data": "https://example.com/page",
  "msg": "获取成功"
}
```

## [GET] 获取Google登录URL

- 接口路径: GET /common/site/google-url
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取Google OAuth登录跳转URL
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | string | Google登录跳转URL |

响应示例:

```json
{
  "code": 0,
  "data": "https://example.com/page",
  "msg": "获取成功"
}
```

## [GET] 获取GitHub登录URL

- 接口路径: GET /common/site/github-url
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取GitHub OAuth登录跳转URL；服务端自动生成防 CSRF state 并存入 Redis，在授权URL中携带并在登录回调时校验；支持可选 PKCE codeChallenge
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| codeChallenge | query | string | 否 | PKCE codeChallenge；传入后会拼接到 GitHub 授权URL |
| codeChallengeMethod | query | string | 否 | PKCE challenge 计算方式，默认 S256 可选: S256 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | string | GitHub登录跳转URL |

响应示例:

```json
{
  "code": 0,
  "data": "https://example.com/page",
  "msg": "获取成功"
}
```

## [GET] 获取LinuxDo登录URL

- 接口路径: GET /common/site/linuxdo-url
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取 LinuxDo OAuth 登录跳转URL；服务端自动生成防 CSRF state 并存入 Redis，在授权URL中携带并在登录回调时校验
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | string | LinuxDo 登录跳转URL |

响应示例:

```json
{
  "code": 0,
  "data": "https://connect.linux.do/oauth2/authorize?client_id=example&redirect_uri=http%3A%2F%2Flocalhost%3A3000%2Foauth%2Flinuxdo&response_type=code&scope=user&state=linuxdo-state",
  "msg": "获取成功"
}
```

## [GET] 获取X登录URL

- 接口路径: GET /common/site/x-url
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取X OAuth登录跳转URL；服务端自动生成防 CSRF state 并存入 Redis，并在登录回调时校验；前端需传入 PKCE codeChallenge
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| codeChallenge | query | string | 是 | X PKCE codeChallenge |
| codeChallengeMethod | query | string | 否 | PKCE challenge 计算方式，默认 S256 可选: S256/plain |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | string | X登录跳转URL |

响应示例:

```json
{
  "code": 0,
  "data": "https://example.com/page",
  "msg": "获取成功"
}
```

## [GET] 获取站点配置

- 接口路径: GET /common/site/config
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取站点公开配置
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.site | PublicSiteConfig | - |
| data.site.defaultUserBannerID | integer(uint) | 默认用户主页横幅ID |
| data.site.defaultUserBannerIDs | array<integer(uint)> | 系统默认用户主页横幅ID列表 |
| data.site.contentReview | AdminSiteContentReviewConfig | - |
| data.site.contentReview.enable | boolean | 是否开启内容审核 |
| data.site.login | AdminSiteLoginConfig | - |
| data.site.login.qqLogin | boolean | 是否开启QQ登录 |
| data.site.login.googleLogin | boolean | 是否开启Google登录 |
| data.site.login.githubLogin | boolean | 是否开启GitHub登录 |
| data.site.login.linuxdoLogin | boolean | 是否开启LinuxDo登录 |
| data.site.login.xLogin | boolean | 是否开启X登录 |
| data.site.login.usernamePwdLogin | boolean | 是否开启用户名密码登录 |
| data.site.login.textGraphicCaptcha | boolean | 是否开启文本图形验证码登录 |
| data.site.login.textClickCaptcha | boolean | 是否开启文本点击验证码登录 |
| data.site.login.slideCaptcha | boolean | 是否开启登录滑块验证码；与点选同时开启时随机展示一种 |
| data.site.login.slideCaptchaTTL | integer | 登录滑块有效期（秒），不大于 0 时使用 300 |
| data.site.login.slideCaptchaPadding | integer | 登录滑块容错像素，0 表示不放宽落点 |
| data.site.login.textClickCaptchaTTL | integer | 文本点击验证码有效期(秒) |
| data.site.login.textClickCaptchaPadding | integer | 文本点击验证码容错像素 |
| data.site.register | AdminSiteRegisterConfig | - |
| data.site.register.emailCaptcha | boolean | 是否开启邮箱验证码注册 |
| data.site.register.textGraphicCaptcha | boolean | 是否开启文本图形验证码注册 |
| data.site.register.textClickCaptcha | boolean | 是否开启发邮件点选验证码；与登录配置独立 |
| data.site.register.textClickCaptchaTTL | integer | 发邮件点选有效期（秒），不大于 0 时使用 300 |
| data.site.register.textClickCaptchaPadding | integer | 发邮件点选容错像素，0 表示不放宽落点 |
| data.site.register.slideCaptcha | boolean | 是否开启滑块验证码注册 |
| data.site.register.slideCaptchaTTL | integer | 滑块验证码有效期(秒) |
| data.site.register.slideCaptchaPadding | integer | 滑块验证码容错像素 |
| data.site.storage | Storage | - |
| data.site.storage.maxChunkSize | integer(int64) | 最大分块大小(MB) |
| data.site.storage.chunkSize | integer(int64) | 分块大小(MB) |
| data.site.storage.maxFileSize | integer(int64) | 最大文件大小(MB) |
| data.site.storage.maxUploadNum | integer(int64) | 单个作品的分P上限 |
| data.site.storage.dailyUploadLimit | integer(int64) | 普通用户每日不同源文件上限 |
| data.site.storage.unmergedChunkRetentionHours | integer(int64) | 未合并分片保留小时数 |
| data.site.storage.unpublishedSourceRetentionHours | integer(int64) | 未投稿源视频保留小时数 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "site": {
      "defaultUserBannerID": 19,
      "defaultUserBannerIDs": [19, 21],
      "contentReview": {
        "enable": true
      },
      "login": {
        "qqLogin": true,
        "googleLogin": true,
        "githubLogin": true,
        "linuxdoLogin": true,
        "xLogin": true,
        "usernamePwdLogin": true,
        "textGraphicCaptcha": true,
        "textClickCaptcha": true,
        "slideCaptcha": true,
        "slideCaptchaTTL": 300,
        "slideCaptchaPadding": 10,
        "textClickCaptchaTTL": 1,
        "textClickCaptchaPadding": 1
      },
      "storage": {
        "maxChunkSize": 100,
        "chunkSize": 10,
        "maxFileSize": 1901,
        "maxUploadNum": 10,
        "dailyUploadLimit": 10,
        "unmergedChunkRetentionHours": 2,
        "unpublishedSourceRetentionHours": 1
      },
      "register": {
        "emailCaptcha": true,
        "textClickCaptcha": true,
        "textClickCaptchaTTL": 300,
        "textClickCaptchaPadding": 20,
        "textGraphicCaptcha": true,
        "slideCaptcha": true,
        "slideCaptchaTTL": 1,
        "slideCaptchaPadding": 1
      }
    }
  },
  "msg": "获取成功"
}
```

## [POST] 站点打点

- 接口路径: POST /common/site/stat/touch
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 增加站点流量（PV）并统计今日UV（含游客）；若已登录则当日首次打点时发放登录经验和 +1.0 硬币
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | SiteTouchResult | - |
| data.pv | integer(int64) | 今日PV |
| data.uv | integer(int64) | 今日UV（含游客与登录用户） |
| data.online | integer(int64) | 在线人数（5分钟窗口） |

响应示例:

```json
{
  "code": 0,
  "data": {
    "pv": 1,
    "uv": 1,
    "online": 1
  },
  "msg": "打点成功"
}
```

## [POST] 在线心跳

- 接口路径: POST /common/site/stat/heartbeat
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 维护UV与在线人数，不增加PV（支持游客）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | SiteHeartbeatResult | - |
| data.uv | integer(int64) | 今日UV（含游客与登录用户） |
| data.online | integer(int64) | 在线人数（5分钟窗口） |

响应示例:

```json
{
  "code": 0,
  "data": {
    "uv": 1,
    "online": 1
  },
  "msg": "心跳成功"
}
```

## 视频投稿开关

- 字段：`site.disableUserSubmission`，布尔值，默认 `false`。
- 管理端 `GET /admin/site/config?name=site` 返回 `data.disableUserSubmission`；公开配置返回 `data.site.disableUserSubmission`。
- 使用 `PUT /admin/site/config?name=site` 保存完整 site 对象。开启后，仅拥有管理员角色的账号可上传视频及投稿封面、查询续传状态、完成上传、发布和修改视频。保存后后续请求立即按新值校验。
- 旧配置缺少字段时默认允许投稿；旧管理端更新时遗漏此字段会保留当前值，显式传 `false` 才会关闭限制。
- 客户端通过 `GET /common/user/info` 的 `data.isAdmin` 判断管理员身份；前端显示提示，后端独立校验身份。
