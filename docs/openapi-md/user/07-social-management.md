# 社交管理

分类说明：关注/粉丝/关系链

Base URL：/v1

## [POST] 关注用户

- 接口路径: POST /common/social/focus
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 关注指定用户（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| focusUserId | body | integer | 是 | 要关注的用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "关注成功"
}
```

## [DELETE] 取消关注

- 接口路径: DELETE /common/social/focus
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 取消关注指定用户（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| focusUserId | body | integer | 是 | 要取消关注的用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "取消关注成功"
}
```

## [GET] 关注列表

- 接口路径: GET /common/social/focus
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户的关注列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 用户ID |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<SocialUserInfo> | - |
| data.list[].id | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 头像URL |
| data.list[].description | string | 个人简介 |
| data.list[].level | integer | 用户等级 |

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
        "description": "示例说明",
        "level": 3
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 粉丝列表

- 接口路径: GET /common/social/fans
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户的粉丝列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 用户ID |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<SocialUserInfo> | - |
| data.list[].id | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 头像URL |
| data.list[].description | string | 个人简介 |
| data.list[].level | integer | 用户等级 |

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
        "description": "示例说明",
        "level": 3
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 查询关注关系

- 接口路径: GET /common/social/relation
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 查询与指定用户的关注关系（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | path | integer | 是 | 目标用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.isFocus | boolean | 我是否关注了他 |
| data.isFans | boolean | 他是否关注了我 |
| data.isMutualFollow | boolean | 是否互相关注 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "isFocus": true,
    "isFans": true,
    "isMutualFollow": true
  },
  "msg": "查询成功"
}
```
