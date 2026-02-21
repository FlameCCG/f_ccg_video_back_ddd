# 验证码管理

分类说明：图形/滑块/点击/邮箱验证码获取与校验

Base URL：/v1

## [POST] 发送邮箱验证码

- 接口路径: POST /common/captcha/email
- 认证: 无需登录
- 依赖接口: 图形/滑块/点选验证码接口
- 接口说明: 发送邮箱验证码，用于注册、重置密码、绑定邮箱
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| type | body | integer | 是 | 验证码类型（1注册 2重置密码 3绑定邮箱） |
| email | body | string | 是 | 邮箱地址 |
| captchaID | body | string | 是 | 图形验证码ID |
| captchaCode | body | string | 是 | 图形验证码 |
| slideCaptchaToken | body | string | 是 | 滑块验证码Token |
| slideCaptchaX | body | integer | 是 | 滑块验证码X坐标 |
| slideCaptchaY | body | integer | 是 | 滑块验证码Y坐标 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.emailID | string | 邮箱验证码ID，后续验证时需要 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "emailID": "alice@example.com"
  },
  "msg": "发送成功"
}
```

## [GET] 获取图形验证码

- 接口路径: GET /common/captcha/graphics-text
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取图形文字验证码
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| type | query | integer | 否 | 验证码类型（1算术运算 2数字字母混合 3纯数字 4纯字母） 可选: 1/2/3/4 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.captchaID | string | 验证码ID |
| data.captchaB64 | string | Base64编码的验证码图片 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "captchaID": "captcha_123456",
    "captchaB64": "data:image/png;base64,iVBORw0KGgoAAA..."
  },
  "msg": "获取成功"
}
```

## [GET] 获取点击验证码

- 接口路径: GET /common/captcha/click
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取点击式验证码（用于登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.masterImage | string | 主图片Base64 |
| data.thumbImage | string | 缩略图Base64 |
| data.token | string | 验证token |

响应示例:
```json
{
  "code": 0,
  "data": {
    "masterImage": "https://cdn.example.com/captcha/image.png",
    "thumbImage": "https://cdn.example.com/captcha/image.png",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "获取成功"
}
```

## [GET] 获取滑块验证码

- 接口路径: GET /common/captcha/slide
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取滑块式验证码（用于注册）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.masterImage | string | 主图片Base64 |
| data.tileImage | string | 滑块图片Base64 |
| data.token | string | 验证token |
| data.thumbY | integer | 滑块Y坐标 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "masterImage": "https://cdn.example.com/captcha/image.png",
    "tileImage": "https://cdn.example.com/captcha/image.png",
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "thumbY": 1
  },
  "msg": "获取成功"
}
```
