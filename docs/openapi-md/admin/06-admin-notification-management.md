# 后台-通知管理

分类说明：通知管理

Base URL：/v1

## [POST] 发送系统通知

- 接口路径: POST /admin/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员发送系统通知（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| title | body | string | 是 | 通知标题 |
| content | body | string | 是 | 通知内容 |
| receiverId | body | integer | 是 | 接收者ID（-1表示全员） |
| videoId | body | integer | 否 | 关联视频ID（可选） |
| videoTitle | body | string | 否 | 关联视频标题（可选） |
| link | body | string | 否 | 跳转链接（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:

```json
{
  "code": 0,
  "data": {},
  "msg": "创建成功"
}
```

## [PUT] 更新系统通知

- 接口路径: PUT /admin/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员更新系统通知（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | body | integer | 是 | 通知ID |
| title | body | string | 是 | 通知标题 |
| content | body | string | 是 | 通知内容 |
| receiverId | body | integer | 是 | 接收者ID（-1表示全员） |
| videoId | body | integer | 否 | 关联视频ID（可选） |
| videoTitle | body | string | 否 | 关联视频标题（可选） |
| link | body | string | 否 | 跳转链接（可选） |

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

## [GET] 系统通知列表

- 接口路径: GET /admin/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员查看系统通知列表（需管理员权限）
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
| data.list | array<UserGlobalNotification> | - |
| data.list[].id | integer(uint) | 通知ID |
| data.list[].type | integer | 通知类型 |
| data.list[].receiverID | integer | 接收者用户ID（-1表示全员） |
| data.list[].actionUserID | integer(uint) | 操作用户ID |
| data.list[].actionUserAvatar | string | 操作用户头像 |
| data.list[].actionUserName | string | 操作用户名 |
| data.list[].title | string | 通知标题 |
| data.list[].content | string | 通知内容 |
| data.list[].link | string | 外部链接 |
| data.list[].videoID | integer(uint) | 视频ID |
| data.list[].videoTitle | string | 视频标题 |
| data.list[].commentID | integer(uint) | 评论ID |
| data.list[].isRead | boolean | 是否已读 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 5001,
        "type": 1,
        "receiverID": 1001,
        "actionUserID": 1001,
        "actionUserAvatar": "https://cdn.example.com/avatar/1001.png",
        "actionUserName": "alice",
        "title": "示例标题",
        "content": "示例内容",
        "link": "https://example.com/page",
        "videoID": 2001,
        "videoTitle": "示例标题",
        "commentID": 3001,
        "isRead": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 删除系统通知

- 接口路径: DELETE /admin/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员批量删除系统通知（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| ids | body | array<integer> | 是 | 通知ID列表 |

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
