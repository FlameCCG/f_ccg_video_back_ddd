# 后台-视频管理

分类说明：视频审核、举报、分区管理、转码进度

Base URL：/v1

## [GET] 管理员弹幕列表

- 接口路径: GET /admin/video/danmu/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员查看弹幕列表（需管理员权限）
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
| data.list | array<AdminDanmuItem> | - |
| data.list[].id | integer(uint) | 弹幕ID |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].username | string | 用户名 |
| data.list[].avatar | string | 用户头像 |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].videoTitle | string | 视频标题 |
| data.list[].videoCover | string | 视频封面 |
| data.list[].videoPartId | integer(uint) | 分P ID |
| data.list[].content | string | 弹幕内容 |
| data.list[].timeOffset | integer | 弹幕时间偏移(秒) |
| data.list[].color | string | 弹幕颜色 |
| data.list[].fontSize | integer | 字体大小 |
| data.list[].position | integer | 位置: 0-滚动 1-顶部 2-底部 |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 9001,
        "userId": 1001,
        "username": "alice",
        "avatar": "https://cdn.example.com/avatar/1001.png",
        "videoId": 2001,
        "videoTitle": "示例标题",
        "videoCover": "https://cdn.example.com/cover/2001.jpg",
        "videoPartId": 2001,
        "content": "示例内容",
        "timeOffset": 1,
        "color": "#FFFFFF",
        "fontSize": 1,
        "position": 1,
        "likeCount": 1,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 管理员删除弹幕

- 接口路径: DELETE /admin/video/danmu
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员批量删除弹幕（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| danmuIds | body | array<integer> | 是 | 弹幕ID列表 |

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

## [GET] 举报记录列表

- 接口路径: GET /admin/video/report/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取用户举报记录列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| type | query | string | 是 | 举报类型 可选: video/danmu |
| status | query | integer | 否 | 处理状态（1待处理 2已处理 3已驳回） 可选: 1/2/3 |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<oneOf(AdminVideoReportItem \| AdminDanmuReportItem)> | - |
| data.list[].id | integer(uint) | 举报ID |
| data.list[].reporterUsername | string | 举报人用户名 |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].videoTitle | string | 视频标题 |
| data.list[].videoCover | string | 视频封面 |
| data.list[].reason | string | 举报原因 |
| data.list[].detail | string | 举报详情 |
| data.list[].imageUrls | array<string> | 证据图片 |
| data.list[].status | integer | 处理状态 |
| data.list[].handleNote | string | 处理说明 |
| data.list[].handledBy | integer(uint) | 处理人ID |
| data.list[].handledAt | string(date-time) | 处理时间 |
| data.list[].createdAt | string(date-time) | 举报时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 9101,
        "reporterUsername": "alice",
        "videoId": 2001,
        "videoTitle": "示例标题",
        "videoCover": "https://cdn.example.com/cover/2001.jpg",
        "reason": "示例原因",
        "detail": "处理完成",
        "imageUrls": [
          "https://example.com/page"
        ],
        "status": 1,
        "handleNote": "处理完成",
        "handledBy": 9101,
        "handledAt": "2024-06-01T12:00:00Z",
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [PUT] 处理举报记录

- 接口路径: PUT /admin/video/report/handle
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员处理视频/弹幕举报（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| type | body | string | 是 | 举报类型 |
| reportIds | body | array<integer> | 是 | 举报记录ID列表 |
| status | body | integer | 是 | 处理状态（2已处理 3已驳回） |
| handleNote | body | string | 否 | 处理说明（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.handled | integer(int64) | 实际处理数量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "handled": 1
  },
  "msg": "处理成功"
}
```

## [GET] 视频详情（管理）

- 接口路径: GET /admin/video/detail
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员获取视频详情（包含资源与分P信息）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | VideoItem | - |
| data.id | integer(uint) | 视频ID |
| data.title | string | 视频标题 |
| data.description | string | 视频描述 |
| data.cover | string | 封面URL |
| data.authorUsername | string | 作者用户名 |
| data.author | VideoAuthorBrief | - |
| data.author.id | integer(uint) | 用户ID |
| data.author.username | string | 用户名 |
| data.author.avatar | string | 头像URL |
| data.author.level | integer | 用户等级 |
| data.author.description | string | 个人简介 |
| data.duration | number(float) | 视频时长（秒；转码消费者探测后更新，分P为合计） |
| data.views | integer(int64) | 播放量 |
| data.likes | integer(int64) | 点赞数 |
| data.commentCount | integer(int64) | 评论数 |
| data.coinCount | integer(int64) | 投币数 |
| data.favoriteCount | integer(int64) | 收藏数 |
| data.danmuCount | integer(int64) | 弹幕数 |
| data.isOriginal | boolean | 是否原创 |
| data.storageType | string | 存储类型（Local/Minio） |
| data.status | integer | 视频状态 |
| data.createdAt | string(date-time) | 创建时间 |
| data.partition | Partition | - |
| data.partition.id | integer(uint) | 分区ID |
| data.partition.name | string | 分区名称 |
| data.partition.icon | string | 分区图标SVG字符串 |
| data.partition.sortOrder | integer | 排序顺序 |
| data.partition.isActive | boolean | 是否启用 |
| data.partition.isSubmittable | boolean | 是否允许投稿 |
| data.tags | array<Tag> | - |
| data.tags[].id | integer(uint) | 标签ID |
| data.tags[].name | string | 标签名称 |
| data.resources | array<VideoResourceItem> | 视频资源列表（转码任务开始后先写入源资源，转码完成后补齐清晰度） |
| data.resources[].id | integer(uint) | 资源ID |
| data.resources[].resolution | string | 清晰度标识（如 1080p 高清） |
| data.resources[].sourceFileName | string | 源文件名 |
| data.resources[].fileUrl | string | 资源URL |
| data.resources[].fileSize | integer(int64) | 文件大小 |
| data.resources[].bitrate | integer | 码率 |
| data.resources[].format | string | 容器格式 |
| data.resources[].codec | string | 视频编码 |
| data.resources[].isVip | boolean | 是否会员资源 |
| data.resources[].isSource | boolean | 是否源文件 |
| data.parts | array<VideoPartItem> | 分P列表（转码任务开始后回填时长与资源） |
| data.parts[].id | integer(uint) | 分P ID |
| data.parts[].title | string | 分P标题 |
| data.parts[].sortOrder | integer | 分P排序 |
| data.parts[].duration | integer | 分P时长（秒，转码消费者探测后更新） |
| data.parts[].danmuCount | integer(int64) | 分P弹幕数 |
| data.parts[].resources | array<VideoResourceItem> | - |
| data.parts[].resources[].id | integer(uint) | 资源ID |
| data.parts[].resources[].resolution | string | 清晰度标识（如 1080p 高清） |
| data.parts[].resources[].sourceFileName | string | 源文件名 |
| data.parts[].resources[].fileUrl | string | 资源URL |
| data.parts[].resources[].fileSize | integer(int64) | 文件大小 |
| data.parts[].resources[].bitrate | integer | 码率 |
| data.parts[].resources[].format | string | 容器格式 |
| data.parts[].resources[].codec | string | 视频编码 |
| data.parts[].resources[].isVip | boolean | 是否会员资源 |
| data.parts[].resources[].isSource | boolean | 是否源文件 |
| data.isLiked | boolean | 是否已点赞 |
| data.isFavorited | boolean | 是否已收藏 |
| data.isCoined | boolean | 是否已投币 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 2001,
    "title": "示例标题",
    "description": "示例说明",
    "cover": "https://cdn.example.com/cover/2001.jpg",
    "authorUsername": "alice",
    "author": {
      "id": 1001,
      "username": "alice",
      "avatar": "https://cdn.example.com/avatar/1001.png",
      "level": 3,
      "description": "示例说明"
    },
    "duration": 0.5,
    "views": 1,
    "likes": 1,
    "commentCount": 1,
    "coinCount": 1,
    "favoriteCount": 1,
    "danmuCount": 1,
    "isOriginal": true,
    "storageType": "pwd",
    "status": 1,
    "createdAt": "2024-06-01T12:00:00Z",
    "partition": {
      "id": 2001,
      "name": "示例名称",
      "icon": "user",
      "sortOrder": 1,
      "isActive": true,
      "isSubmittable": true
    },
    "tags": [
      {
        "id": 2001,
        "name": "示例名称"
      }
    ],
    "resources": [
      {
        "id": 2001,
        "resolution": "1080p",
        "sourceFileName": "示例名称",
        "fileUrl": "https://example.com/page",
        "fileSize": 1,
        "bitrate": 1,
        "format": "mp4",
        "codec": "123456",
        "isVip": true,
        "isSource": true
      }
    ],
    "parts": [
      {
        "id": 2001,
        "title": "示例标题",
        "sortOrder": 1,
        "duration": 60,
        "danmuCount": 1,
        "resources": [
          {
            "id": 2001,
            "resolution": "1080p",
            "sourceFileName": "示例名称",
            "fileUrl": "https://example.com/page",
            "fileSize": 1,
            "bitrate": 1,
            "format": "mp4",
            "codec": "123456",
            "isVip": true,
            "isSource": true
          }
        ]
      }
    ],
    "isLiked": true,
    "isFavorited": true,
    "isCoined": true
  },
  "msg": "获取成功"
}
```

## [GET] 视频列表（管理）

- 接口路径: GET /admin/video/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取视频列表（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| userId | query | integer | 否 | 作者用户ID（可选） |
| keyword | query | string | 否 | 标题/描述关键词（可选） |
| status | query | integer | 否 | 视频状态（1已发布 2私密 3已删除 4审核中） 可选: 1/2/3/4 |
| partitionId | query | integer | 否 | 分区ID（可选） |
| sort | query | string | 否 | 排序（latest最新 oldest最早 views播放量 likes点赞） 可选: latest/oldest/views/likes |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<AdminVideoItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].likes | integer(int64) | 点赞数 |
| data.list[].commentCount | integer(int64) | 评论数 |
| data.list[].coinCount | integer(int64) | 投币数 |
| data.list[].favoriteCount | integer(int64) | 收藏数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].isOriginal | boolean | 是否原创 |
| data.list[].storageType | string | 存储类型 |
| data.list[].status | integer | 视频状态 |
| data.list[].partitionId | integer(uint) | 分区ID |
| data.list[].partitionName | string | 分区名称 |
| data.list[].author | AdminVideoAuthor | - |
| data.list[].author.id | integer(uint) | 用户ID |
| data.list[].author.username | string | 用户名 |
| data.list[].author.avatar | string | 头像URL |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 2001,
        "title": "示例标题",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "duration": 60,
        "views": 1,
        "likes": 1,
        "commentCount": 1,
        "coinCount": 1,
        "favoriteCount": 1,
        "danmuCount": 1,
        "isOriginal": true,
        "storageType": "pwd",
        "status": 1,
        "partitionId": 2001,
        "partitionName": "示例名称",
        "author": {
          "id": 1001,
          "username": "alice",
          "avatar": "https://cdn.example.com/avatar/1001.png"
        },
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 删除视频（管理）

- 接口路径: DELETE /admin/video/delete
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员删除视频（可选软删或物理删除）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | body | array<integer> | 是 | 视频ID列表 |
| hardDelete | body | boolean | 否 | 是否物理删除（默认 false） |

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

## [PUT] 恢复上架视频

- 接口路径: PUT /admin/video/restore
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 管理员批量恢复已删除视频（软删恢复）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | body | array<integer> | 是 | 视频ID列表 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.restored | integer(int64) | 恢复数量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "restored": 1
  },
  "msg": "恢复成功"
}
```

## [PUT] 审核视频

- 接口路径: PUT /admin/video/review
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 审核视频（需管理员权限，仅处理审核中视频）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | body | array<integer> | 是 | 视频ID列表 |
| status | body | integer | 是 | 审核结果（1通过发布 2转私密 3驳回删除） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.updated | integer(int64) | 更新数量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "updated": 1
  },
  "msg": "审核成功"
}
```

## [POST] 创建分区

- 接口路径: POST /admin/video/partition
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 创建视频分区（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | body | string | 是 | 分区名称 |
| icon | body | string | 否 | 分区图标SVG字符串（可选） |
| sortOrder | body | integer | 否 | 排序顺序（可选） |
| isActive | body | boolean | 否 | 是否启用 |
| isSubmittable | body | boolean | 否 | 是否允许投稿 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | Partition | - |
| data.id | integer(uint) | 分区ID |
| data.name | string | 分区名称 |
| data.icon | string | 分区图标SVG字符串 |
| data.sortOrder | integer | 排序顺序 |
| data.isActive | boolean | 是否启用 |
| data.isSubmittable | boolean | 是否允许投稿 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 2,
    "name": "示例名称",
    "icon": "user",
    "sortOrder": 1,
    "isActive": true,
    "isSubmittable": true
  },
  "msg": "创建成功"
}
```

## [PUT] 更新分区

- 接口路径: PUT /admin/video/partition
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 更新视频分区（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | body | integer | 是 | 分区ID |
| name | body | string | 是 | 分区名称 |
| icon | body | string | 否 | 分区图标SVG字符串（可选） |
| sortOrder | body | integer | 否 | 排序顺序（可选） |
| isActive | body | boolean | 否 | 是否启用 |
| isSubmittable | body | boolean | 否 | 是否允许投稿 |

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

## [DELETE] 删除分区

- 接口路径: DELETE /admin/video/partition
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 删除视频分区（需管理员权限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| id | body | integer | 是 | 分区ID |

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

## [GET] 分区列表（管理）

- 接口路径: GET /admin/video/partitions
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 获取所有分区列表（需管理员权限）
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
| data.list | array<Partition> | - |
| data.list[].id | integer(uint) | 分区ID |
| data.list[].name | string | 分区名称 |
| data.list[].icon | string | 分区图标SVG字符串 |
| data.list[].sortOrder | integer | 排序顺序 |
| data.list[].isActive | boolean | 是否启用 |
| data.list[].isSubmittable | boolean | 是否允许投稿 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 2,
        "name": "示例名称",
        "icon": "user",
        "sortOrder": 1,
        "isActive": true,
        "isSubmittable": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 查询视频转码进度（快照）

- 接口路径: GET /admin/video/transcode/progress
- 认证: 需要登录（客户端全局自动携带 Token）
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: 按 videoIds 批量读取 Redis 转码进度快照，适合轮询兜底
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | query | string | 是 | 逗号分隔视频ID，如 `1,2,3`（最多100个） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.items | array<TranscodeProgressItem> | 进度列表 |
| data.items[].videoId | integer(uint) | 视频ID |
| data.items[].partId | integer(uint) | 分P ID |
| data.items[].status | string | queued / running / succeeded / failed |
| data.items[].percent | number | 整体进度 0-100 |
| data.items[].stage | string | 当前阶段 |
| data.items[].stagePercent | number | 阶段进度 0-1 |
| data.items[].message | string | 状态说明 |
| data.items[].error | string | 失败原因 |
| data.items[].updatedAt | string(date-time) | 更新时间 |

响应示例:
```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "items": [
      {
        "videoId": 11,
        "partId": 0,
        "status": "running",
        "percent": 42.5,
        "stage": "multi_res",
        "stagePercent": 0.6,
        "message": "多分辨率转码中",
        "updatedAt": "2026-07-12T10:00:00Z"
      }
    ]
  }
}
```

## [GET] 订阅视频转码进度（SSE）

- 接口路径: GET /admin/video/transcode/progress/stream
- 认证: 需要登录；EventSource 可通过 query `token` 传 JWT
- 权限: 需要RBAC权限
- 依赖接口: 无
- 接口说明: SSE 推送转码进度；服务端约 800ms 轮询 Redis，20s 心跳；全部结束后 `complete` 并关闭连接
- HTTP 状态码: 200（成功为 text/event-stream；建立前失败为业务 JSON）

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | query | string | 是 | 逗号分隔视频ID（最多100个） |
| token | query | string | 否 | JWT（EventSource 场景，可不带 Bearer 前缀） |

SSE 事件:
| event | data 说明 |
| --- | --- |
| snapshot | 初始快照 `{items:[...]}` |
| progress | 单条进度对象 TranscodeProgressItem |
| done | 某任务终态（succeeded/failed） |
| heartbeat | `{ts:unix秒}` |
| complete | `{ok:true}` 全部结束，连接关闭 |
| error | `{message:...}` |

SSE 示例:
```
event: snapshot
data: {"items":[{"videoId":11,"status":"running","percent":10,"stage":"prepare","stagePercent":0.2}]}

event: progress
data: {"videoId":11,"status":"running","percent":55,"stage":"multi_res","stagePercent":0.5}

event: done
data: {"videoId":11,"status":"succeeded","percent":100,"stage":"cleanup","stagePercent":1,"message":"转码完成"}

event: complete
data: {"ok":true}
```

## [POST] 重建视频向量索引

- 接口路径: POST /admin/video/vector/rebuild
- Tag: 后台-视频管理
- 认证及权限: 需要登录并通过 RBAC 权限校验
- 请求体: JSON 对象；`recreate` 可选，默认 `false`，为 `true` 时删除并重建索引；`workers` 可选，省略或传 `0` 使用默认并发数 `4`，否则允许 `1` 至 `16`
- 说明: 异步启动全量向量回填，已有任务运行时返回 `code=1`
- 响应: HTTP 200，`code=0` 时 `data` 为当前重建任务状态

## [GET] 视频向量索引重建状态

- 接口路径: GET /admin/video/vector/status
- Tag: 后台-视频管理
- 认证及权限: 需要登录并通过 RBAC 权限校验
- 请求参数: 无
- 响应: HTTP 200，`code=0` 时 `data` 包含 `running`、`total`、`succeeded`、`failed`、`startedAt`、`finishedAt`、`lastError`（可选）
- 说明: 状态仅保存在当前服务进程中，服务重启后不保留任务快照
