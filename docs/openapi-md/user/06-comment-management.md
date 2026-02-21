# 评论管理

分类说明：评论、回复与互动

Base URL：/v1

## [GET] 评论列表

- 接口路径: GET /common/comment/list
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取视频或动态的评论列表
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 否 | 视频ID（与dynamicId二选一） |
| dynamicId | query | integer | 否 | 动态ID（与videoId二选一） |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| sortBy | query | string | 否 | 排序方式（time时间 hot热度） 可选: time/hot |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<CommentItem> | - |
| data.list[].id | integer(uint) | 评论ID |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 用户头像 |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].dynamicId | integer(uint) | 动态ID |
| data.list[].content | string | 评论内容 |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].replyCount | integer(int64) | 回复数 |
| data.list[].isPinned | boolean | 是否置顶 |
| data.list[].pinnedAt | string(date-time) | 置顶时间 |
| data.list[].isLiked | boolean | 当前用户是否已点赞 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 3001,
        "userId": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "videoId": 2001,
        "dynamicId": 3001,
        "content": "示例内容",
        "likeCount": 1,
        "replyCount": 1,
        "isPinned": true,
        "pinnedAt": "2024-06-01T12:00:00Z",
        "isLiked": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 回复列表

- 接口路径: GET /common/comment/replies
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取评论的回复列表
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| rootId | query | integer | 是 | 根评论ID |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<CommentItem> | - |
| data.list[].id | integer(uint) | 评论ID |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 用户头像 |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].dynamicId | integer(uint) | 动态ID |
| data.list[].content | string | 评论内容 |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].replyCount | integer(int64) | 回复数 |
| data.list[].isPinned | boolean | 是否置顶 |
| data.list[].pinnedAt | string(date-time) | 置顶时间 |
| data.list[].isLiked | boolean | 当前用户是否已点赞 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 3001,
        "userId": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "videoId": 2001,
        "dynamicId": 4001,
        "content": "示例内容",
        "likeCount": 1,
        "replyCount": 1,
        "isPinned": true,
        "pinnedAt": "2024-06-01T12:00:00Z",
        "isLiked": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [POST] 发表评论

- 接口路径: POST /common/comment/create
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 发表评论或回复（需登录）；`videoId`/`dynamicId` 二选一；`content` 与 `pictures` 不能同时为空
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 否 | 视频ID（与dynamicId二选一） |
| dynamicId | body | integer | 否 | 动态ID（与videoId二选一） |
| content | body | string | 否 | 评论内容（可与 pictures 组合；纯图评时可空） |
| parentId | body | integer | 否 | 父评论ID（回复时需要） |
| atUserIds | body | array<integer> | 否 | @的用户ID列表 |
| pictures | body | array<string> | 否 | 评论附带图片 URL 列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | CommentItem | - |
| data.id | integer(uint) | 评论ID |
| data.content | string | 评论内容 |
| data.userId | integer(uint) | 用户ID |
| data.username | string | 用户名 |
| data.avatar | string | 用户头像 |
| data.videoId | integer(uint) | 视频ID |
| data.dynamicId | integer(uint) | 动态ID |
| data.videoTitle | string | 关联视频标题（可选） |
| data.videoCover | string | 关联视频封面（可选） |
| data.parentId | integer(uint) | 父评论ID |
| data.rootId | integer(uint) | 根评论ID |
| data.atUserIds | array<integer> | @的用户ID列表 |
| data.pictures | array<string> | 评论图片列表 |
| data.likeCount | integer(int64) | 点赞数 |
| data.replyCount | integer(int64) | 回复数 |
| data.isPinned | boolean | 是否置顶 |
| data.pinnedAt | string(date-time) | 置顶时间 |
| data.commenterAddr | string | 评论者归属地 |
| data.createdAt | string(date-time) | 创建时间 |
| data.isLiked | boolean | 当前用户是否已点赞 |
| data.replyTo | object | 回复对象 `{userId,username}`（可选） |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 3001,
    "userId": 1001,
    "username": "alice",
    "avatar": "https://cdn.example.com/avatar/1001.png",
    "videoId": 2001,
    "dynamicId": 3001,
    "content": "示例内容",
    "likeCount": 1,
    "replyCount": 1,
    "isPinned": true,
    "pinnedAt": "2024-06-01T12:00:00Z",
    "isLiked": true,
    "createdAt": "2024-06-01T12:00:00Z"
  },
  "msg": "发表成功"
}
```

## [DELETE] 删除评论

- 接口路径: DELETE /common/comment/delete
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 删除自己的评论（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| commentId | body | integer | 是 | 评论ID |

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

## [POST] 点赞评论

- 接口路径: POST /common/comment/like
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 点赞/取消点赞评论（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| commentId | body | integer | 是 | 评论ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.likeCount | integer(int64) | 当前点赞数 |
| data.isLiked | boolean | 是否已点赞 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "likeCount": 1,
    "isLiked": true
  },
  "msg": "操作成功"
}
```

## [PUT] 置顶评论

- 接口路径: PUT /common/comment/pin
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 作品作者置顶/取消置顶评论（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| commentId | body | integer | 是 | 评论ID |
| pinned | body | boolean | 否 | 是否置顶 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "操作成功"
}
```

## [GET] 创作者评论列表

- 接口路径: GET /common/comment/creator/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取自己视频下的评论列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| sort | query | integer | 否 | 排序方式（0最近 1点赞最多 2回复最多） 可选: 0/1/2 |
| keyword | query | string | 否 | 关键字搜索，支持匹配评论内容、评论用户、视频标题 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<CommentItem> | - |
| data.list[].id | integer(uint) | 评论ID |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 用户头像 |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].dynamicId | integer(uint) | 动态ID |
| data.list[].content | string | 评论内容 |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].replyCount | integer(int64) | 回复数 |
| data.list[].isPinned | boolean | 是否置顶 |
| data.list[].pinnedAt | string(date-time) | 置顶时间 |
| data.list[].isLiked | boolean | 当前用户是否已点赞 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 3001,
        "userId": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "videoId": 2001,
        "dynamicId": 3001,
        "content": "示例内容",
        "likeCount": 1,
        "replyCount": 1,
        "isPinned": true,
        "pinnedAt": "2024-06-01T12:00:00Z",
        "isLiked": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```
