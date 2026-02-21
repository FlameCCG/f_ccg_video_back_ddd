# 后台-评论管理

分类说明：评论管理

Base URL：/v1

## [GET] 管理员评论列表

- 接口路径: GET /admin/comment/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员查看评论列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 否 | 用户ID筛选（可选） |
| username | query | string | 否 | 用户名筛选（可选） |
| keyword | query | string | 否 | 关键词筛选（可选） |
| sort | query | string | 否 | 排序方式（可选） 可选: latest/oldest/likes/replies |
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

## [DELETE] 管理员删除评论

- 接口路径: DELETE /admin/comment
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员批量删除评论（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| commentIds | body | array<integer> | 是 | 评论ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.deleted | integer(int64) | 删除数量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "deleted": 1
  },
  "msg": "删除成功"
}
```
