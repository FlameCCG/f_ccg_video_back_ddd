# 通知管理

分类说明：通知与消息

Base URL：/v1

## [GET] 通知列表

- 接口路径: GET /common/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取用户通知列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| category | query | string | 否 | 通知类别（all全部 reply回复 like点赞 at@我 system系统） 可选: all/reply/like/at/system |

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
| data.list[].dynamicID | integer(uint) | 动态ID |
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
        "dynamicID": 4001,
        "commentID": 3001,
        "isRead": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 删除通知

- 接口路径: DELETE /common/notification
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 批量删除通知（需登录）
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

## [GET] 通知数量统计

- 接口路径: GET /common/notification/counts
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取各类型未读通知数量统计（需登录，包含未读私信总数 message）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:

- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | NotificationCountSummary | - |
| data.reply | integer(int64) | 未读回复通知数量 |
| data.like | integer(int64) | 未读点赞通知数量 |
| data.at | integer(int64) | 未读@我通知数量 |
| data.system | integer(int64) | 未读系统通知数量 |
| data.message | integer(int64) | 未读私信总数 |

响应示例:

```json
{
  "code": 0,
  "data": {
    "reply": 1,
    "like": 1,
    "at": 1,
    "system": 1,
    "message": 1
  },
  "msg": "获取成功"
}
```

## [PUT] 标记已读

- 接口路径: PUT /common/notification/read
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 批量标记通知为已读（需登录）
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
  "msg": "更新成功"
}
```
