# AI 管理

分类说明：AI 文本生成、图像理解、图片生成/编辑、视频生成与状态查询、资源代理

Base URL：/v1

## [POST] AI 文本生成 / 图像理解

- 接口路径: POST /common/ai/responses
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 后台已配置 `ai` 的 `chatModelBaseURL/chatModelAPIKey/chatModel/systemPrompt`
- 接口说明: 代理 AI `/v1/responses`，用于文本对话与图像理解
- 说明:
  - 当前对外仅保留 `model`、`input` 两个字段
  - 若后台配置了 `systemPrompt`，且本轮没有显式传 `system` 消息，后端会自动注入
  - 正式流式请求前，后端会先发起一轮非流式 AI 分析请求，让模型判断当前是否是“找视频/找作品”请求，并返回关键词数组
  - 如果第一轮返回的关键词数组为空，后端不会查询数据库，直接进入第二轮正式回答
  - 如果第一轮返回的关键词数组非空，后端会使用这些 AI 提炼后的关键词查询数据库
  - 如果数据库没有命中任何已发布作品，后端会回退到按最新发布时间取前 `vectorTopK` 个作品
  - 第二轮正式回答时，后端会把作品标题、描述、tags、分P标题和路由一起发给模型
  - `vectorTopK` 由配置文件控制，同时限制向量召回条数与第二轮注入候选作品数
  - 是否输出 `video_results` 结果块由第二轮模型结合当前对话自主判断；如果不是找视频意图，模型会忽略候选作品并继续正常聊天
  - 找视频时返回的标题会使用 Markdown 链接，路由格式为 `/video/:id/:p?`，便于前端直接渲染和跳转
  - 服务端固定 `store=false`，不会把对话保存到 AI 服务器
  - 服务端固定走 SSE，不需要前端传 `stream`
- HTTP 状态码: 200
- 响应说明:
  - 成功建立流后，返回 `text/event-stream`
  - 若在建立流之前就失败（例如参数错误、上游 AI 返回 4xx/5xx、内部分析请求失败且正式请求也失败），则返回标准 JSON：`{code,data,msg}`

请求体参数:
| 名称 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| model | string | 否 | 文本模型；未传时使用后台默认值 |
| input | array | 是 | 消息数组；`content` 可混合 `input_text` 与 `input_image` |

请求体示例:

```json
{
  "model": "grok-4.20",
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "请描述图中内容并提取可见文字"
        },
        {
          "type": "input_image",
          "image_url": "https://example.com/sample.png",
          "detail": "high"
        }
      ]
    }
  ]
}
```

找视频请求示例:

```json
{
  "model": "grok-4.20",
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "帮我找做饭的猫咪视频"
        }
      ]
    }
  ]
}
```

响应示例（SSE）:

```text
event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"图中是一只坐在沙发上的橘猫。"}

event: response.completed
data: {"type":"response.completed","response":{"id":"resp_123","status":"completed","model":"grok-4.20"}}
```

找视频响应示例（SSE 文本内容）:

```text
event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"[video_results]\n- [猫咪做饭](/video/11)：一只猫认真做饭，和你的需求最接近。\n- [猫咪日常合集 · P1 做饭篇](/video/12/1)：如果你想看合集里的做饭片段，可以直接点这一P。"}

event: response.output_text.delta
data: {"type":"response.output_text.delta","delta":"\n[/video_results]"}
```

## [GET] AI 文本流式输出（EventSource）

- 接口路径: GET /common/ai/responses/stream
- 认证: 需要登录；原生 `EventSource` 不能自定义请求头，因此需要通过 query 参数传 `token`
- 依赖接口: 后台已配置 `ai` 的 `chatModelBaseURL/chatModelAPIKey/chatModel/systemPrompt`
- 接口说明: 面向浏览器原生 `new EventSource()` 的简化流式入口；适合纯文本聊天或找视频场景
- 说明:
  - 推荐传 `text`，后端会自动包装成用户消息
  - 如需高级用法，也可传 URL 编码后的 `input` JSON
  - 当 `input` 与 `text` 同时存在时，后端优先使用 `input`
  - 成功建立流后返回标准 `text/event-stream`；若建立流前失败，则返回标准 JSON：`{code,data,msg}`

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| token | query | string | 是 | JWT token，可不带 `Bearer ` 前缀 |
| text | query | string | 否 | 用户文本；和 `input` 至少传一个 |
| input | query | string | 否 | URL 编码后的原始 input JSON |
| model | query | string | 否 | 文本模型；未传时使用后台默认值 |

EventSource 示例:

```js
const url = new URL("/v1/common/ai/responses/stream", window.location.origin);
url.searchParams.set("token", token);
url.searchParams.set("text", "帮我找做饭的猫咪视频");
url.searchParams.set("model", "grok-4.20");

const es = new EventSource(url);
es.addEventListener("response.output_text.delta", (event) => {
  const payload = JSON.parse(event.data);
  console.log(payload.delta);
});
es.addEventListener("response.completed", () => {
  es.close();
});
```

## [POST] AI 图片生成

- 接口路径: POST /common/ai/images/generations
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 后台已配置 `ai`
- 接口说明: 代理 AI `/v1/images/generations`
- 说明: 支持文生图、批量生成、纵横比与分辨率控制
- HTTP 状态码: 200（业务码 code 判断成功/失败）

请求体示例:

```json
{
  "model": "grok-imagine-image",
  "prompt": "A cinematic portrait of a robot librarian in a futuristic archive",
  "n": 1,
  "aspect_ratio": "16:9",
  "resolution": "1k",
  "response_format": "url"
}
```

响应示例:

```json
{
  "code": 0,
  "data": {
    "data": [
      {
        "url": "https://example.com/temp-image.png",
        "mime_type": "image/png"
      }
    ],
    "usage": {
      "cost_in_usd_ticks": 12345
    }
  },
  "msg": "ok"
}
```

## [POST] AI 图片编辑

- 接口路径: POST /common/ai/images/edits
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 后台已配置 `ai`
- 接口说明: 代理 AI `/v1/images/edits`
- 说明:
  - `image` 与 `images` 必须二选一
  - 单图编辑不支持自定义 `aspect_ratio`
- HTTP 状态码: 200（业务码 code 判断成功/失败）

请求体示例:

```json
{
  "model": "grok-imagine-image",
  "prompt": "让第一张图的人物穿上第二张图的服装，背景改为黄昏街景",
  "images": ["https://example.com/source.png", "https://example.com/style.png"],
  "aspect_ratio": "16:9",
  "resolution": "1k",
  "response_format": "url"
}
```

## [POST] AI 视频生成

- 接口路径: POST /common/ai/videos/generations
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 后台已配置 `ai`
- 接口说明: 代理 AI `/v1/videos/generations`
- 说明:
  - 支持文生视频、图生视频、参考图视频
  - 不包含 `edit-video` 与 `extend-video`
  - `reference_images` 最多 7 张，且不能与 `image` / `input_reference` 混用
- HTTP 状态码: 200（业务码 code 判断成功/失败）

请求体示例（文生视频）:

```json
{
  "model": "grok-imagine-video",
  "prompt": "A slow aerial shot of a neon city at night in the rain",
  "duration": 8,
  "aspect_ratio": "16:9",
  "resolution": "480p"
}
```

响应示例:

```json
{
  "code": 0,
  "data": {
    "request_id": "video_req_xxx"
  },
  "msg": "ok"
}
```

## [GET] 查询 AI 视频任务状态

- 接口路径: GET /common/ai/videos/status
- 认证: 需要登录（客户端全局自动携带 Token）
- 依赖接口: 先调用视频生成接口拿到 `request_id`
- 接口说明: 查询视频生成任务状态
- 说明: `status` 可能为 `pending` / `done` / `failed` / `expired`
- HTTP 状态码: 200（业务码 code 判断成功/失败）

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| requestID | path | string | 是 | 视频生成任务 ID |

响应示例:

```json
{
  "code": 0,
  "data": {
    "status": "done",
    "progress": 100,
    "model": "grok-imagine-video",
    "video": {
      "url": "https://example.com/temp-video.mp4",
      "duration": 8,
      "respect_moderation": true
    }
  },
  "msg": "ok"
}
```

## [GET] AI 资源代理（图片/视频流）

- 接口路径: GET /common/ai/assets
- 认证: 无需登录
- 依赖接口: 无
- 接口说明: 代理拉取上游 AI 临时资源（生成图/视频 URL），透传上游状态码与部分响应头，响应体为原始二进制流
- HTTP 状态码: 透传上游；参数错误时可能返回业务 JSON 错误

请求参数:
| 名称 | 位置 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- | --- |
| url | query | string(uri) | 是 | 上游资源完整 URL |

响应说明:
- Content-Type 随上游（如 image/png、video/mp4、application/octet-stream）
- 响应体为二进制流，非标准 `{code,data,msg}` JSON

请求示例:
```
GET /v1/common/ai/assets?url=https%3A%2F%2Fexample.com%2Ftemp-image.png
```
