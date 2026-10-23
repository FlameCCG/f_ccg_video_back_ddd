# 验证码管理

分类说明：图形/滑块/点击/邮箱验证码获取与校验

Base URL：/v1

## 交互验证码调用顺序

用户名密码登录（普通用户和管理员）使用 `scene=login`；发送邮箱验证码（注册、重置密码、绑定邮箱）统一使用 `scene=register`。

1. 调用 `GET /common/captcha/type?scene=login` 或 `?scene=register`，读取 `data.type`。
2. 返回 `click` 时获取 `GET /common/captcha/click?scene=<同一场景>`；返回 `slide` 时获取 `GET /common/captcha/slide?scene=<同一场景>`。返回 `none` 时不需要交互验证码。
3. 在登录或发邮件的请求中，只提交对应的一组凭证：点选为 `captchaToken + captchaDots`，滑块为 `slideCaptchaToken + slideCaptchaX + slideCaptchaY`。同时提交两种 token 会失败；启用交互验证码时漏交凭证或提交已关闭的类型也会失败。

点选和滑块可同时开启，此时类型接口等概率返回一种；只开一种就固定返回该类型，都关闭则返回 `none`。类型接口只选择展示类型，不签发 token，也不保存选择结果；校验端按当前场景启用的类型检查提交的凭证。token 绑定签发场景，不能跨登录与发邮件复用。登录和发邮件的开关、TTL、容错边距分别配置，不互相回退；TTL 未配置或不大于 0 时使用 300 秒，padding 为 0 表示不放宽落点。

发邮件还独立受 `site.register.textGraphicCaptcha` 控制：开启时须额外提交 `captchaID + captchaCode`，即使交互类型返回 `none` 也不能省略图形验证码。配置字段见[后台站点管理](../admin/07-admin-site-management.md#交互验证码配置)。

## [GET] 获取交互验证码类型

- 接口路径: GET /common/captcha/type
- 认证: 无需登录
- 接口说明: 按当前场景的开关返回本次展示的交互验证码类型
- HTTP 状态码: 200；`code=0` 成功，非法或缺失场景时 `code=1`、`msg=invalid captcha scene`
- 缓存: 响应头 `Cache-Control: no-store`；客户端应在刷新挑战时重新获取类型

请求参数:

| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| scene | query | string | 是 | `login` 或 `register`；无默认值 |

响应字段:

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data.type | string | `click` / `slide` / `none` |

响应示例（两种类型都开启时，本次选中滑块）:

```json
{
  "code": 0,
  "data": {
    "type": "slide"
  },
  "msg": "ok"
}
```

## [POST] 发送邮箱验证码

- 接口路径: POST /common/captcha/email
- 认证: 无需登录
- 依赖接口: 类型接口（`scene=register`），以及按配置需要的图形/滑块/点选验证码接口
- 接口说明: 发送邮箱验证码，用于注册、重置密码、绑定邮箱；交互凭证统一按 `register` 场景校验。可提交 JSON；原 multipart/form-data 入口仍保留，点选数组推荐使用 JSON。
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| type | body | integer | 是 | 验证码类型（1注册 2重置密码 3绑定邮箱） |
| email | body | string | 是 | 邮箱地址 |
| captchaID | body | string | 条件必填 | `site.register.textGraphicCaptcha=true` 时必填 |
| captchaCode | body | string | 条件必填 | 与 captchaID 一起提交的图形验证码答案 |
| captchaToken | body | string | 条件必填 | 点选类型的 token，须以 `scene=register` 签发 |
| captchaDots | body | array<ClickCaptchaPoint> | 条件必填 | 点选类型的非空点位列表，元素包含 index、x、y，按点击顺序提交 |
| slideCaptchaToken | body | string | 条件必填 | 滑块类型的 token，须以 `scene=register` 签发 |
| slideCaptchaX | body | integer | 条件必填 | 滑块类型的 X 坐标 |
| slideCaptchaY | body | integer | 条件必填 | 滑块类型的 Y 坐标 |

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

请求示例（点选；同时开启了图形验证码）:

```json
{
  "type": 1,
  "email": "alice@example.com",
  "captchaID": "graphic-id",
  "captchaCode": "1234",
  "captchaToken": "register-click-token",
  "captchaDots": [
    {"index": 0, "x": 120, "y": 80},
    {"index": 1, "x": 60, "y": 100},
    {"index": 2, "x": 200, "y": 120}
  ]
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
- 接口说明: 获取点选式验证码，支持登录和发邮件场景；后续提交时必须使用同一场景
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| scene | query | string | 否 | `login` / `register`，默认 `login`；建议始终显式传入 |

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
- 接口说明: 获取滑块式验证码，支持登录和发邮件场景；后续提交时必须使用同一场景
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| scene | query | string | 否 | `login` / `register`，默认 `register`；建议始终显式传入 |

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
