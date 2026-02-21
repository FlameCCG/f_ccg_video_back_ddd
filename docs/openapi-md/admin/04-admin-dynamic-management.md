# 后台-动态管理

分类说明：动态管理

Base URL：/v1

## [GET] 管理员动态列表

- 接口路径: GET /admin/dynamic/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员查看动态列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 否 | 用户ID筛选（可选） |
| username | query | string | 否 | 用户名筛选（可选） |
| keyword | query | string | 否 | 关键词筛选（可选） |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<DynamicItem> | - |
| data.list[].id | integer(uint) | 动态ID |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].content | string | 动态内容 |
| data.list[].imageUrl | string | 图片URL |
| data.list[].isPinned | boolean | 是否置顶 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 4001,
        "userId": 1001,
        "content": "示例内容",
        "imageUrl": "https://example.com/page",
        "isPinned": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 管理员删除动态

- 接口路径: DELETE /admin/dynamic
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员批量删除动态（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| dynamicIds | body | array<integer> | 是 | 动态ID列表 |

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
