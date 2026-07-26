# 用户管理

分类说明：注册登录、用户信息、设置、记录

Base URL：/v1

## [POST] 邮箱注册

- 接口路径: POST /common/user/register/email
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: POST /common/captcha/email
- 接口说明: 通过邮箱注册新用户；滑块验证码在发送邮箱验证码阶段校验，注册阶段仅校验邮箱验证码
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "注册成功"
}
```

## [POST] 用户名密码登录

- 接口路径: POST /common/user/login/pwd
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 图形/滑块/点选验证码接口
- 接口说明: 使用用户名/邮箱和密码登录，需要先通过点击验证码
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| username | body | string | 是 | 用户名或邮箱 |
| password | body | string | 是 | 密码 |
| captchaToken | body | string | 是 | 点击验证码token（来自 /common/captcha/click） |
| captchaDots | body | array<ClickCaptchaPoint> | 是 | 点击点位列表（按顺序） |
| captchaDots[].index | body | integer | 是 | 点击顺序索引（从0开始） |
| captchaDots[].x | body | integer | 是 | 点击的X坐标 |
| captchaDots[].y | body | integer | 是 | 点击的Y坐标 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] QQ登录

- 接口路径: POST /common/user/login/qq
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 使用QQ授权码登录，未注册用户会自动注册
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| code | body | string | 否 | QQ授权码 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] Google登录

- 接口路径: POST /common/user/login/google
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 使用Google授权码登录，未注册用户会自动注册
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| code | body | string | 否 | Google授权码 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] GitHub登录

- 接口路径: POST /common/user/login/github
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 使用GitHub授权码登录，未注册用户会自动注册；需回传与获取登录URL时一致的 state，支持可选 PKCE codeVerifier
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| code | body | string | 是 | GitHub授权码 |
| state | body | string | 是 | GitHub登录流程中的 state；后端会校验并一次性消费 |
| codeVerifier | body | string | 否 | PKCE codeVerifier，若生成登录URL时传了 codeChallenge 则需要回传 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] LinuxDo登录

- 接口路径: POST /common/user/login/linuxdo
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 使用 LinuxDo 授权码登录，未注册用户会自动注册；需回传与获取登录URL时一致的 state
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| code | body | string | 是 | LinuxDo 授权码 |
| state | body | string | 是 | LinuxDo 登录流程中的 state；后端会校验并一次性消费 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] X登录

- 接口路径: POST /common/user/login/x
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 使用X授权码登录，未注册用户会自动注册；需回传与获取登录URL时一致的 state，并配合 PKCE 的 codeVerifier
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| code | body | string | 是 | X授权码 |
| state | body | string | 是 | X登录流程中的 state；后端会校验并一次性消费 |
| codeVerifier | body | string | 是 | X PKCE codeVerifier |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "登录成功"
}
```

## [POST] 刷新访问令牌

- 接口路径: POST /common/user/login/refresh
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 登录接口获取 refreshToken
- 接口说明: 使用 refreshToken 刷新访问令牌
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| refreshToken | body | string | 是 | 刷新令牌 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | JwtToken | - |
| data.accessToken | string | 访问令牌 |
| data.refreshToken | string | 刷新令牌 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
  },
  "msg": "刷新成功"
}
```

## [GET] 搜索用户

- 接口路径: GET /common/user/search
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 根据关键词搜索用户
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| keyword | query | string | 是 | 搜索关键词 |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| userSort | query | integer | 否 | 排序方式（0相关性 1粉丝数 2等级 3注册时间） 可选: 0/1/2/3 |
| userOrder | query | integer | 否 | 排序顺序（0降序 1升序） 可选: 0/1 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | UserSearchResult | - |
| data.userTotal | integer(int64) | - |
| data.users | array<SearchUserHit> | - |
| data.users[].id | integer(uint) | - |
| data.users[].username | string | - |
| data.users[].avatar | string | - |
| data.users[].highlight | SearchHighlight | - |
| data.users[].level | integer | - |
| data.users[].followerCount | integer(int64) | - |

响应示例:

```json
{
  "code": 0,
  "data": {
    "userTotal": 1,
    "users": [
      {
        "id": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "highlight": {
          "key": ["value"]
        },
        "level": 3,
        "followerCount": 1
      }
    ]
  },
  "msg": "搜索成功"
}
```

## [GET] 获取用户详情

- 接口路径: GET /common/user/detail
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 根据用户ID获取用户公开信息
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | UserDetail | - |
| data.id | integer(uint) | 用户ID |
| data.username | string | 用户名 |
| data.avatar | string | 头像URL |
| data.bannerUrl | string | 用户主页横幅URL |
| data.description | string | 个人简介 |
| data.gender | integer | 性别（0未知 1男 2女） |
| data.birthday | string | 生日（YYYY-MM-DD格式） |
| data.level | integer | 用户等级 |
| data.exp | integer(int64) | 经验值 |
| data.followCount | integer(int64) | 关注数 |
| data.fansCount | integer(int64) | 粉丝数 |
| data.totalLikes | integer(int64) | 获赞总数 |
| data.totalViews | integer(int64) | 播放总数 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "id": 1001,
    "username": "alice",
    "avatar": "https://cdn.example.com/avatar/1001.png",
    "bannerUrl": "https://example.com/page",
    "description": "示例说明",
    "gender": 1,
    "birthday": "2024-06-01",
    "level": 3,
    "exp": 120,
    "followCount": 1,
    "fansCount": 1,
    "totalLikes": 1,
    "totalViews": 1
  },
  "msg": "获取成功"
}
```

## [GET] 获取当前用户信息

- 接口路径: GET /common/user/info
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前登录用户的详细信息（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | UserInfo | - |
| data.id | integer(uint) | 用户ID |
| data.username | string | 用户名 |
| data.email | string | 邮箱 |
| data.avatar | string | 头像URL |
| data.bannerUrl | string | 用户主页横幅URL |
| data.description | string | 个人简介 |
| data.gender | integer | 性别（0未知 1男 2女） |
| data.birthday | string | 生日（YYYY-MM-DD格式） |
| data.level | integer | 用户等级 |
| data.exp | integer(int64) | 经验值 |
| data.coinCount | number(double) | 硬币数量（支持1位小数） |
| data.followCount | integer(int64) | 关注数 |
| data.fansCount | integer(int64) | 粉丝数 |
| data.dynamicCount | integer(int64) | 动态数量 |
| data.registerSource | string | 注册来源 |
| data.likeTags | array<string> | 用户兴趣标签列表 |
| data.hasPassword | boolean | 是否已设置登录密码（第三方首次登录可能为 false） |
| data.isThirdParty | boolean | 是否绑定第三方登录（OpenID 非空） |

响应示例:

```json
{
  "code": 0,
  "data": {
    "id": 1001,
    "username": "alice",
    "email": "alice@example.com",
    "avatar": "https://cdn.example.com/avatar/1001.png",
    "bannerUrl": "https://example.com/page",
    "description": "示例说明",
    "gender": 1,
    "birthday": "2024-06-01",
    "level": 3,
    "exp": 120,
    "coinCount": 1.23,
    "followCount": 1,
    "fansCount": 1,
    "dynamicCount": 1,
    "registerSource": "email",
    "likeTags": ["动画", "音乐"],
    "hasPassword": true,
    "isThirdParty": false
  },
  "msg": "获取成功"
}
```

## [PUT] 更新用户信息

- 接口路径: PUT /common/user/info
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 更新当前登录用户的信息（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| username | body | string | 否 | 用户名（可选） |
| avatar | body | string | 否 | 头像URL（可选） |
| description | body | string | 否 | 个人简介（可选） |
| gender | body | integer | 否 | 性别（0未知 1男 2女）（可选） |
| birthday | body | string | 否 | 生日（YYYY-MM-DD格式）（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [POST] 绑定邮箱

- 接口路径: POST /common/user/email/bind
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 邮箱验证码接口
- 接口说明: 绑定邮箱（需登录，邮箱验证码类型3）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| emailID | body | string | 是 | 邮箱验证码ID |
| emailCode | body | string | 是 | 邮箱验证码 |
| email | body | string | 是 | 邮箱 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "绑定成功"
}
```

## [POST] 忘记密码重置

- 接口路径: POST /common/user/password/reset
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 邮箱验证码接口
- 接口说明: 通过邮箱验证码重置密码（验证码类型2）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| emailID | body | string | 是 | 邮箱验证码ID |
| emailCode | body | string | 是 | 邮箱验证码 |
| email | body | string | 是 | 邮箱 |
| newPassword | body | string | 是 | 新密码 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "重置成功"
}
```

## [PUT] 修改密码

- 接口路径: PUT /common/user/password/change
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 修改当前用户密码（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| oldPassword | body | string | 是 | 旧密码 |
| newPassword | body | string | 是 | 新密码 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "修改成功"
}
```

## [GET] 获取用户配置

- 接口路径: GET /common/user/info/conf
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前登录用户的隐私配置（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "获取成功"
}
```

## [PUT] 更新用户配置

- 接口路径: PUT /common/user/info/conf
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 更新当前登录用户的隐私配置（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| openCollect | body | boolean | 否 | 是否公开收藏（可选） |
| openFans | body | boolean | 否 | 是否公开粉丝（可选） |
| openFollow | body | boolean | 否 | 是否公开关注（可选） |
| openLikeVideo | body | boolean | 否 | 是否公开点赞视频（可选） |
| openCoinVideo | body | boolean | 否 | 是否公开投币视频（可选） |
| bannerId | body | integer(uint) | 否 | 系统默认横幅ID（可选，仅允许传系统默认横幅列表中的ID；传入后会清空已有 bannerUrl） |
| bannerUrl | body | string | 否 | 自定义横幅URL（可选） |
| likeTags | body | array<string> | 否 | 喜欢的标签列表（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "更新成功"
}
```

## [GET] 获取系统默认用户主页横幅列表

- 接口路径: GET /common/user/banner/defaults
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取系统配置的默认用户主页横幅列表，返回默认横幅列表中的 bannerId 顺序，并在列表末尾补充注册默认横幅（若未重复配置）及对应可展示的横幅详情
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.bannerIds | array<integer(uint)> | 系统默认横幅ID列表；若注册默认横幅未出现在默认列表中，也会追加返回 |
| data.currentBannerId | integer(uint) | 当前用户配置的 bannerId；若用户使用自定义 bannerUrl，则固定返回 0 |
| data.currentBannerImageUrl | string | 当前用户配置的 bannerId 对应图片URL；若用户使用自定义 bannerUrl，则返回自定义图片URL |
| data.list | array<BannerItem> | 实际查询到的可展示横幅列表（按 bannerIds 顺序返回） |
| data.list[].type | integer | 轮播图类型（1首页/分区轮播图 2顶部横幅 3用户主页横幅） |

响应示例:

```json
{
  "code": 0,
  "data": {
    "bannerIds": [21, 19],
    "currentBannerId": 19,
    "currentBannerImageUrl": "https://cdn.example.com/banner-user-19.jpg",
    "list": [
      {
        "id": 21,
        "cover": "https://cdn.example.com/banner-user-21.jpg",
        "href": "",
        "show": true,
        "type": 3,
        "partitionId": 0,
        "createdAt": "2024-06-01T12:00:00Z",
        "updatedAt": "2024-06-01T12:00:00Z"
      },
      {
        "id": 19,
        "cover": "https://cdn.example.com/banner-user-19.jpg",
        "href": "",
        "show": true,
        "type": 3,
        "partitionId": 0,
        "createdAt": "2024-06-01T12:00:00Z",
        "updatedAt": "2024-06-01T12:00:00Z"
      }
    ]
  },
  "msg": "获取成功"
}
```

## [POST] 上传用户主页横幅

- 接口路径: POST /common/user/banner/upload
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 上传用户主页横幅图片（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| fileHash | body | string | 是 | 文件哈希值 |
| banner | body | string(binary) | 是 | 横幅图片文件 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.bannerUrl | string | 横幅图片URL |

响应示例:

```json
{
  "code": 0,
  "data": {
    "bannerUrl": "https://example.com/page"
  },
  "msg": "上传成功"
}
```

## [GET] @联想用户

- 接口路径: GET /common/user/mention/suggest
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 提供评论@联想功能，按好友优先返回关注列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| keyword | query | string | 否 | 搜索关键词（可选） |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| keyword | query | string | 否 | 搜索关键词 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<MentionUser> | - |
| data.list[].id | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 头像URL |
| data.list[].followerCount | integer(int64) | 粉丝数 |
| data.list[].isFriend | boolean | 是否互相关注 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "followerCount": 1,
        "isFriend": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 登录IP记录

- 接口路径: GET /common/user/record/login-ip
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前用户登录IP记录（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserLoginRecordItem> | - |
| data.list[].id | integer(uint) | 记录ID |
| data.list[].ip | string | 登录IP |
| data.list[].addr | string | IP归属地 |
| data.list[].userAgent | string | User-Agent |
| data.list[].loginType | string | 登录方式（pwd/qq） |
| data.list[].createdAt | string(date-time) | 登录时间 |
| data.total | integer(int64) | - |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "ip": "127.0.0.1",
        "addr": "中国-北京",
        "userAgent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
        "loginType": "pwd",
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 经验记录

- 接口路径: GET /common/user/record/exp
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前用户经验记录（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserExpRecordItem> | - |
| data.list[].id | integer(uint) | 记录ID |
| data.list[].delta | integer(int64) | 经验变化 |
| data.list[].reason | string | 原因 |
| data.list[].createdAt | string(date-time) | 变更时间 |
| data.total | integer(int64) | - |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "delta": 1,
        "reason": "示例原因",
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 硬币记录

- 接口路径: GET /common/user/record/coin
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前用户硬币记录（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserCoinRecordItem> | - |
| data.list[].id | integer(uint) | 记录ID |
| data.list[].delta | number(double) | 硬币数量变化（支持1位小数，可为负） |
| data.list[].reason | string | 原因 |
| data.list[].createdAt | string(date-time) | 变更时间 |
| data.total | integer(int64) | - |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "delta": 1.23,
        "reason": "示例原因",
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 创作中心数据分析

- 接口路径: GET /common/user/creator/analytics
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 按指标类型返回单个趋势序列，支持近7天/近30天/自然月（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| range | query | string | 否 | 时间范围 可选: 7d/30d/month |
| type | query | string | 否 | 指标类型 可选: fans/views/comments/coins/danmu/favorites |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | CreatorAnalyticsTrendResult | - |
| data.range | string | 时间范围（7d/30d/month） |
| data.type | string | 指标类型（fans/views/comments/coins/danmu/favorites） |
| data.x | array<string> | 日期轴（YYYY-MM-DD） |
| data.values | array<integer(int64)> | 对应指标的按天增量值 |
| data.total | integer(int64) | 当前时间范围内该指标总增量 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "range": "2024-06-01T12:00:00Z",
    "type": "pwd",
    "x": ["2024-06-01"],
    "values": [1],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 创作中心总览

- 接口路径: GET /common/user/creator/overview
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 返回播放/粉丝/评论/硬币/弹幕/收藏六项总量（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | CreatorOverview | - |
| data.fans | integer(int64) | 粉丝总量 |
| data.views | integer(int64) | 播放总量 |
| data.comments | integer(int64) | 评论总量 |
| data.coins | integer(int64) | 硬币总量 |
| data.danmu | integer(int64) | 弹幕总量 |
| data.favorites | integer(int64) | 收藏总量 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "fans": 1,
    "views": 1,
    "comments": 1,
    "coins": 5,
    "danmu": 1,
    "favorites": 1
  },
  "msg": "获取成功"
}
```

## [GET] 用户视频列表

- 接口路径: GET /common/user/video/list
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户的视频列表
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 是 | 用户ID |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| keyword | query | string | 否 | 关键字搜索，仅匹配用户名 |
| sort | query | integer | 否 | 排序方式（0最新 1最多播放 2最多收藏） 可选: 0/1/2 |
| auditStatus | query | integer | 否 | 状态筛选（1已发布 2私密 4审核中，仅本人查看自己的视频列表时生效；不传时本人默认排除已删除视频，非本人访问时会被忽略） 可选: 1/2/4 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserVideoItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].description | string | 视频描述 |
| data.list[].cover | string | 封面 |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].progress | integer | 播放进度（秒） |
| data.list[].createdAt | string(date-time) | 创建时间 |
| data.list[].status | integer | 视频状态（1已发布 2私密 4审核中） |
| data.list[].statusText | string | 视频状态文案 |
| data.list[].auditStatus | integer | 状态（1已发布 2私密 4审核中，仅本人查看自己的视频列表时返回） |
| data.list[].auditStatusText | string | 状态文案，仅本人查看自己的视频列表时返回 |
| data.list[].authorId | integer(uint) | 作者ID |
| data.list[].authorName | string | 作者名称 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1001,
        "title": "示例标题",
        "description": "示例说明",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "views": 1,
        "danmuCount": 1,
        "duration": 60,
        "progress": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "status": 1,
        "statusText": "enabled",
        "auditStatus": 1,
        "auditStatusText": "enabled",
        "authorId": 1001,
        "authorName": "示例名称"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 最近点赞的视频

- 接口路径: GET /common/user/video/recent/liked
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户最近点赞的10条视频
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userID | query | integer | 是 | 用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserVideoItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].description | string | 视频描述 |
| data.list[].cover | string | 封面 |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].progress | integer | 播放进度（秒） |
| data.list[].createdAt | string(date-time) | 创建时间 |
| data.list[].status | integer | 视频状态（1已发布 2私密 4审核中） |
| data.list[].statusText | string | 视频状态文案 |
| data.list[].auditStatus | integer | 状态（1已发布 2私密 4审核中，仅本人查看自己的视频列表时返回） |
| data.list[].auditStatusText | string | 状态文案，仅本人查看自己的视频列表时返回 |
| data.list[].authorId | integer(uint) | 作者ID |
| data.list[].authorName | string | 作者名称 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 2001,
        "title": "示例标题",
        "description": "示例说明",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "views": 1,
        "danmuCount": 1,
        "duration": 60,
        "progress": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "status": 1,
        "statusText": "enabled",
        "auditStatus": 1,
        "auditStatusText": "enabled",
        "authorId": 2001,
        "authorName": "示例名称"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 最近投币的视频

- 接口路径: GET /common/user/video/recent/coined
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户最近投币的10条视频
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userID | query | integer | 是 | 用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<UserVideoItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].description | string | 视频描述 |
| data.list[].cover | string | 封面 |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].progress | integer | 播放进度（秒） |
| data.list[].createdAt | string(date-time) | 创建时间 |
| data.list[].status | integer | 视频状态（1已发布 2私密 4审核中） |
| data.list[].statusText | string | 视频状态文案 |
| data.list[].auditStatus | integer | 状态（1已发布 2私密 4审核中，仅本人查看自己的视频列表时返回） |
| data.list[].auditStatusText | string | 状态文案，仅本人查看自己的视频列表时返回 |
| data.list[].authorId | integer(uint) | 作者ID |
| data.list[].authorName | string | 作者名称 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 2001,
        "title": "示例标题",
        "description": "示例说明",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "views": 1,
        "danmuCount": 1,
        "duration": 60,
        "progress": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "status": 1,
        "statusText": "enabled",
        "auditStatus": 1,
        "auditStatusText": "enabled",
        "authorId": 2001,
        "authorName": "示例名称"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 删除视频

- 接口路径: DELETE /common/user/video/delete
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 用户删除自己的视频（逻辑删除，需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "删除成功"
}
```

## [POST] 创建标签

- 接口路径: POST /common/user/video/tag
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 用户创建视频标签（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | body | string | 是 | 标签名称 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | Tag | - |
| data.id | integer(uint) | 标签ID |
| data.name | string | 标签名称 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "id": 7,
    "name": "示例名称"
  },
  "msg": "创建成功"
}
```
