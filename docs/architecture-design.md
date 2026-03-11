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
| **SDK-First** | 核心逻辑封装为可独立引用的 Go package，HTTP 服务仅为薄壳 |
| **Hook 可扩展** | 请求生命周期各阶段暴露 Hook 点，支持自定义回调（借鉴 LiteLLM CustomLogger） |
| **Deadline 驱动** | 全局时间预算贯穿请求生命周期，所有重试/等待共享 deadline |

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
                          └──────────┬───────────────┘
                                     │
               ┌─────────────────────┼─────────────────────┐
               │                     │                     │
    ┌──────────▼──────────┐ ┌───────▼────────┐ ┌─────────▼─────────┐
    │   Go SDK (import)    │ │   HTTP API     │ │      gRPC         │
    │ gateway.New(cfg)     │ │ /v1/chat/...   │ │  (future)         │
    └──────────┬──────────┘ └───────┬────────┘ └─────────┬─────────┘
               │                    │                     │
               └────────────────────┼─────────────────────┘
                                    │
                          ┌─────────▼──────────┐
                          │   pkg/gateway      │
                          │   Client (SDK核心)  │
                          └─────────┬──────────┘
                                    │
                          ┌─────────▼──────────┐
                          │   pkg/hook         │
                          │   Hook Registry    │
                          └─────────┬──────────┘
                                    │
                          ┌─────────▼──────────┐
                          │       Manager       │
                          │ 路由·熔断·缓存·限流  │
                          └─────────┬──────────┘
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
│       └── main.go                    # 服务入口（薄壳：加载配置 → gateway.New() → 包装 HTTP Handler）
├── config/
│   ├── config.go                      # 配置结构定义
│   ├── config.yaml                    # 默认配置模板
│   └── models.yaml                    # 模型目录与定价
├── pkg/
│   ├── gateway/                       # ========== SDK 入口（NEW）==========
│   │   ├── client.go                 # Gateway Client，SDK 核心入口
│   │   ├── options.go                # 功能选项（WithCache, WithHook, WithLogger 等）
│   │   └── client_test.go            # SDK 集成测试
│   │
│   ├── hook/                          # ========== Hook/Callback 系统（NEW，借鉴 LiteLLM CustomLogger）==========
│   │   ├── hook.go                   # Hook 接口定义（Phase, HookEvent）
│   │   └── registry.go               # Hook 注册与调度
│   │
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
│   │   ├── openai/                   # OpenAI 及所有兼容接口的适配器
│   │   │   ├── provider.go           # OpenAI struct + NewWithName() 复用
│   │   │   ├── chat.go               # Chat Completions + 请求/响应映射
│   │   │   ├── stream.go             # 流式 SSE 解析
│   │   │   ├── responses.go          # Responses API（推理模型优化）
│   │   │   ├── embedding.go          # Embeddings（Sprint 6）
│   │   │   ├── image.go              # DALL-E / GPT Image（Sprint 7）
│   │   │   └── audio.go              # TTS / STT（Sprint 7）
│   │   │
│   │   ├── anthropic/                # Anthropic 原生 Messages API
│   │   │   ├── provider.go           # Anthropic struct + 可配置 endpoints
│   │   │   ├── chat.go               # Chat + 消息转换（system 抽取等）
│   │   │   └── stream.go             # Anthropic SSE 事件解析
│   │   │
│   │   └── google/                   # Google Gemini（Sprint 7）
│   │       ├── provider.go
│   │       ├── chat.go
│   │       ├── stream.go
│   │       └── embedding.go
│   │
│   ├── manager/                       # ========== 编排层 ==========
│   │   ├── manager.go                # 核心编排器（路由/缓存/指标）
│   │   ├── router.go                 # 模型分层路由 + 降级策略
│   │   ├── circuit_breaker.go        # 熔断器
│   │   ├── cooldown.go               # Per-Model 冷却（NEW，借鉴 LLM-API-Key-Proxy）
│   │   ├── retry.go                  # 重试策略矩阵（按错误码分类）
│   │   ├── timeout.go                # 超时分级策略
│   │   ├── stream_failover.go        # 流式中途失败策略
│   │   ├── idempotency.go            # 幂等键存储与检查
│   │   ├── hedger.go                 # 对冲请求
│   │   ├── cache.go                  # DualCache（Memory + Redis，借鉴 LiteLLM）
│   │   ├── rate_limiter.go           # 限流器
│   │   ├── token_counter.go          # Token 估算与 headroom 计算
│   │   ├── quota.go                  # 预扣+结算配额（NEW，借鉴 New-API）
│   │   └── spend_writer.go           # 异步批量消费记录（NEW，借鉴 LiteLLM DBSpendUpdateWriter）
│   │
│   ├── observability/                 # ========== 可观测性 ==========
│   │   ├── tracing.go                # OpenTelemetry tracing
│   │   ├── metrics.go                # Prometheus 指标定义与上报
│   │   └── slo.go                    # SLI/SLO 定义与告警规则
│   │
│   ├── auth/                          # ========== 认证鉴权 ==========
│   │   ├── tenant.go                 # 租户定义与管理
│   │   └── rbac.go                   # RBAC 鉴权
│   │
│   ├── secret/                        # ========== 密钥管理 ==========
│   │   └── provider.go               # SecretProvider 接口（env/KMS/Vault）
│   │
│   ├── audit/                         # ========== 审计日志 ==========
│   │   └── logger.go                 # 审计事件定义与输出
│   │
│   └── transport/                     # ========== 传输层 ==========
│       ├── http_client.go            # 统一 HTTP 客户端（per-provider proxy / 超时 / 连接池）
│       ├── auth.go                   # 上游 Provider 认证策略（Bearer/x-api-key/access_token/Dynamic）
│       └── sse.go                    # SSE 通用解析器 + SSE 写入器
│
├── api/                               # ========== 对外 API ==========
│   ├── handler/
│   │   ├── chat.go                   # POST /v1/chat/completions
│   │   ├── responses.go             # POST /v1/responses（Responses API）
│   │   ├── embedding.go             # POST /v1/embeddings（Sprint 6）
│   │   ├── image.go                 # POST /v1/images/generations（Sprint 7）
│   │   └── audio.go                 # POST /v1/audio/speech | /transcriptions（Sprint 7）
│   ├── middleware/
│   │   ├── auth.go                  # 租户认证 + RBAC 鉴权
│   │   ├── ratelimit.go
│   │   ├── logging.go
│   │   ├── tracing.go               # OpenTelemetry trace 注入
│   │   ├── sanitizer.go             # 请求/响应日志脱敏
│   │   └── audit.go                 # 审计日志记录
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
    Type     ContentType `json:"type"`               // text / image_url / audio / document
    Text     string      `json:"text,omitempty"`
    ImageURL *ImageURL   `json:"image_url,omitempty"`
    Audio    *Audio      `json:"audio,omitempty"`
    Document *Document   `json:"document,omitempty"`  // 文档附件
}

type ContentType string
const (
    ContentTypeText     ContentType = "text"
    ContentTypeImageURL ContentType = "image_url"
    ContentTypeAudio    ContentType = "audio"
    ContentTypeDocument ContentType = "document"
)

type ImageURL struct {
    URL    string      `json:"url"`              // URL 或 data:base64
    Detail ImageDetail `json:"detail,omitempty"` // low / high / auto
}

type ImageDetail string
const (
    ImageDetailLow  ImageDetail = "low"
    ImageDetailHigh ImageDetail = "high"
    ImageDetailAuto ImageDetail = "auto"
)
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

    // --- 推理模型参数 (o1, o3, gpt-5, etc.) ---
    ReasoningEffort string `json:"reasoning_effort,omitempty"` // "none", "minimal", "low", "medium", "high"

    // --- Tool Calling ---
    Tools      []Tool `json:"tools,omitempty"`
    ToolChoice any    `json:"tool_choice,omitempty"` // string 或 object

    // --- 路由控制 ---
    Provider  string    `json:"provider,omitempty"`   // 指定平台（覆盖路由）
    ModelTier ModelTier `json:"model_tier,omitempty"` // 模型分层（small/medium/large）

    // --- 可靠性 ---
    IdempotencyKey string `json:"idempotency_key,omitempty"` // 幂等键，防止重试导致重复写入

    // --- 动态凭证（BYOK，借鉴 TensorZero）---
    Credentials map[string]string `json:"credentials,omitempty"` // 请求级覆盖 provider API Key

    // --- 扩展 ---
    ResponseFormat *ResponseFormat `json:"response_format,omitempty"` // JSON mode
    Extra          map[string]any  `json:"extra,omitempty"`           // 平台专属字段
}

// Credentials 说明：
// - 支持 BYOK（Bring Your Own Key）场景
// - 静态配置（config/models.yaml）为默认，credentials 字段优先
// - 示例：{"api_key": "sk-xxx"} 覆盖 provider 的默认 API Key

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
type StreamEventType string

const (
    StreamEventContentDelta  StreamEventType = "content_delta"
    StreamEventToolCallDelta StreamEventType = "tool_call_delta"
    StreamEventUsage         StreamEventType = "usage"
    StreamEventDone          StreamEventType = "done"
    StreamEventError         StreamEventType = "error"
)

type StreamEvent struct {
    Type         StreamEventType `json:"type"`
    Delta        string          `json:"delta,omitempty"`
    ToolCall     *ToolCall       `json:"tool_call,omitempty"`
    Usage        *TokenUsage     `json:"usage,omitempty"`
    FinishReason string          `json:"finish_reason,omitempty"`
    Error        string          `json:"error,omitempty"`
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
    ErrAuthentication        ErrorCode = "authentication_error"
    ErrRateLimit             ErrorCode = "rate_limit_error"
    ErrInvalidRequest        ErrorCode = "invalid_request_error"
    ErrModelNotFound         ErrorCode = "model_not_found"
    ErrProviderNotFound      ErrorCode = "provider_not_found"       // NEW
    ErrProviderError         ErrorCode = "provider_error"
    ErrTimeout               ErrorCode = "timeout_error"
    ErrCapabilityUnavailable ErrorCode = "capability_unavailable"
    ErrQuotaExceeded         ErrorCode = "quota_exceeded"           // NEW
    ErrCircuitOpen           ErrorCode = "circuit_open"             // NEW
    ErrCooldown              ErrorCode = "cooldown"                 // NEW
    ErrInternalError         ErrorCode = "internal_error"           // NEW
)

// ErrorAction 四级错误分类（用于重试和降级决策）
type ErrorAction int
const (
    ActionRetry     ErrorAction = iota // 可重试（5xx, timeout）
    ActionRotateKey                    // 轮换 key 重试（401, 403, 429-key-level）
    ActionFallback                     // 切换 provider（模型不支持等）
    ActionAbort                        // 不可恢复（400 参数错误、余额不足）
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
    TaskStatusPending   TaskStatus = "pending"
    TaskStatusRunning   TaskStatus = "running"
    TaskStatusSucceeded TaskStatus = "succeeded"
    TaskStatusFailed    TaskStatus = "failed"
    TaskStatusCanceled  TaskStatus = "canceled"
)

type AsyncTask struct {
    ID        string            `json:"id"`
    Provider  string            `json:"provider"`
    Model     string            `json:"model"`
    Type      string            `json:"type"`       // image_gen / video_gen
    Status    TaskStatus        `json:"status"`
    Progress  int               `json:"progress"`   // 0-100
    ResultURL string            `json:"result_url,omitempty"`
    Error     string            `json:"error,omitempty"`
    CreatedAt int64             `json:"created_at"`
    UpdatedAt int64             `json:"updated_at"`
    ExpiresAt int64             `json:"expires_at,omitempty"`
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

// P0: Responses API（OpenAI 推理模型优化接口）
type ResponsesProvider interface {
    Provider
    Responses(ctx context.Context, req *ResponsesRequest) (*ResponsesResponse, error)
    ResponsesStream(ctx context.Context, req *ResponsesRequest) (<-chan ResponsesStreamEvent, error)
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
    // --- 接口级能力（与 Provider 接口一一对应，注册时自动检测） ---
    CapChat      Capability = "chat"
    CapResponses Capability = "responses"   // NEW: Responses API
    CapEmbed     Capability = "embedding"
    CapImageGen  Capability = "image_gen"
    CapVideoGen  Capability = "video_gen"
    CapTTS       Capability = "tts"
    CapSTT       Capability = "stt"
    CapAgent     Capability = "agent"
    CapWorkflow  Capability = "workflow"

    // --- 特性级能力（由 Provider.Supports() 声明，无独立接口） ---
    CapStream    Capability = "stream"
    CapTools     Capability = "tools"
    CapVision    Capability = "vision"
    CapJSONMode  Capability = "json_mode"
    CapReasoning Capability = "reasoning"
)

// capInterfaceMap 接口级能力 -> 对应的 interface 类型
// 注册时通过此表自动检测 provider 实现了哪些接口，写入能力分桶
var capInterfaceMap = map[Capability]reflect.Type{
    CapChat:      reflect.TypeFor[ChatProvider](),
    CapResponses: reflect.TypeFor[ResponsesProvider](),
    CapEmbed:     reflect.TypeFor[EmbeddingProvider](),
    CapImageGen:  reflect.TypeFor[ImageGenProvider](),
    CapVideoGen:  reflect.TypeFor[VideoGenProvider](),
    CapTTS:       reflect.TypeFor[TTSProvider](),
    CapSTT:       reflect.TypeFor[STTProvider](),
    CapAgent:     reflect.TypeFor[AgentProvider](),
    CapWorkflow:  reflect.TypeFor[WorkflowProvider](),
}
```

> **设计决策**：能力查询不再依赖运行时类型断言（`p.(ChatProvider)`），改为**注册期一次性检测 + 分桶存储**。Manager/Router 通过 Registry 的类型安全方法获取已确认能力的 provider，消除 panic 风险。

### 5.3 Provider Registry（能力分桶注册表）

```go
// pkg/provider/registry.go

type Registry struct {
    mu        sync.RWMutex
    providers map[string]Provider                    // name -> provider
    tierRoute map[ModelTier][]TierEntry              // tier -> 按优先级排序的 provider
    modelMap  map[string]string                      // "gpt-4o" -> "openai"

    // 能力分桶索引：注册时按接口类型自动分桶，查询时 O(1) 获取
    capIndex  map[Capability][]Provider              // capability -> 具备该能力的 provider 列表
}

type TierEntry struct {
    ProviderName string
    ModelID      string
    Priority     int
}

// Register 注册 provider 并自动检测能力、填充分桶索引
func (r *Registry) Register(p Provider) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    r.providers[p.Name()] = p

    // 1. 接口级能力：通过 reflect 检测 provider 实现了哪些接口
    providerType := reflect.TypeOf(p)
    for cap, ifaceType := range capInterfaceMap {
        if providerType.Implements(ifaceType) {
            r.capIndex[cap] = append(r.capIndex[cap], p)
        }
    }

    // 2. 特性级能力：通过 Supports() 声明
    for _, cap := range []Capability{CapStream, CapTools, CapVision, CapJSONMode, CapReasoning} {
        if p.Supports(cap) {
            r.capIndex[cap] = append(r.capIndex[cap], p)
        }
    }

    // 3. 一致性校验：Supports(CapChat)==true 但未实现 ChatProvider → 启动失败
    for cap, ifaceType := range capInterfaceMap {
        declared := p.Supports(cap)
        implemented := providerType.Implements(ifaceType)
        if declared != implemented {
            return fmt.Errorf("provider %q: capability %q mismatch: Supports()=%v but interface implemented=%v",
                p.Name(), cap, declared, implemented)
        }
    }

    return nil
}

// --- 类型安全的获取方法（消除调用方类型断言） ---

func (r *Registry) GetChatProvider(name string) (ChatProvider, bool) {
    p, ok := r.providers[name]
    if !ok { return nil, false }
    cp, ok := p.(ChatProvider) // 仅此处做断言，已有注册期保证
    return cp, ok
}

func (r *Registry) GetEmbeddingProvider(name string) (EmbeddingProvider, bool) { ... }
func (r *Registry) GetImageGenProvider(name string) (ImageGenProvider, bool) { ... }
// ... 其他能力接口同理

func (r *Registry) Get(name string) (Provider, bool) { ... }
func (r *Registry) GetByModel(modelID string) (Provider, bool) { ... }
func (r *Registry) GetForTier(tier ModelTier) []TierEntry { ... }
func (r *Registry) ListCapable(cap Capability) []Provider { ... }
```

> **启动期强校验**：`Register()` 会对每个 provider 做接口实现与 `Supports()` 声明的一致性检查，不一致直接返回 error 阻止启动。这确保了运行时能力查询 100% 可靠。

---

## 6. 适配器实现策略

### 6.1 OpenAI — 原生适配（基准实现）

```go
// pkg/adapter/openai/provider.go

type OpenAI struct {
    client        *transport.HTTPClient    // 使用统一 HTTP 客户端（支持 per-provider proxy）
    auth          transport.AuthStrategy   // BearerAuth（始终使用 Bearer，不随 name 变化）
    baseURL       string                   // 默认 "https://api.openai.com/v1"
    chatPath      string                   // 可配置 chat endpoint 路径（默认 "/chat/completions"）
    responsesPath string                   // 可配置 responses endpoint 路径（默认 "/responses"）
    modelsPath    string                   // 可配置 models endpoint 路径（默认 "/models"）
    name          string                   // provider 名称（"openai" 或自定义名，用于复用）
    models        []types.ModelConfig
}

// 实现的能力接口
var _ provider.ChatProvider      = (*OpenAI)(nil)
var _ provider.ResponsesProvider = (*OpenAI)(nil)  // NEW: Responses API
// 以下待后续 Sprint 实现：
// var _ provider.EmbeddingProvider = (*OpenAI)(nil)
// var _ provider.ImageGenProvider  = (*OpenAI)(nil)
// var _ provider.TTSProvider       = (*OpenAI)(nil)
// var _ provider.STTProvider       = (*OpenAI)(nil)

// New 创建 OpenAI provider
func New(cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAI, error) {
    return NewWithName("openai", cfg, models)
}

// NewWithName 创建 OpenAI 兼容 provider，自定义名称
// 国内平台（阿里百炼、火山引擎、智谱等）复用此构造函数，只需传不同的 name + baseURL
func NewWithName(name string, cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAI, error) {
    chatPath := cfg.GetExtra("chat_path", "/chat/completions")
    responsesPath := cfg.GetExtra("responses_path", "/responses")
    modelsPath := cfg.GetExtra("models_path", "/models")

    // 始终使用 BearerAuth（不因 name 不同而切换认证方式）
    auth := &transport.BearerAuth{APIKey: cfg.APIKey}

    return &OpenAI{
        client:        transport.NewHTTPClientWithProxy(cfg.Proxy),
        auth:          auth,
        baseURL:       cfg.BaseURL,
        chatPath:      chatPath,
        responsesPath: responsesPath,
        modelsPath:    modelsPath,
        name:          name,
        models:        models,
    }, nil
}

// Ping 通过 GET /models 验证连通性
func (p *OpenAI) Ping(ctx context.Context) error { ... }

// getAuth 支持 BYOK 动态凭证
func (p *OpenAI) getAuth(credentials map[string]string) transport.AuthStrategy {
    if len(credentials) > 0 {
        return transport.WithDynamicCredentials(p.auth, credentials)
    }
    return p.auth
}
```

**实现要点**：

- 请求格式即为内部标准，**近乎直通**（仅需处理 `max_tokens` vs `max_completion_tokens` 选择、`Extra` 字段合并）
- `Extra map[string]any` 通过自定义 `MarshalJSON()` 合并到 JSON 顶层（支持 DashScope `enable_thinking` 等平台专属字段）
- Reasoning models（o1/o3/o4/gpt-5/gpt-4.1+）自动使用 `max_completion_tokens` 替代 `max_tokens`
- DashScope qwen3+ 非流式请求自动注入 `enable_thinking=false`（仅当 `p.name == "alibaba"` 时）
- Endpoint 路径（`chat_path`、`responses_path`、`models_path`）通过 `config.yaml` 的 `extra` 字段可配置

### 6.2 Anthropic — 原生适配（重点处理差异）

> **实现决策**：Mapper 不独立为 `pkg/mapper/` 包，而是内联在各 adapter 中（`chat.go` 内的 `buildRequest()` / `buildResponse()` / `extractSystemAndConvert()`）。原因：每个平台的映射逻辑与其私有 API 类型强耦合，独立包会增加不必要的跨包引用。

```go
// pkg/adapter/anthropic/provider.go

type Anthropic struct {
    client       *transport.HTTPClient     // 使用统一 HTTP 客户端（支持 per-provider proxy）
    auth         transport.AuthStrategy    // AnthropicAuth（x-api-key + anthropic-version）
    baseURL      string                    // 默认 "https://api.anthropic.com"
    messagesPath string                    // 可配置 endpoint 路径（默认 "/v1/messages"）
    maxTokens    int                       // 可配置默认 max_tokens（默认 4096）
    models       []types.ModelConfig
}

// 可配置的 extra 字段：
//   anthropic_version:  "2023-06-01"     # API version header
//   default_max_tokens: 4096             # default max_tokens（Anthropic 必填）
//   messages_path:      "/v1/messages"   # endpoint path（代理场景可改）
//   api_format:         "openai"         # 设为 "openai" 时走 OpenAI 兼容协议（如 OneAPI 代理）

// Ping 通过发送 minimal 请求验证连通性
func (p *Anthropic) Ping(ctx context.Context) error { ... }
```

```go
// pkg/adapter/anthropic/chat.go — 请求/响应映射（内联在 adapter 中）

// buildRequest 将统一 ChatRequest 转换为 Anthropic 格式
func (p *Anthropic) buildRequest(req *types.ChatRequest) *anthropicRequest {
    system, messages := extractSystemAndConvert(req.Messages)
    ar := &anthropicRequest{
        Model:    req.Model,
        Messages: messages,
        System:   system,
    }

    // max_tokens 必填（Anthropic 要求）
    if req.MaxTokens != nil {
        ar.MaxTokens = *req.MaxTokens
    } else {
        ar.MaxTokens = p.maxTokens  // 使用可配置默认值
    }

    // stop → stop_sequences
    ar.StopSequences = req.Stop

    // tool_choice 格式转换: string → object
    //   "auto" → {"type":"auto"}
    //   "required" → {"type":"any"}
    //   {"function":{"name":"xxx"}} → {"type":"tool","name":"xxx"}
    ar.ToolChoice = convertToolChoice(req.ToolChoice)

    // tools 格式转换: parameters → input_schema
    ar.Tools = convertTools(req.Tools)

    // 传递 thinking 等 Anthropic 独有参数
    if v, ok := req.Extra["thinking"]; ok {
        ar.Thinking = v
    }

    return ar
}

// buildResponse 将 Anthropic 响应转换为统一格式
func (p *Anthropic) buildResponse(resp *anthropicResponse) *types.ChatResponse {
    return &types.ChatResponse{
        ID:           resp.ID,
        Model:        resp.Model,
        Content:      extractText(resp.Content),
        FinishReason: mapStopReason(resp.StopReason),  // end_turn→stop, tool_use→tool_calls
        ToolCalls:    extractToolCalls(resp.Content),   // tool_use blocks → []ToolCall
        Usage: types.TokenUsage{
            PromptTokens:     resp.Usage.InputTokens,   // 字段名映射
            CompletionTokens: resp.Usage.OutputTokens,
            TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
        },
    }
}

// extractSystemAndConvert 抽取 system 消息为顶层字段，转换其余消息
// 特殊处理：tool role → user + tool_result block，assistant + tool_calls → content blocks
func extractSystemAndConvert(messages []types.Message) (string, []anthropicMessage) { ... }
```

**关键映射点**:
- `system` 在 messages 中 → 顶层字段
- `max_tokens` 可选 → 必填（默认 4096，可通过 extra 配置）
- `stop` → `stop_sequences`
- `finish_reason` ← `stop_reason`（`end_turn`→`stop`, `tool_use`→`tool_calls`）
- `prompt_tokens/completion_tokens` ← `input_tokens/output_tokens`
- `tool_choice: "auto"` → `tool_choice: {"type":"auto"}`
- `tool_choice: "required"` → `tool_choice: {"type":"any"}`
- 流式 SSE: `choices[0].delta.content` ← `content_block_delta.delta.text`
- `tool` role → `user` role + `tool_result` content block
- multimodal `image_url` → `image` type + `source` object

> **`api_format: "openai"` 支持**：Anthropic provider 配置中如果 `extra.api_format == "openai"`，
> Manager 在初始化时会使用 `openai.NewWithName("anthropic", cfg, models)` 代替原生 Anthropic adapter，
> 走 OpenAI 兼容协议（适用于 OneAPI 等代理场景）。

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

> **实现决策**：不创建独立的 `pkg/adapter/compatible/` 包，而是**直接复用 OpenAI adapter + `NewWithName()`**。
> 原因：国内平台（阿里百炼、火山引擎、智谱、DeepSeek 等）均实现了 OpenAI 兼容协议，
> Chat/Stream 逻辑与 OpenAI 完全一致（仅 `baseURL` 不同），无需任何额外适配。

```go
// 在 Manager 初始化时，根据 config.yaml 中的 provider 名称创建对应实例：
//
// providers:
//   alibaba:
//     base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
//     api_key: "${DASHSCOPE_API_KEY}"
//     proxy: "none"
//
// 代码中直接调用：
//   openai.NewWithName("alibaba", cfg, models)
//
// 等价于创建一个名为 "alibaba" 的 OpenAI 适配器，baseURL 指向百炼。
// 火山引擎、智谱、DeepSeek 等同理。

// 平台特殊行为通过以下方式处理：
// 1. Extra 字段透传：req.Extra 中的平台专属字段通过 MarshalJSON 合并到 JSON 顶层
// 2. Provider 名称判断：如 DashScope qwen3+ 非流式需要 enable_thinking=false
//    仅当 p.name == "alibaba" 时自动注入
// 3. 可配置 endpoint 路径：通过 extra.chat_path / extra.responses_path 自定义
```

图像/视频/Agent 等非 Chat 能力待后续 Sprint 实现，届时如果需要平台专属逻辑，
可以在 OpenAI adapter 中通过 `p.name` 判断分支处理，或在必要时拆出独立 adapter。

---

## 7. 编排层设计（Manager）

### 7.1 Manager 核心

```go
// pkg/manager/manager.go

type Manager struct {
    registry        *Registry
    router          *Router
    cache           *DualCache                  // 升级为 DualCache（Memory + Redis）
    circuitBreakers map[string]*CircuitBreaker
    cooldowns       *CooldownManager            // Per-Model 冷却（NEW）
    rateLimiters    map[string]*RateLimiter
    tokenCounter    *TokenCounter
    costCalculator  *CostCalculator             // 费用估算器（NEW）
    quotaManager    *QuotaManager               // 预扣+结算配额（NEW）
    spendWriter     *SpendWriter                // 异步消费批量写入（NEW）
    hookRegistry    *hook.Registry              // Hook 调度器（NEW）
    retrier         *Retrier                    // 重试执行器（NEW）
    metrics         *Metrics
    config          *Config
}

// CostCalculator 根据模型和 token 数估算费用
type CostCalculator struct {
    pricing map[string]ModelPricing  // model -> pricing
}

type ModelPricing struct {
    InputPer1K  float64  // 输入每千 token 费用 (USD)
    OutputPer1K float64  // 输出每千 token 费用 (USD)
}

func (c *CostCalculator) Estimate(model string, estimatedTokens int) float64 {
    p, ok := c.pricing[model]
    if !ok {
        return 0  // 未知模型，返回 0 不限制
    }
    // 保守估算：假设 50% 输入 50% 输出
    return float64(estimatedTokens) / 1000 * (p.InputPer1K + p.OutputPer1K) / 2
}

func (c *CostCalculator) Calculate(model string, inputTokens, outputTokens int) float64 {
    p, ok := c.pricing[model]
    if !ok {
        return 0
    }
    return float64(inputTokens)/1000*p.InputPer1K + float64(outputTokens)/1000*p.OutputPer1K
}

// Chat 统一对话入口（完整执行链）
func (m *Manager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // ========== Phase 1: PreRoute Hook ==========
    event := &hook.HookEvent{Request: req, Phase: hook.PhasePreRoute}
    if err := m.hookRegistry.Dispatch(ctx, hook.PhasePreRoute, event); err != nil {
        return nil, err  // PreRoute 可拦截请求
    }

    // ========== Phase 2: 路由选择 ==========
    cp, model, err := m.router.SelectChat(req)
    if err != nil {
        return nil, err
    }
    event.Provider = cp.Name()
    event.Model = model

    // PostRoute Hook（非阻塞）
    m.hookRegistry.Dispatch(ctx, hook.PhasePostRoute, event)

    // ========== Phase 3: 限流检查 ==========
    if err := m.rateLimiters[cp.Name()].Allow(); err != nil {
        return nil, err
    }

    // ========== Phase 4: 缓存查询 ==========
    if !req.Stream && len(req.Tools) == 0 {
        if cached := m.cache.Get(req); cached != nil {
            return cached, nil
        }
    }

    // ========== Phase 5: 配额预扣 ==========
    estimatedTokens := m.tokenCounter.Estimate(req)
    estimatedCost := m.costCalculator.Estimate(model, estimatedTokens)
    quotaID, err := m.quotaManager.PreConsume(ctx, req.TenantID, estimatedTokens, estimatedCost)
    if err != nil {
        return nil, err  // 配额不足
    }

    // ========== Phase 6: Token headroom 计算 ==========
    req.MaxTokens = m.tokenCounter.ClampMaxTokens(req, model)

    // ========== Phase 7: 冷却检查 ==========
    keyHash := hashKey(req.Credentials["api_key"])
    if !m.cooldowns.IsAvailable(cp.Name(), keyHash, model) {
        // 当前 key+model 处于冷却，尝试路由到其他 provider
        cp, model, err = m.router.SelectChatExcluding(req, cp.Name())
        if err != nil {
            m.quotaManager.Rollback(ctx, quotaID)
            return nil, fmt.Errorf("all providers in cooldown: %w", err)
        }
    }

    // ========== Phase 8: PreCall Hook ==========
    event.Phase = hook.PhasePreCall
    if err := m.hookRegistry.Dispatch(ctx, hook.PhasePreCall, event); err != nil {
        m.quotaManager.Rollback(ctx, quotaID)
        return nil, err  // PreCall 可拦截请求
    }

    // ========== Phase 9: 调用 provider（带熔断器 + 重试 + Deadline） ==========
    var resp *ChatResponse
    err = m.circuitBreakers[cp.Name()].Execute(func() error {
        var callErr error
        resp, callErr = m.retrier.ExecuteWithDeadline(ctx, func() (*ChatResponse, error) {
            return cp.Chat(ctx, req)
        })
        return callErr
    })

    // ========== Phase 10: 错误处理 + 冷却记录 ==========
    if err != nil {
        m.cooldowns.RecordFailure(cp.Name(), keyHash, model)
        m.quotaManager.Rollback(ctx, quotaID)

        // OnError Hook（非阻塞）
        event.Error = err
        m.hookRegistry.Dispatch(ctx, hook.PhaseOnError, event)

        // 尝试 fallback
        if isTransient(err) {
            resp, err = m.fallbackChat(ctx, req, cp.Name())
        }
        if err != nil {
            return nil, err
        }
    } else {
        m.cooldowns.RecordSuccess(cp.Name(), keyHash, model)
    }

    // ========== Phase 11: 计算实际费用 ==========
    actualCost := m.costCalculator.Calculate(model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

    // ========== Phase 12: 配额结算（token + cost 原子性） ==========
    m.quotaManager.Settle(ctx, quotaID, resp.Usage.TotalTokens, actualCost)

    // ========== Phase 13: 消费记录（异步） ==========
    m.spendWriter.Record(SpendUpdate{
        TenantID:    req.TenantID,
        Model:       model,
        Provider:    cp.Name(),
        InputTokens: resp.Usage.PromptTokens,
        OutputTokens: resp.Usage.CompletionTokens,
        Cost:        actualCost,
    })

    // ========== Phase 14: 缓存写入 ==========
    if m.cache.IsSafeToCache(resp) {
        m.cache.Set(req, resp)
    }

    // ========== Phase 15: OnSuccess Hook ==========
    event.Response = resp
    m.hookRegistry.Dispatch(ctx, hook.PhaseOnSuccess, event)

    // ========== Phase 16: 指标上报 ==========
    m.metrics.RecordRequest(cp.Name(), model, resp, nil)

    return resp, nil
}

// ChatStream 统一流式入口
func (m *Manager) ChatStream(ctx context.Context, req *ChatRequest) (<-chan StreamEvent, error) {
    cp, model, err := m.router.SelectChat(req)
    if err != nil { return nil, err }

    req.MaxTokens = m.tokenCounter.ClampMaxTokens(req, model)

    ch, err := cp.ChatStream(ctx, req)
    if err != nil && isTransient(err) {
        // 流式降级
        ch, err = m.fallbackStream(ctx, req, cp.Name())
    }

    // 包装 channel 添加指标收集
    return m.wrapStreamMetrics(ch, cp.Name(), model), err
}
```

### 7.2 路由器

```go
// pkg/manager/router.go

type Router struct {
    registry *Registry
    config   *RouterConfig
}

// SelectChat 选择 ChatProvider 和模型（类型安全，无需调用方断言）
func (r *Router) SelectChat(req *ChatRequest) (ChatProvider, string, error) {
    // 优先级 1: 显式指定 provider
    if req.Provider != "" {
        cp, ok := r.registry.GetChatProvider(req.Provider)
        if !ok { return nil, "", ErrProviderNotFound }
        return cp, req.Model, nil
    }

    // 优先级 2: 显式指定 model -> 反查 provider
    if req.Model != "" {
        providerName, ok := r.registry.GetProviderByModel(req.Model)
        if !ok { return nil, "", ErrModelNotFound }
        cp, ok := r.registry.GetChatProvider(providerName)
        if !ok { return nil, "", ErrCapabilityUnavailable }
        return cp, req.Model, nil
    }

    // 优先级 3: 按 ModelTier 路由（带熔断检查）
    tier := req.ModelTier
    if tier == "" { tier = TierMedium } // 默认中档

    entries := r.registry.GetForTier(tier)
    for _, entry := range entries {
        if r.isHealthy(entry.ProviderName) {
            cp, ok := r.registry.GetChatProvider(entry.ProviderName)
            if !ok { continue } // 该 provider 不支持 Chat，跳过
            return cp, entry.ModelID, nil
        }
    }

    return nil, "", ErrNoAvailableProvider
}

// SelectEmbed / SelectImageGen / ... 其他能力同理，各返回对应类型安全的接口
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

### 7.4 Per-Model 冷却（借鉴 LLM-API-Key-Proxy）

```go
// pkg/manager/cooldown.go

// CooldownManager 细粒度冷却管理
// 与 CircuitBreaker（provider 粒度）互补，不替换
// 冷却键：provider + apiKey + model（同一 key 不同 model 独立冷却）
type CooldownManager struct {
    cooldowns sync.Map // key: "provider:keyHash:model" → *CooldownEntry
}

type CooldownEntry struct {
    mu           sync.Mutex
    failureCount int
    cooldownUntil time.Time
    backoffLevel int // 当前退避级别（0-4）
}

// 退避序列：10s → 30s → 60s → 120s → 300s
var backoffDurations = []time.Duration{
    10 * time.Second,
    30 * time.Second,
    60 * time.Second,
    120 * time.Second,
    300 * time.Second,
}

func (c *CooldownManager) IsCoolingDown(provider, keyHash, model string) bool {
    key := fmt.Sprintf("%s:%s:%s", provider, keyHash, model)
    if entry, ok := c.cooldowns.Load(key); ok {
        e := entry.(*CooldownEntry)
        return time.Now().Before(e.cooldownUntil)
    }
    return false
}

func (c *CooldownManager) RecordFailure(provider, keyHash, model string) {
    key := fmt.Sprintf("%s:%s:%s", provider, keyHash, model)
    entry, _ := c.cooldowns.LoadOrStore(key, &CooldownEntry{})
    e := entry.(*CooldownEntry)
    e.mu.Lock()
    defer e.mu.Unlock()

    e.failureCount++
    if e.backoffLevel < len(backoffDurations)-1 {
        e.backoffLevel++
    }
    e.cooldownUntil = time.Now().Add(backoffDurations[e.backoffLevel])
}

func (c *CooldownManager) RecordSuccess(provider, keyHash, model string) {
    key := fmt.Sprintf("%s:%s:%s", provider, keyHash, model)
    c.cooldowns.Delete(key) // 成功后清除冷却状态
}
```

> **冷却 vs 熔断**：熔断器按 provider 粒度，适合整体故障；冷却器按 provider+key+model 粒度，适合单个 Key 限流或特定模型不可用的场景。两者互补使用。

### 7.5 重试策略矩阵

```go
// pkg/manager/retry.go

// RetryPolicy 按错误类型和请求特征决定重试行为
type RetryPolicy struct {
    MaxAttempts   int           `yaml:"max_attempts"`    // 最大重试次数（含首次）
    InitialDelay  time.Duration `yaml:"initial_delay"`   // 初始退避间隔
    MaxDelay      time.Duration `yaml:"max_delay"`       // 最大退避间隔
    BackoffFactor float64       `yaml:"backoff_factor"`  // 退避倍数
    RetryBudget   float64       `yaml:"retry_budget"`    // 重试预算：重试请求占总请求的比例上限（如 0.1 = 10%）
    Deadline      time.Duration `yaml:"deadline"`        // 全局时间预算，所有重试共享（借鉴 LLM-API-Key-Proxy）
}

// ErrorAction 四级错误分类（借鉴 LLM-API-Key-Proxy 错误分类树）
type ErrorAction int
const (
    ActionRetry     ErrorAction = iota // 可重试（5xx, timeout）
    ActionRotateKey                    // 轮换 key 重试（401, 403, 429-key-level）
    ActionFallback                     // 切换 provider（模型不支持等）
    ActionAbort                        // 不可恢复（400 参数错误、余额不足）
)

// RetryDecision 根据错误类型决定处理方式
type RetryDecision struct {
    Action  ErrorAction
    Delay   time.Duration
    Reason  string
}

// ClassifyForRetry 错误分类器（增强版）
func ClassifyForRetry(err error) RetryDecision {
    pe, ok := err.(*ProviderError)
    if !ok {
        return RetryDecision{Action: ActionAbort, Reason: "unknown error type"}
    }

    switch pe.StatusCode {
    case 429: // Rate Limit
        // 区分 key 级限流和全局限流
        if isKeyLevelRateLimit(pe) {
            return RetryDecision{Action: ActionRotateKey, Delay: parseRetryAfter(pe), Reason: "key_rate_limited"}
        }
        return RetryDecision{Action: ActionRetry, Delay: parseRetryAfter(pe), Reason: "rate_limited"}
    case 500, 502, 503: // Server Error
        return RetryDecision{Action: ActionRetry, Delay: 1 * time.Second, Reason: "server_error"}
    case 408, 504: // Timeout
        return RetryDecision{Action: ActionRetry, Delay: 500 * time.Millisecond, Reason: "timeout"}
    case 401: // Unauthorized — 轮换 key
        return RetryDecision{Action: ActionRotateKey, Reason: "unauthorized"}
    case 403: // Forbidden — 可能是 key 问题或模型权限
        if isModelPermissionError(pe) {
            return RetryDecision{Action: ActionFallback, Reason: "model_permission"}
        }
        return RetryDecision{Action: ActionRotateKey, Reason: "forbidden"}
    case 400, 422: // Client Error — 不可恢复
        return RetryDecision{Action: ActionAbort, Reason: "client_error"}
    case 404: // Model Not Found — 降级到其他 provider
        return RetryDecision{Action: ActionFallback, Reason: "model_not_found"}
    default:
        if pe.Retryable {
            return RetryDecision{Action: ActionRetry, Reason: "default_retryable"}
        }
        return RetryDecision{Action: ActionAbort, Reason: "default"}
    }
}

// RetryBudgetTracker 滑动窗口重试预算跟踪器
// 当一段时间内重试比例超过阈值时停止重试，防止雪崩
type RetryBudgetTracker struct {
    mu          sync.Mutex
    window      time.Duration   // 滑动窗口长度，默认 60s
    budget      float64         // 重试比例上限，如 0.1 = 10%
    totalCount  int64           // 窗口内总请求数
    retryCount  int64           // 窗口内重试请求数
    lastReset   time.Time       // 上次重置时间
}

func NewRetryBudgetTracker(budget float64, window time.Duration) *RetryBudgetTracker {
    return &RetryBudgetTracker{
        window:    window,
        budget:    budget,
        lastReset: time.Now(),
    }
}

func (t *RetryBudgetTracker) RecordRequest() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.maybeReset()
    t.totalCount++
}

func (t *RetryBudgetTracker) AllowRetry() bool {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.maybeReset()
    if t.totalCount == 0 {
        return true
    }
    return float64(t.retryCount)/float64(t.totalCount) < t.budget
}

func (t *RetryBudgetTracker) RecordRetry() {
    t.mu.Lock()
    defer t.mu.Unlock()
    t.retryCount++
}

func (t *RetryBudgetTracker) maybeReset() {
    if time.Since(t.lastReset) > t.window {
        t.totalCount = 0
        t.retryCount = 0
        t.lastReset = time.Now()
    }
}

// Retrier 重试执行器
type Retrier struct {
    policy        RetryPolicy
    budgetTracker *RetryBudgetTracker
    onRotateKey   func()
}

// ExecuteWithDeadline Deadline 驱动的重试执行器
func (r *Retrier) ExecuteWithDeadline(ctx context.Context, fn func() (*ChatResponse, error)) (*ChatResponse, error) {
    // 记录请求到预算跟踪器
    r.budgetTracker.RecordRequest()

    // 设置全局 deadline
    if r.policy.Deadline > 0 {
        var cancel context.CancelFunc
        ctx, cancel = context.WithTimeout(ctx, r.policy.Deadline)
        defer cancel()
    }

    for attempt := 0; attempt < r.policy.MaxAttempts; attempt++ {
        // 检查剩余时间是否足够
        if deadline, ok := ctx.Deadline(); ok {
            remaining := time.Until(deadline)
            if remaining < r.policy.InitialDelay {
                return nil, fmt.Errorf("deadline exceeded: remaining %v < min delay", remaining)
            }
        }

        // 重试前检查重试预算（首次请求跳过检查）
        if attempt > 0 && !r.budgetTracker.AllowRetry() {
            return nil, fmt.Errorf("retry budget exhausted: retry ratio exceeded %.0f%%", r.policy.RetryBudget*100)
        }

        resp, err := fn()
        if err == nil {
            return resp, nil
        }

        decision := ClassifyForRetry(err)
        switch decision.Action {
        case ActionAbort:
            return nil, err
        case ActionRotateKey:
            // 通知 Manager 轮换 key 后继续
            r.onRotateKey()
        case ActionFallback:
            return nil, &FallbackError{Original: err}
        case ActionRetry:
            // 继续下一次重试
        }

        // 记录本次重试到预算跟踪器
        r.budgetTracker.RecordRetry()

        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(decision.Delay):
        }
    }
    return nil, ErrMaxRetriesExceeded
}
```

重试策略配置：

```yaml
# config/config.yaml
manager:
  retry:
    max_attempts: 3
    initial_delay: "500ms"
    max_delay: "10s"
    backoff_factor: 2.0
    retry_budget: 0.1          # 重试请求不超过总请求的 10%，防止雪崩
    deadline: "60s"            # 全局时间预算（借鉴 LLM-API-Key-Proxy Deadline-driven）
```

> **重试预算**：引入 `retry_budget` 滑动窗口计数，当一段时间内重试比例超过阈值时停止重试，防止下游故障时大量重试加剧雪崩。

> **Deadline 驱动**：全局时间预算贯穿整个请求生命周期，所有重试/等待共享 deadline。每次重试前检查 `ctx.Deadline()`，剩余时间不足则提前终止，避免超时后仍在重试的资源浪费。

### 7.6 超时分级策略

```go
// pkg/manager/timeout.go

// TimeoutConfig 分级超时，区分不同阶段的超时容忍度
type TimeoutConfig struct {
    Connect       time.Duration `yaml:"connect"`        // TCP 连接超时，默认 5s
    FirstToken    time.Duration `yaml:"first_token"`    // 首个 token 超时（TTFT），默认 30s
    TotalNonStream time.Duration `yaml:"total_non_stream"` // 非流式总超时，默认 120s
    TotalStream   time.Duration `yaml:"total_stream"`   // 流式总超时，默认 300s
    IdleBetweenChunks time.Duration `yaml:"idle_between_chunks"` // 流式两个 chunk 之间最大间隔，默认 30s
}

// 按 ModelTier 差异化超时
var TierTimeoutDefaults = map[ModelTier]TimeoutConfig{
    TierSmall:  {Connect: 5*time.Second, FirstToken: 15*time.Second, TotalNonStream: 60*time.Second},
    TierMedium: {Connect: 5*time.Second, FirstToken: 30*time.Second, TotalNonStream: 120*time.Second},
    TierLarge:  {Connect: 5*time.Second, FirstToken: 60*time.Second, TotalNonStream: 180*time.Second},
}
```

### 7.7 流式中途失败策略

```go
// pkg/manager/stream_failover.go

// StreamFailoverConfig 流式传输中途失败的处理策略
type StreamFailoverConfig struct {
    Enabled         bool   `yaml:"enabled"`
    Strategy        string `yaml:"strategy"`          // abort / fallback_non_stream / switch_provider
    MaxRetainTokens int    `yaml:"max_retain_tokens"` // 记录已接收内容的最大 token 数（用于断点恢复）
}

// StreamWatcher 包装流式 channel，监控并处理中途失败
type StreamWatcher struct {
    source    <-chan StreamEvent
    collected []StreamEvent     // 已接收的事件（用于故障恢复时拼接）
    config    StreamFailoverConfig
    onFailure func(collected []StreamEvent, err error) (<-chan StreamEvent, error)
}

// Watch 包装原始流，监听错误事件
func (w *StreamWatcher) Watch() <-chan StreamEvent {
    out := make(chan StreamEvent)
    go func() {
        defer close(out)
        for event := range w.source {
            w.collected = append(w.collected, event)
            if event.Type == "error" {
                // 根据策略决定：
                // 1. abort: 直接发送 error 事件，关闭
                // 2. fallback_non_stream: 切换到非流式请求，用已收集内容拼接
                // 3. switch_provider: 向备选 provider 发起新的流式请求
                recovered, err := w.onFailure(w.collected, fmt.Errorf(event.Error))
                if err != nil {
                    out <- event // 恢复失败，透传错误
                    return
                }
                // 恢复成功，继续转发
                for ev := range recovered {
                    out <- ev
                }
                return
            }
            out <- event
        }
    }()
    return out
}
```

配置示例：

```yaml
manager:
  stream_failover:
    enabled: true
    strategy: "switch_provider"   # abort / fallback_non_stream / switch_provider
    max_retain_tokens: 4096       # 保留已收内容用于恢复

  timeout:
    connect: "5s"
    first_token: "30s"
    total_non_stream: "120s"
    total_stream: "300s"
    idle_between_chunks: "30s"
```

### 7.8 幂等性保证

```go
// pkg/manager/idempotency.go

// IdempotencyStore 幂等键存储
type IdempotencyStore interface {
    // GetOrSet 原子性地检查 key 是否存在：
    // - 存在：返回已缓存的响应
    // - 不存在：标记为 in-flight，返回 nil
    GetOrSet(ctx context.Context, key string, ttl time.Duration) (*ChatResponse, error)
    // Complete 请求完成后写入结果
    Complete(ctx context.Context, key string, resp *ChatResponse) error
    // Fail 请求失败后清除 in-flight 标记
    Fail(ctx context.Context, key string) error
}

// 实现：内存（单机） / Redis（分布式）
type MemoryIdempotencyStore struct{ ... }
type RedisIdempotencyStore struct{ ... }
```

> **幂等键使用场景**：客户端设置 `idempotency_key` 后，Gateway 保证同一 key 在 TTL 内只产生一次实际调用。适用于 tool calling 等有副作用的请求，避免重试导致重复执行。

### 7.9 对冲请求（借鉴 Shannon，Go 天然优势）

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

### 7.10 DualCache（借鉴 LiteLLM）

```go
// pkg/manager/cache.go — 升级为 DualCache

// DualCache 双层缓存架构
type DualCache struct {
    memory *MemoryCache  // LRU, TTL=5min, 低延迟
    redis  *RedisCache   // TTL=1h, 跨实例共享（可选）
}

type MemoryCache struct {
    lru *lru.Cache
    ttl time.Duration
}

type RedisCache struct {
    client *redis.Client
    ttl    time.Duration
}

// Get 读取顺序：memory → redis（命中后回填 memory）
func (c *DualCache) Get(req *ChatRequest) *ChatResponse {
    key := c.buildKey(req)

    // 1. 先查内存缓存
    if resp := c.memory.Get(key); resp != nil {
        return resp
    }

    // 2. 再查 Redis（如果配置了）
    if c.redis != nil {
        if resp := c.redis.Get(key); resp != nil {
            // 回填到内存缓存
            c.memory.Set(key, resp)
            return resp
        }
    }

    return nil
}

// Set 写入：同时写 memory + redis
func (c *DualCache) Set(req *ChatRequest, resp *ChatResponse) {
    if !c.IsSafeToCache(resp) {
        return
    }

    key := c.buildKey(req)
    c.memory.Set(key, resp)
    if c.redis != nil {
        c.redis.Set(key, resp)
    }
}

func (c *DualCache) IsSafeToCache(resp *ChatResponse) bool {
    // 不缓存截断的响应
    if resp.FinishReason == "length" { return false }
    // 不缓存被内容过滤的响应
    if resp.FinishReason == "content_filter" { return false }
    // 不缓存空响应
    if resp.Content == "" && len(resp.ToolCalls) == 0 { return false }
    return true
}
```

缓存配置示例：

```yaml
manager:
  cache:
    enabled: true
    memory:
      max_size: 1000       # LRU 容量
      ttl: "5m"            # 内存缓存 TTL
    redis:
      enabled: true        # 可选，不配置则仅用内存
      url: "${REDIS_URL}"
      ttl: "1h"            # Redis 缓存 TTL
```

### 7.11 Hook 系统（借鉴 LiteLLM CustomLogger）

```go
// pkg/hook/hook.go

// Phase 请求生命周期阶段
type Phase string
const (
    PhasePreRoute   Phase = "pre_route"    // 路由前
    PhasePostRoute  Phase = "post_route"   // 路由后、调用前
    PhasePreCall    Phase = "pre_call"     // Provider 调用前
    PhasePostCall   Phase = "post_call"    // Provider 调用后（成功或失败）
    PhaseOnError    Phase = "on_error"     // 错误发生时
    PhaseOnSuccess  Phase = "on_success"   // 成功完成时
)

// Hook 回调接口
type Hook interface {
    Name() string
    Phase() Phase
    Execute(ctx context.Context, event *HookEvent) error
}

// HookEvent 回调事件数据
type HookEvent struct {
    Request    *types.ChatRequest
    Response   *types.ChatResponse  // post_call/on_success 阶段可用
    Error      error                // on_error 阶段可用
    Provider   string
    Model      string
    Latency    time.Duration
    TokensUsed int
    CostUSD    float64
    Metadata   map[string]any       // 自定义元数据
}

// pkg/hook/registry.go

// Registry Hook 注册与调度
type Registry struct {
    mu    sync.RWMutex
    hooks map[Phase][]Hook
}

func (r *Registry) Register(h Hook) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.hooks[h.Phase()] = append(r.hooks[h.Phase()], h)
}

// Dispatch 调度指定阶段的 Hook
// 阻塞语义：PreRoute / PreCall 阶段的 Hook 如果返回 error，中止请求（拦截器语义）
// 非阻塞语义：PostCall / OnSuccess / OnError 等后置阶段的 Hook 如果返回 error，仅记录日志不影响主流程
func (r *Registry) Dispatch(ctx context.Context, phase Phase, event *HookEvent) error {
    r.mu.RLock()
    hooks := r.hooks[phase]
    r.mu.RUnlock()

    blocking := phase == PhasePreRoute || phase == PhasePreCall

    for _, h := range hooks {
        if err := h.Execute(ctx, event); err != nil {
            if blocking {
                // 前置 Hook 返回 error → 中止请求（拦截器语义）
                return fmt.Errorf("hook %q blocked request: %w", h.Name(), err)
            }
            // 后置 Hook 返回 error → 仅记录日志，不影响主流程
            slog.Warn("hook execution failed", "hook", h.Name(), "phase", phase, "error", err)
        }
    }
    return nil
}
```

内置 Hook 示例：

```go
// 审计日志 Hook
type AuditHook struct{}
func (h *AuditHook) Name() string { return "audit" }
func (h *AuditHook) Phase() Phase { return PhaseOnSuccess }
func (h *AuditHook) Execute(ctx context.Context, event *HookEvent) error {
    // 写入审计日志
    return auditLogger.Log(ctx, event)
}

// Prometheus 指标 Hook
type MetricsHook struct{}
func (h *MetricsHook) Name() string { return "metrics" }
func (h *MetricsHook) Phase() Phase { return PhasePostCall }
func (h *MetricsHook) Execute(ctx context.Context, event *HookEvent) error {
    // 上报 Prometheus 指标
    return metrics.Record(event)
}

// 自定义过滤 Hook（拦截器示例）
// 注册在 PhasePreCall 阶段，返回 error 会中止请求
type ContentFilterHook struct{}
func (h *ContentFilterHook) Name() string { return "content_filter" }
func (h *ContentFilterHook) Phase() Phase { return PhasePreCall }
func (h *ContentFilterHook) Execute(ctx context.Context, event *HookEvent) error {
    // PreCall 阶段返回 error → Dispatch 会中止请求（拦截器语义）
    if containsSensitiveContent(event.Request) {
        return ErrContentBlocked
    }
    return nil
}
```

### 7.12 异步消费批量写入（借鉴 LiteLLM DBSpendUpdateWriter）

```go
// pkg/manager/spend_writer.go

// SpendWriter 异步批量消费记录
type SpendWriter struct {
    queue     chan SpendUpdate
    interval  time.Duration  // 默认 60s
    batchSize int            // 默认 100
    db        SpendStorage
    wal       *WAL           // 本地 WAL 文件，同步写入也失败时的最终兜底
    done      chan struct{}
}

// WAL 预写日志，用于极端情况下保证计费数据不丢失
type WAL struct {
    mu   sync.Mutex
    file *os.File
}

func NewWAL(path string) (*WAL, error) {
    f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
    if err != nil { return nil, err }
    return &WAL{file: f}, nil
}

func (w *WAL) Append(update SpendUpdate) {
    w.mu.Lock()
    defer w.mu.Unlock()
    data, _ := json.Marshal(update)
    w.file.Write(append(data, '\n'))
}

func (w *WAL) Close() error { return w.file.Close() }

type SpendUpdate struct {
    TenantID  string
    Model     string
    Provider  string
    Tokens    int
    CostUSD   float64
    Timestamp time.Time
}

type SpendStorage interface {
    BatchUpdate(ctx context.Context, updates []SpendUpdate) error
}

func NewSpendWriter(db SpendStorage, walPath string, interval time.Duration, batchSize int) (*SpendWriter, error) {
    wal, err := NewWAL(walPath)
    if err != nil {
        return nil, fmt.Errorf("init WAL: %w", err)
    }
    sw := &SpendWriter{
        queue:     make(chan SpendUpdate, 10000),
        interval:  interval,
        batchSize: batchSize,
        db:        db,
        wal:       wal,
        done:      make(chan struct{}),
    }
    go sw.run()
    return sw, nil
}

func (sw *SpendWriter) Record(update SpendUpdate) {
    select {
    case sw.queue <- update:
    default:
        // 队列满时降级为同步写入，保证计费数据不丢失
        metrics.SpendWriterOverflow.Inc()
        if err := sw.db.BatchUpdate(context.Background(), []SpendUpdate{update}); err != nil {
            // 同步写入也失败，写入本地 WAL 文件作为最终兜底
            slog.Error("spend record lost, writing to WAL", "tenant", update.TenantID, "error", err)
            sw.wal.Append(update)
        }
    }
}

func (sw *SpendWriter) run() {
    ticker := time.NewTicker(sw.interval)
    defer ticker.Stop()

    var buffer []SpendUpdate

    for {
        select {
        case update := <-sw.queue:
            buffer = append(buffer, update)
            if len(buffer) >= sw.batchSize {
                sw.flush(buffer)
                buffer = nil
            }
        case <-ticker.C:
            if len(buffer) > 0 {
                sw.flush(buffer)
                buffer = nil
            }
        case <-sw.done:
            if len(buffer) > 0 {
                sw.flush(buffer)
            }
            return
        }
    }
}

func (sw *SpendWriter) flush(updates []SpendUpdate) {
    // 按 tenant+model 合并后批量写入
    merged := sw.mergeUpdates(updates)
    if err := sw.db.BatchUpdate(context.Background(), merged); err != nil {
        slog.Error("spend batch update failed, writing to WAL", "count", len(merged), "error", err)
        // 批量写入失败，降级写入 WAL 保证数据不丢失
        for _, u := range merged {
            sw.wal.Append(u)
        }
        metrics.SpendWriterFlushFailed.Inc()
    }
}

func (sw *SpendWriter) Close() error {
    close(sw.done)
    // 等待 run() 退出（flush 完剩余数据）
    // 然后关闭 WAL 文件
    return sw.wal.Close()
}
```

### 7.13 预扣+结算配额（借鉴 New-API）

```go
// pkg/manager/quota.go

// QuotaManager 配额管理器（预扣+结算模式）
type QuotaManager struct {
    store QuotaStore
    mu    sync.Mutex
}

type QuotaStore interface {
    // GetQuota 获取租户配额信息
    GetQuota(ctx context.Context, tenantID string) (*TenantQuota, error)

    // PreConsume 原子性预扣 token 和费用
    // 参数：estimatedTokens 预估 token 数，estimatedCost 预估费用 (USD)
    // 返回：quotaID 用于后续结算，内部事务保证 token + cost 同时预扣
    PreConsume(ctx context.Context, tenantID string, estimatedTokens int, estimatedCost float64) (quotaID string, err error)

    // Settle 按实际用量结算，退回差额
    // 参数：actualTokens 实际 token 数，actualCost 实际费用 (USD)
    // 内部事务保证 token 差额退回 + cost 差额退回的原子性
    Settle(ctx context.Context, quotaID string, actualTokens int, actualCost float64) error

    // Rollback 请求失败时退回全部预扣额度（token + cost）
    Rollback(ctx context.Context, quotaID string) error
}

// PreConsumeRecord 预扣记录，用于结算时计算差额
type PreConsumeRecord struct {
    QuotaID          string
    TenantID         string
    EstimatedTokens  int
    EstimatedCost    float64
    CreatedAt        time.Time
}

type TenantQuota struct {
    TenantID      string
    DailyLimit    int64    // 日 token 限额
    MonthlyLimit  float64  // 月费用限额 (USD)
    DailyUsed     int64
    MonthlySpent  float64
}

// PreConsume 按估算 token 和费用预扣额度
// 返回 quotaID 用于后续结算
func (q *QuotaManager) PreConsume(ctx context.Context, tenantID string, estimatedTokens int, estimatedCost float64) (quotaID string, err error) {
    quota, err := q.store.GetQuota(ctx, tenantID)
    if err != nil {
        return "", err
    }

    // 检查日 token 限额
    if quota.DailyLimit > 0 && quota.DailyUsed+int64(estimatedTokens) > quota.DailyLimit {
        return "", ErrDailyQuotaExceeded
    }

    // 检查月费用限额
    if quota.MonthlyLimit > 0 && quota.MonthlySpent+estimatedCost > quota.MonthlyLimit {
        return "", ErrMonthlyQuotaExceeded
    }

    // 预扣（token + cost 原子性）
    return q.store.PreConsume(ctx, tenantID, estimatedTokens, estimatedCost)
}

var (
    ErrDailyQuotaExceeded   = errors.New("daily token quota exceeded")
    ErrMonthlyQuotaExceeded = errors.New("monthly spend quota exceeded")
)

// Settle 请求完成后按实际用量结算，退回差额（token + cost 原子性）
func (q *QuotaManager) Settle(ctx context.Context, quotaID string, actualTokens int, actualCost float64) error {
    return q.store.Settle(ctx, quotaID, actualTokens, actualCost)
}

// Rollback 请求失败时退回预扣额度
func (q *QuotaManager) Rollback(ctx context.Context, quotaID string) error {
    return q.store.Rollback(ctx, quotaID)
}
```

使用流程：

```go
// Manager.Chat 中的配额处理（简化示例，完整版见 Section 7.1 Manager.Chat）
func (m *Manager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 估算 token 和费用
    estimatedTokens := m.tokenCounter.Estimate(req)
    estimatedCost := m.costCalculator.Estimate(req.Model, estimatedTokens)

    // 2. 预扣配额（同时检查日 token 限额和月费用限额）
    quotaID, err := m.quotaManager.PreConsume(ctx, req.TenantID, estimatedTokens, estimatedCost)
    if err != nil {
        return nil, err
    }

    // 3. 调用 provider
    resp, err := m.doChat(ctx, req)

    // 4. 结算或退回
    if err != nil {
        m.quotaManager.Rollback(ctx, quotaID)
        return nil, err
    }
    // 按实际用量结算（token + cost 原子性退回差额）
    actualCost := m.costCalculator.Calculate(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
    m.quotaManager.Settle(ctx, quotaID, resp.Usage.TotalTokens, actualCost)

    return resp, nil
}
```

### 7.14 可观测性

#### 7.14.1 OpenTelemetry Tracing

```go
// pkg/observability/tracing.go

import "go.opentelemetry.io/otel"

// TracingMiddleware 为每个请求创建 trace span
// Span 层级：
//   Gateway.Request (root)
//   ├── Auth.Verify
//   ├── Router.Select
//   ├── RateLimiter.Check
//   ├── Cache.Lookup (命中则直接返回)
//   ├── Provider.Call (调用上游)
//   │   ├── Mapper.ToProviderFormat
//   │   ├── Transport.HTTP (含 retry span)
//   │   └── Mapper.FromProviderFormat
//   ├── Cache.Write
//   └── Metrics.Record

// 关键 Span Attributes
type SpanAttrs struct {
    TenantID      string  // 租户标识
    Provider      string  // 使用的 provider
    Model         string  // 使用的模型
    ModelTier     string  // 模型层级
    Stream        bool    // 是否流式
    HasTools      bool    // 是否使用 tool calling
    TokensIn      int     // 输入 token 数
    TokensOut     int     // 输出 token 数
    CostUSD       float64 // 本次请求成本
    CacheHit      bool    // 是否缓存命中
    RetryCount    int     // 重试次数
    FallbackUsed  bool    // 是否触发了降级
    FinishReason  string  // 完成原因
}

// TraceID 透传：响应 header 中返回 X-Trace-ID，便于客户端关联排查
// 流式场景：首个 StreamEvent 中携带 trace_id 字段
```

#### 7.14.2 Prometheus 指标

```go
// pkg/observability/metrics.go

// ========== 全局指标（低基数，适合 Prometheus label）==========

// --- 请求维度（不含 tenant_id，避免高基数爆炸）---
// llm_gateway_requests_total{provider, model, tier, status}
// llm_gateway_request_duration_seconds{provider, model, tier}  -- histogram
// llm_gateway_ttft_seconds{provider, model, tier}              -- Time To First Token (流式)

// --- Token 与成本（全局聚合）---
// llm_gateway_tokens_total{provider, model, direction="input|output"}
// llm_gateway_cost_usd_total{provider, model}

// --- 可靠性 ---
// llm_gateway_retry_total{provider, reason}
// llm_gateway_fallback_total{from_provider, to_provider}
// llm_gateway_circuit_breaker_state{provider}  -- gauge: 0=closed, 1=open, 2=half_open
// llm_gateway_cooldown_active{provider, model} -- gauge: 当前冷却中的 key 数
// llm_gateway_stream_failures_total{provider, strategy}

// --- 缓存 ---
// llm_gateway_cache_hits_total{layer="memory|redis"} / llm_gateway_cache_misses_total

// ========== 租户指标（独立上报通道，避免全局指标基数爆炸）==========
// 租户维度的 token/成本/限流数据通过以下两种方式上报，不放入全局 Prometheus label：
//
// 方案 A：独立的租户指标 exporter（按租户 ID 分桶聚合后再暴露）
//   llm_gateway_tenant_requests_total{tenant_id, status}       -- 仅在租户数可控（<100）时启用
//   llm_gateway_tenant_tokens_total{tenant_id, direction}
//   llm_gateway_tenant_budget_remaining{tenant_id, type}
//   llm_gateway_tenant_rate_limit_rejected_total{tenant_id}
//
// 方案 B（推荐）：租户级数据写入 SpendWriter → 数据库，用 Grafana 直接查 DB
//   不进入 Prometheus，彻底避免高基数问题
//   Dashboard 通过 SQL/ClickHouse 查询租户维度数据
```

> **高基数防护**：`tenant_id` 不作为全局请求指标的 label，避免租户数增长导致 Prometheus 内存和查询性能劣化。租户维度数据通过 SpendWriter 写入数据库，用 Grafana 混合数据源（Prometheus + DB）构建统一 Dashboard。

#### 7.14.3 SLI / SLO 定义

| SLI | 计算方式 | SLO 目标 | 告警阈值 |
|-----|---------|---------|---------|
| **可用性** | 成功请求数 / 总请求数（排除 4xx） | 99.9% / 30d | burn rate > 3x 持续 5min |
| **延迟 P95** (非流式) | request_duration_seconds P95 | small < 2s, medium < 5s, large < 10s | 超标持续 5min |
| **首 Token 延迟 P95** (流式) | ttft_seconds P95 | small < 1s, medium < 3s, large < 5s | 超标持续 5min |
| **错误率** | 5xx 请求数 / 总请求数 | < 0.1% | > 1% 持续 3min |
| **成本偏差** | 实际成本 / 预估成本 | 偏差 < 10% | 偏差 > 20% |

```yaml
# config/config.yaml
observability:
  tracing:
    enabled: true
    exporter: "otlp"           # otlp / jaeger / stdout
    endpoint: "localhost:4317"
    sample_rate: 1.0           # 生产建议 0.1

  metrics:
    enabled: true
    endpoint: "/metrics"       # Prometheus scrape 端点

  logging:
    level: "info"
    format: "json"             # json / text
    # 每条请求日志自动携带 trace_id、tenant_id、provider、model

  slo:
    # SLO burn rate 告警规则（输出为 Prometheus alerting rules）
    alerts:
      - name: "HighErrorRate"
        expr: 'rate(llm_gateway_requests_total{status="error"}[5m]) / rate(llm_gateway_requests_total[5m]) > 0.01'
        severity: "critical"
      - name: "HighLatency"
        expr: 'histogram_quantile(0.95, rate(llm_gateway_request_duration_seconds_bucket[5m])) > 10'
        severity: "warning"
      - name: "CircuitBreakerOpen"
        expr: 'llm_gateway_circuit_breaker_state > 0'
        severity: "warning"
```

> **排障目标**：任一失败请求可在 5 分钟内通过 `trace_id → 请求 span → provider/model/错误类型 → 上游原始错误` 链路完成定位。

---

## 8. 消息转换层

> **实现决策**：不创建独立的 `pkg/mapper/` 包，消息转换逻辑**内联在各 adapter 中**。
>
> 原因：每个平台的映射逻辑与其私有 API 类型（如 `anthropicMessage`、`anthropicContentBlock`）强耦合，
> 独立包会增加不必要的跨包引用和类型导出。实际实现中：
> - OpenAI adapter: `chat.go` 中的 `convertMessages()` / `convertMessage()` / `convertContentBlock()` / `convertTools()`
> - Anthropic adapter: `chat.go` 中的 `extractSystemAndConvert()` / `convertContentBlock()` / `convertTools()` / `convertToolChoice()`
> - Google adapter: 待 Sprint 7 实现

### 8.1 OpenAI 消息转换（chat.go 内联）

```go
// 由于内部格式以 OpenAI 为基准，转换几乎是直通（pass-through）：
// - types.Message → openAIMessage（Role、Content、ToolCalls 直接映射）
// - types.ContentBlock → openAIContentPart（text / image_url 类型映射）
// - types.Tool → openAITool（结构一致，直接复制）
// 唯一的非直通逻辑：
// - Extra map[string]any 通过自定义 MarshalJSON 合并到 JSON 顶层
// - max_tokens vs max_completion_tokens 按模型名自动选择
```

### 8.2 Anthropic 消息转换（chat.go 内联）

```go
// 核心函数：extractSystemAndConvert()
// 处理与 OpenAI 的所有差异：
// 1. system role → 抽取到顶层 system 字段
// 2. tool role → 转为 user role + tool_result content block
// 3. assistant + tool_calls → 拆分为 text block + tool_use blocks
// 4. multimodal image_url → image type + source object
// 5. stop → stop_sequences
// 6. tool_choice string → object format
// 7. parameters → input_schema
```

### 8.3 Gemini 消息转换（待实现）

```go
// 待 Sprint 7 实现，关键差异：
// - messages → contents[].parts[]
// - role: "assistant" → role: "model"
// - system → systemInstruction
// - 所有参数嵌套在 generationConfig 中
// - tools → functionDeclarations
// - URL 路径包含模型名: /models/{model}:generateContent
```

---

## 9. 流式 SSE 统一解析

```go
// pkg/transport/sse.go

// SSEEvent 表示一个 Server-Sent Event
type SSEEvent struct {
    Event string // 事件类型（Anthropic 用 content_block_delta / message_stop 等）
    Data  string // 事件数据
    ID    string // 事件 ID（可选）
    Retry int    // 重试间隔（可选）
}

// IsDone 判断是否为结束事件
// 兼容 OpenAI 的 [DONE] 和 Anthropic 的 message_stop
func (e *SSEEvent) IsDone() bool {
    return e.Data == "[DONE]" || e.Event == "message_stop"
}

// SSEReader 通用 SSE 流读取器
type SSEReader struct {
    reader *bufio.Reader
}

func NewSSEReader(r io.Reader) *SSEReader { ... }
func (r *SSEReader) Read() (*SSEEvent, error) { ... }
func (r *SSEReader) ReadAll() ([]*SSEEvent, error) { ... }

// SSEWriter 写入 SSE 事件到 HTTP 响应
type SSEWriter struct {
    writer io.Writer
}

func NewSSEWriter(w io.Writer) *SSEWriter { ... }
func (w *SSEWriter) Write(event *SSEEvent) error { ... }
func (w *SSEWriter) WriteData(data string) error { ... }
func (w *SSEWriter) WriteDone() error { ... }
```

> **实现决策**：流式事件解析（OpenAI chunk → StreamEvent、Anthropic delta → StreamEvent）
> 内联在各 adapter 的 `stream.go` 中，而非独立的 `pkg/mapper/stream.go`。
> `SSEReader` / `SSEWriter` 作为通用工具保留在 `transport` 包中。

```go
// pkg/adapter/openai/stream.go — OpenAI 流式解析（内联在 adapter 中）
// 使用 SSEReader 读取 SSE 事件，解析 JSON chunk 为 StreamEvent

// pkg/adapter/anthropic/stream.go — Anthropic 流式解析（内联在 adapter 中）
// 处理 Anthropic 独有的 SSE 事件类型：
//   content_block_delta → StreamEventContentDelta
//   message_delta       → StreamEventUsage + FinishReason
//   message_stop        → StreamEventDone
//   content_block_start → tool_call 开始（type=="tool_use" 时）
```

---

## 10. 安全与合规

### 10.1 上游 Provider 认证策略

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

// GoogleAuth — API Key as query parameter
type GoogleAuth struct {
    APIKey string
}
func (a *GoogleAuth) Apply(req *http.Request) error {
    q := req.URL.Query()
    q.Set("key", a.APIKey)
    req.URL.RawQuery = q.Encode()
    return nil
}

// NoAuth — 无认证（用于不需要 key 的场景）
type NoAuth struct{}
func (a *NoAuth) Apply(req *http.Request) error { return nil }

// NewAuthStrategy 工厂函数，根据 provider 名称创建合适的认证策略
func NewAuthStrategy(provider, apiKey string) AuthStrategy {
    switch provider {
    case "anthropic":
        return &AnthropicAuth{APIKey: apiKey}
    case "google":
        return &GoogleAuth{APIKey: apiKey}
    default:
        return &BearerAuth{APIKey: apiKey}
    }
}

// DynamicAuth 动态凭证支持（BYOK）
// 实现 AuthStrategy 接口，构造时注入 credentials
// 优先使用动态凭证，fallback 到静态配置
type DynamicAuth struct {
    StaticAuth  AuthStrategy      // 静态配置的认证策略
    Credentials map[string]string // 来自 ChatRequest.Credentials
}

func (a *DynamicAuth) Apply(req *http.Request) error {
    // 优先使用动态 api_key（Bearer 方式）
    if apiKey, ok := a.Credentials["api_key"]; ok && apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+apiKey)
        return nil
    }
    // 支持 Anthropic 格式的动态凭证（x_api_key）
    if apiKey, ok := a.Credentials["x_api_key"]; ok && apiKey != "" {
        req.Header.Set("x-api-key", apiKey)
        req.Header.Set("anthropic-version", "2023-06-01")
        return nil
    }
    // Fallback 到静态配置
    if a.StaticAuth != nil {
        return a.StaticAuth.Apply(req)
    }
    return nil
}

// WithDynamicCredentials 包装函数，简化 BYOK 凭证注入
func WithDynamicCredentials(staticAuth AuthStrategy, credentials map[string]string) AuthStrategy {
    if len(credentials) == 0 {
        return staticAuth
    }
    return &DynamicAuth{StaticAuth: staticAuth, Credentials: credentials}
}
```

```go
// pkg/transport/http_client.go — 统一 HTTP 客户端

type HTTPClient struct {
    client  *http.Client
    timeout time.Duration
}

type HTTPClientConfig struct {
    Timeout         time.Duration
    MaxIdleConns    int
    IdleConnTimeout time.Duration
    Proxy           string // per-provider proxy 配置
}

// NewHTTPClient 创建 HTTPClient
func NewHTTPClient(cfg HTTPClientConfig) *HTTPClient { ... }

// DefaultHTTPClient 使用默认配置（使用环境变量代理）
func DefaultHTTPClient() *HTTPClient { ... }

// NewHTTPClientWithProxy 快捷方式：仅指定 proxy 配置
func NewHTTPClientWithProxy(proxy string) *HTTPClient {
    return NewHTTPClient(HTTPClientConfig{Proxy: proxy})
}

// resolveProxyFunc 根据 proxy 配置字符串返回对应的代理函数
//   - "http://host:port" / "socks5://host:port": 使用固定代理
//   - "none" / "direct": 不使用代理（直连）
//   - "" (空): 使用系统环境变量 HTTP_PROXY/HTTPS_PROXY
func resolveProxyFunc(proxy string) func(*http.Request) (*url.URL, error) { ... }

// Do 发送 HTTP 请求
func (c *HTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) { ... }

// DoJSON 发送 JSON 请求并解码响应（自动错误分类）
func (c *HTTPClient) DoJSON(ctx context.Context, method, url string, auth AuthStrategy, body any, result any) error { ... }

// DoStream 发送请求并返回响应体（用于 SSE 流式读取）
func (c *HTTPClient) DoStream(ctx context.Context, method, url string, auth AuthStrategy, body any) (io.ReadCloser, error) { ... }

// parseProviderError 解析 provider 错误响应，自动分类错误码
func parseProviderError(statusCode int, body []byte) *types.ProviderError { ... }
```

> **Per-Provider Proxy**：每个 provider 可在 `config.yaml` 中独立配置 `proxy` 字段，
> 实现海外 API 走代理、国内 API 直连的混合策略。`NewHTTPClientWithProxy()` 在
> 每个 adapter 的构造函数中被调用，确保每个 provider 使用独立的 HTTP Transport。

> **BYOK（Bring Your Own Key）场景**：企业客户可在请求中携带自己的 API Key，Gateway 使用客户提供的凭证调用上游 Provider。`DynamicAuth` 支持 Bearer（`api_key`）和 Anthropic（`x_api_key`）两种格式的动态凭证。

### 10.2 多租户与访问控制

```go
// pkg/auth/tenant.go

// Tenant 租户定义
type Tenant struct {
    ID          string            `yaml:"id"`
    Name        string            `yaml:"name"`
    APIKeys     []HashedKey       `yaml:"api_keys"`     // 支持多 key 轮换
    Role        Role              `yaml:"role"`          // admin / user / readonly
    AllowedModels []string        `yaml:"allowed_models"` // 可用模型白名单，空=全部
    AllowedProviders []string     `yaml:"allowed_providers"`
    Budget      *TenantBudget     `yaml:"budget"`
    RateLimit   *TenantRateLimit  `yaml:"rate_limit"`
}

type Role string
const (
    RoleAdmin    Role = "admin"    // 全部权限 + 管理接口
    RoleUser     Role = "user"     // 调用所有模型接口
    RoleReadonly Role = "readonly" // 仅查询模型列表/任务状态
)

type HashedKey struct {
    Prefix   string    `yaml:"prefix"`    // 明文前 8 位，用于日志标识（sk-abcd****）
    Hash     string    `yaml:"hash"`      // bcrypt hash
    ExpireAt time.Time `yaml:"expire_at"` // 过期时间，零值=永不过期
}

// TenantBudget 租户预算控制
type TenantBudget struct {
    DailyTokenLimit   int64   `yaml:"daily_token_limit"`    // 日 token 上限，0=不限
    MonthlySpendLimit float64 `yaml:"monthly_spend_limit"`  // 月费用上限(USD)，0=不限
}

// TenantRateLimit 租户级限流（覆盖全局限流）
type TenantRateLimit struct {
    RPM int `yaml:"rpm"` // 每分钟请求数
    TPM int `yaml:"tpm"` // 每分钟 token 数
}
```

```go
// pkg/auth/rbac.go

// Authorizer 鉴权中间件
type Authorizer struct {
    // prefixIndex: API Key 前缀（前 8 字符）→ 候选 Tenant 列表
    // 查找时先用 O(1) 前缀匹配缩小范围，再对候选集做 bcrypt 比较
    prefixIndex map[string][]*tenantKeyPair
}

type tenantKeyPair struct {
    Tenant *Tenant
    Key    *HashedKey
}

// buildIndex 启动时构建前缀索引
func (a *Authorizer) buildIndex(tenants []*Tenant) {
    a.prefixIndex = make(map[string][]*tenantKeyPair)
    for _, t := range tenants {
        for i := range t.APIKeys {
            k := &t.APIKeys[i]
            a.prefixIndex[k.Prefix] = append(a.prefixIndex[k.Prefix], &tenantKeyPair{
                Tenant: t, Key: k,
            })
        }
    }
}

// Authenticate 从请求中提取 API Key，验证身份
// 性能优化：先用前缀索引 O(1) 定位候选，再做 bcrypt 比较，避免全量遍历
func (a *Authorizer) Authenticate(apiKey string) (*Tenant, error) {
    if len(apiKey) < 8 {
        return nil, ErrUnauthorized
    }

    prefix := apiKey[:8]
    candidates, ok := a.prefixIndex[prefix]
    if !ok {
        return nil, ErrUnauthorized
    }

    for _, pair := range candidates {
        if !pair.Key.ExpireAt.IsZero() && pair.Key.ExpireAt.Before(time.Now()) {
            continue // 已过期
        }
        if bcrypt.CompareHashAndPassword([]byte(pair.Key.Hash), []byte(apiKey)) == nil {
            return pair.Tenant, nil
        }
    }
    return nil, ErrUnauthorized
}

// Authorize 检查租户是否有权访问指定模型/provider
func (a *Authorizer) Authorize(tenant *Tenant, model, provider string) error {
    // 1. 角色检查
    // 2. 模型白名单检查
    // 3. Provider 白名单检查
    // 4. 预算检查（调用 BudgetTracker）
    ...
}
```

### 10.3 密钥管理

```go
// pkg/secret/provider.go

// SecretProvider 密钥存储抽象，避免明文硬编码
type SecretProvider interface {
    // GetSecret 获取密钥，key 格式如 "openai/api_key"
    GetSecret(ctx context.Context, key string) (string, error)
    // Watch 监听密钥变更（用于自动轮换）
    Watch(ctx context.Context, key string) (<-chan string, error)
}

// 内置实现
type EnvSecretProvider struct{}          // 从环境变量读取（开发环境）
type KMSSecretProvider struct{}          // 对接云 KMS（阿里 KMS / AWS KMS）
type VaultSecretProvider struct{}        // 对接 HashiCorp Vault
```

配置示例：

```yaml
# config/config.yaml
secret:
  provider: "env"  # env / kms / vault
  kms:
    region: "cn-hangzhou"
    key_id: "alias/llm-gateway"
  vault:
    addr: "https://vault.internal:8200"
    path: "secret/data/llm-gateway"
```

### 10.4 请求日志脱敏

```go
// pkg/middleware/sanitizer.go

// Sanitizer 日志脱敏策略
type SanitizeConfig struct {
    // Messages 内容脱敏策略
    MessagePolicy  SanitizePolicy `yaml:"message_policy"`  // none / hash / truncate / mask
    TruncateLength int            `yaml:"truncate_length"` // truncate 模式下保留长度，默认 100
    // 敏感字段掩码
    MaskFields     []string       `yaml:"mask_fields"`     // 自定义需掩码的字段路径
}

type SanitizePolicy string
const (
    SanitizeNone     SanitizePolicy = "none"     // 不记录 message 内容
    SanitizeHash     SanitizePolicy = "hash"     // 记录 SHA256 摘要（可用于去重分析）
    SanitizeTruncate SanitizePolicy = "truncate" // 截断保留前 N 字符
    SanitizeMask     SanitizePolicy = "mask"     // 正则匹配敏感信息替换为 ***
)

// SanitizeForLog 对请求/响应做日志脱敏
func SanitizeForLog(req *ChatRequest, cfg *SanitizeConfig) map[string]any {
    logEntry := map[string]any{
        "model":      req.Model,
        "provider":   req.Provider,
        "msg_count":  len(req.Messages),
        "has_tools":  len(req.Tools) > 0,
        "stream":     req.Stream,
    }
    switch cfg.MessagePolicy {
    case SanitizeHash:
        logEntry["content_hash"] = sha256Hex(req.Messages)
    case SanitizeTruncate:
        logEntry["content_preview"] = truncate(req.Messages, cfg.TruncateLength)
    // none 和 mask 同理
    }
    return logEntry
}
```

### 10.5 审计日志

```go
// pkg/audit/logger.go

// AuditEvent 审计事件，独立于业务日志，不可篡改
type AuditEvent struct {
    Timestamp   time.Time `json:"timestamp"`
    TraceID     string    `json:"trace_id"`
    TenantID    string    `json:"tenant_id"`
    APIKeyPrefix string   `json:"api_key_prefix"` // sk-abcd****
    Action      string    `json:"action"`          // chat / embed / image_gen / ...
    Model       string    `json:"model"`
    Provider    string    `json:"provider"`
    StatusCode  int       `json:"status_code"`
    TokensUsed  int       `json:"tokens_used"`
    CostUSD     float64   `json:"cost_usd"`
    Latency     int64     `json:"latency_ms"`
    ClientIP    string    `json:"client_ip"`
    UserAgent   string    `json:"user_agent"`
    Error       string    `json:"error,omitempty"`
}

// AuditLogger 审计日志输出器
type AuditLogger interface {
    Log(ctx context.Context, event *AuditEvent) error
}

// 内置实现：文件（JSON Lines）、stdout、远程（Kafka/SLS）
type FileAuditLogger struct{ ... }
type StdoutAuditLogger struct{ ... }
type RemoteAuditLogger struct{ ... }
```

配置示例：

```yaml
# config/config.yaml
security:
  # 租户配置
  tenants_file: "config/tenants.yaml"

  # 密钥管理：引用 secret 顶级配置（见 Section 10.3）
  # 此处不再重复定义，统一使用 secret.provider

  # 日志脱敏
  sanitize:
    message_policy: "truncate"
    truncate_length: 100

  # 审计日志
  audit:
    enabled: true
    output: "file"           # file / stdout / remote
    file_path: "/var/log/llm-gateway/audit.jsonl"
    retention_days: 90

  # 工具调用安全
  tool_sandbox:
    enabled: true
    allowed_domains:         # 出网白名单（tool calling 中的 URL 访问）
      - "*.internal.company.com"
      - "api.example.com"
    max_response_size: "1MB"
    timeout: "10s"
```

---

## 11. 配置文件设计

> **实现说明**：配置分为两个文件 — `config/config.yaml`（providers + 编排参数）和 `config/models.yaml`（模型目录 + Tier 路由）。
> `config.Load()` 自动从同目录加载 `models.yaml`。支持 `${VAR_NAME}` 和 `${VAR:-default}` 环境变量替换。
> API Key 为空的 provider 在启动时自动移除（`pruneUnavailableProviders()`），无需注释配置。

### 11.1 Go 配置结构

```go
// config/config.go

type Config struct {
    Server        ServerConfig              `yaml:"server"`
    Providers     map[string]ProviderConfig `yaml:"providers"`
    ModelCatalog  []ModelCatalogEntry       `yaml:"model_catalog"`   // 从 models.yaml 加载
    TierRouting   map[string][]RouteEntry   `yaml:"tier_routing"`    // 从 models.yaml 加载
    Manager       ManagerConfig             `yaml:"manager"`
    Security      SecurityConfig            `yaml:"security"`
    Secret        SecretConfig              `yaml:"secret"`
    Observability ObservabilityConfig       `yaml:"observability"`
}

type ProviderConfig struct {
    BaseURL   string         `yaml:"base_url"`
    APIKey    string         `yaml:"api_key"`
    Platform  string         `yaml:"platform,omitempty"`  // 兼容平台标识（alibaba, volcengine 等）
    RateLimit int            `yaml:"rate_limit"`
    Timeout   time.Duration  `yaml:"timeout,omitempty"`

    // Per-provider HTTP proxy 配置：
    //   "http://host:port" / "socks5://host:port": 使用指定代理
    //   "none" / "direct": 不使用代理，直连
    //   "" (空/不写): 使用系统环境变量 HTTP_PROXY/HTTPS_PROXY
    Proxy string `yaml:"proxy,omitempty"`

    // 厂商自定义配置（不同 provider 有不同的 extra 字段）
    Extra map[string]any `yaml:"extra,omitempty"`
}

// GetExtra 从 Extra 中取 string 值，支持默认值
func (c ProviderConfig) GetExtra(key, defaultVal string) string { ... }

// GetExtraInt 从 Extra 中取 int 值（兼容 YAML float64），支持默认值
func (c ProviderConfig) GetExtraInt(key string, defaultVal int) int { ... }
```

### 11.2 配置加载流程

```go
func Load(path string) (*Config, error) {
    // 1. 读取 config.yaml
    // 2. expandEnvVars(): 替换 ${VAR_NAME} 和 ${VAR:-default}
    // 3. YAML 解析
    // 4. loadModelsCatalog(): 从同目录的 models.yaml 加载模型目录和 Tier 路由
    // 5. pruneUnavailableProviders(): 移除 API Key 为空的 provider，
    //    同时过滤相关的 model_catalog 和 tier_routing 条目
    // 6. applyDefaults(): 填充缺省值
    // 7. Validate(): 校验一致性（provider 引用、model 引用等）
}
```

### 11.3 Provider 配置示例

```yaml
# config/config.yaml

server:
  host: "0.0.0.0"
  port: 8080

providers:
  openai:
    base_url: "https://api.openai.com/v1"
    api_key: "${OPENAI_API_KEY}"
    rate_limit: 500
    # proxy: ""                             # 默认使用环境变量代理（适合海外 API）
    # extra:
    #   chat_path: "/chat/completions"      # 自定义 endpoint 路径
    #   responses_path: "/responses"
    #   models_path: "/models"

  anthropic:
    base_url: "https://api.anthropic.com"
    api_key: "${ANTHROPIC_API_KEY}"
    rate_limit: 200
    # proxy: ""                              # 海外 API 需要代理
    extra:
      anthropic_version: "2023-06-01"        # API 版本
      default_max_tokens: 4096               # 默认 max_tokens
      # api_format: "openai"                 # 设为 "openai" 时走 OpenAI 兼容协议

  alibaba:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "${DASHSCOPE_API_KEY}"
    platform: "alibaba"
    rate_limit: 300
    proxy: "none"                            # 国内平台，直连

  volcengine:
    base_url: "https://ark.cn-beijing.volces.com/api/v3"
    api_key: "${ARK_API_KEY}"
    platform: "volcengine"
    rate_limit: 300
    proxy: "none"

  deepseek:
    base_url: "https://api.deepseek.com/v1"
    api_key: "${DEEPSEEK_API_KEY}"
    platform: "deepseek"
    rate_limit: 300
    proxy: "none"
```

### 11.4 模型目录 + Tier 路由

```yaml
# config/models.yaml（独立文件，由 Load() 自动加载）

model_catalog:
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

  - id: "qwen-turbo"
    provider: "alibaba"
    tier: "small"
    context_window: 131072
    max_output: 8192
    capabilities: { chat: true, tools: true, streaming: true }

  # 更多模型...

tier_routing:
  small:
    - { provider: "alibaba",    model: "qwen-turbo",   priority: 1 }
    - { provider: "openai",     model: "gpt-4o-mini",  priority: 2 }
  medium:
    - { provider: "alibaba",    model: "qwen-plus",    priority: 1 }
    - { provider: "volcengine", model: "doubao-pro",   priority: 2 }
  large:
    - { provider: "openai",     model: "gpt-4o",       priority: 1 }
    - { provider: "alibaba",    model: "qwen-max",     priority: 3 }
```

### 11.5 编排配置

```yaml
# config/config.yaml（续）

manager:
  cache:
    enabled: true
    memory:
      max_size: 1000     # LRU 容量
      ttl: 5m            # 内存缓存 TTL
    redis:
      enabled: false     # 可选，启用后为 DualCache 模式
      url: "${REDIS_URL:-redis://localhost:6379}"
      ttl: 1h

  circuit_breaker:
    failure_threshold: 5
    recovery_timeout: 30s

  hedged_request:
    enabled: true
    delay: 500ms

  token_safety_margin: 256

  retry:
    max_attempts: 3
    initial_delay: 100ms
    max_delay: 5s
    backoff_factor: 2.0
    retry_budget: 0.1
    deadline: 30s
    budget_window: 60s

  cooldown:
    enabled: true
    backoff_sequence: [10s, 30s, 60s, 120s, 300s]
    max_failures: 5

  quota:
    enabled: false
    store: "memory"
    preconsumed_ttl: 10m

  spend_writer:
    enabled: false
    batch_size: 100
    flush_interval: 5s
    queue_size: 10000
    wal_path: "/var/lib/llm-gateway/spend.wal"

  cost_calculator:
    pricing_file: "config/models.yaml"
    fallback_price:
      input_per_1k: 0.001
      output_per_1k: 0.002

  timeout:
    connect: 5s
    first_token: 30s
    total_non_stream: 120s
    total_stream: 300s
    idle_between_chunks: 30s
```

### 11.6 Extra 字段参考

| 字段 | 适用 provider | 默认值 | 说明 |
|------|-------------|--------|------|
| `chat_path` | OpenAI 兼容 | `/chat/completions` | Chat 接口路径 |
| `responses_path` | OpenAI 兼容 | `/responses` | Responses 接口路径 |
| `models_path` | OpenAI 兼容 | `/models` | 模型列表接口路径 |
| `api_format` | Anthropic | `anthropic` | 设为 `"openai"` 时走 OpenAI 兼容协议 |
| `anthropic_version` | Anthropic 原生 | `2023-06-01` | API 版本 header |
| `default_max_tokens` | Anthropic 原生 | `4096` | 默认 max_tokens |
| `messages_path` | Anthropic 原生 | `/v1/messages` | Messages 接口路径 |

---

## 12. 对外 API 设计

对外暴露 **OpenAI 兼容的 HTTP API**，让上层业务可以像调用 OpenAI 一样调用我们的服务，只需增加 `provider` 和 `model_tier` 扩展字段：

```
# 已实现
POST /v1/chat/completions      -> handler.Chat          # 对话（兼容 OpenAI 格式）
POST /v1/responses             -> handler.Responses      # Responses API（推理模型优化）
GET  /v1/models                -> router.handleListModels # 模型列表（OpenAI 格式）
GET  /health                   -> router.handleHealth     # 健康检查
GET  /healthz                  -> router.handleHealth     # 健康检查（K8s 探针）

# 待实现（后续 Sprint）
POST /v1/embeddings            -> handler.Embedding      # 向量化（Sprint 6）
POST /v1/images/generations    -> handler.ImageGen       # 图像生成（Sprint 7）
POST /v1/audio/speech          -> handler.TTS            # 语音合成（Sprint 7）
POST /v1/audio/transcriptions  -> handler.STT            # 语音识别（Sprint 7）
GET  /v1/tasks/{id}            -> handler.GetTask        # 异步任务查询（Sprint 7）
```

路由注册使用标准库 `http.ServeMux`，`api.Router` 实现 `http.Handler` 接口，
自带请求日志中间件（method/path/status/duration/remote）。`responseWriter` 包装
支持 `http.Flusher`，确保 SSE 流式响应正常刷新。

扩展字段（通过请求体传递）：
```json
{
  "model": "gpt-4o",
  "messages": [...],
  "provider": "openai",        // 可选：指定平台
  "model_tier": "large",       // 可选：按分层路由
  "credentials": {             // 可选：BYOK 动态凭证
    "api_key": "sk-xxx"
  },
  "extra": {                   // 可选：平台专属参数
    "thinking": {"type": "enabled", "budget_tokens": 2048}
  }
}
```

### 12.1 SDK 模式入口（pkg/gateway）

除 HTTP API 外，提供 Go SDK 模式，允许 Go 项目直接 import 使用，无需启动独立的 HTTP 服务。

```go
// pkg/gateway/client.go — SDK 模式入口

type Client struct {
    manager *manager.Manager
    logger  *slog.Logger
    opts    *clientOptions
}

// New 从配置文件创建 SDK Client
func New(cfgPath string, opts ...Option) (*Client, error) {
    cfg, err := config.Load(cfgPath)
    if err != nil {
        return nil, err
    }
    return NewWithConfig(cfg, opts...)
}

// NewWithConfig 从 Config 结构体创建 SDK Client
func NewWithConfig(cfg *config.Config, opts ...Option) (*Client, error) { ... }

// --- 核心方法 ---

func (c *Client) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) { ... }
func (c *Client) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) { ... }

// Responses API（OpenAI 推理模型优化接口）
func (c *Client) Responses(ctx context.Context, req *types.ResponsesRequest) (*types.ResponsesResponse, error) { ... }
func (c *Client) ResponsesStream(ctx context.Context, req *types.ResponsesRequest) (<-chan types.ResponsesStreamEvent, error) { ... }

// 查询方法
func (c *Client) ListModels() []types.ModelConfig { ... }
func (c *Client) ListProviders() []types.ProviderStatus { ... }

func (c *Client) Close() error { ... }
```

```go
// pkg/gateway/options.go — 函数式选项

type Option func(*clientOptions)

type clientOptions struct {
    cacheEnabled bool
    logger       *slog.Logger
}

func WithCache(enabled bool) Option { ... }
func WithLogger(l *slog.Logger) Option { ... }
// WithHook 待 Sprint 5 Hook 系统实现后添加
```

**SDK 使用示例**：

```go
package main

import (
    "context"
    "fmt"

    "github.com/lex1ng/llm-gateway/pkg/gateway"
    "github.com/lex1ng/llm-gateway/pkg/types"
)

func main() {
    client, err := gateway.New("config/config.yaml")
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 非流式对话
    resp, err := client.Chat(context.Background(), &types.ChatRequest{
        Model: "gpt-4o-mini",
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("Hello!")},
        },
    })
    fmt.Println(resp.Content)

    // 流式对话
    stream, _ := client.ChatStream(ctx, &types.ChatRequest{
        Model: "qwen-turbo", Stream: true,
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("写一首诗")},
        },
    })
    for event := range stream {
        if event.Type == types.StreamEventContentDelta {
            fmt.Print(event.Delta)
        }
    }

    // Responses API
    respAPI, _ := client.Responses(ctx, &types.ResponsesRequest{
        Model: "gpt-4o",
        Input: "What is 2+2?",
    })
    fmt.Println(respAPI.OutputText)
}
```

**HTTP 服务与 SDK 的关系**：

`cmd/server/main.go` 作为薄壳，仅做以下工作：
1. 解析命令行参数（`--config`、`--env`）
2. 加载 `.env` 文件（可选）
3. `gateway.New(cfg)` 创建 SDK Client
4. `api.NewRouter(client)` 将 Client 方法包装为 HTTP Handler
5. 启动 HTTP 服务

```go
// cmd/server/main.go — 薄壳

func main() {
    client, err := gateway.New(configPath)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    router := api.NewRouter(client)
    server := &http.Server{
        Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
        Handler: router,
    }
    server.ListenAndServe()
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
| 1.7 | OpenAI Compatible 适配器（阿里/火山/智谱/百度/MiniMax） | `pkg/adapter/openai/`（复用 `NewWithName()`） |
| 1.8 | Manager 编排层：路由、熔断器、限流 | `pkg/manager/` |
| 1.9 | 对外 API：`/v1/chat/completions` Handler | `api/` |
| 1.10 | 集成测试：各平台 Chat + Stream 全通 | `tests/` |
| 1.11 | **SDK 入口 `pkg/gateway/` 实现**（NEW） | `pkg/gateway/client.go`, `options.go` |
| 1.12 | **Hook 系统 `pkg/hook/` 基础实现**（NEW） | `pkg/hook/hook.go`, `registry.go` |

**Phase 1 交付标准**: 8 个平台的对话和流式全部跑通，支持 ModelTier 路由和自动降级。SDK 模式可独立使用。

### Phase 2 — Tool Calling + Embeddings（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 2.1 | Tool 类型定义：Tool, ToolCall, ToolResult | `pkg/types/tool.go` |
| 2.2 | Tool Mapper：OpenAI ↔ Anthropic ↔ Gemini 格式互转 | 各 `adapter/` 内联 |
| 2.3 | 各适配器支持 tools 参数和 tool_calls 响应 | 各 `adapter/` |
| 2.4 | Embedding 接口 + OpenAI/Compatible 实现 | `EmbeddingProvider` |
| 2.5 | Google Embeddings 适配（`embedContent` 格式） | `adapter/google/embedding.go` |
| 2.6 | 对外 API：`/v1/embeddings` | `api/handler/embedding.go` |
| 2.7 | **DualCache 实现（Memory + Redis）**（NEW） | `pkg/manager/cache.go` |
| 2.8 | 对冲请求实现 | `pkg/manager/hedger.go` |
| 2.9 | **预扣+结算配额**（NEW） | `pkg/manager/quota.go` |

**Phase 2 交付标准**: Tool Calling 在 OpenAI/Anthropic/兼容平台全通；Embeddings 可用；DualCache 和配额管理可用。

### Phase 3 — Agent + Workflow 调用（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 3.1 | Agent/Workflow 请求/响应类型定义 | `pkg/types/` |
| 3.2 | AgentProvider / WorkflowProvider 接口 | `pkg/provider/interface.go` |
| 3.3 | 阿里百炼 Agent/Workflow 适配（应用调用 API） | `adapter/openai/` 或独立 adapter |
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
| 5.7 | **异步消费批量写入**（NEW） | `pkg/manager/spend_writer.go` |
| 5.8 | **Per-Model 冷却增强**（NEW） | `pkg/manager/cooldown.go` |

---

## 14. 关键技术决策总结

| 决策 | 选择 | 理由 |
|------|------|------|
| 内部数据格式 | OpenAI 格式 | 事实标准，6/8 平台兼容 |
| Provider 架构 | Interface 组合 + 能力分桶注册 | 能力分离，注册期强校验，运行时零类型断言 |
| 国内平台策略 | 复用 OpenAI adapter + `NewWithName()` | 避免重复代码，Chat/Stream 完全一致，仅 baseURL 不同 |
| 消息转换 | 内联在各 adapter 中（非独立 Mapper 层） | 映射逻辑与私有 API 类型强耦合，独立包增加不必要复杂度 |
| 流式接口 | 统一 `<-chan StreamEvent` + 中途失败策略 | 一套接口 + 可配置的 failover 策略 |
| 异步任务 | 统一 Submit + Poll 模型 | 覆盖图像/视频所有异步场景 |
| 缓存安全 | 借鉴 Shannon 全套检查 | 不缓存截断/过滤/空响应 |
| 可靠性 | 熔断 + 对冲 + 重试矩阵 + 超时分级 + 幂等键 | 分层容错，防雪崩 + 防重复 |
| 可观测性 | OpenTelemetry + Prometheus + SLO | 全链路 trace + 指标 + 告警闭环 |
| 安全 | 多租户 RBAC + KMS 密钥 + PII 脱敏 + 审计日志 | 企业级合规要求 |
| HTTP 框架 | 标准库 `net/http`（`http.ServeMux`） | 轻量，Go 1.22+ 的 ServeMux 已支持方法匹配 |
| 对外 API | OpenAI 兼容格式 + Responses API | 上层业务零成本切换，推理模型有更优接口 |
| **SDK 模式** | `pkg/gateway` 作为 library 入口 | Go 项目可直接 import，无需 HTTP 开销 |
| **Hook 系统** | 生命周期回调接口 | 借鉴 LiteLLM CustomLogger，支持自定义监控/审计/过滤 |
| **缓存架构** | DualCache (Memory+Redis) | 借鉴 LiteLLM，本地低延迟 + 跨实例共享 |
| **配额管理** | 预扣+结算 | 借鉴 New-API，防止并发超额 |
| **动态凭证** | Request-level BYOK | 借鉴 TensorZero，支持客户自带 Key |
| **Per-Provider Proxy** | 每个 provider 独立配置 proxy | 海外走代理、国内直连的混合策略 |
| **错误处理** | 四级错误分类 (Retry/RotateKey/Fallback/Abort) | 借鉴 LLM-API-Key-Proxy，精确区分重试/轮换/降级/中止 |
| **冷却粒度** | Provider+Key+Model | 借鉴 LLM-API-Key-Proxy，细粒度冷却避免误伤 |
| **Deadline 驱动** | 全局时间预算贯穿生命周期 | 借鉴 LLM-API-Key-Proxy，所有重试共享 deadline |
