# 后台-用户管理

分类说明：用户封禁与后台管理

Base URL：/v1

## [GET] 用户列表

- 接口路径: GET /admin/user/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取用户列表（需管理员权限，可按角色筛选）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| keyword | query | string | 否 | 用户名或邮箱关键词（可选） |
| status | query | integer | 否 | 状态（1正常 2封禁 3永久封禁） 可选: 1/2/3 |
| roleId | query | integer(uint) | 否 | 角色ID（可选） |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<AdminUserListItem> | - |
| data.list[].id | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].email | string | 邮箱 |
| data.list[].avatar | string | 头像URL |
| data.list[].status | integer | 状态（1正常 2封禁 3永久封禁） |
| data.list[].level | integer | 用户等级 |
| data.list[].exp | integer(int64) | 经验值 |
| data.list[].coinCount | number(double) | 硬币数量（支持1位小数） |
| data.list[].currentRoleId | integer(uint) | 当前角色ID |
| data.list[].registerSource | string | 注册来源 |
| data.list[].roleNames | array<string> | 角色名列表 |
| data.list[].createdAt | string(date-time) | 注册时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1001,
        "username": "alice",
        "email": "alice@example.com",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "status": 1,
        "level": 3,
        "exp": 120,
        "coinCount": 1.23,
        "currentRoleId": 1001,
        "registerSource": "email",
        "roleNames": [
          "普通用户"
        ],
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 后台当前用户信息

- 接口路径: GET /admin/user/info
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取后台当前登录用户信息（含角色）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | AdminUserInfo | - |
| data.id | integer(uint) | 用户ID |
| data.username | string | 用户名 |
| data.email | string | 邮箱 |
| data.avatar | string | 头像URL |
| data.description | string | 个人简介 |
| data.gender | integer | 性别（0未知 1男 2女） |
| data.birthday | string | 生日（YYYY-MM-DD格式） |
| data.level | integer | 用户等级 |
| data.exp | integer(int64) | 经验值 |
| data.coinCount | number(double) | 硬币数量（支持1位小数） |
| data.followCount | integer(int64) | 关注数 |
| data.fansCount | integer(int64) | 粉丝数 |
| data.dynamicCount | integer(int64) | 动态数量 |
| data.currentRoleID | integer(uint) | 当前角色ID |
| data.registerSource | string | 注册来源 |
| data.roleNames | array<string> | 角色名列表 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 1001,
    "username": "alice",
    "email": "alice@example.com",
    "avatar": "https://cdn.example.com/avatar/1001.png",
    "description": "示例说明",
    "gender": 1,
    "birthday": "2024-06-01",
    "level": 3,
    "exp": 120,
    "coinCount": 1.23,
    "followCount": 1,
    "fansCount": 1,
    "dynamicCount": 1,
    "currentRoleID": 1001,
    "registerSource": "email",
    "roleNames": [
      "普通用户"
    ]
  },
  "msg": "获取成功"
}
```

## [PUT] 管理员更新用户信息

- 接口路径: PUT /admin/user/info
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员更新指定用户信息（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | body | integer(uint) | 是 | 用户ID |
| username | body | string | 否 | 用户名 |
| avatar | body | string | 否 | 头像URL |
| description | body | string | 否 | 个人简介 |
| gender | body | integer | 否 | 性别（0未知 1男 2女） |
| birthday | body | string | 否 | 生日（YYYY-MM-DD格式） |

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

## [PUT] 管理员设置用户主页横幅

- 接口路径: PUT /admin/user/banner
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 设置全局默认用户主页横幅（需管理员权限；banner 必须为 type=3 且 show=true；会同步写入配置文件）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| bannerId | body | integer(uint) | 是 | 全局默认用户主页横幅ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "设置成功"
}
```

## [PUT] 管理员增删系统默认用户主页横幅列表

- 接口路径: PUT /admin/user/banner/defaults
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 按操作类型维护系统默认用户主页横幅ID列表；action=1 为添加，action=2 为删除；仅允许 type=3 且 show=true 的 banner 进入列表；会同步更新配置文件和内存值
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| bannerIds | body | array<integer(uint)> | 是 | 要添加或删除的用户主页横幅ID列表 |
| action | body | integer | 是 | 操作类型（1添加 2删除） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.bannerIds | array<integer(uint)> | 更新后的系统默认横幅ID列表 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "bannerIds": [
      19,
      21
    ]
  },
  "msg": "ok"
}
```

## [POST] 管理员用户名密码登录

- 接口路径: POST /admin/user/login/pwd
- 认证: 无需登录
- 权限: 无需RBAC权限
- 依赖接口: 图形/滑块/点选验证码接口
- 接口说明: 管理员用户名密码登录（需通过滑块验证码）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| username | body | string | 是 | 用户名或邮箱 |
| password | body | string | 是 | 密码 |
| slideCaptchaToken | body | string | 是 | 滑块验证码Token（来自 /common/captcha/slide） |
| slideCaptchaX | body | integer | 是 | 滑块验证码X坐标 |
| slideCaptchaY | body | integer | 是 | 滑块验证码Y坐标 |

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

## [PUT] 切换用户角色

- 接口路径: PUT /admin/user/role/switch
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 切换当前登录用户的角色（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| roleId | body | integer(uint) | 是 | 角色ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "切换成功"
}
```

## [POST] 封禁/解封用户

- 接口路径: POST /admin/user/ban
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 设置用户封禁状态（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | body | integer | 是 | 用户ID |
| days | body | integer | 否 | 封禁天数（状态为封禁时必填） |
| reason | body | string | 否 | 封禁原因 |
| status | body | integer | 否 | 状态（1正常 2封禁 3永久封禁；不传时按days判断） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.id | integer | - |
| data.userId | integer | - |
| data.status | integer | - |
| data.reason | string | - |
| data.days | integer | - |
| data.startAt | string(date-time) | - |
| data.endAt | string(date-time) | - |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 1001,
    "userId": 1001,
    "status": 1,
    "reason": "示例原因",
    "days": 1,
    "startAt": "2024-06-01T12:00:00Z",
    "endAt": "2024-06-01T12:00:00Z"
  },
  "msg": "操作成功"
}
```

## [GET] 封禁记录列表

- 接口路径: GET /admin/user/ban/records
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取用户封禁记录（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 否 | 用户ID（可选） |
| username | query | string | 否 | 用户名（可选，模糊匹配） |
| status | query | integer | 否 | 状态（1正常 2封禁 3永久封禁） 可选: 1/2/3 |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<AdminBanRecordItem> | - |
| data.list[].id | integer(uint) | 记录ID |
| data.list[].userId | integer(uint) | 被封禁用户ID |
| data.list[].username | string | 用户名 |
| data.list[].status | integer | 用户状态（1正常 2封禁中 3永久封禁） |
| data.list[].reason | string | 封禁原因 |
| data.list[].days | integer | 封禁天数 |
| data.list[].startAt | string(date-time) | 封禁开始时间 |
| data.list[].endAt | string(date-time) | 封禁结束时间 |
| data.list[].operatorId | integer(uint) | 操作人ID |
| data.list[].operatorUsername | string | 操作人用户名 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "userId": 1001,
        "username": "alice",
        "status": 1,
        "reason": "示例原因",
        "days": 1,
        "startAt": "2024-06-01T12:00:00Z",
        "endAt": "2024-06-01T12:00:00Z",
        "operatorId": 1,
        "operatorUsername": "alice",
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```
