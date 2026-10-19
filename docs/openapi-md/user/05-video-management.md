# 视频管理

分类说明：视频浏览、搜索、互动、投稿、上传、弹幕、收藏/历史

Base URL：/v1

## [GET] 首页推荐流

- 接口路径: GET /common/video/home
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取首页视频推荐流，登录用户优先按历史偏好推荐
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| sort | query | integer | 否 | 排序方式（0最近收藏 1最多播放 2最多点赞） 可选: 0/1/2 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FeedItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].author | AuthorBrief | - |
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
        "danmuCount": 1,
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

## [GET] 综合热门

- 接口路径: GET /common/video/hot
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取综合热门视频列表（时间衰减热度，最多保留Top200）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| keyword | query | string | 否 | 关键字搜索，支持匹配用户名和个人简介 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FeedItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].author | AuthorBrief | - |
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
        "danmuCount": 1,
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

## [GET] 综合排行榜

- 接口路径: GET /common/video/rank
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取综合排行榜视频列表（仅Top100）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| keyword | query | string | 否 | 关键字搜索，支持匹配用户名和个人简介 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FeedItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].author | AuthorBrief | - |
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
        "danmuCount": 1,
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

## [POST] 举报视频

- 接口路径: POST /common/video/report
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 举报视频（需登录；最多上传5张图片）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| reason | body | string | 是 | 举报原因 |
| detail | body | string | 否 | 详情描述（可选） |
| images | body | array<string(binary)> | 否 | 证据图片（可选，最多5张） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "举报成功"
}
```

## [POST] 举报弹幕

- 接口路径: POST /common/video/danmu/report
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 举报弹幕（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| danmuId | body | integer | 是 | 弹幕ID |
| reason | body | string | 是 | 举报原因 |
| detail | body | string | 否 | 详情描述（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "举报成功"
}
```

## [GET] 搜索视频

- 接口路径: GET /common/video/search
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 根据关键词搜索视频
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| keyword | query | string | 是 | 搜索关键词 |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| videoSort | query | integer | 否 | 排序方式（0相关性 1播放量 2弹幕数 3时长） 可选: 0/1/2/3 |
| videoOrder | query | integer | 否 | 排序顺序（0降序 1升序） 可选: 0/1 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | VideoSearchResult | - |
| data.videoTotal | integer(int64) | - |
| data.userTotal | integer(int64) | - |
| data.videos | array<SearchVideoHit> | - |
| data.videos[].id | integer(uint) | - |
| data.videos[].title | string | - |
| data.videos[].cover | string | - |
| data.videos[].authorUsername | string | - |
| data.videos[].createdAt | string(date-time) | - |
| data.videos[].highlight | SearchHighlight | - |
| data.videos[].views | integer(int64) | - |
| data.videos[].danmuCount | integer(int64) | - |
| data.videos[].duration | integer | - |
| data.videos[].tags | array<string> | - |

响应示例:
```json
{
  "code": 0,
  "data": {
    "videoTotal": 1,
    "userTotal": 1,
    "videos": [
      {
        "id": 2001,
        "title": "示例标题",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "authorUsername": "alice",
        "createdAt": "2024-06-01T12:00:00Z",
        "highlight": {
          "key": [
            "value"
          ]
        },
        "views": 2001,
        "danmuCount": 1,
        "duration": 60,
        "tags": [
          "id_1001"
        ]
      }
    ]
  },
  "msg": "搜索成功"
}
```

## [GET] 首页搜索关键词前十

- 接口路径: GET /common/video/home/search/top
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取首页搜索关键词前十列表，用于展示热搜；优先读取视频搜索累计热度，空数据时回退为热门视频标题
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<HotKeywordItem> | - |
| data[].keyword | string | 热搜关键词 |
| data[].score | integer(int64) | 关键词热度分值/累计次数 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "keyword": "key_123",
      "score": 1
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 搜索建议

- 接口路径: GET /common/video/search/suggest
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 根据前缀获取搜索建议
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| prefix | query | string | 是 | 搜索前缀 |
| size | query | integer | 否 | 返回数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<SuggestionItem> | 搜索建议列表 |
| data[].value | string | 建议文本 |
| data[].highlight | string | 高亮文本（使用 <em> 包裹匹配前缀） |
| data[].docType | string | 类型标签 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "value": "视频剪辑",
      "highlight": "视频剪辑",
      "docType": "videos"
    }
  ],
  "msg": "获取成功"
}
```

## [GET] 视频详情

- 接口路径: GET /common/video/detail
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取视频详细信息，包含播放进度（如已登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
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
| data.watchProgress | integer | 播放进度（秒） |

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
    "isCoined": true,
    "watchProgress": 1
  },
  "msg": "获取成功"
}
```

## [GET] 视频周边推荐

- 接口路径: GET /common/video/recommend
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取视频详情页周边推荐列表
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |
| size | query | integer | 否 | 推荐数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FeedItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].author | AuthorBrief | - |
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
        "danmuCount": 1,
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

## [GET] 投稿分区列表

- 接口路径: GET /common/video/partitions
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 获取可投稿的视频分区列表
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
- 无

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | array<Partition> | - |
| data[].id | integer(uint) | 分区ID |
| data[].name | string | 分区名称 |
| data[].icon | string | 分区图标SVG字符串 |
| data[].sortOrder | integer | 排序顺序 |
| data[].isActive | boolean | 是否启用 |
| data[].isSubmittable | boolean | 是否允许投稿 |

响应示例:
```json
{
  "code": 0,
  "data": [
    {
      "id": 2,
      "name": "示例名称",
      "icon": "user",
      "sortOrder": 1,
      "isActive": true,
      "isSubmittable": true
    }
  ],
  "msg": "获取成功"
}
```

## [POST] 增加播放量

- 接口路径: POST /common/video/view
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 记录视频播放，增加播放量
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.views | integer(int64) | 当前播放量 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "views": 1
  },
  "msg": "成功"
}
```

## [POST] 点赞/取消点赞

- 接口路径: POST /common/video/like
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 切换视频点赞状态（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.likes | integer(int64) | 当前点赞数 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "likes": 1
  },
  "msg": "成功"
}
```

## [POST] 投币

- 接口路径: POST /common/video/coin
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 给视频投币（需登录，每个视频最多投2个币；会扣除当前用户等额硬币，作者获得投币数10%的硬币奖励，余额不足时返回失败）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| coins | body | integer | 否 | 投币数量（1-2） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.coinCount | integer(int64) | 当前投币数 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "coinCount": 1
  },
  "msg": "成功"
}
```

## [POST] 收藏/取消收藏

- 接口路径: POST /common/video/favorite
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 切换视频收藏状态（需登录）；当某用户首次收藏某视频时，作者获得0.1硬币奖励
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| folderId | body | integer | 否 | 收藏夹ID（可选，不填使用默认收藏夹） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.favoriteCount | integer(int64) | 当前收藏数 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "favoriteCount": 1
  },
  "msg": "成功"
}
```

## [POST] 一键三连

- 接口路径: POST /common/video/triple
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 同时点赞/投币/收藏（需登录）；若本次需要投币，会先校验硬币余额，不足则直接返回失败
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| folderId | body | integer | 否 | 收藏夹ID（可选，默认使用默认收藏夹） |
| coins | body | integer | 否 | 投币数量（1-2，可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.likes | integer(int64) | 当前点赞数 |
| data.coinCount | integer(int64) | 当前投币数 |
| data.favoriteCount | integer(int64) | 当前收藏数 |
| data.isLiked | boolean | 是否已点赞 |
| data.isFavorited | boolean | 是否已收藏 |
| data.isCoined | boolean | 是否已投币 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "likes": 1,
    "coinCount": 1,
    "favoriteCount": 1,
    "isLiked": true,
    "isFavorited": true,
    "isCoined": true
  },
  "msg": "成功"
}
```

## [POST] 发布视频

- 接口路径: POST /common/video/publish
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 发布新视频（需登录；支持单文件与分P投稿）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| title | body | string | 是 | 视频标题 |
| description | body | string | 否 | 视频描述（可选） |
| partitionId | body | integer | 是 | 分区ID |
| tags | body | array<integer> | 否 | 标签ID列表（可选，最多10个） |
| isOriginal | body | boolean | 否 | 是否原创（可选） |
| isPrivate | body | boolean | 否 | 是否私密（可选，默认 false；传 true 时发布为私密作品） |
| coverUrl | body | string | 是 | 封面图片URL |
| filePath | body | string | 是 | 视频文件路径 |
| fileName | body | string | 是 | 视频文件名 |
| fileHash | body | string | 是 | 文件哈希值 |
| publishTime | body | string(date-time) | 否 | 定时发布时间（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.videoId | integer | 视频ID |
| data.status | integer | 视频状态（1已发布 2私密 4审核中） |
| data.publishTime | string(date-time) | 发布时间 |
| data.cover | string | 封面URL |

响应示例:
```json
{
  "code": 0,
  "data": {
    "videoId": 2001,
    "status": 1,
    "publishTime": "2024-06-01T12:00:00Z",
    "cover": "https://cdn.example.com/cover/2001.jpg"
  },
  "msg": "发布成功"
}
```

## [PUT] 更新视频

- 接口路径: PUT /common/video/publish
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 更新已发布的视频信息（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| title | body | string | 否 | 视频标题（可选） |
| description | body | string | 否 | 视频描述（可选） |
| partitionId | body | integer | 否 | 分区ID（可选） |
| tags | body | array<integer> | 否 | 标签ID列表（可选） |
| isOriginal | body | boolean | 否 | 是否原创（可选） |
| isPrivate | body | boolean | 否 | 是否私密（可选；传 true 转为私密，传 false 转为公开/审核中） |
| coverUrl | body | string | 否 | 封面图片URL（可选） |
| publishTime | body | string(date-time) | 否 | 发布时间（可选） |

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
  "msg": "更新成功"
}
```

## [POST] 上传分片

- 接口路径: POST /common/video/upload/chunk
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 上传视频分片（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| fileHash | body | string | 是 | 整个文件内容哈希（用于断点续传） |
| index | body | string | 是 | 当前分片索引（从0开始） |
| chunk | body | string(binary) | 是 | 分片文件内容 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "上传成功"
}
```

## [GET] 查询上传状态

- 接口路径: GET /common/video/upload/status
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 查询文件上传状态，支持断点续传与秒传判断（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| fileHash | query | string | 是 | 文件哈希值 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.fileHash | string | 当前文件标识，可直接用于后续投稿 |
| data.completed | boolean | 是否已完整上传完成，可用于秒传判断 |
| data.filePath | string | 已完成上传时的最终文件路径 |
| data.uploadedChunks | array<string> | 未完成上传时已存在的分片索引列表 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "fileHash": "示例字符串",
    "completed": true,
    "filePath": "/path/to/resource",
    "uploadedChunks": [
      "0"
    ]
  },
  "msg": "查询成功"
}
```

## [POST] 完成上传

- 接口路径: POST /common/video/upload/complete
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 合并分片完成上传（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| fileHash | body | string | 是 | 文件哈希值 |
| fileName | body | string | 是 | 文件名 |
| totalChunks | body | integer | 是 | 总分片数 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.filePath | string | 合并后的文件路径 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "filePath": "/path/to/resource"
  },
  "msg": "合并成功"
}
```

## [POST] 上传图片

- 接口路径: POST /common/video/image/upload
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 上传图片（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| fileHash | body | string | 是 | 文件哈希值 |
| cover | body | string(binary) | 是 | 图片文件 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.imageUrl | string | 封面图片URL |

响应示例:
```json
{
  "code": 0,
  "data": {
    "imageUrl": "https://example.com/page"
  },
  "msg": "上传成功"
}
```

## [POST] 保存播放进度

- 接口路径: POST /common/video/history
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 保存/更新视频播放进度（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| progress | body | integer | 否 | 播放进度（秒） |
| duration | body | integer | 是 | 视频总时长（秒） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "保存成功"
}
```

## [DELETE] 删除播放历史

- 接口路径: DELETE /common/video/history
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 批量删除播放历史（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoIds | body | array<integer> | 是 | 视频ID列表 |

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

## [GET] 播放历史列表

- 接口路径: GET /common/video/history/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取用户播放历史记录（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| keyword | query | string | 否 | 关键字搜索，支持匹配视频标题、作者名称 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<HistoryItem> | - |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 视频时长（秒） |
| data.list[].progress | integer | 播放进度（秒） |
| data.list[].createdAt | string(date-time) | 观看时间 |
| data.list[].authorId | integer(uint) | 作者ID |
| data.list[].author | string | 作者名称 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "videoId": 2001,
        "title": "示例标题",
        "cover": "https://cdn.example.com/cover/2001.jpg",
        "duration": 60,
        "progress": 1,
        "createdAt": "2024-06-01T12:00:00Z",
        "authorId": 1,
        "author": "alice"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [POST] 创建收藏夹

- 接口路径: POST /common/video/folder
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 创建新收藏夹（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| name | body | string | 是 | 收藏夹名称 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | FolderItem | - |
| data.id | integer(uint) | 收藏夹ID |
| data.name | string | 收藏夹名称 |
| data.isDefault | boolean | 是否为默认收藏夹 |
| data.videoCount | integer | 视频数量 |
| data.createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 1,
    "name": "示例名称",
    "isDefault": true,
    "videoCount": 1,
    "createdAt": "2024-06-01T12:00:00Z"
  },
  "msg": "创建成功"
}
```

## [GET] 收藏夹列表

- 接口路径: GET /common/video/folder
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取用户收藏夹列表（需登录）；传入 videoId 时可同时返回该视频在各收藏夹中的收藏状态
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer(uint) | 否 | 视频ID（传入后返回该视频在各收藏夹中的收藏状态） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FolderWithCount> | - |
| data.list[].id | integer(uint) | 收藏夹ID |
| data.list[].name | string | 收藏夹名称 |
| data.list[].isDefault | boolean | 是否为默认收藏夹 |
| data.list[].videoCount | integer(int64) | 视频数量 |
| data.list[].isFavorited | boolean | 当前 videoId 是否已收藏到该收藏夹；未传 videoId 时固定为 false |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 1,
        "name": "示例名称",
        "isDefault": true,
        "videoCount": 1,
        "isFavorited": true
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [DELETE] 删除收藏夹

- 接口路径: DELETE /common/video/folder
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 删除收藏夹（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| folderId | body | integer | 是 | 收藏夹ID |

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

## [GET] 收藏夹视频列表

- 接口路径: GET /common/video/folder/videos
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取收藏夹内的视频列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| folderId | query | integer | 是 | 收藏夹ID |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<FolderVideoItem> | - |
| data.list[].id | integer(uint) | 视频ID |
| data.list[].title | string | 标题 |
| data.list[].cover | string | 封面 |
| data.list[].duration | integer | 时长（秒） |
| data.list[].views | integer(int64) | 播放数 |
| data.list[].danmuCount | integer(int64) | 弹幕数 |
| data.list[].favoriteCount | integer(int64) | 收藏数 |
| data.list[].favoritedAt | string(date-time) | 该视频被收藏到当前收藏夹的时间 |
| data.list[].author | AuthorBrief | - |
| data.list[].author.id | integer(uint) | 用户ID |
| data.list[].author.username | string | 用户名 |
| data.list[].author.avatar | string | 头像URL |

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
        "danmuCount": 1,
        "favoriteCount": 1,
        "favoritedAt": "2024-06-01T12:00:00Z",
        "author": {
          "id": 1001,
          "username": "alice",
          "avatar": "https://cdn.example.com/avatar/1001.png"
        }
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [POST] 发送弹幕

- 接口路径: POST /common/video/danmu/send
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 发送弹幕（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | body | integer | 是 | 视频ID |
| partId | body | integer | 否 | 分P ID（分P视频必填） |
| content | body | string | 是 | 弹幕内容 |
| timeOffset | body | integer | 是 | 视频时间偏移（毫秒） |
| color | body | string | 否 | 弹幕颜色（可选） |
| fontSize | body | integer | 否 | 字体大小（可选） |
| position | body | integer | 否 | 弹幕位置（0滚动 1顶部 2底部）（可选） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | DanmuItem | - |
| data.id | integer(uint) | 弹幕ID |
| data.videoId | integer(uint) | 视频ID |
| data.videoPartId | integer(uint) | 分P ID（单视频为 0） |
| data.userId | integer(uint) | 用户ID |
| data.content | string | 弹幕内容 |
| data.timeOffset | integer | 视频时间偏移（毫秒） |
| data.color | string | 弹幕颜色 |
| data.fontSize | integer | 字体大小 |
| data.position | integer | 弹幕位置（0滚动 1顶部 2底部） |
| data.likeCount | integer(int64) | 点赞数 |
| data.isLiked | boolean | 当前用户是否已点赞 |
| data.createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": 9001,
    "videoId": 2001,
    "videoPartId": 2001,
    "userId": 1001,
    "content": "示例内容",
    "timeOffset": 1,
    "color": "#FFFFFF",
    "fontSize": 1,
    "position": 1,
    "likeCount": 1,
    "isLiked": true,
    "createdAt": "2024-06-01T12:00:00Z"
  },
  "msg": "发送成功"
}
```

## [POST] 点赞弹幕

- 接口路径: POST /common/video/danmu/like
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 点赞/取消点赞弹幕（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| danmuId | body | integer | 是 | 弹幕ID |

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

## [DELETE] 删除弹幕

- 接口路径: DELETE /common/video/danmu
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 用户删除自己的弹幕（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| danmuId | body | integer | 是 | 弹幕ID |

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

## [GET] 历史弹幕

- 接口路径: GET /common/video/danmu/history
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取指定时间段的历史弹幕
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |
| partId | query | integer | 否 | 分P ID（分P视频必填） |
| start | query | integer | 否 | 开始时间偏移（毫秒） |
| end | query | integer | 否 | 结束时间偏移（毫秒） |
| limit | query | integer | 否 | 返回数量上限 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<DanmuItem> | - |
| data.list[].id | integer(uint) | 弹幕ID |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].videoPartId | integer(uint) | 分P ID（单视频为 0） |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].content | string | 弹幕内容 |
| data.list[].timeOffset | integer | 视频时间偏移（毫秒） |
| data.list[].color | string | 弹幕颜色 |
| data.list[].fontSize | integer | 字体大小 |
| data.list[].position | integer | 弹幕位置（0滚动 1顶部 2底部） |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].isLiked | boolean | 当前用户是否已点赞 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 9001,
        "videoId": 2001,
        "videoPartId": 2001,
        "userId": 1001,
        "content": "示例内容",
        "timeOffset": 1,
        "color": "#FFFFFF",
        "fontSize": 1,
        "position": 1,
        "likeCount": 1,
        "isLiked": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 弹幕列表

- 接口路径: GET /common/video/danmu/room/list
- 认证: 可选登录（客户端可携带 Token）
- 依赖接口: 无
- 接口说明: 获取房间弹幕列表；传 start/end 时按播放时间分段返回（按 timeOffset 升序）；不传 start/end 时为历史弹幕，按 createdAt 降序固定返回最近 500 条
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |
| partId | query | integer | 否 | 分P ID（分P视频必填） |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量；分段模式最多 5000、默认 3000；历史弹幕（不传 start/end）忽略该参数，固定返回最近 500 条 |
| start | query | integer | 否 | 播放时间起点（毫秒，含）；与 end 配合使用 |
| end | query | integer | 否 | 播放时间终点（毫秒，不含）；end - start 最大 60000 |
| date | query | string | 否 | 日期筛选（YYYY-MM-DD格式，可选）；历史弹幕模式表示查询该日期当天及更早的弹幕，分段模式表示限定在该日期当天 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<DanmuItem> | - |
| data.total | integer(int64) | 当前查询条件下的弹幕总数 |
| data.returned | integer | 分段模式返回数量 |
| data.truncated | boolean | 分段模式是否因 pageSize 截断 |
| data.start | integer | 分段模式播放时间起点（毫秒，含） |
| data.end | integer | 分段模式播放时间终点（毫秒，不含） |
| data.list[].id | integer(uint) | 弹幕ID |
| data.list[].videoId | integer(uint) | 视频ID |
| data.list[].videoPartId | integer(uint) | 分P ID（单视频为 0） |
| data.list[].userId | integer(uint) | 用户ID |
| data.list[].content | string | 弹幕内容 |
| data.list[].timeOffset | integer | 视频时间偏移（毫秒） |
| data.list[].color | string | 弹幕颜色 |
| data.list[].fontSize | integer | 字体大小 |
| data.list[].position | integer | 弹幕位置（0滚动 1顶部 2底部） |
| data.list[].likeCount | integer(int64) | 点赞数 |
| data.list[].isLiked | boolean | 当前用户是否已点赞 |
| data.list[].createdAt | string(date-time) | 创建时间 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": 9001,
        "videoId": 2001,
        "videoPartId": 2001,
        "userId": 1001,
        "content": "示例内容",
        "timeOffset": 1,
        "color": "#FFFFFF",
        "fontSize": 1,
        "position": 1,
        "likeCount": 1,
        "isLiked": true,
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [GET] 创作者弹幕列表

- 接口路径: GET /common/video/danmu/creator/list
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取自己视频下的弹幕列表（需登录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量 |
| sort | query | integer | 否 | 排序方式（0最近 1点赞最多） 可选: 0/1 |
| keyword | query | string | 否 | 关键字搜索，支持匹配弹幕内容、发送用户、视频标题 |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<CreatorDanmuItem> | - |
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
| data.list[].position | integer | 弹幕位置（0滚动 1顶部 2底部） |
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

## [GET] 弹幕WebSocket

- 接口路径: GET /common/video/danmu/ws
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 弹幕实时推送WebSocket连接
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| videoId | query | integer | 是 | 视频ID |
| partId | query | integer | 否 | 分P ID（分P视频必填） |
| token | query | string | 否 | 用户token（可选） |

响应字段:
- 无

响应示例:
- 无

## 投稿权限限制

当公开站点配置 `data.site.disableUserSubmission=true` 时，以下接口仅管理员可调用：

| 方法 | 接口 | 用途 |
| --- | --- | --- |
| POST | `/common/user/video/tag` | 创建投稿标签 |
| POST | `/common/video/image/upload` | 投稿封面上传 |
| POST | `/common/video/upload/chunk` | 上传分片 |
| GET | `/common/video/upload/status` | 查询续传状态 |
| POST | `/common/video/upload/complete` | 合并上传 |
| POST | `/common/video/publish` | 单文件、分 P、私密及定时投稿 |
| PUT | `/common/video/publish` | 改稿 |

普通用户请求返回业务码 `1`，提示“网站已暂停用户投稿，仅管理员可上传、发布或修改视频”。管理员仍需登录并通过封禁校验。

通用图片上传改用 `POST /common/image/upload`，请求和响应与原图片上传接口一致（multipart 字段 `fileHash`、`cover`，返回 `data.imageUrl`）。该入口供头像、评论、动态和私信使用，不受视频投稿开关影响。

## [PUT] 更新收藏夹

- 接口路径: PUT /common/video/folder
- Tag: 视频管理
- 认证: 需要登录，只允许修改当前用户自己的收藏夹
- 请求体: `folderId` 必填正整数；`name` 必填字符串，长度为 `1` 至 `50`
- 响应: HTTP 200，`code=0` 时 `data` 包含更新后的 `id`、`userId`、`name`、`isDefault`；失败时 `code=1`，原因见 `msg`
