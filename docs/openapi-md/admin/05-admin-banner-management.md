# 后台-轮播图管理

分类说明：轮播图管理

Base URL：/v1

## [GET] 轮播图列表（管理）

- 接口路径: GET /admin/banners
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取轮播图列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| type | query | integer | 否 | 轮播图类型（1首页/分区轮播图 2顶部横幅 3用户主页横幅） 可选: 1/2/3 |
| partitionId | query | integer(uint) | 否 | 分区ID，仅type=1时生效；0代表首页，>0代表对应分区 |
| show | query | boolean | 否 | 是否显示 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<BannerItem> | - |
| data.list[].id | integer(uint) | 轮播图ID |
| data.list[].cover | string | 图片链接 |
| data.list[].href | string | 跳转链接 |
| data.list[].show | boolean | 是否显示 |
| data.list[].type | integer | 轮播图类型（1首页/分区轮播图 2顶部横幅 3用户主页横幅） |
| data.list[].partitionId | integer(uint) | 分区ID，仅type=1生效；0代表首页，>0代表对应分区 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 6001,
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "href": "https://example.com/page",
        "show": true,
        "type": 1,
        "partitionId": 6001
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [POST] 创建轮播图

- 接口路径: POST /admin/banner
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 创建轮播图（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| cover | body | string | 是 | 图片链接 |
| href | body | string | 否 | 跳转链接 |
| show | body | boolean | 否 | 是否显示 |
| type | body | integer | 否 | 轮播图类型（1首页/分区轮播图 2顶部横幅 3用户主页横幅） |
| partitionId | body | integer(uint) | 否 | 分区ID，仅type=1时生效；0代表首页，>0代表对应分区 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | BannerItem | - |
| data.id | integer(uint) | 轮播图ID |
| data.cover | string | 图片链接 |
| data.href | string | 跳转链接 |
| data.show | boolean | 是否显示 |
| data.type | integer | 轮播图类型（1首页/分区轮播图 2顶部横幅 3用户主页横幅） |
| data.partitionId | integer(uint) | 分区ID，仅type=1生效；0代表首页，>0代表对应分区 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 6001,
    "cover": "https://cdn.example.com/cover/2001.jpg",
    "href": "https://example.com/page",
    "show": true,
    "type": 1,
    "partitionId": 6001
  },
  "msg": "创建成功"
}
```

## [PUT] 更新轮播图

- 接口路径: PUT /admin/banner
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 更新轮播图（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | body | integer | 是 | 轮播图ID |
| cover | body | string | 否 | 图片链接（可选） |
| href | body | string | 否 | 跳转链接（可选） |
| show | body | boolean | 否 | 是否显示（可选） |
| type | body | integer | 否 | 轮播图类型（可选，1首页/分区轮播图 2顶部横幅 3用户主页横幅） |
| partitionId | body | integer(uint) | 否 | 分区ID（可选，仅type=1时生效；0代表首页，>0代表对应分区） |

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

## [DELETE] 删除轮播图

- 接口路径: DELETE /admin/banner
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 删除轮播图（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| bannerIds | body | array<integer> | 是 | 轮播图ID列表 |

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
