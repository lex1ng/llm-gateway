# 多平台 AI API 统一接入层 — 技术分析报告

## 1. 调研范围

| 平台 | 简称 | Base URL | 认证方式 |
|------|------|----------|----------|
| OpenAI | openai | `https://api.openai.com/v1` | `Authorization: Bearer <key>` |
| Anthropic (Claude) | anthropic | `https://api.anthropic.com/v1` | `x-api-key: <key>` + `anthropic-version: 2023-06-01` |
| Google (Gemini) | google | `https://generativelanguage.googleapis.com/v1beta` | `x-goog-api-key: <key>` |
| 阿里云百炼 (DashScope) | alibaba | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `Authorization: Bearer <key>` |
| 百度千帆 (ModelBuilder) | baidu | `https://qianfan.baidubce.com/v2` (V2兼容OpenAI) | `Authorization: Bearer <key>` (V2) / access_token (V1) |
| 火山引擎方舟 (Doubao) | volcengine | `https://ark.cn-beijing.volces.com/api/v3` | `Authorization: Bearer <key>` |
| 智谱AI (GLM) | zhipu | `https://open.bigmodel.cn/api/paas/v4` | `Authorization: Bearer <key>` |
| MiniMax | minimax | `https://api.minimax.io/v1` | `Authorization: Bearer <key>` |

---

## 2. 接口能力矩阵

| 能力 | OpenAI | Anthropic | Google | 阿里百炼 | 百度千帆 | 火山方舟 | 智谱AI | MiniMax |
|------|--------|-----------|--------|----------|----------|----------|--------|---------|
| **对话 Chat** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **流式 Stream** | ✅ SSE | ✅ SSE | ✅ SSE | ✅ SSE | ✅ SSE | ✅ SSE | ✅ SSE | ✅ SSE |
| **Tool/Function Calling** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **视觉理解 (图片输入)** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **图像生成** | ✅ DALL-E/GPT | ❌ | ✅ Imagen | ✅ Wan/通义 | ✅ iRAG | ✅ | ✅ CogView | ✅ image-01 |
| **视频生成** | ✅ Sora | ❌ | ✅ Veo | ✅ Wan2.1 | ✅ | ✅ | ✅ CogVideoX | ✅ Hailuo |
| **Embeddings** | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **语音合成 TTS** | ✅ | ❌ | ✅ | ✅ Qwen-TTS | ✅ | ❌(需确认) | ❌(需确认) | ✅ speech-2.8 |
| **语音识别 STT** | ✅ Whisper | ❌ | ✅ | ✅ | ✅ | ❌(需确认) | ❌(需确认) | ❌(需确认) |
| **Agent 调用** | ✅ Assistants | ✅ Skills(Beta) | ❌ | ✅ 应用API | ✅ 插件应用 | ✅ Bot API | ✅ 智能体 | ❌ |
| **Workflow 调用** | ❌ | ❌ | ❌ | ✅ 工作流API | ❌ | ❌ | ❌ | ❌ |
| **Batch 批量推理** | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ |
| **文件管理** | ✅ | ✅ (Beta) | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| **音乐生成** | ❌ | ❌ | ✅ Lyria | ❌ | ❌ | ❌ | ❌ | ✅ music-2.0 |

---

## 3. 核心接口共性分析（以 OpenAI/Anthropic 为基准）

### 3.1 对话接口 — 最高共性，统一优先级最高

**OpenAI 标准已成为事实标准**。百度(V2)、阿里百炼、火山方舟、智谱、MiniMax 均提供 OpenAI 兼容接口。

#### OpenAI Chat Completions 格式

```
POST /chat/completions
```

```json
{
  "model": "gpt-4o",
  "messages": [
    {"role": "system", "content": "..."},
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "temperature": 0.7,
  "top_p": 1.0,
  "max_tokens": 4096,
  "stream": false,
  "tools": [...],
  "tool_choice": "auto",
  "stop": ["..."],
  "n": 1,
  "presence_penalty": 0,
  "frequency_penalty": 0
}
```

#### Anthropic Messages 格式（关键差异点）

```
POST /v1/messages
```

```json
{
  "model": "claude-opus-4-6",
  "system": "...",                    // ⚠️ system 是顶层字段，不在 messages 中
  "messages": [
    {"role": "user", "content": "..."},
    {"role": "assistant", "content": "..."}
  ],
  "max_tokens": 4096,                // ⚠️ 必填字段（OpenAI 可选）
  "temperature": 1.0,
  "top_p": 1.0,
  "top_k": 40,                       // ⚠️ OpenAI 无此参数
  "stream": false,
  "tools": [...],
  "tool_choice": {"type": "auto"},   // ⚠️ 是 object，OpenAI 是 string
  "stop_sequences": ["..."],         // ⚠️ 字段名不同（OpenAI: stop）
  "thinking": {"type": "enabled", "budget_tokens": 1024}  // ⚠️ 独有
}
```

#### Google Gemini 格式（较大差异）

```
POST /v1beta/models/{model}:generateContent
```

```json
{
  "contents": [
    {
      "role": "user",
      "parts": [{"text": "..."}]
    }
  ],
  "systemInstruction": {"parts": [{"text": "..."}]},
  "generationConfig": {
    "temperature": 0.7,
    "topP": 1.0,
    "topK": 40,
    "maxOutputTokens": 4096
  },
  "tools": [...]
}
```

#### 对话接口差异汇总

| 差异点 | OpenAI | Anthropic | Google | 国内平台(兼容模式) |
|--------|--------|-----------|--------|-------------------|
| 端点路径 | `/chat/completions` | `/v1/messages` | `/models/{m}:generateContent` | `/chat/completions` |
| system prompt 位置 | messages 数组中 role=system | 顶层 `system` 字段 | 顶层 `systemInstruction` | messages 数组中 |
| max_tokens | 可选 | **必填** | `maxOutputTokens`(在 generationConfig 中) | 可选 |
| stop | `stop` (string/array) | `stop_sequences` (array) | `stopSequences` (在 generationConfig 中) | `stop` |
| 消息内容结构 | `content: string \| array` | `content: string \| array` | `parts: [{text: "..."}]` | `content: string \| array` |
| tool_choice 类型 | string (`"auto"`) | object (`{"type":"auto"}`) | 不同格式 | string |
| 响应 ID 前缀 | `chatcmpl-` | `msg_` | 无固定前缀 | 各平台不同 |
| stop_reason 字段名 | `finish_reason` | `stop_reason` | `finishReason` | `finish_reason` |
| usage 字段 | `prompt_tokens` / `completion_tokens` | `input_tokens` / `output_tokens` | 不同命名 | `prompt_tokens` / `completion_tokens` |
| n (多候选) | ✅ 支持 | ❌ 不支持 | `candidateCount` | 部分支持 |

### 3.2 流式响应 — 高共性但格式有差异

所有平台都使用 **Server-Sent Events (SSE)** 协议，但事件格式不同：

**OpenAI 风格** (国内平台基本相同):
```
data: {"id":"...","choices":[{"delta":{"content":"Hello"},"index":0}]}
data: [DONE]
```

**Anthropic 风格**:
```
event: message_start
data: {"type":"message_start","message":{...}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}

event: message_stop
data: {"type":"message_stop"}
```

**Google 风格**:
```
data: {"candidates":[{"content":{"parts":[{"text":"Hello"}]}}]}
```

### 3.3 Tool / Function Calling — 高共性

**工具定义格式**基本一致（JSON Schema）：

| 平台 | 工具定义 | 调用格式 | 结果回传 |
|------|---------|---------|---------|
| OpenAI | `tools: [{type:"function", function:{name, description, parameters}}]` | response `tool_calls` 数组 | `role: "tool"`, `tool_call_id` |
| Anthropic | `tools: [{name, description, input_schema}]` | response `tool_use` content block | `tool_result` content block in user message |
| Google | `tools: [{functionDeclarations:[{name, description, parameters}]}]` | response `functionCall` part | `functionResponse` part |
| 国内(兼容) | 同 OpenAI | 同 OpenAI | 同 OpenAI |

### 3.4 Embeddings — 中等共性

**OpenAI 标准**:
```
POST /embeddings
{
  "model": "text-embedding-3-small",
  "input": "text to embed",
  "dimensions": 1536,
  "encoding_format": "float"
}
```

**响应**:
```json
{
  "data": [{"object": "embedding", "embedding": [0.1, 0.2, ...], "index": 0}],
  "model": "...",
  "usage": {"prompt_tokens": 5, "total_tokens": 5}
}
```

| 平台 | 兼容 OpenAI 格式 | 独有参数 | 备注 |
|------|-----------------|---------|------|
| OpenAI | 标准 | `dimensions`, `encoding_format` | — |
| Anthropic | ❌ 不提供 | — | 需跳过或用其他平台 |
| Google | ❌ 自有格式 `embedContent` | `taskType` | 需适配 |
| 阿里百炼 | ✅ | — | 通过兼容模式 |
| 百度千帆 | ✅ (V2) | — | V1 有旧格式 |
| 火山方舟 | ✅ | — | Doubao-embedding |
| 智谱AI | ✅ | — | embedding-2/3 |
| MiniMax | ✅ | — | 通过兼容接口 |

### 3.5 图像生成 — 中等共性，异步模式为主

**OpenAI 标准**:
```
POST /images/generations
{
  "model": "gpt-image-1",
  "prompt": "a cat",
  "n": 1,
  "size": "1024x1024",
  "response_format": "url" | "b64_json"
}
```

**差异分析**:

| 平台 | 接口风格 | 同步/异步 | 尺寸参数 | 额外能力 |
|------|---------|----------|---------|---------|
| OpenAI | `/images/generations` | 同步 | `size` (固定选项) | 编辑、变体 |
| Anthropic | ❌ | — | — | — |
| Google | `models.predict` | 异步 | 不同参数 | — |
| 阿里百炼 | 兼容 + DashScope格式 | 异步任务 | `size` / 宽高比 | Wan系列 |
| 百度千帆 | 自有格式 | 异步任务 | 自有参数 | iRAG |
| 火山方舟 | 自有格式 | 异步/流式 | 自有参数 | — |
| 智谱AI | 兼容OpenAI | 异步任务 | — | CogView-3 |
| MiniMax | 自有格式 | 异步 `task_id` | 宽高比 | image-01 |

**关键差异**: 国内平台图像生成多为**异步任务模式**（提交 -> 轮询 task_id -> 获取结果），而 OpenAI 是同步返回。需要统一封装异步轮询机制。

### 3.6 视频生成 — 低共性，全部异步

所有平台的视频生成都是**异步任务模式**：

```
1. POST 创建任务 -> 返回 task_id
2. GET 查询任务状态 -> 返回 status + result_url
3. （可选）GET 下载/获取结果
```

| 平台 | 模型 | 输入模式 | 最大时长 |
|------|------|---------|---------|
| OpenAI | Sora | 文生视频 | — |
| Google | Veo | 文/图生视频 | — |
| 阿里百炼 | Wan2.1 | 文/图生视频 | — |
| 火山方舟 | 自有 | 文/图生视频 | — |
| 智谱AI | CogVideoX | 文/图生视频 | — |
| MiniMax | Hailuo 2.3 | 文/图生视频 | — |
| Anthropic | ❌ | — | — |

### 3.7 语音合成 TTS — 中等共性

**OpenAI 标准**:
```
POST /audio/speech
{
  "model": "tts-1",
  "input": "Hello world",
  "voice": "alloy",
  "response_format": "mp3",
  "speed": 1.0
}
```
返回：二进制音频流

| 平台 | 兼容 OpenAI | 独有特性 | 备注 |
|------|------------|---------|------|
| OpenAI | 标准 | — | 6种预设音色 |
| Anthropic | ❌ | — | — |
| 阿里百炼 | 部分兼容 | 实时流式、语音复刻、多语言方言 | Qwen-TTS |
| MiniMax | 自有格式 | 300+音色、WebSocket实时、语音克隆、语音设计 | 最丰富 |
| 百度千帆 | 自有格式 | — | — |

### 3.8 语音识别 STT — 中等共性

**OpenAI 标准**:
```
POST /audio/transcriptions
Content-Type: multipart/form-data
- file: (binary)
- model: "whisper-1"
- language: "en"
- response_format: "json"
```

各平台格式差异较大，需逐一适配。

### 3.9 Agent / 应用调用 — 低共性，差异极大

| 平台 | 接口 | 调用方式 | 备注 |
|------|------|---------|------|
| OpenAI | Assistants API | Threads + Runs | 有状态管理 |
| Anthropic | Skills API (Beta) | 独立接口 | 尚不稳定 |
| 阿里百炼 | 应用调用 API | `app_id` + messages | 智能体/工作流/编排 |
| 百度千帆 | 插件应用 API | 自有格式 | 联网/知识库插件 |
| 火山方舟 | Bot API | 自有格式 | 集成插件 |
| 智谱AI | 智能体 API | 自有格式 | — |

**结论**: Agent/Workflow 接口各平台差异极大，无法统一抽象，建议作为平台专属扩展实现。

---

## 4. 认证方式统一分析

| 分类 | 平台 | 方式 | 需要适配 |
|------|------|------|---------|
| **Bearer Token** (主流) | OpenAI, 阿里, 百度V2, 火山, 智谱, MiniMax | `Authorization: Bearer <key>` | ✅ 统一 |
| **自定义 Header** | Anthropic | `x-api-key` + `anthropic-version` | 需特殊处理 |
| **Query Param** | Google | `x-goog-api-key` 或 URL 参数 | 需特殊处理 |
| **双步骤** | 百度V1 | API Key + Secret -> access_token | 需特殊处理(已可用V2) |

---

## 5. 统一接入层架构设计建议

### 5.1 分层架构

```
┌─────────────────────────────────────────────┐
│              Unified Client API              │  <- 用户面对的统一接口
│  Chat() / Embed() / ImageGen() / TTS() ...  │
├─────────────────────────────────────────────┤
│           Request/Response Mapper            │  <- 请求/响应格式转换
│   OpenAI ↔ Anthropic ↔ Google ↔ ...        │
├─────────────────────────────────────────────┤
│            Provider Adapters                 │  <- 各平台适配器
│  ┌────────┬──────────┬────────┬──────────┐  │
│  │ OpenAI │Anthropic │ Google │ China(*) │  │
│  └────────┴──────────┴────────┴──────────┘  │
├─────────────────────────────────────────────┤
│         Transport Layer (HTTP/SSE/WS)        │  <- 传输层
│    Auth / Retry / Rate Limit / Logging       │
└─────────────────────────────────────────────┘
```

### 5.2 核心接口定义建议

```go
// 统一的 Provider 接口
type Provider interface {
    Name() string
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (ChatStream, error)
}

// 可选能力接口 — 各平台按需实现
type EmbeddingProvider interface {
    Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
}

type ImageGenProvider interface {
    GenerateImage(ctx context.Context, req *ImageGenRequest) (*ImageGenResponse, error)
}

type VideoGenProvider interface {
    CreateVideoTask(ctx context.Context, req *VideoGenRequest) (*AsyncTask, error)
    GetTaskStatus(ctx context.Context, taskID string) (*AsyncTask, error)
}

type TTSProvider interface {
    Synthesize(ctx context.Context, req *TTSRequest) (io.ReadCloser, error)
}

type STTProvider interface {
    Transcribe(ctx context.Context, req *STTRequest) (*STTResponse, error)
}

type AgentProvider interface {
    CallAgent(ctx context.Context, req *AgentRequest) (*AgentResponse, error)
}
```

### 5.3 统一数据模型（以 OpenAI 为基准）

```go
// ChatRequest — 统一请求
type ChatRequest struct {
    Model       string          `json:"model"`
    Messages    []Message       `json:"messages"`
    System      string          `json:"system,omitempty"`      // Anthropic 需要
    MaxTokens   *int            `json:"max_tokens,omitempty"`
    Temperature *float64        `json:"temperature,omitempty"`
    TopP        *float64        `json:"top_p,omitempty"`
    TopK        *int            `json:"top_k,omitempty"`       // Anthropic/Google
    Stream      bool            `json:"stream,omitempty"`
    Stop        []string        `json:"stop,omitempty"`
    Tools       []Tool          `json:"tools,omitempty"`
    ToolChoice  interface{}     `json:"tool_choice,omitempty"` // string 或 object
    N           *int            `json:"n,omitempty"`           // OpenAI only
    Extra       map[string]any  `json:"extra,omitempty"`       // 平台专属字段
}

// Message — 统一消息
type Message struct {
    Role    string    `json:"role"`    // system/user/assistant/tool
    Content Content   `json:"content"` // 可以是 string 或 []ContentBlock
}

// ContentBlock — 多模态内容块
type ContentBlock struct {
    Type     string `json:"type"`                // text/image/audio/document/tool_use/tool_result
    Text     string `json:"text,omitempty"`
    ImageURL string `json:"image_url,omitempty"` // URL 或 base64
    // ... 更多字段
}

// ChatResponse — 统一响应
type ChatResponse struct {
    ID           string         `json:"id"`
    Model        string         `json:"model"`
    Content      []ContentBlock `json:"content"`
    FinishReason string         `json:"finish_reason"`
    Usage        Usage          `json:"usage"`
    ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
}
```

---

## 6. 各接口实现难度评估

| 接口 | 难度 | 原因 |
|------|------|------|
| **Chat (对话)** | ⭐⭐ 简单 | 大部分平台兼容 OpenAI 格式；Anthropic 和 Google 需要适配器但差异可控 |
| **Stream (流式)** | ⭐⭐⭐ 中等 | SSE 解析逻辑各平台不同，Anthropic 事件粒度更细，需统一抽象 |
| **Tool Calling** | ⭐⭐⭐ 中等 | 工具定义基本一致(JSON Schema)，但调用/回传格式有差异 |
| **Embeddings** | ⭐⭐ 简单 | 大部分兼容 OpenAI；Google 需适配；Anthropic 不支持 |
| **图像生成** | ⭐⭐⭐⭐ 较难 | 同步/异步模式混合；参数差异大（尺寸、格式）；需统一异步轮询 |
| **视频生成** | ⭐⭐⭐ 中等 | 全部异步任务模式，抽象为 create + poll 即可；但各平台参数差异大 |
| **TTS 语音合成** | ⭐⭐⭐ 中等 | 返回格式（二进制流）一致，但音色/语言/参数差异大 |
| **STT 语音识别** | ⭐⭐⭐ 中等 | multipart 上传格式差异；返回格式差异 |
| **Agent 调用** | ⭐⭐⭐⭐⭐ 困难 | 各平台模型完全不同，无法有效统一，建议平台专属实现 |
| **Workflow 调用** | ⭐⭐⭐⭐⭐ 困难 | 仅阿里百炼等少数平台支持，无标准可言 |

---

## 7. 需要定义的核心组件

### 7.1 配置 (Config)

```go
type ProviderConfig struct {
    Name      string            // 平台名称
    BaseURL   string            // API 基地址
    APIKey    string            // API Key
    SecretKey string            // 部分平台需要（百度V1）
    Headers   map[string]string // 额外 headers（Anthropic version 等）
    Region    string            // 地域（阿里/火山等多区域）
    Timeout   time.Duration
    Retry     RetryConfig
}
```

### 7.2 适配器注册

```go
// 全局注册表
var registry = map[string]ProviderFactory{}

func Register(name string, factory ProviderFactory) { ... }
func Get(name string, cfg ProviderConfig) (Provider, error) { ... }
```

### 7.3 流式响应统一抽象

```go
type ChatStream interface {
    Recv() (*StreamEvent, error)  // 统一事件
    Close() error
}

type StreamEvent struct {
    Type    string       // "content_delta" / "tool_call_delta" / "done" / "error"
    Delta   string       // 文本增量
    Usage   *Usage       // 最终 usage（在 done 事件中）
    // ...
}
```

### 7.4 异步任务统一抽象（图像/视频生成）

```go
type AsyncTask struct {
    TaskID    string
    Status    string  // "pending" / "running" / "succeeded" / "failed"
    ResultURL string
    Error     string
    Extra     map[string]any
}

type AsyncTaskProvider interface {
    Submit(ctx context.Context, req any) (*AsyncTask, error)
    Poll(ctx context.Context, taskID string) (*AsyncTask, error)
    Cancel(ctx context.Context, taskID string) error
}
```

---

## 8. 关键注意事项

### 8.1 Anthropic 的特殊性

- **system prompt 位于顶层**而非 messages 数组中 — 需在适配器中抽取
- **max_tokens 必填** — 统一层需提供默认值
- **不支持 Embeddings/TTS/STT/图像生成/视频生成** — 能力检测机制必不可少
- **流式事件格式独特** — 需独立解析器
- **tool_choice 是 object 不是 string** — 需类型转换
- **stop_sequences vs stop** — 字段名映射
- **thinking (扩展思考)** — 独有功能，放入 Extra

### 8.2 Google Gemini 的特殊性

- **完全不同的请求结构**: `contents` + `parts` 而非 `messages` + `content`
- **URL 中包含模型名**: `/models/{model}:generateContent`
- **参数嵌套在 generationConfig 中**
- **流式端点不同**: `:streamGenerateContent` vs 同端点加 `stream=true`
- **Embeddings 端点**: `embedContent` 而非 `/embeddings`

### 8.3 国内平台共性

- 大部分提供 **OpenAI 兼容模式**，是最大的利好
- 百度V1旧格式使用 access_token 认证，V2 已兼容 Bearer Token
- 图像/视频生成多为**异步任务**模式
- 部分平台有**区域端点**差异（阿里北京/新加坡/弗吉尼亚）
- Agent/Workflow 接口差异极大，各有各的抽象

### 8.4 错误处理

各平台错误响应格式不同：

| 平台 | 错误格式 |
|------|---------|
| OpenAI | `{"error": {"message": "...", "type": "...", "code": "..."}}` |
| Anthropic | `{"type": "error", "error": {"type": "...", "message": "..."}}` |
| Google | `{"error": {"code": 400, "message": "...", "status": "..."}}` |
| 国内平台 | 各有不同，但兼容模式下多同 OpenAI |

需统一为:
```go
type APIError struct {
    Provider   string
    StatusCode int
    Code       string
    Message    string
    Raw        json.RawMessage
}
```

### 8.5 Rate Limiting

各平台都有频率限制，响应头字段不同：
- OpenAI: `x-ratelimit-limit-requests`, `x-ratelimit-remaining-requests`
- Anthropic: `retry-after` + HTTP 429
- 国内平台: 各有不同

建议在 Transport 层统一处理重试和退避。

---

## 9. 推荐实施路径

### Phase 1 — 核心对话能力（最高优先级）

1. 定义统一 `ChatRequest` / `ChatResponse` / `ChatStream` 模型
2. 实现 OpenAI 适配器（基准，最简单）
3. 实现 Anthropic 适配器（处理 system prompt、max_tokens、流式差异）
4. 实现 Google 适配器（处理 contents/parts 结构转换）
5. 国内平台 — 基于 OpenAI 兼容模式，大部分只需配置 BaseURL 即可

### Phase 2 — Embeddings + 图像生成

6. Embeddings: OpenAI格式为标准，Google 做适配，Anthropic 标记不支持
7. 图像生成: 定义统一异步任务接口，各平台适配

### Phase 3 — 语音 + 视频

8. TTS: 统一为 `input + voice + format -> audio stream`
9. STT: 统一为 `audio file -> text`
10. 视频生成: 统一异步任务模式

### Phase 4 — Agent / Workflow（平台专属）

11. 不建议过度抽象，提供平台专属入口 + 基础 interface 即可

---

## 10. 总结

| 结论 | 说明 |
|------|------|
| **对话接口统一性最高** | 以 OpenAI 格式为基准，6/8 平台原生兼容，Anthropic 和 Google 需适配器 |
| **OpenAI 兼容是最大利好** | 国内平台几乎全部支持，只需替换 BaseURL + APIKey |
| **Anthropic 差异最需关注** | system prompt 位置、max_tokens 必填、流式格式、无多媒体生成能力 |
| **Google 差异最大** | 完全不同的请求结构，需要完整的格式转换层 |
| **多媒体生成以异步为主** | 图像/视频生成统一为 submit + poll 模式 |
| **Agent/Workflow 不宜统一** | 差异过大，建议平台专属扩展 |
| **能力检测机制必不可少** | 不是所有平台都支持所有接口，需要 `Supports(capability)` 方法 |
