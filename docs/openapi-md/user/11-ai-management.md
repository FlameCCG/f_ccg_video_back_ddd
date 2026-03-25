# AI 管理

Base URL：`/v1`。模型与能力以当前后台配置为准；聊天使用 Eino Claude ChatModel 适配 Anthropic Messages 协议，图片通过 Eino Ark、视频通过方舟 runtime SDK 调用。这里的站内路由不是上游 OpenAI Responses API 的原样代理。

普通 JSON 响应采用 `{code,data,msg}`；聊天返回 SSE，资源代理返回二进制流。聊天、图片生成/编辑和视频创建共用 AI 额度限制，耗尽时返回 HTTP **429**、`code=1` 和 `Retry-After: 60`；Redis 额度服务不可用时拒绝生成请求。

## [GET] 模型与思考选项

- 路径：`GET /common/ai/options`，无需登录。
- 返回 `data.chatModel`、`chatModels`、`thinkingEnabled`、`thinkingEffort`、`thinkingEfforts`、`imageModel`、`imageModels`、`videoModel`、`videoModels`，不返回密钥。
- 选项项为 `{label,value}`。界面展示 label，请求使用 value；请求省略 model 时使用后台默认值。以下模型名来自配置示例，实际调用应使用当前选项。

## [POST] AI 对话与站内视频检索

- 路径：`POST /common/ai/responses`，需要登录。
- 聊天依赖 `ai.chatModelBaseURL`、`chatModelAPIKey`、`chatModel`，系统提示词由 `systemPrompt` 配置。
- 请求体：

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| model | string | 否 | 当前配置允许的聊天模型，省略时用默认值 |
| input | array | 是 | 消息数组；role 与 content 表达历史和当前输入 |
| thinking | boolean | 否 | 省略时跟随后台开关；后台关闭时不能由请求强制开启 |
| thinkingEffort | string | 否 | 思考强度，省略时用后台默认值；当前 DeepSeek 强度为 high / max |

```json
{
  "model": "deepseek-v4-flash",
  "thinking": true,
  "thinkingEffort": "high",
  "input": [
    {"role":"user","content":[{"type":"input_text","text":"帮我找做饭的猫咪视频"}]}
  ]
}
```

`input` 可包含 `input_image`，但当前聊天转换把图片变为文本标记，不把图片作为视觉输入传给聊天模型；最近用户图片可参与站内向量检索，不能把此接口当作通用图像理解接口。

历史由客户端随请求提供，服务端按 `historyMaxTurns` / `historyMaxChars` 选择窗口。站内候选查询可用时，后端先分析找视频意图；需要检索时优先向量/关键词召回，故障或无结果时按相应用例回退到 MySQL 或最新已发布作品。候选作品、分 P 和 `/video/:id/:p?` 路由注入提示词，数量由 `vectorTopK` 控制；是否输出 `video_results` 由模型回答决定。分析超时可降级为普通回答。

成功建立流后，Content-Type 为 `text/event-stream`。每一帧只有 `data:`，事件类型在 JSON 的 `type` 字段中，**没有具名 `event:` 行**：

```text
data: {"type":"response.reasoning_summary_text.delta","delta":"先整理检索条件。"}

data: {"type":"response.output_text.delta","delta":"[video_results]\n- [猫咪做饭](/video/11)\n[/video_results]"}

data: {"type":"response.completed"}

data: [DONE]
```

思考增量可省略；`response.completed` 不附带上游响应 ID、模型或 usage。建立流前失败返回普通 JSON；流建立后发生上游错误，发送 `{"type":"response.failed","message":"AI 回答中断，请稍后重试"}`，随后结束流，不再发送成功完成标记。客户端断开时流直接结束。

## [GET] EventSource 对话入口

- 路径：`GET /common/ai/responses/stream`，需要登录；可用 query token 认证。
- 参数：

| 字段 | 位置 | 必填 | 说明 |
|---|---|---|---|
| token | query | 原生 EventSource 时需要 | 访问令牌；也支持 Authorization header |
| text | query | 与 input 至少一个 | 自动包装成 user 文本消息 |
| input | query | 与 text 至少一个 | URL 编码后的消息数组 JSON；同时提供时优先使用 input |
| model | query | 否 | 当前允许的聊天模型 |
| thinking | query | 否 | 与 POST 相同 |
| thinkingEffort | query | 否 | 与 POST 相同 |

SSE 与 POST 一致。原生 EventSource 应监听默认 `message`，再分派 JSON 的 type；完成或失败后主动关闭，避免自动重连重新生成：

```js
const url = new URL("/v1/common/ai/responses/stream", window.location.origin);
url.searchParams.set("token", token);
url.searchParams.set("text", "帮我找做饭的猫咪视频");
const es = new EventSource(url);
es.onmessage = (event) => {
  if (event.data === "[DONE]") { es.close(); return; }
  const payload = JSON.parse(event.data);
  if (payload.type === "response.output_text.delta") console.log(payload.delta);
  if (payload.type === "response.reasoning_summary_text.delta") console.log(payload.delta);
  if (payload.type === "response.completed" || payload.type === "response.failed") es.close();
};
es.onerror = () => es.close();
```

## [POST] 图片生成

- 路径：`POST /common/ai/images/generations`，需要登录与 `ai.doubaoModelAPIKey`。
- `prompt` 必填；`n` 为 1～15，省略时 1；model 使用配置的图片模型。
- `response_format` 支持 `url` / `b64_json`；`size` 优先于 `resolution` / `aspect_ratio`。分辨率映射支持 1k、2k、3k、4k，实际尺寸按模型最小像素要求归一。

```json
{
  "model":"doubao-seedream-5-0-lite-260128",
  "prompt":"黄昏图书馆中的机器人管理员",
  "n":1,
  "aspect_ratio":"16:9",
  "resolution":"2k",
  "response_format":"url"
}
```

```json
{
  "code":0,
  "data":{
    "model":"doubao-seedream-5-0-lite-260128",
    "data":[{"url":"/v1/common/ai/assets?url=URL_ENCODED_UPSTREAM_URL"}]
  },
  "msg":"ok"
}
```

URL 结果由 Handler 改写为站内代理地址，base64 结果放在 `b64_json`；不保证返回 usage 或成本字段。

## [POST] 图片编辑

- 路径：`POST /common/ai/images/edits`，需要登录与豆包密钥。
- `prompt` 必填，`image` 与 `images` 二选一；引用支持 URL 字符串或 `{url,type}` 对象。
- 参考图最多 14 张，且参考图数量 + 请求生成数量不得超过 15；`n`、尺寸和返回格式与图片生成相同。
- `quality`、`style`、`user`、`mask` 等保留兼容输入字段，当前调用链不将它们作为图像生成选项下发。

```json
{
  "prompt":"让第一张图的人物穿上第二张图的服装，背景改为黄昏街景",
  "images":["https://example.com/source.png","https://example.com/style.png"],
  "n":1,
  "aspect_ratio":"16:9",
  "resolution":"2k",
  "response_format":"url"
}
```

## [POST] 视频生成

- 路径：`POST /common/ai/videos/generations`，需要登录与豆包密钥。
- 支持文本、首帧 `image` / `input_reference`、`reference_images`；参考图最多 7 张，须带 prompt，不能与首帧混用。
- `duration` 优先于兼容字段 `seconds`；示例 pro 模型允许 4～15 秒，fast 模型允许 2～12 秒，其他模型按当前政策允许 1～15 秒。
- `resolution` 接受 480p / 720p / 1080p，`aspect_ratio` 指定比例；可传 `generate_audio`、`camera_fixed`、`watermark`、`seed`。

```json
{
  "model":"doubao-seedance-1-5-pro-251215",
  "prompt":"雨夜霓虹城市的缓慢航拍镜头",
  "duration":8,
  "aspect_ratio":"16:9",
  "resolution":"720p"
}
```

创建结果保留方舟 SDK 字段，并补充 camelCase `requestId` 供后续查询使用：

```json
{"code":0,"data":{"id":"task_xxx","requestId":"task_xxx"},"msg":"ok"}
```

## [GET] 视频任务状态

- 路径：`GET /common/ai/videos/status?requestID=task_xxx`，需要登录。
- `requestID` 是 **query 参数**，值取创建结果的 `data.requestId` 或 `data.id`。
- 返回方舟任务结构并补充 `requestId`；本地 SDK 状态包括 queued / running / succeeded / failed / cancelled。不要按旧示例的 pending / done 判断，也不要假设有 progress 字段。
- 成功结果的视频 URL 在 `data.content.video_url`，由 Handler 改写为资源代理；其他 SDK 字段按上游实际响应返回。

## [GET] AI 资源代理

- 路径：`GET /common/ai/assets?url=<编码后的上游URL>`，无需登录。
- 仅允许当前领域白名单中的火山相关域名（volces.com、volcengine.com、byteimg.com、bytedance.com 及其子域），并受适配器地址校验约束；不能代理任意 example.com 地址。
- 转发 Range 等请求头及部分响应头，HTTP 状态透传上游，响应体为原始二进制。参数或访问检查失败时返回业务 JSON。

详细字段定义见 [OpenAPI](../../openapi.yaml)，实际映射见 [AI Handler](../../../internal/interfaces/http/handler/ai) 与 [方舟适配器](../../../internal/infrastructure/external/ark)。
