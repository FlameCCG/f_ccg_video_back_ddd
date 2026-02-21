# 后台-站点管理

分类说明：站点统计与配置管理

Base URL：/v1

## [GET] 站点统计

- 接口路径: GET /admin/site/stats
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理后台站点统计（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | AdminSiteStats | - |
| data.overview | object | - |
| data.overview.totalUsers | integer(int64) | - |
| data.overview.onlineUsers | integer(int64) | - |
| data.overview.totalVideos | integer(int64) | - |
| data.overview.totalComments | integer(int64) | - |
| data.overview.todayLoginUsers | integer(int64) | - |
| data.overview.todayRegisters | integer(int64) | - |
| data.daily | object | - |
| data.daily.visitUsers | TrendSeries | - |
| data.daily.visitUsers.x | array<string> | - |
| data.daily.visitUsers.values | array<integer(int64)> | - |
| data.daily.visitUsers.rates | array<number(float)> | - |
| data.daily.loginUsers | TrendSeries | - |
| data.daily.loginUsers.x | array<string> | - |
| data.daily.loginUsers.values | array<integer(int64)> | - |
| data.daily.loginUsers.rates | array<number(float)> | - |
| data.daily.publishVideos | TrendSeries | - |
| data.daily.publishVideos.x | array<string> | - |
| data.daily.publishVideos.values | array<integer(int64)> | - |
| data.daily.publishVideos.rates | array<number(float)> | - |
| data.daily.newUsers | TrendSeries | - |
| data.daily.newUsers.x | array<string> | - |
| data.daily.newUsers.values | array<integer(int64)> | - |
| data.daily.newUsers.rates | array<number(float)> | - |
| data.monthly | object | - |
| data.monthly.newVideos | TrendSeries | - |
| data.monthly.newVideos.x | array<string> | - |
| data.monthly.newVideos.values | array<integer(int64)> | - |
| data.monthly.newVideos.rates | array<number(float)> | - |

响应示例:

```json
{
  "code": 0,
  "data": {
    "overview": {
      "totalUsers": 1,
      "onlineUsers": 1,
      "totalVideos": 1,
      "totalComments": 1,
      "todayLoginUsers": 1,
      "todayRegisters": 1
    },
    "daily": {
      "visitUsers": {
        "x": ["2024-06-01"],
        "values": [1],
        "rates": [0.5]
      },
      "loginUsers": {
        "x": ["2024-06-01"],
        "values": [1],
        "rates": [0.5]
      },
      "publishVideos": {
        "x": ["2024-06-01"],
        "values": [2001],
        "rates": [0.5]
      },
      "newUsers": {
        "x": ["2024-06-01"],
        "values": [1],
        "rates": [0.5]
      }
    },
    "monthly": {
      "newVideos": {
        "x": ["2024-06-01"],
        "values": [2001],
        "rates": [0.5]
      }
    }
  },
  "msg": "获取成功"
}
```

## [GET] 站点配置（按名称）

- 接口路径: GET /admin/site/config
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要 RBAC 权限
- 依赖接口: 无
- 接口说明: 获取单项站点配置（需管理员权限）
- 说明: 不支持/不返回 mysql/redis/es/mongo/kafka/river；管理端返回密钥明文（secretKey/authCode/APIKey/clientSecret/JWT secret 等），便于直接查看与编辑
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | path | string | 是 | 配置名称（site/logger/email/ai/transcode/thirdLogin/jwt） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 单项配置对象 |

响应示例（按 name 分类）:

name=site

```json
{
  "code": 0,
  "data": {
    "defaultUserBannerID": 19,
    "defaultUserBannerIDs": [19, 21],
    "contentReview": {
      "enable": false
    },
    "login": {
      "qqLogin": true,
      "googleLogin": true,
      "githubLogin": true,
      "linuxdoLogin": true,
      "xLogin": true,
      "usernamePwdLogin": true,
      "textGraphicCaptcha": false,
      "textClickCaptcha": false,
      "textClickCaptchaTTL": 300,
      "textClickCaptchaPadding": 20
    },
    "register": {
      "emailCaptcha": false,
      "textGraphicCaptcha": false,
      "slideCaptcha": false,
      "slideCaptchaTTL": 300,
      "slideCaptchaPadding": 10
    },
    "storage": {
      "maxChunkSize": 10,
      "chunkSize": 10,
      "maxFileSize": 100,
      "maxUploadNum": 10,
      "chunkDir": "./runtime/chunks",
      "local": {
        "enable": false,
        "path": "uploads"
      },
      "minio": {
        "userUploadPrefix": "user_files",
        "enable": true,
        "endpoint": "172.17.209.73:9000",
        "accessKey": "minioadmin",
        "secretKey": "minio-secret",
        "bucket": "videos",
        "useSSL": false,
        "publicPrefixes": ["user_files", "covers", "images"]
      }
    }
  },
  "msg": "ok"
}
```

name=logger

```json
{
  "code": 0,
  "data": {
    "debugFileName": "./runtime/debug/f_ccg_video-debug.log",
    "infoFileName": "./runtime/info/f_ccg_video-info.log",
    "warnFileName": "./runtime/warn/f_ccg_video-warn.log",
    "errorFileName": "./runtime/error/f_ccg_video-error.log",
    "enableConsole": true,
    "maxSize": 500,
    "maxAge": 28,
    "maxBackups": 3,
    "development": true
  },
  "msg": "ok"
}
```

name=email

```json
{
  "code": 0,
  "data": {
    "domain": "smtp.qq.com",
    "port": 587,
    "sendEmail": "2740451290@qq.com",
    "authCode": "smtp-auth-code",
    "sendNickName": "柳冠一的视频网站",
    "ssl": true,
    "tls": true,
    "isExpire": 10
  },
  "msg": "ok"
}
```

name=ai

```json
{
  "code": 0,
  "data": {
    "chatModelBaseURL": "https://api.deepseek.com/anthropic",
    "chatModelAPIKey": "sk-chat-key",
    "chatModel": "deepseek-v4-flash",
    "embeddingModel": "doubao-embedding-vision-251215",
    "embeddingDimensions": 2048,
    "doubaoModelAPIKey": "sk-doubao-key",
    "systemPrompt": "你是站内视频智能助手，找视频时返回 video_results 链接块。",
    "imageModel": "doubao-seedream-5-0-lite-260128",
    "imageModels": [],
    "videoModel": "doubao-seedance-1-5-pro-251215",
    "videoModels": [],
    "vectorIndex": "search_videos_vector",
    "vectorTopK": 8,

    "timeoutSec": 90
  },
  "msg": "ok"
}
```

name=transcode

```json
{
  "code": 0,
  "data": {
    "maxWorkers": 2,
    "transcodeResolutions": [360, 720, 1080, 2160],
    "highBitrateThreshold": 8000,
    "bitrate2160KbpsReduceRatio": 20,
    "bitrate1080KbpsReduceRatio": 40,
    "bitrate720KbpsReduceRatio": 60,
    "bitrate360KbpsReduceRatio": 80,
    "cpuMode": true,
    "crfHigh": 18,
    "crfMedium": 23,
    "crf720": 24,
    "crf360": 28,
    "cpuPreset": "medium",
    "gpuCQPHigh": 18,
    "gpuCQPMedium": 22,
    "gpuCQP720": 24,
    "gpuCQP360": 28,
    "gpuPreset": "p4",
    "gpuMode": true,
    "threads": 8,
    "hardwareScale": false
  },
  "msg": "ok"
}
```

name=thirdLogin

```json
{
  "code": 0,
  "data": {
    "qq": {
      "appID": "102799890",
      "appKey": "qq-app-key",
      "redirect": "http://blog.flameccg.space/oauth"
    },
    "google": {
      "clientID": "381890901654-8ult37ugk4sgu5qo7r1m36b6ger9dqk9.apps.googleusercontent.com",
      "clientSecret": "oauth-client-secret",
      "redirect": "http://localhost:3000/oauth/google"
    },
    "github": {
      "clientID": "Iv1.examplegithubclient",
      "clientSecret": "oauth-client-secret",
      "redirect": "http://localhost:3000/oauth/github"
    },
    "x": {
      "clientID": "example-x-client-id",
      "clientSecret": "oauth-client-secret",
      "redirect": "http://localhost:3000/oauth/x"
    },
    "linuxdo": {
      "clientID": "example-linuxdo-client-id",
      "clientSecret": "oauth-client-secret",
      "redirect": "http://localhost:3000/oauth/linuxdo"
    }
  },
  "msg": "ok"
}
```

name=jwt

```json
{
  "code": 0,
  "data": {
    "accessExp": 7,
    "refreshExp": 30,
    "accessSecret": "jwt-access-secret",
    "refreshSecret": "jwt-refresh-secret"
  },
  "msg": "ok"
}
```

## [PUT] 更新站点配置（按名称）

- 接口路径: PUT /admin/site/config
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要 RBAC 权限
- 依赖接口: 无
- 接口说明: 更新单项站点配置（需管理员权限）
- 说明: 不支持 mysql/redis/es/mongo/kafka/river；密钥字段提交空字符串或历史占位符 `******` 时服务端保留原值，避免误覆盖
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | path | string | 是 | 配置名称（site/logger/email/ai/transcode/thirdLogin/jwt） |

请求体:

- 直接提交该配置对应的 JSON 对象（字段同配置）

请求体示例（按 name 分类）:

name=site

```json
{
  "defaultUserBannerID": 19,
  "defaultUserBannerIDs": [19, 21],
  "contentReview": {
    "enable": false
  },
  "login": {
    "qqLogin": true,
    "googleLogin": true,
    "githubLogin": true,
    "linuxdoLogin": true,
    "xLogin": true,
    "usernamePwdLogin": true,
    "textGraphicCaptcha": false,
    "textClickCaptcha": false,
    "textClickCaptchaTTL": 300,
    "textClickCaptchaPadding": 20
  },
  "register": {
    "emailCaptcha": false,
    "textGraphicCaptcha": false,
    "slideCaptcha": false,
    "slideCaptchaTTL": 300,
    "slideCaptchaPadding": 10
  },
  "storage": {
    "maxChunkSize": 10,
    "chunkSize": 10,
    "maxFileSize": 100,
    "maxUploadNum": 10,
    "chunkDir": "./runtime/chunks",
    "local": {
      "enable": false,
      "path": "uploads"
    },
    "minio": {
      "userUploadPrefix": "user_files",
      "enable": true,
      "endpoint": "172.17.209.73:9000",
      "accessKey": "minioadmin",
      "secretKey": "minio-secret",
      "bucket": "videos",
      "useSSL": false,
      "publicPrefixes": ["user_files", "covers", "images"]
    }
  }
}
```

name=logger

```json
{
  "debugFileName": "./runtime/debug/f_ccg_video-debug.log",
  "infoFileName": "./runtime/info/f_ccg_video-info.log",
  "warnFileName": "./runtime/warn/f_ccg_video-warn.log",
  "errorFileName": "./runtime/error/f_ccg_video-error.log",
  "enableConsole": true,
  "maxSize": 500,
  "maxAge": 28,
  "maxBackups": 3,
  "development": true
}
```

name=email

```json
{
  "domain": "smtp.qq.com",
  "port": 587,
  "sendEmail": "2740451290@qq.com",
  "authCode": "smtp-auth-code",
  "sendNickName": "柳冠一的视频网站",
  "ssl": true,
  "tls": true,
  "isExpire": 10
}
```

name=ai

```json
{
  "chatModelBaseURL": "https://api.deepseek.com/anthropic",
  "chatModelAPIKey": "sk-your-chat-api-key",
  "chatModel": "deepseek-v4-flash",
  "embeddingModel": "doubao-embedding-vision-251215",
  "embeddingDimensions": 2048,
  "doubaoModelAPIKey": "ark-your-doubao-api-key",
  "systemPrompt": "你是站内视频智能助手，找视频时返回 video_results 链接块。",
  "imageModel": "doubao-seedream-5-0-lite-260128",
  "imageModels": [],
  "videoModel": "doubao-seedance-1-5-pro-251215",
  "videoModels": [],
  "vectorIndex": "search_videos_vector",
  "vectorTopK": 8,

  "timeoutSec": 90
}
```

name=transcode

```json
{
  "maxWorkers": 2,
  "transcodeResolutions": [360, 720, 1080, 2160],
  "highBitrateThreshold": 8000,
  "bitrate2160KbpsReduceRatio": 20,
  "bitrate1080KbpsReduceRatio": 40,
  "bitrate720KbpsReduceRatio": 60,
  "bitrate360KbpsReduceRatio": 80,
  "cpuMode": true,
  "crfHigh": 18,
  "crfMedium": 23,
  "crf720": 24,
  "crf360": 28,
  "cpuPreset": "medium",
  "gpuCQPHigh": 18,
  "gpuCQPMedium": 22,
  "gpuCQP720": 24,
  "gpuCQP360": 28,
  "gpuPreset": "p4",
  "gpuMode": true,
  "threads": 8,
  "hardwareScale": false
}
```

name=thirdLogin

```json
{
  "qq": {
    "appID": "102799890",
    "appKey": "qq-app-key",
    "redirect": "http://blog.flameccg.space/oauth"
  },
  "google": {
    "clientID": "381890901654-8ult37ugk4sgu5qo7r1m36b6ger9dqk9.apps.googleusercontent.com",
    "clientSecret": "oauth-client-secret",
    "redirect": "http://localhost:3000/oauth/google"
  },
  "github": {
    "clientID": "Iv1.examplegithubclient",
    "clientSecret": "oauth-client-secret",
    "redirect": "http://localhost:3000/oauth/github"
  },
  "x": {
    "clientID": "example-x-client-id",
    "clientSecret": "oauth-client-secret",
    "redirect": "http://localhost:3000/oauth/x"
  },
  "linuxdo": {
    "clientID": "example-linuxdo-client-id",
    "clientSecret": "oauth-client-secret",
    "redirect": "http://localhost:3000/oauth/linuxdo"
  }
}
```

name=jwt

```json
{
  "accessExp": 7,
  "refreshExp": 30,
  "accessSecret": "jwt-access-secret",
  "refreshSecret": "jwt-refresh-secret"
}
```

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 空对象 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```
