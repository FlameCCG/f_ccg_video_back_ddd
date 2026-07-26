# 动态管理

分类说明：动态发布与列表

Base URL：/v1

## [POST] 发布动态

- 接口路径: POST /common/dynamic/create
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 发布新动态（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| content | body | string | 是 | 动态内容 |
| imageUrl | body | string | 否 | 图片URL（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | DynamicItem | - |
| data.id | integer(uint) | 动态ID |
| data.userId | integer(uint) | 用户ID |
| data.content | string | 动态内容 |
| data.imageUrl | string | 图片URL |
| data.isPinned | boolean | 是否置顶 |
| data.createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 4001,
    "userId": 1001,
    "content": "示例内容",
    "imageUrl": "https://example.com/page",
    "isPinned": true,
    "createdAt": "2024-06-01T12:00:00Z"
  },
  "msg": "发布成功"
}
```

## [DELETE] 删除动态

- 接口路径: DELETE /common/dynamic/delete
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 删除自己的动态（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| dynamicId | body | integer | 是 | 动态ID |

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

## [PUT] 置顶动态

- 接口路径: PUT /common/dynamic/pin
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 置顶/取消置顶动态或视频（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| dynamicId | body | integer | 否 | 动态ID（与videoId二选一） |
| videoId | body | integer | 否 | 视频ID（与dynamicId二选一） |
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

## [POST] 动态点赞

- 接口路径: POST /common/dynamic/like
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 点赞/取消点赞动态（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| dynamicId | body | integer | 是 | 动态ID |

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

## [GET] 动态列表

- 接口路径: GET /common/dynamic/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定用户的作品动态列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 是 | 指定用户ID |
| type | query | integer | 否 | 类型筛选（0全部 1仅视频作品 2仅图文动态；不传或传0时返回两者） 可选: 0/1/2 |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<WorkFeedItem> | - |
| data.list[].workType | integer | 作品类型（1视频 2动态） |
| data.list[].createdAt | string(date-time) | 发布时间 |
| data.list[].author | AuthorBrief | - |
| data.list[].author.id | integer(uint) | 用户ID |
| data.list[].author.username | string | 用户名 |
| data.list[].author.avatar | string | 头像URL |
| data.list[].video | VideoBrief | - |
| data.list[].video.id | integer(uint) | 视频ID |
| data.list[].video.title | string | 标题 |
| data.list[].video.description | string | 视频描述 |
| data.list[].video.cover | string | 封面 |
| data.list[].video.duration | integer | 时长（秒） |
| data.list[].video.views | integer(int64) | 播放量 |
| data.list[].video.danmuCount | integer(int64) | 弹幕数 |
| data.list[].video.likeCount | integer(int64) | 点赞数 |
| data.list[].video.commentCount | integer(int64) | 评论数 |
| data.list[].video.isLiked | boolean | 当前用户是否已点赞 |
| data.list[].video.isPinned | boolean | 是否置顶 |
| data.list[].video.pinnedAt | string(date-time) | 置顶时间 |
| data.list[].dynamic | DynamicBrief | - |
| data.list[].dynamic.id | integer(uint) | 动态ID |
| data.list[].dynamic.content | string | 动态内容 |
| data.list[].dynamic.imageUrl | string | 图片URL |
| data.list[].dynamic.likeCount | integer(int64) | 点赞数 |
| data.list[].dynamic.commentCount | integer(int64) | 评论数 |
| data.list[].dynamic.isLiked | boolean | 当前用户是否已点赞 |
| data.list[].dynamic.isPinned | boolean | 是否置顶 |
| data.list[].dynamic.pinnedAt | string(date-time) | 置顶时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "workType": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "author": {
          "id": 1001,
          "username": "alice",
          "avatar": "https://cdn.example.com/avatar/1001.png"
        },
        "video": {
          "id": 2001,
          "title": "示例标题",
          "description": "这是视频简介",
          "cover": "https://cdn.example.com/cover/2001.jpg",
          "duration": 60,
          "views": 1024,
          "danmuCount": 18,
          "likeCount": 12,
          "commentCount": 3,
          "isLiked": true,
          "isPinned": true,
          "pinnedAt": "2024-06-01T12:00:00Z"
        },
        "dynamic": {
          "id": 4001,
          "content": "示例内容",
          "imageUrl": "https://example.com/page",
          "likeCount": 0,
          "commentCount": 2,
          "isLiked": false,
          "isPinned": true,
          "pinnedAt": "2024-06-01T12:00:00Z"
        }
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 关注用户动态

- 接口路径: GET /common/dynamic/follows
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取关注用户的动态列表（需登录）
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
| data.list | array<FollowUserItem> | - |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 头像URL |
| data.list[].followedAt | string(date-time) | 关注时间 |
| data.list[].latestWorkAt | string(date-time) | 最新作品时间 |
| data.list[].latestWorkType | integer | 最新作品类型（1视频 2动态） |
| data.list[].isUnread | boolean | 是否有未读新动态 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "userId": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "followedAt": "2024-06-01T12:00:00Z",
        "latestWorkAt": "2024-06-01T12:00:00Z",
        "latestWorkType": 1,
        "isUnread": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 未读动态数量

- 接口路径: GET /common/dynamic/counts
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取关注用户的未读动态数量（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.unreadCount | integer(int64) | 未读动态数量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "unreadCount": 1
  },
  "msg": "获取成功"
}
```

## [POST] 标记动态已读

- 接口路径: POST /common/dynamic/read
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 标记指定作者的动态为已读（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| authorId | body | integer | 是 | 作者ID |
| page | body | integer | 否 | 页码 |
| pageSize | body | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<WorkFeedItem> | - |
| data.list[].workType | integer | 作品类型（1视频 2动态） |
| data.list[].createdAt | string(date-time) | 发布时间 |
| data.list[].author | AuthorBrief | - |
| data.list[].author.id | integer(uint) | 用户ID |
| data.list[].author.username | string | 用户名 |
| data.list[].author.avatar | string | 头像URL |
| data.list[].video | VideoBrief | - |
| data.list[].video.id | integer(uint) | 视频ID |
| data.list[].video.title | string | 标题 |
| data.list[].video.description | string | 视频描述 |
| data.list[].video.cover | string | 封面 |
| data.list[].video.duration | integer | 时长（秒） |
| data.list[].video.views | integer(int64) | 播放量 |
| data.list[].video.danmuCount | integer(int64) | 弹幕数 |
| data.list[].video.likeCount | integer(int64) | 点赞数 |
| data.list[].video.commentCount | integer(int64) | 评论数 |
| data.list[].video.isLiked | boolean | 当前用户是否已点赞 |
| data.list[].video.isPinned | boolean | 是否置顶 |
| data.list[].video.pinnedAt | string(date-time) | 置顶时间 |
| data.list[].dynamic | DynamicBrief | - |
| data.list[].dynamic.id | integer(uint) | 动态ID |
| data.list[].dynamic.content | string | 动态内容 |
| data.list[].dynamic.imageUrl | string | 图片URL |
| data.list[].dynamic.likeCount | integer(int64) | 点赞数 |
| data.list[].dynamic.commentCount | integer(int64) | 评论数 |
| data.list[].dynamic.isLiked | boolean | 当前用户是否已点赞 |
| data.list[].dynamic.isPinned | boolean | 是否置顶 |
| data.list[].dynamic.pinnedAt | string(date-time) | 置顶时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "workType": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "author": {
          "id": 1001,
          "username": "alice",
          "avatar": "https://cdn.example.com/avatar/1001.png"
        },
        "video": {
          "id": 2001,
          "title": "示例标题",
          "description": "这是视频简介",
          "cover": "https://cdn.example.com/cover/2001.jpg",
          "duration": 60,
          "views": 1024,
          "danmuCount": 18,
          "likeCount": 12,
          "commentCount": 3,
          "isLiked": true,
          "isPinned": true,
          "pinnedAt": "2024-06-01T12:00:00Z"
        },
        "dynamic": {
          "id": 4001,
          "content": "示例内容",
          "imageUrl": "https://example.com/page",
          "likeCount": 0,
          "commentCount": 2,
          "isLiked": false,
          "isPinned": true,
          "pinnedAt": "2024-06-01T12:00:00Z"
        }
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```
