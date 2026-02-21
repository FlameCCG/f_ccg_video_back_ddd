# 私信管理

分类说明：私信/聊天相关

Base URL：/v1

## [GET] 私信WebSocket

- 接口路径: GET /common/chat/ws
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 私信实时推送WebSocket连接（需登录；支持 `Authorization: Bearer <token>` 或 `?token=<token>` 传递登录态）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| token | query | string | 否 | 登录 token；WebSocket 客户端不便设置 Authorization header 时可通过 query 传递 |

响应字段:
- 无

响应示例:
- 无

### WebSocket 消息协议

连接示例：
- `ws://<host>/v1/common/chat/ws?token=<accessToken>`

**客户端发送事件（ChatClientEvent）**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| type | string | 是 | 事件类型：`ping` / `send` / `read` |
| to | integer(uint) | `type=send` 时是 | 接收方用户ID |
| msgType | string | `type=send` 时是 | 消息类型：`text` / `image` / `emoji` / `sticker` |
| text | string | 按类型 | 文本内容；`msgType=text` 时必填，`msgType=emoji` 时可作为兼容回退字段 |
| emoji | string | `msgType=emoji` 时推荐 | emoji 字符，如 `😀` |
| media | ChatMessageMedia | `msgType=image/sticker` 时是 | 媒体信息，至少需要 `media.url` |
| clientMsgId | string | 否 | 客户端消息ID，用于收到 `ack` 时匹配本地 pending 消息 |
| peerId | integer(uint) | `type=read` 时是 | 需要标记已读的对方用户ID |

**服务端推送事件（ChatWSEvent）**

| 字段 | 类型 | 出现场景 | 说明 |
| --- | --- | --- | --- |
| type | string | 全部 | 事件类型：`ack` / `message` / `read_ack` / `system` |
| message | ChatMessage | `ack` / `message` | 消息实体；`ack` 返回自己刚发出的落库消息，`message` 表示收到的新消息 |
| clientMsgId | string | `ack` | 回传客户端消息ID，便于前端把 pending 消息替换为正式消息 |
| peerId | integer(uint) | `read_ack` | 已读确认对应的对方用户ID |
| system | string | `system` | 系统提示文本，如校验失败信息 |

**客户端发送示例**

`ping`:
```json
{ "type": "ping" }
```

`send` 文本消息:
```json
{
  "type": "send",
  "to": 1002,
  "msgType": "text",
  "text": "你好，在吗？",
  "clientMsgId": "2d1a0b41-5a90-4c83-ae2d-0b3e0d43e617"
}
```

`send` emoji 消息:
```json
{
  "type": "send",
  "to": 1002,
  "msgType": "emoji",
  "emoji": "😀",
  "clientMsgId": "9a2c5a2a-9756-4582-b7dc-c3c2d31f2e0a"
}
```

`send` 图片/表情包消息:
```json
{
  "type": "send",
  "to": 1002,
  "msgType": "sticker",
  "media": {
    "url": "https://example.com/sticker.png",
    "mime": "image/png",
    "width": 256,
    "height": 256,
    "size": 20480
  },
  "clientMsgId": "ca3b7b09-ef79-410e-a2c0-55909e35f764"
}
```

`read` 已读事件:
```json
{
  "type": "read",
  "peerId": 1002
}
```

**服务端推送示例**

`ack`:
```json
{
  "type": "ack",
  "clientMsgId": "2d1a0b41-5a90-4c83-ae2d-0b3e0d43e617",
  "message": {
    "id": "65e8c7a0f1a2b3c4d5e6f789",
    "conversationId": "1001_1002",
    "senderId": 1001,
    "receiverId": 1002,
    "type": "text",
    "text": "你好，在吗？",
    "createdAt": "2024-06-01T12:00:00Z"
  }
}
```

`message`:
```json
{
  "type": "message",
  "message": {
    "id": "65e8c7a0f1a2b3c4d5e6f790",
    "conversationId": "1001_1002",
    "senderId": 1002,
    "receiverId": 1001,
    "type": "emoji",
    "emoji": "😀",
    "createdAt": "2024-06-01T12:00:05Z"
  }
}
```

`read_ack`:
```json
{
  "type": "read_ack",
  "peerId": 1002
}
```

`system`:
```json
{
  "type": "system",
  "system": "to 不能为空"
}
```

## [GET] 私信会话列表

- 接口路径: GET /common/chat/conversations
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取当前用户私信会话列表（需登录，pageSize 不限制上限）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| page | query | integer | 否 | 页码 |
| pageSize | query | integer | 否 | 每页数量（不设最大值） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | - |
| data.list | array<ChatConversationItem> | - |
| data.list[].id | string | 会话ID（minUserId_maxUserId） |
| data.list[].peerId | integer(uint) | 对方用户ID |
| data.list[].unread | integer(int64) | 未读数 |
| data.list[].lastMessage | ChatConversationLastMessage | - |
| data.list[].lastMessage.msgId | string | 消息ID（ObjectID Hex） |
| data.list[].lastMessage.type | string | 消息类型 |
| data.list[].lastMessage.preview | string | 预览内容 |
| data.list[].lastMessage.senderId | integer(uint) | 发送者用户ID |
| data.list[].lastMessage.createdAt | string(date-time) | 创建时间 |
| data.list[].updatedAt | integer(int64) | 更新时间（Unix秒） |
| data.total | integer(int64) | - |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": "1001_1002",
        "peerId": 1001,
        "unread": 1,
        "lastMessage": {
          "msgId": "65e8c7a0f1a2b3c4d5e6f789",
          "type": "text",
          "preview": "你好，在吗？",
          "senderId": 1001,
          "createdAt": "2024-06-01T12:00:00Z"
        },
        "updatedAt": 1
      }
    ],
    "total": 1
  },
  "msg": "获取成功"
}
```

## [POST] 创建会话

- 接口路径: POST /common/chat/conversations
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 创建与指定用户的私信会话（需登录，幂等；对方不会看到空会话，直到有人发送消息）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| peerId | body | integer | 是 | 对方用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | ChatConversationItem | - |
| data.id | string | 会话ID（minUserId_maxUserId） |
| data.peerId | integer(uint) | 对方用户ID |
| data.unread | integer(int64) | 未读数 |
| data.lastMessage | ChatConversationLastMessage | - |
| data.lastMessage.msgId | string | 消息ID（ObjectID Hex） |
| data.lastMessage.type | string | 消息类型 |
| data.lastMessage.preview | string | 预览内容 |
| data.lastMessage.senderId | integer(uint) | 发送者用户ID |
| data.lastMessage.createdAt | string(date-time) | 创建时间 |
| data.updatedAt | integer(int64) | 更新时间（Unix秒） |

响应示例:
```json
{
  "code": 0,
  "data": {
    "id": "1001_1002",
    "peerId": 1001,
    "unread": 1,
    "lastMessage": {
      "msgId": "65e8c7a0f1a2b3c4d5e6f789",
      "type": "text",
      "preview": "你好，在吗？",
      "senderId": 1001,
      "createdAt": "2024-06-01T12:00:00Z"
    },
    "updatedAt": 1
  },
  "msg": "创建成功"
}
```

## [DELETE] 删除会话

- 接口路径: DELETE /common/chat/conversations
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 删除与指定用户的会话（需登录；会硬删除该会话及其消息记录）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| peerId | body | integer | 是 | 对方用户ID |

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

## [GET] 私信消息列表

- 接口路径: GET /common/chat/messages
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 获取与指定用户的私信消息（需登录，固定返回10条，按 before 游标向前翻页）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| peerId | query | integer | 是 | 对方用户ID |
| before | query | string | 否 | 游标（ObjectID Hex，取上一页返回的 nextBefore） |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | ChatMessageListResult | - |
| data.list | array<ChatMessage> | 消息列表（旧->新） |
| data.list[].id | string | 消息ID（ObjectID Hex） |
| data.list[].conversationId | string | 会话ID（minUserId_maxUserId） |
| data.list[].senderId | integer(uint) | 发送者用户ID |
| data.list[].receiverId | integer(uint) | 接收者用户ID |
| data.list[].type | string | 消息类型 |
| data.list[].text | string | 文本内容（type=text） |
| data.list[].emoji | string | emoji（type=emoji） |
| data.list[].media | ChatMessageMedia | - |
| data.list[].media.url | string | 资源URL |
| data.list[].media.mime | string | MIME类型 |
| data.list[].media.width | integer | 宽度 |
| data.list[].media.height | integer | 高度 |
| data.list[].media.size | integer(int64) | 文件大小（字节） |
| data.list[].createdAt | string(date-time) | 创建时间 |
| data.nextBefore | string | 下一页游标（用于 before 参数） |
| data.hasMore | boolean | 是否还有更多 |

响应示例:
```json
{
  "code": 0,
  "data": {
    "list": [
      {
        "id": "65e8c7a0f1a2b3c4d5e6f789",
        "conversationId": "1001_1002",
        "senderId": 1001,
        "receiverId": 1001,
        "type": "text",
        "text": "text",
        "emoji": "text",
        "media": {
          "url": "https://example.com/page",
          "mime": "image/png",
          "width": 1,
          "height": 1,
          "size": 1
        },
        "createdAt": "2024-06-01T12:00:00Z"
      }
    ],
    "nextBefore": "2024-06-01T12:00:00Z",
    "hasMore": true,
    "total": 1
  },
  "msg": "获取成功"
}
```

## [PUT] 私信标记已读

- 接口路径: PUT /common/chat/read
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 无
- 接口说明: 标记与指定用户的私信为已读（需登录，红点清零）
- HTTP 状态码: 200（业务码 code 判断成功/失败）
- 响应结构: code=0 成功，code=1 失败；msg 为提示信息

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| peerId | body | integer | 是 | 对方用户ID |

响应字段:
| 字段 | 类型 | 说明 |
| --- | --- | --- |
| data | object | 响应数据 |

响应示例:
```json
{
  "code": 0,
  "data": {},
  "msg": "标记成功"
}
```
