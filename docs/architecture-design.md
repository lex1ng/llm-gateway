# LLM Gateway — Go 多平台大模型统一接入层架构设计

## 1. 项目概述

### 1.1 目标

构建一个 Go 语言实现的多平台大模型统一接入层，为上层业务提供**一致的 API 调用体验**，屏蔽各平台接口差异。

### 1.2 设计原则

| 原则 | 说明 |
|------|------|
| **OpenAI 为内部标准** | 统一数据模型以 OpenAI 格式为基准，其他平台做适配转换 |
| **能力接口分离** | 不用一个大 interface 包揽一切，按能力拆分，provider 按需实现 |
| **国内平台走兼容层** | 阿里/百度/火山/智谱/MiniMax 均支持 OpenAI 兼容，复用同一适配器 |
| **借鉴 Shannon 优点** | 采用 ModelTier 分层路由、熔断器、对冲请求、缓存安全防护 |
| **改进 Shannon 不足** | 统一流式接口、独立消息转换层、从一开始覆盖多媒体能力 |

### 1.3 支持平台

| 优先级 | 平台 | 适配方式 |
|--------|------|---------|
| P0 | OpenAI | 原生适配 |
| P0 | Anthropic (Claude) | 原生适配 |
| P0 | 阿里百炼 (DashScope) | OpenAI 兼容层 |
| P1 | 百度千帆 V2 | OpenAI 兼容层 |
| P1 | 火山方舟 (Doubao) | OpenAI 兼容层 |
| P1 | 智谱AI (GLM) | OpenAI 兼容层 |
| P1 | MiniMax | OpenAI 兼容层 |
| P2 | Google (Gemini) | 原生适配 |

---

## 2. 整体架构

```
                          ┌──────────────────────────┐
                          │      业务调用方            │
                          │  (HTTP API / gRPC / SDK)  │
                          └────────────┬─────────────┘
                                       │
                          ┌────────────▼─────────────┐
                          │        Manager            │
                          │  路由 · 熔断 · 缓存 · 限流  │
                          └────────────┬─────────────┘
                                       │
                    ┌──────────────────┼──────────────────┐
                    │                  │                   │
           ┌────────▼──────┐  ┌───────▼───────┐  ┌───────▼───────┐
           │  MessageMapper │  │  StreamParser  │  │  AsyncTaskMgr │
           │  消息格式转换    │  │  流式事件解析   │  │  异步任务编排   │
           └────────┬──────┘  └───────┬───────┘  └───────┬───────┘
                    │                 │                   │
           ┌────────▼─────────────────▼───────────────────▼───────┐
           │                   Provider Registry                   │
           │            注册 · 发现 · 能力查询                       │
           └──┬──────┬──────┬──────┬──────┬──────┬──────┬────────┘
              │      │      │      │      │      │      │
           ┌──▼─┐┌──▼──┐┌──▼──┐┌──▼──┐┌──▼──┐┌──▼──┐┌──▼──┐
           │O.AI││Claud││Goog.││Compat││Compat││Compat││Compat│
           │    ││  e  ││     ││ 阿里  ││ 百度 ││ 火山 ││智谱/MM│
           └────┘└─────┘└─────┘└──────┘└─────┘└─────┘└──────┘
              │      │      │      │       │      │       │
           ┌──▼──────▼──────▼──────▼───────▼──────▼───────▼──┐
           │              Transport Layer                      │
           │      HTTP Client · Retry · Auth · Logging         │
           └──────────────────────────────────────────────────┘
```

---

## 3. 目录结构

```
llm-service/
├── cmd/
│   └── server/
│       └── main.go                    # 服务入口
├── config/
│   ├── config.go                      # 配置结构定义
│   ├── config.yaml                    # 默认配置模板
│   └── models.yaml                    # 模型目录与定价
├── pkg/
│   ├── types/                         # ========== 核心类型 ==========
│   │   ├── message.go                 # Message / ContentBlock / Role
│   │   ├── request.go                 # ChatRequest / EmbedRequest / ImageGenRequest ...
│   │   ├── response.go               # ChatResponse / EmbedResponse / StreamEvent ...
│   │   ├── tool.go                    # Tool / ToolCall / ToolResult 定义
│   │   ├── usage.go                   # TokenUsage / Cost
│   │   ├── model.go                   # ModelConfig / ModelTier / ModelCapability
│   │   ├── error.go                   # 统一错误类型
│   │   └── async_task.go             # AsyncTask (图像/视频生成用)
│   │
│   ├── provider/                      # ========== Provider 接口 ==========
│   │   ├── interface.go               # 所有能力接口定义
│   │   ├── registry.go               # Provider 注册与发现
│   │   └── capability.go             # 能力枚举与查询
│   │
│   ├── adapter/                       # ========== 平台适配器 ==========
│   │   ├── openai/
│   │   │   ├── provider.go           # OpenAI 原生适配
│   │   │   ├── chat.go               # Chat Completions
│   │   │   ├── stream.go             # 流式解析
│   │   │   ├── embedding.go          # Embeddings
│   │   │   ├── image.go              # DALL-E / GPT Image
│   │   │   ├── audio.go              # TTS / STT
│   │   │   ├── video.go              # Sora
│   │   │   └── mapper.go            # 请求/响应映射
│   │   │
│   │   ├── anthropic/
│   │   │   ├── provider.go           # Anthropic 原生适配
│   │   │   ├── chat.go
│   │   │   ├── stream.go
│   │   │   └── mapper.go            # system prompt 抽取、字段映射
│   │   │
│   │   ├── google/
│   │   │   ├── provider.go           # Google Gemini 适配
│   │   │   ├── chat.go
│   │   │   ├── stream.go
│   │   │   ├── embedding.go
│   │   │   └── mapper.go            # contents/parts 结构转换
│   │   │
│   │   └── compatible/               # OpenAI 兼容层（国内平台共用）
│   │       ├── provider.go           # 通用 OpenAI 兼容适配
│   │       ├── chat.go
│   │       ├── stream.go
│   │       ├── embedding.go
│   │       ├── image.go              # 异步图像生成（国内平台特有）
│   │       ├── video.go              # 异步视频生成
│   │       ├── audio.go              # TTS / STT
│   │       ├── agent.go              # Agent 调用（平台专属）
│   │       ├── workflow.go           # Workflow 调用（平台专属）
│   │       └── platforms.go          # 各平台差异配置（base_url/model/特殊参数）
│   │
│   ├── mapper/                        # ========== 消息转换层 ==========
│   │   ├── message.go                # 统一 Message <-> 各平台格式 转换
│   │   ├── tool.go                   # 统一 Tool <-> 各平台 tool 格式
│   │   └── stream.go                # 统一 StreamEvent <-> 各平台 SSE 解析
│   │
│   ├── manager/                       # ========== 编排层 ==========
│   │   ├── manager.go                # 核心编排器（路由/缓存/指标）
│   │   ├── router.go                 # 模型分层路由 + 降级策略
│   │   ├── circuit_breaker.go        # 熔断器
│   │   ├── hedger.go                 # 对冲请求
│   │   ├── cache.go                  # 缓存层（内存 + Redis）
│   │   ├── rate_limiter.go           # 限流器
│   │   ├── token_counter.go          # Token 估算与 headroom 计算
│   │   └── metrics.go               # Prometheus 指标
│   │
│   └── transport/                     # ========== 传输层 ==========
│       ├── http_client.go            # 统一 HTTP 客户端（重试/超时/日志）
│       ├── auth.go                   # 认证策略（Bearer/x-api-key/access_token）
│       └── sse.go                    # SSE 通用解析器
│
├── api/                               # ========== 对外 API ==========
│   ├── handler/
│   │   ├── chat.go                   # POST /v1/chat/completions
│   │   ├── embedding.go             # POST /v1/embeddings
│   │   ├── image.go                 # POST /v1/images/generations
│   │   ├── audio.go                 # POST /v1/audio/speech | /transcriptions
│   │   ├── video.go                 # POST /v1/videos
│   │   ├── agent.go                 # POST /v1/agent/invoke
│   │   └── workflow.go              # POST /v1/workflow/run
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── ratelimit.go
│   │   └── logging.go
│   └── router.go                     # 路由注册
│
├── docs/
│   ├── multi-platform-api-analysis.md
│   └── architecture-design.md        # 本文档
├── go.mod
└── go.sum
```

---

## 4. 核心类型设计

### 4.1 消息与内容块

```go
// pkg/types/message.go

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
    RoleTool      Role = "tool"
)

// Message 统一消息结构
type Message struct {
    Role       Role      `json:"role"`
    Content    Content   `json:"content"`               // string 或 []ContentBlock
    Name       string    `json:"name,omitempty"`
    ToolCalls  []ToolCall `json:"tool_calls,omitempty"` // assistant 发起的工具调用
    ToolCallID string    `json:"tool_call_id,omitempty"` // tool 角色回传结果时
}

// Content 支持 string 和 []ContentBlock 双态
// 实现自定义 JSON 序列化/反序列化
type Content struct {
    Text   string         // 纯文本模式
    Blocks []ContentBlock // 多模态模式
}

// ContentBlock 多模态内容块
type ContentBlock struct {
    Type     string `json:"type"`               // text / image_url / audio / document
    Text     string `json:"text,omitempty"`
    ImageURL *Image `json:"image_url,omitempty"`
    Audio    *Audio `json:"audio,omitempty"`
}

type Image struct {
    URL    string `json:"url"`              // URL 或 data:base64
    Detail string `json:"detail,omitempty"` // low / high / auto
}
```

### 4.2 请求与响应

```go
// pkg/types/request.go

type ChatRequest struct {
    // --- 基础字段 ---
    Model       string    `json:"model,omitempty"`       // 指定模型，或留空走 Tier 路由
    Messages    []Message `json:"messages"`
    MaxTokens   *int      `json:"max_tokens,omitempty"`
    Temperature *float64  `json:"temperature,omitempty"`
    TopP        *float64  `json:"top_p,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
    Stop        []string  `json:"stop,omitempty"`

    // --- Tool Calling ---
    Tools      []Tool      `json:"tools,omitempty"`
    ToolChoice interface{} `json:"tool_choice,omitempty"` // string 或 object

    // --- 路由控制 ---
    Provider  string    `json:"provider,omitempty"`   // 指定平台（覆盖路由）
    ModelTier ModelTier `json:"model_tier,omitempty"` // 模型分层（SMALL/MEDIUM/LARGE）

    // --- 扩展 ---
    ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // JSON mode
    Extra          map[string]any  `json:"extra,omitempty"`           // 平台专属字段
}

// pkg/types/response.go

type ChatResponse struct {
    ID           string         `json:"id"`
    Model        string         `json:"model"`
    Provider     string         `json:"provider"`
    Content      string         `json:"content"`
    FinishReason string         `json:"finish_reason"`
    ToolCalls    []ToolCall     `json:"tool_calls,omitempty"`
    Usage        TokenUsage     `json:"usage"`
    LatencyMs    int64          `json:"latency_ms"`
    Cached       bool           `json:"cached,omitempty"`
    CreatedAt    int64          `json:"created_at"`
}

// StreamEvent 统一流式事件（解决 Shannon 双套流式接口问题）
type StreamEvent struct {
    Type         string      `json:"type"`           // content_delta / tool_call_delta / usage / done / error
    Delta        string      `json:"delta,omitempty"`
    ToolCall     *ToolCall   `json:"tool_call,omitempty"`
    Usage        *TokenUsage `json:"usage,omitempty"`
    FinishReason string      `json:"finish_reason,omitempty"`
    Error        string      `json:"error,omitempty"`
}
```

### 4.3 模型分层与能力

```go
// pkg/types/model.go

type ModelTier string

const (
    TierSmall  ModelTier = "small"
    TierMedium ModelTier = "medium"
    TierLarge  ModelTier = "large"
)

type ModelConfig struct {
    Provider      string           `yaml:"provider"`
    ModelID       string           `yaml:"model_id"`
    DisplayName   string           `yaml:"display_name"`
    Tier          ModelTier        `yaml:"tier"`
    ContextWindow int              `yaml:"context_window"`
    MaxOutput     int              `yaml:"max_output"`
    InputPrice    float64          `yaml:"input_price_per_1k"`
    OutputPrice   float64          `yaml:"output_price_per_1k"`
    Capabilities  ModelCapabilities `yaml:"capabilities"`
}

type ModelCapabilities struct {
    Chat       bool `yaml:"chat"`
    Vision     bool `yaml:"vision"`
    Tools      bool `yaml:"tools"`
    JSONMode   bool `yaml:"json_mode"`
    Streaming  bool `yaml:"streaming"`
    Reasoning  bool `yaml:"reasoning"`
    Embedding  bool `yaml:"embedding"`
    ImageGen   bool `yaml:"image_gen"`
    VideoGen   bool `yaml:"video_gen"`
    TTS        bool `yaml:"tts"`
    STT        bool `yaml:"stt"`
}
```

### 4.4 统一错误

```go
// pkg/types/error.go

type ErrorCode string

const (
    ErrAuthentication  ErrorCode = "authentication_error"
    ErrRateLimit       ErrorCode = "rate_limit_error"
    ErrInvalidRequest  ErrorCode = "invalid_request_error"
    ErrModelNotFound   ErrorCode = "model_not_found"
    ErrProviderError   ErrorCode = "provider_error"
    ErrTimeout         ErrorCode = "timeout_error"
    ErrCapabilityUnavailable ErrorCode = "capability_unavailable"
)

type ProviderError struct {
    Code       ErrorCode       `json:"code"`
    Message    string          `json:"message"`
    Provider   string          `json:"provider"`
    StatusCode int             `json:"status_code"`
    Retryable  bool            `json:"retryable"`
    Raw        json.RawMessage `json:"raw,omitempty"`
}

func (e *ProviderError) Error() string { ... }

// IsTransient 判断是否可重试（用于熔断器和降级）
func (e *ProviderError) IsTransient() bool {
    return e.Retryable
}
```

### 4.5 异步任务（图像/视频生成）

```go
// pkg/types/async_task.go

type TaskStatus string

const (
    TaskPending   TaskStatus = "pending"
    TaskRunning   TaskStatus = "running"
    TaskSucceeded TaskStatus = "succeeded"
    TaskFailed    TaskStatus = "failed"
    TaskCancelled TaskStatus = "cancelled"
)

type AsyncTask struct {
    TaskID    string            `json:"task_id"`
    Provider  string            `json:"provider"`
    Type      string            `json:"type"`       // image_gen / video_gen
    Status    TaskStatus        `json:"status"`
    Progress  int               `json:"progress"`   // 0-100
    ResultURL string            `json:"result_url,omitempty"`
    Result    json.RawMessage   `json:"result,omitempty"`
    Error     string            `json:"error,omitempty"`
    CreatedAt int64             `json:"created_at"`
    Extra     map[string]any    `json:"extra,omitempty"`
}
```

---

## 5. 能力接口设计

### 5.1 核心理念：接口分离 + 组合

Shannon 的问题在于一个 `LLMProvider` 基类包含所有方法。Go 用 interface 组合天然解决这个问题：

```go
// pkg/provider/interface.go

// ---- 基础接口（所有 provider 必须实现） ----

type Provider interface {
    Name() string
    Models() []ModelConfig
    Supports(cap Capability) bool  // 运行时能力查询
    Close() error
}

// ---- 能力接口（按需实现） ----

// P0: 对话与流式
type ChatProvider interface {
    Provider
    Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error)
    ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error)
}

// P1: Tool Calling（通常与 Chat 一起，但逻辑上可分离）
// Tool Calling 通过 ChatRequest.Tools 字段触发，不需要独立接口
// 但需要通过 Supports(CapTools) 查询能力

// P1: Embeddings
type EmbeddingProvider interface {
    Provider
    Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
}

// P2: Agent 调用
type AgentProvider interface {
    Provider
    InvokeAgent(ctx context.Context, req *AgentRequest) (*AgentResponse, error)
    InvokeAgentStream(ctx context.Context, req *AgentRequest) (<-chan StreamEvent, error)
}

// P2: Workflow 调用
type WorkflowProvider interface {
    Provider
    RunWorkflow(ctx context.Context, req *WorkflowRequest) (*WorkflowResponse, error)
}

// P3: 图像生成
type ImageGenProvider interface {
    Provider
    GenerateImage(ctx context.Context, req *ImageGenRequest) (*ImageGenResponse, error)
    // 异步模式（国内平台）
    SubmitImageTask(ctx context.Context, req *ImageGenRequest) (*AsyncTask, error)
    GetTaskStatus(ctx context.Context, taskID string) (*AsyncTask, error)
}

// P3: 视频生成（全部异步）
type VideoGenProvider interface {
    Provider
    SubmitVideoTask(ctx context.Context, req *VideoGenRequest) (*AsyncTask, error)
    GetVideoTaskStatus(ctx context.Context, taskID string) (*AsyncTask, error)
    CancelVideoTask(ctx context.Context, taskID string) error
}

// P3: 语音合成
type TTSProvider interface {
    Provider
    Synthesize(ctx context.Context, req *TTSRequest) (io.ReadCloser, error)
}

// P3: 语音识别
type STTProvider interface {
    Provider
    Transcribe(ctx context.Context, req *STTRequest) (*STTResponse, error)
}
```

### 5.2 能力查询

```go
// pkg/provider/capability.go

type Capability string

const (
    CapChat      Capability = "chat"
    CapStream    Capability = "stream"
    CapTools     Capability = "tools"
    CapVision    Capability = "vision"
    CapJSONMode  Capability = "json_mode"
    CapReasoning Capability = "reasoning"
    CapEmbed     Capability = "embedding"
    CapImageGen  Capability = "image_gen"
    CapVideoGen  Capability = "video_gen"
    CapTTS       Capability = "tts"
    CapSTT       Capability = "stt"
    CapAgent     Capability = "agent"
    CapWorkflow  Capability = "workflow"
)

// 类型断言 + 配置双重判断
func SupportsCapability(p Provider, cap Capability) bool {
    switch cap {
    case CapEmbed:
        _, ok := p.(EmbeddingProvider)
        return ok
    case CapImageGen:
        _, ok := p.(ImageGenProvider)
        return ok
    // ... 其他能力用类型断言
    default:
        return p.Supports(cap) // 回退到配置层判断
    }
}
```

### 5.3 Provider Registry

```go
// pkg/provider/registry.go

type Registry struct {
    mu        sync.RWMutex
    providers map[string]Provider                    // name -> provider
    tierRoute map[ModelTier][]TierEntry              // tier -> 按优先级排序的 provider
    modelMap  map[string]string                      // "gpt-4o" -> "openai"
}

type TierEntry struct {
    ProviderName string
    ModelID      string
    Priority     int
}

func (r *Registry) Register(p Provider) { ... }
func (r *Registry) Get(name string) (Provider, bool) { ... }
func (r *Registry) GetByModel(modelID string) (Provider, bool) { ... }
func (r *Registry) GetForTier(tier ModelTier) []TierEntry { ... }
func (r *Registry) ListCapable(cap Capability) []Provider { ... }
```

---

## 6. 适配器实现策略

### 6.1 OpenAI — 原生适配（基准实现）

```go
// pkg/adapter/openai/provider.go

type OpenAIProvider struct {
    client  *http.Client
    apiKey  string
    baseURL string // 默认 https://api.openai.com/v1
    models  []ModelConfig
}

// 实现所有能力接口
var _ ChatProvider     = (*OpenAIProvider)(nil)
var _ EmbeddingProvider = (*OpenAIProvider)(nil)
var _ ImageGenProvider  = (*OpenAIProvider)(nil)
var _ TTSProvider       = (*OpenAIProvider)(nil)
var _ STTProvider       = (*OpenAIProvider)(nil)
var _ VideoGenProvider  = (*OpenAIProvider)(nil)
```

请求格式即为内部标准，**无需转换**。

### 6.2 Anthropic — 原生适配（重点处理差异）

```go
// pkg/adapter/anthropic/mapper.go

// MapChatRequest 将统一 ChatRequest 转换为 Anthropic 格式
func MapChatRequest(req *ChatRequest) *anthropicRequest {
    ar := &anthropicRequest{
        Model:     req.Model,
        MaxTokens: resolveMaxTokens(req), // 必填，提供默认值 4096
    }

    // 1. 从 messages 中抽取 system prompt
    ar.System, ar.Messages = extractSystem(req.Messages)

    // 2. 转换 stop -> stop_sequences
    ar.StopSequences = req.Stop

    // 3. 转换 tool_choice: string -> object
    ar.ToolChoice = convertToolChoice(req.ToolChoice)

    // 4. 转换 tools 格式: parameters -> input_schema
    ar.Tools = convertTools(req.Tools)

    // 5. 传递 thinking 等独有参数
    if v, ok := req.Extra["thinking"]; ok {
        ar.Thinking = v
    }

    return ar
}

// MapChatResponse 将 Anthropic 响应转换为统一格式
func MapChatResponse(ar *anthropicResponse) *ChatResponse {
    return &ChatResponse{
        ID:           ar.ID,
        Model:        ar.Model,
        Provider:     "anthropic",
        Content:      extractText(ar.Content),
        FinishReason: mapStopReason(ar.StopReason), // stop_reason -> finish_reason
        ToolCalls:    extractToolCalls(ar.Content),
        Usage: TokenUsage{
            PromptTokens:     ar.Usage.InputTokens,      // 字段名映射
            CompletionTokens: ar.Usage.OutputTokens,
        },
    }
}
```

**关键映射点**:
- `system` 在 messages 中 → 顶层字段
- `max_tokens` 可选 → 必填（默认 4096）
- `stop` → `stop_sequences`
- `finish_reason` ← `stop_reason`
- `prompt_tokens/completion_tokens` ← `input_tokens/output_tokens`
- `tool_choice: "auto"` → `tool_choice: {"type":"auto"}`
- 流式 SSE: `choices[0].delta.content` ← `content_block_delta.delta.text`

### 6.3 Google Gemini — 原生适配（差异最大）

```go
// pkg/adapter/google/mapper.go

func MapChatRequest(req *ChatRequest) *geminiRequest {
    gr := &geminiRequest{}

    // 1. 消息结构转换: messages -> contents[].parts[]
    gr.Contents = convertMessages(req.Messages)

    // 2. system prompt -> systemInstruction
    gr.SystemInstruction = extractSystemInstruction(req.Messages)

    // 3. 参数嵌套到 generationConfig
    gr.GenerationConfig = &genConfig{
        Temperature:     req.Temperature,
        TopP:            req.TopP,
        MaxOutputTokens: req.MaxTokens,
        StopSequences:   req.Stop,
    }

    // 4. tools -> functionDeclarations
    gr.Tools = convertTools(req.Tools)

    return gr
}
```

**关键映射点**:
- URL 路径包含模型名: `/models/{model}:generateContent`
- 流式用不同端点: `:streamGenerateContent`
- `messages[].content` → `contents[].parts[].text`
- `role: "assistant"` → `role: "model"`
- 所有采样参数嵌套在 `generationConfig` 中
- Embeddings: `/models/{model}:embedContent` 格式完全不同

### 6.4 OpenAI Compatible — 国内平台通用层（最大复用）

```go
// pkg/adapter/compatible/provider.go

type CompatibleProvider struct {
    client   *http.Client
    apiKey   string
    baseURL  string
    platform Platform    // alibaba / baidu / volcengine / zhipu / minimax
    models   []ModelConfig
    quirks   PlatformQuirks // 平台特殊行为
}

// PlatformQuirks 各平台微小差异
type PlatformQuirks struct {
    // 图像/视频生成走异步任务
    AsyncImageGen    bool
    AsyncVideoGen    bool
    // Agent/Workflow 端点
    AgentEndpoint    string
    WorkflowEndpoint string
    // 请求修改器（某些平台需要额外 header 或参数调整）
    RequestModifier  func(req *http.Request)
}

// platforms.go — 各平台预设配置
var PlatformPresets = map[Platform]PlatformConfig{
    PlatformAlibaba: {
        BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
        Quirks: PlatformQuirks{
            AsyncImageGen:    true,
            AsyncVideoGen:    true,
            AgentEndpoint:    "/apps/{app_id}/completion",
            WorkflowEndpoint: "/apps/{app_id}/completion",
        },
    },
    PlatformBaidu: {
        BaseURL: "https://qianfan.baidubce.com/v2",
        Quirks:  PlatformQuirks{AsyncImageGen: true, AsyncVideoGen: true},
    },
    PlatformVolcengine: {
        BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
        Quirks:  PlatformQuirks{AsyncVideoGen: true},
    },
    PlatformZhipu: {
        BaseURL: "https://open.bigmodel.cn/api/paas/v4",
        Quirks:  PlatformQuirks{AsyncImageGen: true, AsyncVideoGen: true},
    },
    PlatformMiniMax: {
        BaseURL: "https://api.minimax.io/v1",
        Quirks:  PlatformQuirks{AsyncVideoGen: true},
    },
}
```

Chat/Embed/Stream 直接走 OpenAI 格式（只改 baseURL）。图像/视频/Agent 等走平台专属适配。

---

## 7. 编排层设计（Manager）

### 7.1 Manager 核心

```go
// pkg/manager/manager.go

type Manager struct {
    registry       *Registry
    router         *Router
    cache          *Cache
    circuitBreakers map[string]*CircuitBreaker
    rateLimiters    map[string]*RateLimiter
    tokenCounter   *TokenCounter
    metrics        *Metrics
    config         *Config
}

// Chat 统一对话入口
func (m *Manager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 选择 provider（路由 + 熔断检查）
    p, model, err := m.router.Select(req)

    // 2. 限流检查
    if err := m.rateLimiters[p.Name()].Allow(); err != nil { ... }

    // 3. 缓存查询（非流式、非 tool calling 时）
    if !req.Stream && len(req.Tools) == 0 {
        if cached := m.cache.Get(req); cached != nil { return cached, nil }
    }

    // 4. Token headroom 计算
    req.MaxTokens = m.tokenCounter.ClampMaxTokens(req, model)

    // 5. 调用 provider（带熔断器保护）
    resp, err := m.circuitBreakers[p.Name()].Execute(func() (*ChatResponse, error) {
        return p.(ChatProvider).Chat(ctx, req)
    })

    // 6. 失败降级：尝试下一个 provider
    if err != nil && isTransient(err) {
        resp, err = m.fallback(ctx, req, p.Name())
    }

    // 7. 缓存写入（带安全检查）
    if err == nil && m.cache.IsSafeToCache(resp) {
        m.cache.Set(req, resp)
    }

    // 8. 指标上报
    m.metrics.RecordRequest(p.Name(), model, resp, err)

    return resp, err
}

// ChatStream 统一流式入口
func (m *Manager) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
    p, model, err := m.router.Select(req)
    if err != nil { return nil, err }

    req.MaxTokens = m.tokenCounter.ClampMaxTokens(req, model)

    ch, err := p.(ChatProvider).ChatStream(ctx, req)
    if err != nil && isTransient(err) {
        // 流式降级
        ch, err = m.fallbackStream(ctx, req, p.Name())
    }

    // 包装 channel 添加指标收集
    return m.wrapStreamMetrics(ch, p.Name(), model), err
}
```

### 7.2 路由器

```go
// pkg/manager/router.go

type Router struct {
    registry *Registry
    config   *RouterConfig
}

// Select 选择 provider 和模型
func (r *Router) Select(req *ChatRequest) (ChatProvider, string, error) {
    // 优先级 1: 显式指定 provider
    if req.Provider != "" {
        p, ok := r.registry.Get(req.Provider)
        if !ok { return nil, "", ErrProviderNotFound }
        return p.(ChatProvider), req.Model, nil
    }

    // 优先级 2: 显式指定 model -> 反查 provider
    if req.Model != "" {
        p, ok := r.registry.GetByModel(req.Model)
        if !ok { return nil, "", ErrModelNotFound }
        return p.(ChatProvider), req.Model, nil
    }

    // 优先级 3: 按 ModelTier 路由（带熔断检查）
    tier := req.ModelTier
    if tier == "" { tier = TierMedium } // 默认中档

    entries := r.registry.GetForTier(tier)
    for _, entry := range entries {
        if r.isHealthy(entry.ProviderName) {
            p, _ := r.registry.Get(entry.ProviderName)
            return p.(ChatProvider), entry.ModelID, nil
        }
    }

    return nil, "", ErrNoAvailableProvider
}
```

### 7.3 熔断器（借鉴 Shannon）

```go
// pkg/manager/circuit_breaker.go

type State int
const (
    StateClosed   State = iota // 正常
    StateOpen                  // 断开（拒绝请求）
    StateHalfOpen              // 半开（允许探测）
)

type CircuitBreaker struct {
    mu              sync.Mutex
    state           State
    failures        int
    failureThreshold int           // 默认 5
    recoveryTimeout  time.Duration // 默认 30s
    lastFailure     time.Time
}

func (cb *CircuitBreaker) Execute(fn func() (*ChatResponse, error)) (*ChatResponse, error) {
    if !cb.AllowRequest() {
        return nil, &ProviderError{Code: ErrProviderError, Message: "circuit breaker open", Retryable: true}
    }
    resp, err := fn()
    if err != nil && isTransient(err) {
        cb.RecordFailure()
    } else {
        cb.RecordSuccess()
    }
    return resp, err
}
```

### 7.4 对冲请求（借鉴 Shannon，Go 天然优势）

```go
// pkg/manager/hedger.go

// Hedge 同时发两个请求，谁先回用谁
func Hedge(ctx context.Context, primary, fallback func(context.Context) (*ChatResponse, error), delay time.Duration) (*ChatResponse, error) {
    ctx, cancel := context.WithCancel(ctx)
    defer cancel()

    type result struct {
        resp *ChatResponse
        err  error
    }

    ch := make(chan result, 2)

    // 主请求立即发送
    go func() {
        resp, err := primary(ctx)
        ch <- result{resp, err}
    }()

    // 备选请求延迟发送
    go func() {
        select {
        case <-time.After(delay):
            resp, err := fallback(ctx)
            ch <- result{resp, err}
        case <-ctx.Done():
            return
        }
    }()

    // 取第一个成功的
    r := <-ch
    if r.err == nil {
        return r.resp, nil
    }
    // 第一个失败了，等第二个
    r = <-ch
    return r.resp, r.err
}
```

### 7.5 缓存安全（借鉴 Shannon）

```go
// pkg/manager/cache.go

func (c *Cache) IsSafeToCache(resp *ChatResponse) bool {
    // 不缓存截断的响应
    if resp.FinishReason == "length" { return false }
    // 不缓存被内容过滤的响应
    if resp.FinishReason == "content_filter" { return false }
    // 不缓存空响应
    if resp.Content == "" && len(resp.ToolCalls) == 0 { return false }
    return true
}
```

---

## 8. 消息转换层（独立于 Provider）

Shannon 的问题：消息转换逻辑散落在各 provider 中。我们抽出独立的 mapper 层：

```go
// pkg/mapper/message.go

type Format string
const (
    FormatOpenAI    Format = "openai"
    FormatAnthropic Format = "anthropic"
    FormatGemini    Format = "gemini"
)

// ToAnthropic 将统一 Message 转为 Anthropic 格式
func ToAnthropic(messages []Message) (system string, converted []AnthropicMessage) {
    for _, msg := range messages {
        if msg.Role == RoleSystem {
            system += msg.Content.String() + "\n"
            continue
        }
        converted = append(converted, AnthropicMessage{
            Role:    mapRole(msg.Role),
            Content: convertContent(msg.Content),
        })
    }
    return
}

// ToGemini 将统一 Message 转为 Gemini 格式
func ToGemini(messages []Message) (systemInstruction *GeminiContent, contents []GeminiContent) {
    for _, msg := range messages {
        if msg.Role == RoleSystem {
            systemInstruction = &GeminiContent{
                Parts: []GeminiPart{{Text: msg.Content.String()}},
            }
            continue
        }
        contents = append(contents, GeminiContent{
            Role:  geminiRole(msg.Role), // assistant -> model
            Parts: toParts(msg.Content),
        })
    }
    return
}
```

```go
// pkg/mapper/tool.go

// ToAnthropicTools 转换 tool 定义格式
func ToAnthropicTools(tools []Tool) []AnthropicTool {
    // OpenAI: {type:"function", function:{name, description, parameters}}
    // Claude: {name, description, input_schema}
    ...
}

// ToGeminiTools 转换 tool 定义格式
func ToGeminiTools(tools []Tool) []GeminiFunctionDeclaration {
    // OpenAI: tools[].function.parameters
    // Gemini: tools[].functionDeclarations[].parameters
    ...
}
```

---

## 9. 流式 SSE 统一解析

```go
// pkg/transport/sse.go

// SSEReader 通用 SSE 流读取器
type SSEReader struct {
    reader *bufio.Reader
}

type SSEEvent struct {
    Event string // 事件类型（Anthropic 用）
    Data  string // 数据
}

func (r *SSEReader) Read() (*SSEEvent, error) { ... }

// pkg/mapper/stream.go

// ParseOpenAIStream 解析 OpenAI 格式 SSE -> StreamEvent
func ParseOpenAIStream(event *SSEEvent) (*StreamEvent, error) {
    if event.Data == "[DONE]" {
        return &StreamEvent{Type: "done"}, nil
    }
    var chunk openaiStreamChunk
    json.Unmarshal([]byte(event.Data), &chunk)
    return &StreamEvent{
        Type:  "content_delta",
        Delta: chunk.Choices[0].Delta.Content,
    }, nil
}

// ParseAnthropicStream 解析 Anthropic 格式 SSE -> StreamEvent
func ParseAnthropicStream(event *SSEEvent) (*StreamEvent, error) {
    switch event.Event {
    case "content_block_delta":
        var delta anthropicDelta
        json.Unmarshal([]byte(event.Data), &delta)
        return &StreamEvent{
            Type:  "content_delta",
            Delta: delta.Delta.Text,
        }, nil
    case "message_delta":
        // 包含 usage 和 finish_reason
        ...
    case "message_stop":
        return &StreamEvent{Type: "done"}, nil
    }
    return nil, nil // 忽略其他事件
}

// ParseGeminiStream 解析 Gemini 格式 SSE -> StreamEvent
func ParseGeminiStream(event *SSEEvent) (*StreamEvent, error) { ... }
```

---

## 10. 认证策略

```go
// pkg/transport/auth.go

type AuthStrategy interface {
    Apply(req *http.Request) error
}

// BearerAuth — OpenAI / 国内平台
type BearerAuth struct {
    APIKey string
}
func (a *BearerAuth) Apply(req *http.Request) error {
    req.Header.Set("Authorization", "Bearer "+a.APIKey)
    return nil
}

// AnthropicAuth — x-api-key + version header
type AnthropicAuth struct {
    APIKey  string
    Version string // 默认 "2023-06-01"
}
func (a *AnthropicAuth) Apply(req *http.Request) error {
    req.Header.Set("x-api-key", a.APIKey)
    req.Header.Set("anthropic-version", a.Version)
    return nil
}

// GoogleAuth — API Key
type GoogleAuth struct {
    APIKey string
}
func (a *GoogleAuth) Apply(req *http.Request) error {
    q := req.URL.Query()
    q.Set("key", a.APIKey)
    req.URL.RawQuery = q.Encode()
    return nil
}
```

---

## 11. 配置文件设计

```yaml
# config/models.yaml

providers:
  openai:
    base_url: "https://api.openai.com/v1"
    api_key: "${OPENAI_API_KEY}"
    rate_limit: 500  # RPM

  anthropic:
    base_url: "https://api.anthropic.com/v1"
    api_key: "${ANTHROPIC_API_KEY}"
    rate_limit: 200

  alibaba:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "${DASHSCOPE_API_KEY}"
    platform: "alibaba"  # 标记为兼容平台
    rate_limit: 300

  volcengine:
    base_url: "https://ark.cn-beijing.volces.com/api/v3"
    api_key: "${ARK_API_KEY}"
    platform: "volcengine"
    rate_limit: 300

  zhipu:
    base_url: "https://open.bigmodel.cn/api/paas/v4"
    api_key: "${ZHIPU_API_KEY}"
    platform: "zhipu"
    rate_limit: 200

  minimax:
    base_url: "https://api.minimax.io/v1"
    api_key: "${MINIMAX_API_KEY}"
    platform: "minimax"
    rate_limit: 200

model_catalog:
  # OpenAI 模型
  - id: "gpt-4o"
    provider: "openai"
    tier: "large"
    context_window: 128000
    max_output: 16384
    input_price: 0.0025
    output_price: 0.01
    capabilities: { chat: true, vision: true, tools: true, json_mode: true, streaming: true }

  - id: "gpt-4o-mini"
    provider: "openai"
    tier: "small"
    context_window: 128000
    max_output: 16384
    input_price: 0.00015
    output_price: 0.0006
    capabilities: { chat: true, vision: true, tools: true, json_mode: true, streaming: true }

  - id: "text-embedding-3-small"
    provider: "openai"
    tier: "small"
    capabilities: { embedding: true }

  # Anthropic 模型
  - id: "claude-sonnet-4-5-20250929"
    provider: "anthropic"
    tier: "large"
    context_window: 200000
    max_output: 8192
    input_price: 0.003
    output_price: 0.015
    capabilities: { chat: true, vision: true, tools: true, json_mode: true, streaming: true, reasoning: true }

  # 阿里百炼模型
  - id: "qwen-max"
    provider: "alibaba"
    tier: "large"
    context_window: 32768
    max_output: 8192
    capabilities: { chat: true, vision: true, tools: true, streaming: true }

  - id: "qwen-turbo"
    provider: "alibaba"
    tier: "small"
    context_window: 131072
    max_output: 8192
    capabilities: { chat: true, tools: true, streaming: true }

  # 更多模型...

# 分层路由
tier_routing:
  small:
    - { provider: "alibaba",    model: "qwen-turbo",   priority: 1 }
    - { provider: "openai",     model: "gpt-4o-mini",  priority: 2 }
  medium:
    - { provider: "alibaba",    model: "qwen-plus",    priority: 1 }
    - { provider: "volcengine", model: "doubao-pro",   priority: 2 }
  large:
    - { provider: "openai",     model: "gpt-4o",       priority: 1 }
    - { provider: "anthropic",  model: "claude-sonnet-4-5-20250929", priority: 2 }
    - { provider: "alibaba",    model: "qwen-max",     priority: 3 }

# 编排配置
manager:
  cache:
    enabled: true
    backend: "memory"   # memory / redis
    ttl: "5m"
    max_size: 1000
    redis_url: "${REDIS_URL}"

  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: "30s"

  hedged_request:
    enabled: true
    delay: "500ms"

  token_safety_margin: 256
```

---

## 12. 对外 API 设计

对外暴露 **OpenAI 兼容的 HTTP API**，让上层业务可以像调用 OpenAI 一样调用我们的服务，只需增加 `provider` 和 `model_tier` 扩展字段：

```
POST /v1/chat/completions      -> handler.Chat        # 对话（兼容 OpenAI 格式）
POST /v1/embeddings            -> handler.Embedding    # 向量化
POST /v1/images/generations    -> handler.ImageGen     # 图像生成
POST /v1/audio/speech          -> handler.TTS          # 语音合成
POST /v1/audio/transcriptions  -> handler.STT          # 语音识别
POST /v1/videos                -> handler.VideoGen     # 视频生成
POST /v1/agent/invoke          -> handler.Agent        # Agent 调用
POST /v1/workflow/run           -> handler.Workflow     # Workflow 调用

GET  /v1/models                -> handler.ListModels   # 模型列表
GET  /v1/providers             -> handler.ListProviders # Provider 状态
GET  /v1/tasks/{id}            -> handler.GetTask      # 异步任务查询
```

扩展字段（通过请求体传递）：
```json
{
  "model": "gpt-4o",
  "messages": [...],
  "provider": "openai",        // 可选：指定平台
  "model_tier": "large",       // 可选：按分层路由
  "extra": {                   // 可选：平台专属参数
    "thinking": {"type": "enabled", "budget_tokens": 2048}
  }
}
```

---

## 13. 实施计划

### Phase 1 — 基础框架 + 对话/流式（2~3 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 1.1 | 项目初始化：go mod、目录结构、配置加载 | `go.mod`, `config/` |
| 1.2 | 核心类型定义：Message, ChatRequest/Response, StreamEvent, Error | `pkg/types/` |
| 1.3 | Transport 层：HTTP Client, Auth 策略, SSE Reader | `pkg/transport/` |
| 1.4 | Provider 接口 + Registry | `pkg/provider/` |
| 1.5 | OpenAI 适配器：Chat + Stream | `pkg/adapter/openai/` |
| 1.6 | Anthropic 适配器：Chat + Stream + Mapper | `pkg/adapter/anthropic/` |
| 1.7 | OpenAI Compatible 适配器（阿里/火山/智谱/百度/MiniMax） | `pkg/adapter/compatible/` |
| 1.8 | Manager 编排层：路由、熔断器、限流 | `pkg/manager/` |
| 1.9 | 对外 API：`/v1/chat/completions` Handler | `api/` |
| 1.10 | 集成测试：各平台 Chat + Stream 全通 | `tests/` |

**Phase 1 交付标准**: 8 个平台的对话和流式全部跑通，支持 ModelTier 路由和自动降级。

### Phase 2 — Tool Calling + Embeddings（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 2.1 | Tool 类型定义：Tool, ToolCall, ToolResult | `pkg/types/tool.go` |
| 2.2 | Tool Mapper：OpenAI ↔ Anthropic ↔ Gemini 格式互转 | `pkg/mapper/tool.go` |
| 2.3 | 各适配器支持 tools 参数和 tool_calls 响应 | 各 `adapter/` |
| 2.4 | Embedding 接口 + OpenAI/Compatible 实现 | `EmbeddingProvider` |
| 2.5 | Google Embeddings 适配（`embedContent` 格式） | `adapter/google/embedding.go` |
| 2.6 | 对外 API：`/v1/embeddings` | `api/handler/embedding.go` |
| 2.7 | 缓存层实现（内存 + Redis 可选） | `pkg/manager/cache.go` |
| 2.8 | 对冲请求实现 | `pkg/manager/hedger.go` |

**Phase 2 交付标准**: Tool Calling 在 OpenAI/Anthropic/兼容平台全通；Embeddings 可用。

### Phase 3 — Agent + Workflow 调用（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 3.1 | Agent/Workflow 请求/响应类型定义 | `pkg/types/` |
| 3.2 | AgentProvider / WorkflowProvider 接口 | `pkg/provider/interface.go` |
| 3.3 | 阿里百炼 Agent/Workflow 适配（应用调用 API） | `adapter/compatible/agent.go` |
| 3.4 | 其他平台 Agent 适配（按需） | 各 `adapter/` |
| 3.5 | 对外 API：`/v1/agent/invoke`, `/v1/workflow/run` | `api/handler/` |

**Phase 3 交付标准**: 阿里百炼的智能体和工作流可通过统一接口调用。

### Phase 4 — 多媒体能力（2~3 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 4.1 | AsyncTask 类型 + 异步任务管理器 | `pkg/types/async_task.go`, `pkg/manager/` |
| 4.2 | ImageGenProvider 实现：OpenAI（同步）+ 国内平台（异步） | 各 `adapter/` |
| 4.3 | VideoGenProvider 实现（全部异步 submit+poll） | 各 `adapter/` |
| 4.4 | TTSProvider 实现：OpenAI + 阿里 + MiniMax | 各 `adapter/` |
| 4.5 | STTProvider 实现：OpenAI + 阿里 | 各 `adapter/` |
| 4.6 | Google Gemini 完整适配（含 Imagen/Veo） | `adapter/google/` |
| 4.7 | 对外 API：`/v1/images/`, `/v1/videos`, `/v1/audio/` | `api/handler/` |
| 4.8 | 异步任务查询 API：`/v1/tasks/{id}` | `api/handler/` |

**Phase 4 交付标准**: 图像/视频/语音接口可用，异步任务可提交和查询。

### Phase 5 — 生产化（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 5.1 | Prometheus 指标全覆盖 | `pkg/manager/metrics.go` |
| 5.2 | 结构化日志（slog） | 全局 |
| 5.3 | 配置热更新 | `config/` |
| 5.4 | Dockerfile + Helm Chart | 部署文件 |
| 5.5 | 完整 README + API 文档 | `docs/` |
| 5.6 | 压测与调优 | 性能报告 |

---

## 14. 关键技术决策总结

| 决策 | 选择 | 理由 |
|------|------|------|
| 内部数据格式 | OpenAI 格式 | 事实标准，6/8 平台兼容 |
| Provider 架构 | Interface 组合 | 能力分离，编译期安全 |
| 国内平台策略 | 共用 Compatible 适配器 | 避免重复代码，差异用 Quirks 处理 |
| 消息转换 | 独立 Mapper 层 | 可复用，不散落在 provider 中 |
| 流式接口 | 统一 `<-chan StreamEvent` | 一套接口，不学 Shannon 搞两套 |
| 异步任务 | 统一 Submit + Poll 模型 | 覆盖图像/视频所有异步场景 |
| 缓存安全 | 借鉴 Shannon 全套检查 | 不缓存截断/过滤/空响应 |
| 可靠性 | 熔断 + 对冲 + Tier 降级 | 借鉴 Shannon，Go goroutine 实现更自然 |
| HTTP 框架 | 标准库 `net/http` 或 chi | 轻量，不过度引入依赖 |
| 对外 API | OpenAI 兼容格式 | 上层业务零成本切换 |
