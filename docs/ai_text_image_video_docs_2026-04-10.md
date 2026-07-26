# AI Docs 全面梳理：TEXT / IMAGE / VIDEO 能力、参数、返回结构与注意事项

> **重构提示**：本文档从重构前的 `f_ccg_video_back` 继承，所述**设计与行为在本仓库 1:1 保留**，
> 但文中的**代码路径引用指向重构前的目录布局**。请配合
> [legacy-path-mapping.md](./legacy-path-mapping.md) 换算到新的 DDD 分层位置。


- 文档目标：系统梳理 AI 官方文档中与 **Text / Image / Video** 相关的能力说明、接口参数、返回体、限制、兼容性差异与已知坑点。
- 资料范围：基于 `docs.x.ai` 官方开发者文档与官方 `openapi.json`。
- 检查日期：**2026-04-10（Asia/Tokyo）**
- 结论原则：
  - 以 **官方文档页面** + **官方 OpenAPI schema** 为主。
  - 当页面说明与 OpenAPI schema 不一致时，**单独标注冲突**。
  - 尽量区分：**能力说明**、**请求参数**、**返回结构**、**限制/默认值**、**兼容性说明**。

---

## 目录

1. [总览](#1-总览)
2. [TEXT 能力总览](#2-text-能力总览)
3. [Responses API：文本生成主接口](#3-responses-api文本生成主接口)
4. [TEXT 相关高级能力](#4-text-相关高级能力)
5. [IMAGE：图像理解（Image Understanding）](#5-image图像理解image-understanding)
6. [IMAGE：图像生成与编辑（Image Generation / Editing）](#6-image图像生成与编辑image-generation--editing)
7. [VIDEO：视频生成、参考图生成、编辑、续写](#7-video视频生成参考图生成编辑续写)
8. [返回体字段总表](#8-返回体字段总表)
9. [限制、兼容性与已知坑](#9-限制兼容性与已知坑)
10. [官方文档不一致之处](#10-官方文档不一致之处)
11. [官方来源链接](#11-官方来源链接)

---

## 1. 总览

AI 文档当前把模型能力分为 Text、Images、Video、Audio、Files 等模块。对于你关心的三类：

- **TEXT**：以 **Responses API** 为主，推荐替代旧的 Chat Completions；支持状态化多轮、推理模型、结构化输出、工具调用、流式输出等。
- **IMAGE**：分成两类：
  1. **图像理解**：把图像作为输入，输出文本；本质上仍走文本/Responses 路径。
  2. **图像生成与编辑**：专门的图片生成/编辑接口与 SDK 封装，支持文生图、图像编辑、多图参考编辑、批量生成、分辨率/纵横比控制。
- **VIDEO**：专门的视频生成体系，支持：
  - 文生视频（text-to-video）
  - 图生视频（image-to-video）
  - 参考图视频（reference-to-video）
  - 视频编辑（edit-video）
  - 视频续写 / 延长（extend-video）

从接口设计上看：

- **文本输出类能力**（包括普通文本生成、推理、结构化输出、图像理解）主要汇总在 `/v1/responses`。
- **图片生成/编辑** 与 **视频生成/编辑/轮询** 则是独立媒体接口。
- 官方还提供 AI SDK、OpenAI SDK 兼容调用方式，以及 Vercel AI SDK 示例。

---

## 2. TEXT 能力总览

### 2.1 官方推荐的文本接口

官方明确推荐使用 **Responses API**，而不是旧的 Chat Completions API。核心原因：

- 原生支持 **状态化对话**：通过 `previous_response_id` 续接上下文。
- 支持 **服务端存储**：默认可保存响应，便于继续对话。
- 对 **推理模型** 支持更完整。
- 原生支持更多 **工具能力**（例如搜索、代码执行、MCP 等）。
- 新能力优先落在 Responses API。

### 2.2 文本能力的主要组成

AI 文档中的 TEXT 能力可拆成五块：

1. **普通文本生成**
2. **推理（Reasoning）**
3. **结构化输出（Structured Outputs）**
4. **流式输出（Streaming）**
5. **与旧 Chat Completions API 的映射/迁移**

### 2.3 典型能力边界

官方文档对文本模型能力的描述包括：

- 长上下文文本理解与生成
- 多轮对话
- 结构化 JSON 输出
- 工具调用
- 图像输入后的文本分析（属于多模态文本输出路径）
- 推理 token 统计与部分模型的加密推理内容传递

### 2.4 上下文窗口与知识边界

在模型与定价页中，官方对旗舰模型（如 Grok 4.20）给出了 **2,000,000 token** 级别上下文窗口的说明，并提醒模型默认**不具备实时世界知识**；需要开启搜索类工具才能获取最新信息。官方同时给出了一些模型的知识截止说明（例如部分页面写到 Grok 3 / Grok 4 的知识截止为 2024 年 11 月）。

---

## 3. Responses API：文本生成主接口

> 这是 AI 文档当前最核心的文本接口。

### 3.1 Endpoint

```http
POST /v1/responses
```

Base URL：

```text
https://api.x.ai
```

认证：

```http
Authorization: Bearer <AI_API_KEY>
Content-Type: application/json
```

### 3.2 与 Chat Completions 的参数映射

| Chat Completions | Responses API          | 说明                           |
| ---------------- | ---------------------- | ------------------------------ |
| `messages`       | `input`                | 多条消息输入                   |
| `max_tokens`     | `max_output_tokens`    | 最大输出 token 数              |
| 无               | `previous_response_id` | 续接已存储的前一轮响应         |
| 无               | `store`                | 控制服务端是否存储             |
| 无               | `include`              | 请求额外返回内容，例如推理密文 |

### 3.3 核心请求体字段（综合官方页面 + OpenAPI）

下面按“字段 / 类型 / 含义 / 备注”梳理。

#### 3.3.1 `model`

- 类型：`string`
- 含义：指定使用的模型
- 备注：不同模型支持的能力不同；推理模型、图像理解模型、普通语言模型、媒体模型不是完全同一套。

#### 3.3.2 `input`

- 类型：多态输入
- 含义：主输入内容
- 可包含：
  - 文本
  - 消息数组
  - 图像输入
  - 文件输入
- 说明：在 Responses API 中，`input` 是取代 `messages` 的统一入口。

#### 3.3.3 `instructions`

- 类型：`string`
- 含义：类似“系统指令”的高层说明
- 备注：
  - OpenAPI 中存在该字段，并说明它是 system prompt 的替代形式。
  - 但文档另有页面写明该参数“当前不支持”。
  - 如果使用 `previous_response_id`，某些文档说明 `instructions` 不能与之同时使用。
- 建议：如果追求兼容性，优先把系统层指令写进 `input` 中的首条 system / developer 消息。

#### 3.3.4 `previous_response_id`

- 类型：`string`
- 含义：续接上一轮已保存的 response
- 作用：
  - 不必每轮重发完整历史
  - 由服务端自动关联上下文
- 注意：虽然无需重复传历史，但**计费上**官方说明仍然按完整历史计算，只是部分内容可能被缓存优化。

#### 3.3.5 `store`

- 类型：`boolean`
- 默认值：`true`
- 含义：是否将响应保存在服务端
- 作用：
  - 便于后续通过 `previous_response_id` 继续对话
  - 官方文档说明，默认会保存部分提示词、模型响应与推理内容，保存窗口为 **30 天**
- 关闭方式：`store: false`

#### 3.3.6 `include`

- 类型：数组
- 含义：要求额外返回某些可选字段
- 官方已示例支持：
  - `reasoning.encrypted_content`
- 用途：把前一轮的加密推理内容拿出来，后续再传回模型，以避免重新思考整段链路。

#### 3.3.7 `max_output_tokens`

- 类型：`integer`
- 含义：限制最大输出 token 数
- 关键点：OpenAPI 中说明它统计的不是纯可见输出，还**可能包含 reasoning tokens**。

#### 3.3.8 `max_turns`

- 类型：整数
- 含义：限制模型在多步/代理式流程中可执行的回合数
- 备注：更偏 agentic / multi-turn tool 使用场景。

#### 3.3.9 `temperature`

- 类型：`number`
- 含义：采样温度
- 备注：常规生成控制参数。

#### 3.3.10 `top_p`

- 类型：`number`
- 含义：nucleus sampling 参数

#### 3.3.11 `logprobs`

- 类型：`boolean` 或相关配置
- 含义：请求返回 token 级概率信息
- 注意：部分模型页写明某些模型会忽略 `logprobs`。

#### 3.3.12 `top_logprobs`

- 类型：`integer`
- 范围：文档中可见 **0–8**
- 含义：为每个位置返回 top-N token 的 logprobs

#### 3.3.13 `parallel_tool_calls`

- 类型：`boolean`
- 默认值：`true`
- 含义：是否允许并行工具调用

#### 3.3.14 `tool_choice`

- 类型：字符串或对象
- 含义：指定工具使用策略
- 用途：约束模型是否必须/允许调用工具。

#### 3.3.15 `tools`

- 类型：数组
- 上限：文档里可见 **最多 128 个**
- 说明：当前文档明确提到支持的工具类型包括：
  - function tools
  - web search
  - 以及更广义文档中提到的 MCP / code execution 等 agentic 工具能力

#### 3.3.16 `reasoning`

- 类型：对象或枚举风格配置
- 含义：推理相关设置
- 注意：并非所有模型支持。
- 重要：官方 Reasoning 页面指出，`grok-4.20` 与 `grok-4-1-fast` **不支持**传统 `reasoning_effort`；而 `grok-4.20-multi-agent` 接受 `reasoning` 参数来控制 agent 数量。

#### 3.3.17 `search_parameters`

- 类型：对象
- 含义：覆盖 web search tool 的搜索行为参数
- 备注：只在使用搜索工具时有意义。

#### 3.3.18 `prompt_cache_key`

- 类型：字符串
- 含义：用于 prompt caching 相关优化

#### 3.3.19 `stream`

- 类型：`boolean`
- 默认值：`false`
- 含义：是否使用流式输出（SSE）

#### 3.3.20 `text`

- 类型：对象
- 含义：文本输出配置
- 典型用途：
  - 结构化输出模式
  - 指定 `format` / schema

#### 3.3.21 `metadata`

- 类型：对象
- 说明：OpenAPI 与部分页面把它标成兼容性字段，但文档中也有“当前不支持/仅兼容占位”的语义，不能过度依赖。

#### 3.3.22 `service_tier`

- 类型：字符串
- 默认值：`default`
- 备注：部分页面显示该字段存在，但又标记“当前不支持”或仅兼容保留。

#### 3.3.23 `truncation`

- 类型：字符串/配置
- 备注：有页面显示默认 `disabled`，OpenAPI 相关说明中也出现“not supported”的字样；说明这一能力目前并不稳定，谨慎使用。

#### 3.3.24 `frequency_penalty`

- 备注：部分页面明确写 **NOT SUPPORTED**。

#### 3.3.25 `presence_penalty`

- 备注：部分页面明确写 **NOT SUPPORTED**。

#### 3.3.26 `stop`

- 注意：推理模型页面明确说某些 reasoning model **不支持** `stop`。

---

### 3.4 输入消息结构

Responses API 的 `input` 可以写成消息数组。典型对象结构类似：

```json
[
  {
    "role": "user",
    "content": "Explain quantum tunneling simply."
  }
]
```

也可以写成更细粒度的 content parts，例如文本与图片混合：

```json
[
  {
    "role": "user",
    "content": [
      {
        "type": "input_text",
        "text": "Describe this image"
      },
      {
        "type": "input_image",
        "image_url": "https://example.com/image.jpg"
      }
    ]
  }
]
```

### 3.5 角色（role）

文档中可见：

- `system`
- `developer`
- `user`
- `assistant`

某些页面说明：

- `developer` 是 `system` 的别名或等价高优先级角色
- 只应有一条 system/developer 消息，且建议放在最前面

由于该点在不同页面措辞略有差异，实践中建议：

- 只放一条高优先级指令消息
- 放在最前面
- 避免同时混用多条 system/developer

---

### 3.6 响应体顶层结构（Responses API）

根据页面示例与 OpenAPI，典型返回体核心字段包括：

- `id`
- `object`（通常为 `response`）
- `created_at`
- `completed_at`
- `status`
- `model`
- `output`
- `usage`
- `error`
- `store`
- `temperature`
- `tool_choice`
- `text`
- `reasoning`
- 以及若干兼容字段

### 3.7 `status` 状态值

文档中可见：

- `completed`
- `in_progress`
- `incomplete`

### 3.8 `output` 的结构化返回方式

Responses API 与旧 Chat Completions 最大的结构差异在于：

- Chat Completions：通常在 `choices[0].message.content`
- Responses API：在 `output[]` 中返回**带类型的项**

典型结构：

```json
{
  "id": "resp_123",
  "output": [
    {
      "type": "message",
      "role": "assistant",
      "content": [
        {
          "type": "output_text",
          "text": "Hello! How can I help you?"
        }
      ]
    }
  ]
}
```

### 3.9 `output` 中常见内容项

#### 3.9.1 `message`

- 表示一个 assistant message
- 通常包含 `role: assistant`
- 其下的 `content[]` 再细分为不同内容类型

#### 3.9.2 `output_text`

- 可见自然语言文本输出
- 可能伴随：
  - `annotations`
  - `logprobs`

#### 3.9.3 `annotations`

- 用于引用、来源标注等附加说明
- 在带搜索的输出中尤其有意义

#### 3.9.4 `reasoning`

- 可能以专门对象形式返回
- 可能包括：
  - `content[]`
  - `encrypted_content`
  - `summary`

---

### 3.10 `usage` 计费/用量字段

OpenAPI 与文档示例中可见的 usage 字段较多，包含但不限于：

- `prompt_tokens`
- `completion_tokens`
- `total_tokens`
- `input_tokens`
- `output_tokens`
- `reasoning_tokens`
- `prompt_tokens_details`
- `completion_tokens_details`
- `input_tokens_details`
- `output_tokens_details`
- `cached_tokens`
- `num_sources_used`
- `num_server_side_tools_used`
- `cost_in_usd_ticks`

### 3.11 `usage` 字段的阅读方式

由于不同页面与示例存在命名差异，可按下面方式理解：

- **prompt/input**：输入侧 token 统计
- **completion/output**：输出侧 token 统计
- **reasoning_tokens**：模型内部推理 token
- **cached_tokens**：命中缓存的 token
- **num_sources_used**：使用了多少搜索来源
- **num_server_side_tools_used**：服务端工具调用次数
- **cost_in_usd_ticks**：成本刻度字段

建议在代码里：

1. 同时兼容 `prompt_tokens` / `input_tokens`
2. 同时兼容 `completion_tokens` / `output_tokens`
3. 单独读取 `reasoning_tokens`

---

## 4. TEXT 相关高级能力

### 4.1 状态化多轮（Stateful Conversations）

#### 核心机制

- 默认 `store: true`
- 返回一个 `response.id`
- 下一轮用 `previous_response_id` 继续

#### 优点

- 不必重传历史消息
- 可减少客户端上下文管理复杂度
- 有缓存优化

#### 重要注意

官方说明中提到：

- 即使你不重发历史，**计费仍可能按完整上下文计算**
- 部分历史可能被缓存，从而优化成本或性能
- 存储窗口是 **30 天**

### 4.2 推理（Reasoning）

#### 4.2.1 推理模型的特点

官方把 reasoning model 描述为：

- 会进行更长的逐步思考
- 会在 usage 中暴露 `reasoning_tokens`
- 某些模型支持返回或传递 **encrypted reasoning**

#### 4.2.2 不支持的传统采样参数

Reasoning 页面明确写到，一些推理模型不支持：

- `presencePenalty`
- `frequencyPenalty`
- `stop`

#### 4.2.3 `reasoning_effort` 的支持差异

官方特别说明：

- `grok-4.20`
- `grok-4-1-fast`

这两类不支持传统的 `reasoning_effort`。

而：

- `grok-4.20-multi-agent`

支持通过 `reasoning` 参数控制 agent 数量：

- `low` / `medium`：4 个 agents
- `high` / `xhigh`：16 个 agents

#### 4.2.4 加密推理内容（Encrypted Reasoning）

可通过：

```json
"include": ["reasoning.encrypted_content"]
```

要求响应返回加密推理内容。然后后续请求中可以将其继续传给模型，避免重新思考整段推理过程。

#### 4.2.5 计费注意

官方说明 reasoning tokens 会计费；multi-agent 模式下，领导 agent 与子 agents 的推理成本都会计入。

---

### 4.3 Structured Outputs（结构化输出）

这是 AI 文档中非常重要的一块。

#### 4.3.1 能力定义

Structured Outputs 的目标是：

- 让模型输出**严格符合某个 schema**
- 用于可靠 JSON 输出、数据提取、流程自动化

#### 4.3.2 支持情况

文档说明该能力适用于全部语言模型。

#### 4.3.3 支持的 schema 类型

官方文档列出的支持情况包括：

- `string`
  - 但 `minLength` / `maxLength` 不支持
- `number`
- `integer`
- `float`
- `object`
- `array`
  - 但 `minItems` / `maxItems` 不支持
  - `maxContains` / `minContains` 不支持
- `boolean`
- `enum`
- `anyOf`

不支持：

- `allOf`

#### 4.3.4 Responses API 中的写法

官方文档示例把 schema 放在：

```json
"text": {
  "format": {
    "type": "json_schema",
    "name": "invoice",
    "schema": { ... },
    "strict": true
  }
}
```

#### 4.3.5 与工具共存

文档明确提到 structured outputs 可以与工具能力同时使用。

#### 4.3.6 SDK 差异

文档展示了两种思路：

- `response_format`：返回 JSON 字符串，你自己解析
- `parse()`：SDK 帮你自动解析成结构化对象

---

### 4.4 Streaming（流式输出）

#### 4.4.1 支持范围

官方说明：

- 所有具备**文本输出能力**的模型都支持 streaming
- 包括普通聊天、图像理解后的文本输出
- **不支持图片生成输出** 的 streaming

#### 4.4.2 开启方式

```json
"stream": true
```

#### 4.4.3 协议形态

- 使用 **SSE（Server-Sent Events）**

#### 4.4.4 注意事项

- 推理模型可能需要更长 timeout
- 媒体生成类接口通常不走 SSE 文本流，而是异步任务/轮询模式

---

### 4.5 与旧 Chat Completions 的结构差异

官方给出的关键差异包括：

1. **输入字段不同**：`messages` -> `input`
2. **多轮方式不同**：旧接口必须重发历史；Responses API 用 `previous_response_id`
3. **输出结构不同**：`choices[].message.content` -> `output[]`
4. **存储策略不同**：Responses API 支持服务端 30 天存储
5. **新能力优先支持 Responses API**

---

## 5. IMAGE：图像理解（Image Understanding）

> 这一部分不是“图片生成”，而是“把图片作为输入，输出文本分析结果”。

### 5.1 本质定位

图像理解实际上仍属于 **文本输出类能力**：

- 输入：文本 + 图片
- 输出：文本
- 接口：通常走 `/v1/responses`

### 5.2 输入图片的方式

官方文档展示了两种主要方式：

#### 5.2.1 远程 URL

```json
{
  "type": "input_image",
  "image_url": "https://example.com/cat.jpg"
}
```

#### 5.2.2 Base64 Data URL

```json
{
  "type": "input_image",
  "image_url": "data:image/png;base64,..."
}
```

### 5.3 content part 结构

典型写法：

```json
[
  {
    "role": "user",
    "content": [
      {
        "type": "input_text",
        "text": "What is in this image?"
      },
      {
        "type": "input_image",
        "image_url": "https://example.com/example.jpg",
        "detail": "high"
      }
    ]
  }
]
```

### 5.4 `detail`

文档示例里可见：

- `detail: "high"`

表示高细节图像理解模式。

### 5.5 图片输入限制

官方模型页写明：

- **单张图片最大大小**：`20 MiB`
- **图片数量**：文档写“unlimited number of images”
- **支持格式**：
  - `.jpg`
  - `.jpeg`
  - `.png`
- **图片与文字顺序**：可任意混排，顺序不限

### 5.6 典型用途

根据 overview / quickstart / capability 描述，官方强调的用途包括：

- 图像分析
- OCR / 文本提取
- 内容理解与描述
- 结合文本进行多模态问答

### 5.7 与状态化存储的冲突提示

图像理解页面有一个非常值得注意的提醒：

- 当请求中包含图片时，**建议不要存储请求/响应历史**，否则请求可能失败。

因此实务建议：

```json
"store": false
```

尤其是在多图、多轮图像理解场景下更稳妥。

---

## 6. IMAGE：图像生成与编辑（Image Generation / Editing）

### 6.1 能力范围

官方图片能力页给出的能力包括：

- 文本生成图片（text-to-image）
- 编辑已有图片（image editing）
- 多轮对话式 refinement（迭代优化）
- 批量生成多张图片
- 控制宽高比（aspect ratio）
- 控制分辨率（resolution）
- 多图参考编辑

### 6.2 模型

官方图片页示例中常见模型：

- `grok-imagine-image`

并有带版本/别名的模型页面。

### 6.3 生成接口核心字段（Generate Image）

综合 OpenAPI，可整理如下：

| 字段              | 类型    | 说明                      | 默认值 / 限制   |
| ----------------- | ------- | ------------------------- | --------------- |
| `model`           | string  | 图片生成模型              | 必填            |
| `prompt`          | string  | 生成提示词                | 必填            |
| `n`               | integer | 生成图片数量              | 默认 1，最大 10 |
| `aspect_ratio`    | string  | 输出纵横比                | 默认 `auto`     |
| `resolution`      | string  | 输出分辨率                | 默认 `1k`       |
| `response_format` | string  | `url` 或 `b64_json`       | 默认 `url`      |
| `quality`         | string  | `low` / `medium` / `high` | 当前为 no-op    |
| `user`            | string  | 用户标识                  | 可选            |
| `size`            | string  | OpenAI 兼容字段           | 文档标记不支持  |
| `style`           | string  | 风格字段                  | 文档标记不支持  |

### 6.4 `aspect_ratio` 可选值

OpenAPI 中可见的图片纵横比枚举很全：

- `1:1`
- `3:4`
- `4:3`
- `9:16`
- `16:9`
- `2:3`
- `3:2`
- `9:19.5`
- `19.5:9`
- `9:20`
- `20:9`
- `1:2`
- `2:1`
- `auto`

### 6.5 `resolution`

官方 schema 目前可见：

- `1k`
- `2k`

### 6.6 `quality`

虽然 schema 中有：

- `low`
- `medium`
- `high`

但官方明确标注它**当前是 no-op**，即目前并不真正改变输出效果。

### 6.7 `response_format`

支持：

- `url`
- `b64_json`

#### 6.7.1 `url`

- 返回临时可访问 URL
- 官方文档特别提示：这些 URL **是临时的**

#### 6.7.2 `b64_json`

- 直接在响应中返回 base64 图片内容
- 适合服务端持久化保存

---

### 6.8 图像编辑接口核心字段（Edit Image）

综合 OpenAPI，可整理如下：

| 字段              | 类型      | 说明               | 限制 / 备注        |
| ----------------- | --------- | ------------------ | ------------------ |
| `model`           | string    | 编辑模型           | 必填               |
| `prompt`          | string    | 编辑指令           | 必填               |
| `image`           | 单图输入  | 单图编辑           | 与 `images` 互斥   |
| `images`          | 多图输入  | 多图参考编辑       | 与 `image` 互斥    |
| `mask`            | 图像/遮罩 | 局部编辑遮罩       | 可选               |
| `n`               | integer   | 输出张数           | 最大 10            |
| `aspect_ratio`    | string    | 输出比例           | **仅多图编辑有效** |
| `resolution`      | string    | 输出分辨率         | 默认 `1k`          |
| `response_format` | string    | `url` / `b64_json` | 默认 `url`         |
| `quality`         | string    | 质量选项           | 当前 no-op         |
| `user`            | string    | 用户标识           | 可选               |
| `size`            | string    | OpenAI 兼容字段    | 不支持             |
| `style`           | string    | 风格字段           | 不支持             |

### 6.9 单图编辑 vs 多图编辑

#### 单图编辑

- 传 `image`
- **不要设置** `aspect_ratio`
- 输出会继承输入图的纵横比

#### 多图编辑 / 多图参考

- 传 `images`
- 可以指定 `aspect_ratio`
- 提示词里可引用多张图作为参考对象

### 6.10 多图生成/编辑数量上限

文档限制中明确写到：

- **每次请求最多 10 张图片**

### 6.11 SDK 说明：为什么 OpenAI SDK 的 `images.edit()` 不适合

官方特别指出：

- OpenAI SDK 的 `images.edit()` 使用 `multipart/form-data`
- AI 当前图片编辑接口要求 `application/json`

所以官方建议：

- 使用 AI SDK
- 或 Vercel AI SDK
- 或直接发 HTTP 请求

不要照搬 OpenAI 的 multipart 版图片编辑调用。

---

### 6.12 图片生成/编辑响应体

综合 OpenAPI：

```json
{
  "data": [
    {
      "url": "https://...",
      "mime_type": "image/png",
      "revised_prompt": ""
    }
  ],
  "usage": {
    "cost_in_usd_ticks": 12345
  }
}
```

#### 6.12.1 `data[]` 中字段

- `url`
  - 当 `response_format=url` 时返回
  - 为临时 URL
- `b64_json`
  - 当 `response_format=b64_json` 时返回
- `mime_type`
  - 图片 MIME 类型
- `revised_prompt`
  - 文档中标注为 deprecated，且通常为空

#### 6.12.2 `usage`

媒体返回体通常可见：

- `cost_in_usd_ticks`

某些 SDK 包装对象里还会带：

- `model`
- `respect_moderation`

---

### 6.13 图片能力的实务注意点

1. URL 是临时的，拿到后应尽快下载保存。
2. `quality` 现在没有实际效果，别把它当成强控制手段。
3. `size`、`style` 虽然可能出现在兼容 schema 中，但文档标明不支持。
4. 单图编辑不要乱设 `aspect_ratio`。
5. 如果要程序化存储，优先考虑 `b64_json`。

---

## 7. VIDEO：视频生成、参考图生成、编辑、续写

### 7.1 能力范围

官方视频页给出的能力包括：

- 文生视频（Text-to-Video）
- 图生视频（Image-to-Video）
- 参考图视频（Reference-to-Video）
- 视频编辑（Edit Video）
- 视频延长 / 续写（Extend Video）

### 7.2 调用模式

与文本不同，视频生成不是同步一次返回完整内容，而是：

1. 提交任务
2. 返回 `request_id`
3. 轮询 `/v1/videos/{request_id}`
4. 完成后拿到视频 URL

### 7.3 提交生成任务

官方页面展示的流程核心是：

- `POST /v1/videos/generations` 或 SDK 对应封装
- 返回：

```json
{
  "request_id": "video_req_xxx"
}
```

### 7.4 轮询结果

```http
GET /v1/videos/{request_id}
```

### 7.5 视频生成（Generate Video）请求参数

综合 OpenAPI，可整理如下：

| 字段               | 类型             | 说明                              | 默认值 / 限制                         |
| ------------------ | ---------------- | --------------------------------- | ------------------------------------- |
| `model`            | string           | 视频模型                          | 必填                                  |
| `prompt`           | string           | 提示词                            | 文生视频/参考图视频必填；图生视频可选 |
| `duration`         | integer/string   | 目标视频时长（秒）                | 默认 8；范围 1–15                     |
| `seconds`          | integer/string   | OpenAI 兼容字段                   | 与 `duration` 类似                    |
| `aspect_ratio`     | string           | 输出比例                          | 默认通常 16:9                         |
| `resolution`       | string           | 输出分辨率                        | `480p` / `720p`                       |
| `image`            | image URL/base64 | 图生视频输入图                    | 可选                                  |
| `input_reference`  | 兼容引用字段     | 某些兼容场景使用                  | 可选                                  |
| `reference_images` | 图片数组         | 参考图生成视频                    | 最多 7 张                             |
| `output`           | object           | 可指定上传目的地（如 upload_url） | 可选                                  |
| `user`             | string           | 用户标识                          | 可选                                  |
| `size`             | string           | 旧兼容尺寸字段                    | 已 deprecated，建议不用               |

### 7.6 视频 `aspect_ratio` 可选值

OpenAPI 可见：

- `1:1`
- `16:9`
- `9:16`
- `4:3`
- `3:4`
- `3:2`
- `2:3`

### 7.7 视频 `resolution`

当前官方文档可见：

- `480p`
- `720p`

默认值通常为：

- `480p`

### 7.8 视频时长规则

#### 普通生成

- 范围：`1–15` 秒
- 默认：`8` 秒

#### 视频续写（extend）

- 追加片段范围：`2–10` 秒
- 默认：`6` 秒
- 注意：这个 `duration` 指的是**追加段长度**，不是最终总长度。

---

### 7.9 图生视频（Image-to-Video）

#### 输入方式

文档说明可以用：

- URL
- base64 字符串
- 某些 SDK 中也可传字节数组 / Buffer / ArrayBuffer

#### 行为特点

- 如果提供输入图，系统可把它动画化
- 如未显式指定 aspect ratio，通常继承输入图比例

---

### 7.10 参考图生成视频（Reference-to-Video）

这是视频文档里一个非常重要但容易漏掉的模式。

#### 能力定义

- 使用一张或多张参考图来约束生成视频的内容、主体或风格

#### 限制

- `reference_images` 最多 **7 张**
- `prompt` **不能为空**
- 最长视频时长 **10 秒**
- **不能**和 image-to-video / video-editing 混用
- 同一请求只能有一种模式生效

#### 输入图片形式

- HTTPS URL
- base64 data URI

---

### 7.11 视频编辑（Edit Video）

#### 输入要求

- 输入文件必须是 `.mp4`
- 官方文档提到支持常见编解码器（例如 H.265 / H.264 / AV1 等）

#### 时长限制

- 输入视频最大长度：**8.7 秒**

#### 参数继承规则

编辑模式下，文档明确说以下参数**不支持自定义**：

- `duration`
- `aspect_ratio`
- `resolution`

输出会继承：

- 输入视频时长
- 输入视频比例
- 分辨率上限为 `720p`

---

### 7.12 视频续写（Extend Video）

#### 输入要求

- 输入视频必须是 `.mp4`
- 原始视频时长必须在 **2–15 秒**

#### 追加规则

- extension duration：**2–10 秒**
- 默认 6 秒
- `aspect_ratio` / `resolution` 不支持自定义
- 输出比例与分辨率跟随输入视频，分辨率上限 `720p`

---

### 7.13 视频提交响应

创建任务时，返回大致形如：

```json
{
  "request_id": "video_req_..."
}
```

### 7.14 视频轮询响应（VideoResponse）

综合 OpenAPI，可整理为：

| 字段       | 类型    | 说明             |
| ---------- | ------- | ---------------- |
| `status`   | string  | 任务状态         |
| `progress` | integer | 进度             |
| `video`    | object  | 完成后的视频对象 |
| `usage`    | object  | 成本/用量        |
| `model`    | string  | 实际使用模型     |
| `error`    | object  | 失败时的错误对象 |

### 7.15 `status` / 轮询状态语义

官方页面与 schema 中可见的状态包括：

- `pending`
- `done`
- `failed`
- `expired`

另有 HTTP 语义：

- `202`：仍在处理中
- `200`：任务已完成或已失败（需要进一步看 `status`）

### 7.16 `progress`

官方 OpenAPI 对进度的说明很明确：

- 处理中：`0–99`
- 完成：`100`
- 失败：`0`

### 7.17 完成后的视频对象

典型结构：

```json
{
  "status": "done",
  "progress": 100,
  "video": {
    "url": "https://...",
    "duration": 8,
    "respect_moderation": true
  },
  "model": "grok-imagine-video"
}
```

#### `video` 中常见字段

- `url`
  - 临时 URL
- `duration`
  - 视频长度（秒）
- `respect_moderation`
  - 是否遵守内容审核限制

### 7.18 失败时的错误对象

OpenAPI 中给出结构化错误：

```json
{
  "error": {
    "code": "invalid_argument",
    "message": "..."
  }
}
```

#### `error.code` 可见枚举

- `invalid_argument`
- `permission_denied`
- `failed_precondition`
- `out_of_range`
- `internal_error`

---

### 7.19 轮询与 SDK 超时

文档中可见 SDK 默认行为：

- 默认轮询 timeout：**10 分钟**
- 默认轮询间隔：**100ms**

也可在 SDK 中自定义：

- `pollTimeoutMs`
- `pollIntervalMs`

### 7.20 视频能力限制总结

1. 普通生成最大时长：**15 秒**
2. 编辑输入视频最大：**8.7 秒**
3. 续写输入视频范围：**2–15 秒**
4. 续写追加片段：**2–10 秒**
5. 参考图数量上限：**7 张**
6. 视频 URL 为**临时链接**
7. 输出分辨率当前以 `480p` / `720p` 为主
8. 内容审核适用于视频生成

### 7.21 不允许的组合

文档明确提到：

- `image` 与 `reference_images` 不能同时用
- 多种 mode 不能混用
- 若省略 mode，则按标准视频生成处理

---

## 8. 返回体字段总表

### 8.1 文本（Responses API）

| 字段           | 说明                                       |
| -------------- | ------------------------------------------ |
| `id`           | 响应 ID                                    |
| `object`       | 对象类型，通常为 `response`                |
| `created_at`   | 创建时间                                   |
| `completed_at` | 完成时间                                   |
| `status`       | `completed` / `in_progress` / `incomplete` |
| `model`        | 模型名                                     |
| `output`       | 带类型的输出数组                           |
| `usage`        | token、reasoning、缓存、来源、成本等统计   |
| `error`        | 错误对象                                   |
| `store`        | 是否存储                                   |
| `reasoning`    | 推理相关内容/摘要/密文                     |
| `text`         | 文本输出配置回显                           |
| `tool_choice`  | 工具策略回显                               |

### 8.2 图片生成/编辑

| 字段                    | 说明                 |
| ----------------------- | -------------------- |
| `data[]`                | 生成结果数组         |
| `data[].url`            | 临时图片 URL         |
| `data[].b64_json`       | base64 图片内容      |
| `data[].mime_type`      | 图片 MIME 类型       |
| `data[].revised_prompt` | deprecated，通常为空 |
| `usage`                 | 成本/用量统计        |

### 8.3 视频轮询返回

| 字段                       | 说明                                      |
| -------------------------- | ----------------------------------------- |
| `status`                   | `pending` / `done` / `failed` / `expired` |
| `progress`                 | 进度 0–100                                |
| `video.url`                | 临时视频 URL                              |
| `video.duration`           | 视频时长                                  |
| `video.respect_moderation` | 审核标记                                  |
| `usage`                    | 成本/用量统计                             |
| `model`                    | 实际模型                                  |
| `error.code`               | 机器可读错误码                            |
| `error.message`            | 人类可读错误说明                          |

---

## 9. 限制、兼容性与已知坑

### 9.1 文本/Responses API

1. `store` 默认是开启的，不想保留服务端状态要显式设 `false`。
2. `previous_response_id` 适合多轮，但不意味着历史不计费。
3. reasoning model 不支持一部分经典参数，例如 `presence_penalty`、`frequency_penalty`、`stop`。
4. `max_output_tokens` 可能包含 reasoning tokens。
5. `metadata`、`service_tier`、`truncation` 这些字段存在兼容层痕迹，使用前要做兜底。

### 9.2 图像理解

1. 图片最大 20 MiB。
2. 支持 jpg/jpeg/png。
3. 文档说图片数量可无限，但实务上仍建议控制请求体大小。
4. 当请求中包含图片时，建议 `store: false`，否则可能失败。

### 9.3 图片生成/编辑

1. 返回 URL 是临时的，务必立即落盘。
2. 单次请求最多 10 张图。
3. `quality` 目前 no-op。
4. `size` / `style` 字段当前不支持。
5. OpenAI SDK 的 `images.edit()` 不能直接照搬，因为请求格式不同。

### 9.4 视频生成/编辑

1. 视频生成采用异步任务 + 轮询，不是一次同步返回成品。
2. 视频 URL 是临时的。
3. 编辑模式下不能自由改 duration / aspect_ratio / resolution。
4. 参考图模式最多 7 张，且不能和 image-to-video / edit 混用。
5. 续写时的 `duration` 是追加段长度，不是最终总时长。

---

## 10. 官方文档不一致之处

这一节非常重要。

### 10.1 `instructions` 是否支持

存在冲突：

- OpenAPI schema 中有 `instructions`
- 但另一个文档页又写“当前不支持”

**建议**：

- 生产环境中，把高层系统指令优先放进首条 system/developer message
- 把 `instructions` 视为“可能逐步支持、但存在文档不一致”的字段

### 10.2 `metadata` / `service_tier` / `truncation`

这些字段在 schema 或页面里都能看到，但又带有：

- compatibility only
- not supported
- deprecated

之类表述。

**建议**：

- 作为“兼容保留字段”对待
- 代码中允许字段存在，但不要把核心逻辑建立在这些字段上

### 10.3 `usage` 命名风格不完全统一

有的页面/示例使用：

- `prompt_tokens` / `completion_tokens`

有的示例使用：

- `input_tokens` / `output_tokens`

**建议**：

- 客户端做双路径兼容解析
- reasoning / cached / sources / tools 字段单独兜底读取

### 10.4 图片/视频 SDK 封装与 REST 原始字段不完全一致

例如：

- SDK 里可能暴露 `respect_moderation`
- REST 示例里更偏向 `data` / `video` / `usage`

**建议**：

- 以 REST API 返回为最低对齐标准
- SDK 额外字段视为便利包装

---

## 11. 官方来源链接

以下是本次整理涉及的 AI 官方文档入口（均为官方域名）：

### 11.1 总览与基础

- Overview: <https://docs.x.ai/overview>
- Models and Pricing: <https://docs.x.ai/docs/models-and-pricing>
- REST API Overview: <https://docs.x.ai/api/rest-api>
- OpenAPI Schema: <https://docs.x.ai/openapi.json>

### 11.2 Text

- Generate Text: <https://docs.x.ai/developers/model-capabilities/text/generate-text>
- Reasoning: <https://docs.x.ai/developers/model-capabilities/text/reasoning>
- Structured Outputs: <https://docs.x.ai/developers/model-capabilities/text/structured-outputs>
- Streaming: <https://docs.x.ai/developers/model-capabilities/text/streaming>
- Comparison / Migrating to Responses API: <https://docs.x.ai/developers/model-capabilities/text/comparison>

### 11.3 Images

- Image Understanding: <https://docs.x.ai/developers/model-capabilities/images/understanding>
- Image Generation: <https://docs.x.ai/developers/model-capabilities/images/generation>

### 11.4 Video

- Video Generation: <https://docs.x.ai/developers/model-capabilities/video/generation>

---

## 附：推荐的工程实践

### A. 文本生成的默认模板

```json
{
  "model": "grok-4.20",
  "input": [
    {
      "role": "system",
      "content": "You are a precise assistant."
    },
    {
      "role": "user",
      "content": "Explain this topic clearly."
    }
  ],
  "store": false
}
```

### B. 图像理解的默认模板

```json
{
  "model": "grok-4.20",
  "store": false,
  "input": [
    {
      "role": "user",
      "content": [
        {
          "type": "input_text",
          "text": "请描述图中内容并提取可见文字。"
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

### C. 图片生成的默认模板

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

### D. 视频生成的默认模板

```json
{
  "model": "grok-imagine-video",
  "prompt": "A slow aerial shot of a neon city at night in the rain",
  "duration": 8,
  "aspect_ratio": "16:9",
  "resolution": "480p"
}
```

---

## 最终结论

如果你要把 AI 的三类能力快速落地到工程里，建议采用以下策略：

1. **Text 全部优先走 Responses API**，不要再以 Chat Completions 作为主接口。
2. **图像理解** 仍然当作文本输出接口来处理，并且在带图时优先 `store: false`。
3. **图片生成/编辑** 单独走媒体接口，不要直接套 OpenAI 的 multipart 图片编辑调用。
4. **视频生成** 按“异步任务 + 轮询”设计，不要假设同步返回成品。
5. 对 `instructions`、`metadata`、`service_tier`、`truncation`、`usage` 等存在文档不一致的字段，客户端必须做兼容兜底。
