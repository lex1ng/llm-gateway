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
│       ├── http_client.go            # 统一 HTTP 客户端（重试/超时/日志）
│       ├── auth.go                   # 上游 Provider 认证策略（Bearer/x-api-key/access_token）
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
    // --- 接口级能力（与 Provider 接口一一对应，注册时自动检测） ---
    CapChat      Capability = "chat"
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
    CapChat:     reflect.TypeOf((*ChatProvider)(nil)).Elem(),
    CapEmbed:    reflect.TypeOf((*EmbeddingProvider)(nil)).Elem(),
    CapImageGen: reflect.TypeOf((*ImageGenProvider)(nil)).Elem(),
    CapVideoGen: reflect.TypeOf((*VideoGenProvider)(nil)).Elem(),
    CapTTS:      reflect.TypeOf((*TTSProvider)(nil)).Elem(),
    CapSTT:      reflect.TypeOf((*STTProvider)(nil)).Elem(),
    CapAgent:    reflect.TypeOf((*AgentProvider)(nil)).Elem(),
    CapWorkflow: reflect.TypeOf((*WorkflowProvider)(nil)).Elem(),
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
    registry        *Registry
    router          *Router
    cache           *DualCache                  // 升级为 DualCache（Memory + Redis）
    circuitBreakers map[string]*CircuitBreaker
    cooldowns       *CooldownManager            // Per-Model 冷却（NEW）
    rateLimiters    map[string]*RateLimiter
    tokenCounter    *TokenCounter
    quotaManager    *QuotaManager               // 预扣+结算配额（NEW）
    spendWriter     *SpendWriter                // 异步消费批量写入（NEW）
    hookRegistry    *hook.Registry              // Hook 调度器（NEW）
    metrics         *Metrics
    config          *Config
}

// Chat 统一对话入口
func (m *Manager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 选择 provider（路由返回类型安全的 ChatProvider，无需调用方断言）
    cp, model, err := m.router.SelectChat(req)
    if err != nil { return nil, err }

    // 2. 限流检查
    if err := m.rateLimiters[cp.Name()].Allow(); err != nil { ... }

    // 3. 缓存查询（非流式、非 tool calling 时）
    if !req.Stream && len(req.Tools) == 0 {
        if cached := m.cache.Get(req); cached != nil { return cached, nil }
    }

    // 4. Token headroom 计算
    req.MaxTokens = m.tokenCounter.ClampMaxTokens(req, model)

    // 5. 调用 provider（带熔断器保护，cp 已是 ChatProvider 无需断言）
    resp, err := m.circuitBreakers[cp.Name()].Execute(func() (*ChatResponse, error) {
        return cp.Chat(ctx, req)
    })

    // 6. 失败降级：尝试下一个 provider
    if err != nil && isTransient(err) {
        resp, err = m.fallbackChat(ctx, req, cp.Name())
    }

    // 7. 缓存写入（带安全检查）
    if err == nil && m.cache.IsSafeToCache(resp) {
        m.cache.Set(req, resp)
    }

    // 8. 指标上报
    m.metrics.RecordRequest(cp.Name(), model, resp, err)

    return resp, err
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

// ExecuteWithDeadline Deadline 驱动的重试执行器
func (r *Retrier) ExecuteWithDeadline(ctx context.Context, fn func() (*ChatResponse, error)) (*ChatResponse, error) {
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

func (r *Registry) Dispatch(ctx context.Context, phase Phase, event *HookEvent) {
    r.mu.RLock()
    hooks := r.hooks[phase]
    r.mu.RUnlock()

    for _, h := range hooks {
        if err := h.Execute(ctx, event); err != nil {
            // Hook 执行失败不阻塞主流程，仅记录日志
            slog.Warn("hook execution failed", "hook", h.Name(), "phase", phase, "error", err)
        }
    }
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

// 自定义过滤 Hook（示例）
type ContentFilterHook struct{}
func (h *ContentFilterHook) Name() string { return "content_filter" }
func (h *ContentFilterHook) Phase() Phase { return PhasePreCall }
func (h *ContentFilterHook) Execute(ctx context.Context, event *HookEvent) error {
    // 检查请求内容，必要时拦截
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
    queue    chan SpendUpdate
    interval time.Duration  // 默认 60s
    batchSize int           // 默认 100
    db       SpendStorage
    done     chan struct{}
}

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

func NewSpendWriter(db SpendStorage, interval time.Duration, batchSize int) *SpendWriter {
    sw := &SpendWriter{
        queue:     make(chan SpendUpdate, 10000),
        interval:  interval,
        batchSize: batchSize,
        db:        db,
        done:      make(chan struct{}),
    }
    go sw.run()
    return sw
}

func (sw *SpendWriter) Record(update SpendUpdate) {
    select {
    case sw.queue <- update:
    default:
        // 队列满时丢弃，记录指标
        metrics.SpendWriterDropped.Inc()
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
        slog.Error("spend batch update failed", "count", len(merged), "error", err)
    }
}

func (sw *SpendWriter) Close() {
    close(sw.done)
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
    GetQuota(ctx context.Context, tenantID string) (*TenantQuota, error)
    PreConsume(ctx context.Context, tenantID string, estimated int) (quotaID string, err error)
    Settle(ctx context.Context, quotaID string, actual int) error
    Rollback(ctx context.Context, quotaID string) error
}

type TenantQuota struct {
    TenantID      string
    DailyLimit    int64    // 日 token 限额
    MonthlyLimit  float64  // 月费用限额 (USD)
    DailyUsed     int64
    MonthlySpent  float64
}

// PreConsume 按估算 token 预扣额度
// 返回 quotaID 用于后续结算
func (q *QuotaManager) PreConsume(ctx context.Context, tenantID string, estimatedTokens int) (quotaID string, err error) {
    quota, err := q.store.GetQuota(ctx, tenantID)
    if err != nil {
        return "", err
    }

    // 检查是否超额
    if quota.DailyLimit > 0 && quota.DailyUsed+int64(estimatedTokens) > quota.DailyLimit {
        return "", ErrDailyQuotaExceeded
    }

    // 预扣
    return q.store.PreConsume(ctx, tenantID, estimatedTokens)
}

// Settle 请求完成后按实际用量结算，退回差额
func (q *QuotaManager) Settle(ctx context.Context, quotaID string, actualTokens int) error {
    return q.store.Settle(ctx, quotaID, actualTokens)
}

// Rollback 请求失败时退回预扣额度
func (q *QuotaManager) Rollback(ctx context.Context, quotaID string) error {
    return q.store.Rollback(ctx, quotaID)
}
```

使用流程：

```go
// Manager.Chat 中的配额处理
func (m *Manager) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
    // 1. 估算 token
    estimatedTokens := m.tokenCounter.Estimate(req)

    // 2. 预扣配额
    quotaID, err := m.quotaManager.PreConsume(ctx, req.TenantID, estimatedTokens)
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
    m.quotaManager.Settle(ctx, quotaID, resp.Usage.TotalTokens)

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

// --- 请求维度 ---
// llm_gateway_requests_total{provider, model, tier, status, tenant_id}
// llm_gateway_request_duration_seconds{provider, model, tier, quantile}  -- histogram
// llm_gateway_ttft_seconds{provider, model, tier, quantile}              -- Time To First Token (流式)

// --- Token 与成本 ---
// llm_gateway_tokens_total{provider, model, direction="input|output", tenant_id}
// llm_gateway_cost_usd_total{provider, model, tenant_id}

// --- 可靠性 ---
// llm_gateway_retry_total{provider, reason}
// llm_gateway_fallback_total{from_provider, to_provider}
// llm_gateway_circuit_breaker_state{provider}  -- gauge: 0=closed, 1=open, 2=half_open
// llm_gateway_stream_failures_total{provider, strategy}

// --- 缓存 ---
// llm_gateway_cache_hits_total / llm_gateway_cache_misses_total

// --- 租户预算 ---
// llm_gateway_tenant_budget_remaining{tenant_id, type="token|spend"}
// llm_gateway_tenant_rate_limit_rejected_total{tenant_id}
```

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

// DynamicAuth 动态凭证支持（借鉴 TensorZero BYOK）
// 如果 ChatRequest 携带 credentials["api_key"]，优先使用动态凭证
// 否则 fallback 到静态配置
type DynamicAuth struct {
    StaticAuth AuthStrategy           // 静态配置的认证策略
}

func (a *DynamicAuth) ApplyWithCredentials(req *http.Request, credentials map[string]string) error {
    // 优先使用动态凭证
    if apiKey, ok := credentials["api_key"]; ok && apiKey != "" {
        req.Header.Set("Authorization", "Bearer "+apiKey)
        return nil
    }
    // Fallback 到静态配置
    return a.StaticAuth.Apply(req)
}

// Provider 调用时的凭证解析
func (p *Provider) resolveAuth(req *types.ChatRequest) AuthStrategy {
    if len(req.Credentials) > 0 {
        return &DynamicAuth{StaticAuth: p.defaultAuth}
    }
    return p.defaultAuth
}
```

> **BYOK（Bring Your Own Key）场景**：企业客户可在请求中携带自己的 API Key，Gateway 使用客户提供的凭证调用上游 Provider。适用于：客户有自己的平台账号和计费、客户需要使用特定区域的 endpoint 等。

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
    tenants map[string]*Tenant // keyHash -> tenant
}

// Authenticate 从请求中提取 API Key，验证身份
func (a *Authorizer) Authenticate(apiKey string) (*Tenant, error) {
    for _, t := range a.tenants {
        for _, k := range t.APIKeys {
            if k.ExpireAt.IsZero() || k.ExpireAt.After(time.Now()) {
                if bcrypt.CompareHashAndPassword([]byte(k.Hash), []byte(apiKey)) == nil {
                    return t, nil
                }
            }
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

  # 密钥管理
  secret_provider: "env"

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

// Client SDK 核心入口
type Client struct {
    manager      *manager.Manager
    hookRegistry *hook.Registry
    config       *Config
}

// New 创建 SDK Client
func New(cfgPath string, opts ...Option) (*Client, error) {
    cfg, err := config.Load(cfgPath)
    if err != nil {
        return nil, fmt.Errorf("load config: %w", err)
    }

    c := &Client{
        hookRegistry: hook.NewRegistry(),
        config:       applyOptions(cfg, opts),
    }

    // 初始化 Manager（核心编排层）
    c.manager, err = manager.New(cfg, c.hookRegistry)
    if err != nil {
        return nil, fmt.Errorf("init manager: %w", err)
    }

    return c, nil
}

// Chat 对话（非流式）
func (c *Client) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
    return c.manager.Chat(ctx, req)
}

// ChatStream 对话（流式）
func (c *Client) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan *types.StreamEvent, error) {
    return c.manager.ChatStream(ctx, req)
}

// Embedding 向量化
func (c *Client) Embedding(ctx context.Context, req *types.EmbeddingRequest) (*types.EmbeddingResponse, error) {
    return c.manager.Embed(ctx, req)
}

// Close 关闭客户端，释放资源
func (c *Client) Close() error {
    return c.manager.Close()
}
```

```go
// pkg/gateway/options.go — 函数式选项

type Option func(*clientOptions)

type clientOptions struct {
    cacheEnabled bool
    hooks        []hook.Hook
    logger       *slog.Logger
}

// WithCache 启用/禁用缓存
func WithCache(enabled bool) Option {
    return func(o *clientOptions) {
        o.cacheEnabled = enabled
    }
}

// WithHook 注册自定义 Hook
func WithHook(h hook.Hook) Option {
    return func(o *clientOptions) {
        o.hooks = append(o.hooks, h)
    }
}

// WithLogger 设置日志器
func WithLogger(l *slog.Logger) Option {
    return func(o *clientOptions) {
        o.logger = l
    }
}
```

**SDK 使用示例**：

```go
package main

import (
    "context"
    "fmt"

    "github.com/company/llm-gateway/pkg/gateway"
    "github.com/company/llm-gateway/pkg/types"
)

func main() {
    // 1. 创建 SDK Client
    client, err := gateway.New("config/models.yaml",
        gateway.WithCache(true),
        gateway.WithLogger(slog.Default()),
    )
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 2. 发起对话请求
    resp, err := client.Chat(context.Background(), &types.ChatRequest{
        Model: "gpt-4o",
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("Hello, who are you?")},
        },
    })
    if err != nil {
        panic(err)
    }

    fmt.Println(resp.Content)
}
```

**HTTP 服务与 SDK 的关系**：

`cmd/server/main.go` 作为薄壳，仅做以下工作：
1. 加载配置
2. `gateway.New(cfg)` 创建 SDK Client
3. 将 Client 方法包装为 HTTP Handler
4. 启动 HTTP 服务

```go
// cmd/server/main.go — 薄壳示例

func main() {
    // 1. 创建 SDK Client
    client, err := gateway.New("config/models.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 2. 包装为 HTTP Handler
    handler := api.NewHandler(client)

    // 3. 启动 HTTP 服务
    http.ListenAndServe(":8080", handler)
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
| 1.11 | **SDK 入口 `pkg/gateway/` 实现**（NEW） | `pkg/gateway/client.go`, `options.go` |
| 1.12 | **Hook 系统 `pkg/hook/` 基础实现**（NEW） | `pkg/hook/hook.go`, `registry.go` |

**Phase 1 交付标准**: 8 个平台的对话和流式全部跑通，支持 ModelTier 路由和自动降级。SDK 模式可独立使用。

### Phase 2 — Tool Calling + Embeddings（1~2 周）

| 步骤 | 内容 | 产出 |
|------|------|------|
| 2.1 | Tool 类型定义：Tool, ToolCall, ToolResult | `pkg/types/tool.go` |
| 2.2 | Tool Mapper：OpenAI ↔ Anthropic ↔ Gemini 格式互转 | `pkg/mapper/tool.go` |
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
| 5.7 | **异步消费批量写入**（NEW） | `pkg/manager/spend_writer.go` |
| 5.8 | **Per-Model 冷却增强**（NEW） | `pkg/manager/cooldown.go` |

---

## 14. 关键技术决策总结

| 决策 | 选择 | 理由 |
|------|------|------|
| 内部数据格式 | OpenAI 格式 | 事实标准，6/8 平台兼容 |
| Provider 架构 | Interface 组合 + 能力分桶注册 | 能力分离，注册期强校验，运行时零类型断言 |
| 国内平台策略 | 共用 Compatible 适配器 + 基础兼容能力 | 避免重复代码，差异用 Quirks 处理（非差异化优势） |
| 消息转换 | 独立 Mapper 层 | 可复用，不散落在 provider 中 |
| 流式接口 | 统一 `<-chan StreamEvent` + 中途失败策略 | 一套接口 + 可配置的 failover 策略 |
| 异步任务 | 统一 Submit + Poll 模型 | 覆盖图像/视频所有异步场景 |
| 缓存安全 | 借鉴 Shannon 全套检查 | 不缓存截断/过滤/空响应 |
| 可靠性 | 熔断 + 对冲 + 重试矩阵 + 超时分级 + 幂等键 | 分层容错，防雪崩 + 防重复 |
| 可观测性 | OpenTelemetry + Prometheus + SLO | 全链路 trace + 指标 + 告警闭环 |
| 安全 | 多租户 RBAC + KMS 密钥 + PII 脱敏 + 审计日志 | 企业级合规要求 |
| HTTP 框架 | 标准库 `net/http` 或 chi | 轻量，不过度引入依赖 |
| 对外 API | OpenAI 兼容格式 | 上层业务零成本切换 |
| **SDK 模式** | `pkg/gateway` 作为 library 入口 | Go 项目可直接 import，无需 HTTP 开销 |
| **Hook 系统** | 生命周期回调接口 | 借鉴 LiteLLM CustomLogger，支持自定义监控/审计/过滤 |
| **缓存架构** | DualCache (Memory+Redis) | 借鉴 LiteLLM，本地低延迟 + 跨实例共享 |
| **配额管理** | 预扣+结算 | 借鉴 New-API，防止并发超额 |
| **动态凭证** | Request-level BYOK | 借鉴 TensorZero，支持客户自带 Key |
| **错误处理** | 四级错误分类 (Retry/RotateKey/Fallback/Abort) | 借鉴 LLM-API-Key-Proxy，精确区分重试/轮换/降级/中止 |
| **冷却粒度** | Provider+Key+Model | 借鉴 LLM-API-Key-Proxy，细粒度冷却避免误伤 |
| **Deadline 驱动** | 全局时间预算贯穿生命周期 | 借鉴 LLM-API-Key-Proxy，所有重试共享 deadline |
