# LLM Gateway — 详细开发计划

> 配套文档：[architecture-design.md](./architecture-design.md)
>
> 本文档将架构设计拆解为可逐步执行的开发任务，每个任务标注了**输入依赖**、**产出文件**、**验收标准**和**对应设计文档章节**，使开发者无需口头沟通即可独立完成。

---

## 0. 开发策略

### 0.1 核心原则：垂直切片 + 自底向上

```
一条完整请求链路（垂直切片）：

  用户请求 → Types → Manager → Router → Provider → Transport → 上游API
       ↑                                                        │
       └──────────── 响应/流式事件 ←─────────────────────────────┘
```

**不是**把每一层做完再做下一层，而是：
1. 先用最小代码打通一条完整链路（OpenAI Chat 非流式）
2. 在同一条链路上加流式
3. 横向扩展到其他 Provider
4. 再往 Manager 上叠加编排能力（熔断、缓存、限流…）

这样每一步都能端到端验证，避免"底层写了一堆但没法测"的困境。

### 0.2 依赖关系图

```
  config ─────────────────────────────────────────────────┐
    │                                                      │
    ▼                                                      ▼
  types ──────► provider/interface ──► adapter/* ──► transport
    │                │                    │
    │                ▼                    │
    │           provider/registry ◄──────┘
    │                │
    │                ▼
    │            manager (最小版)
    │                │
    │     ┌──────────┼──────────┐
    │     ▼          ▼          ▼
    │   router    retry    circuit_breaker  ...（可选编排组件）
    │                │
    │                ▼
    │            hook/registry
    │                │
    │                ▼
    │           gateway/client (SDK)
    │                │
    ▼                ▼
  api/handler ← cmd/server (薄壳)
```

### 0.3 开发模式

每个任务遵循统一流程：

1. **接口先行**：先定义 interface / struct / 函数签名
2. **单元测试**：写测试（至少覆盖正常路径 + 一个错误路径）
3. **实现**：填充实现代码
4. **集成验证**：与已完成的上下游组件联调
5. **代码审查**：PR 合并前需通过 lint + test

---

## 1. Sprint 1 — 地基层：Types + Config + Transport

> 目标：定义所有核心数据结构，实现 HTTP 通信基础设施，让后续 Provider 开发有类型可用、有 HTTP 客户端可用。

### Task 1.1 项目初始化

| 项 | 内容 |
|---|------|
| **产出** | `go.mod`, `.gitignore`, `Makefile`, 目录骨架 |
| **依赖** | 无 |
| **设计文档** | Section 3（目录结构） |

**具体步骤**：

```bash
# 1. 初始化 Go module
go mod init github.com/company/llm-gateway

# 2. 创建目录骨架（空 .gitkeep 占位）
mkdir -p cmd/server
mkdir -p config
mkdir -p pkg/{types,provider,adapter/{openai,anthropic,google,compatible},mapper,manager,gateway,hook,transport,observability,auth,secret,audit}
mkdir -p api/{handler,middleware}
mkdir -p tests/{integration,e2e}
```

```makefile
# Makefile
.PHONY: build test lint

build:
	go build -o bin/llm-gateway ./cmd/server

test:
	go test ./... -v -race -count=1

lint:
	golangci-lint run ./...

test-integration:
	go test ./tests/integration/... -v -tags=integration
```

**验收标准**：
- [ ] `go build ./...` 无报错
- [ ] `make test` 可执行（虽然还没有测试）
- [ ] 目录结构与设计文档 Section 3 一致

---

### Task 1.2 核心类型定义

| 项 | 内容 |
|---|------|
| **产出** | `pkg/types/` 下所有文件 |
| **依赖** | Task 1.1 |
| **设计文档** | Section 4（核心类型设计） |

**文件清单与职责**：

| 文件 | 定义内容 | 关键点 |
|------|---------|--------|
| `message.go` | `Role`, `Message`, `Content`, `ContentBlock`, `Image`, `Audio` | `Content` 需实现自定义 JSON Marshal/Unmarshal（string 和 []ContentBlock 双态） |
| `request.go` | `ChatRequest`, `EmbedRequest`, `ImageGenRequest`, `VideoGenRequest`, `TTSRequest`, `STTRequest`, `AgentRequest`, `WorkflowRequest` | `ChatRequest` 包含 `Credentials` 字段（BYOK） |
| `response.go` | `ChatResponse`, `EmbedResponse`, `StreamEvent` | `StreamEvent.Type` 枚举：content_delta / tool_call_delta / usage / done / error |
| `tool.go` | `Tool`, `ToolCall`, `ToolResult` | 遵循 OpenAI function calling 格式 |
| `usage.go` | `TokenUsage`, `CostCalculator` | 包含 `PromptTokens`, `CompletionTokens`, `TotalTokens` |
| `model.go` | `ModelTier`, `ModelConfig`, `ModelCapabilities` | Tier: small/medium/large |
| `error.go` | `ErrorCode`, `ProviderError`, `ErrorAction` | 四级错误分类：Retry/RotateKey/Fallback/Abort |
| `async_task.go` | `TaskStatus`, `AsyncTask` | 图像/视频异步任务结构 |

**关键实现细节**：

```go
// Content 的 JSON 双态序列化是最复杂的部分
// 输入 "content": "hello" → Content{Text: "hello"}
// 输入 "content": [{"type":"text","text":"hello"}] → Content{Blocks: [...]}

func (c *Content) UnmarshalJSON(data []byte) error {
    // 尝试 string
    var s string
    if json.Unmarshal(data, &s) == nil {
        c.Text = s
        return nil
    }
    // 尝试 []ContentBlock
    return json.Unmarshal(data, &c.Blocks)
}

func (c Content) MarshalJSON() ([]byte, error) {
    if len(c.Blocks) > 0 {
        return json.Marshal(c.Blocks)
    }
    return json.Marshal(c.Text)
}
```

**验收标准**：
- [ ] 所有类型有对应的单元测试
- [ ] `Content` 的 JSON 序列化/反序列化双向测试通过
- [ ] `ProviderError` 实现 `error` interface
- [ ] `go vet ./pkg/types/...` 无警告

---

### Task 1.3 配置结构定义与加载

| 项 | 内容 |
|---|------|
| **产出** | `config/config.go`, `config/config.yaml`, `config/models.yaml` |
| **依赖** | Task 1.2（引用 types.ModelConfig） |
| **设计文档** | Section 11（配置文件设计） |

**config.go 核心结构**：

```go
type Config struct {
    Server    ServerConfig              `yaml:"server"`
    Providers map[string]ProviderConfig `yaml:"providers"`
    Models    []types.ModelConfig       `yaml:"model_catalog"`
    Routing   RoutingConfig             `yaml:"tier_routing"`
    Manager   ManagerConfig             `yaml:"manager"`
    Security  SecurityConfig            `yaml:"security"`
    Observability ObservabilityConfig   `yaml:"observability"`
}

type ProviderConfig struct {
    BaseURL  string `yaml:"base_url"`
    APIKey   string `yaml:"api_key"`   // 支持 ${ENV_VAR} 引用
    Platform string `yaml:"platform"`  // 兼容平台标识
    RateLimit int   `yaml:"rate_limit"`
}

func Load(path string) (*Config, error)         // 加载配置文件
func resolveEnvVars(s string) string             // 解析 ${VAR} 环境变量
func Validate(cfg *Config) error                 // 校验必填项
```

**验收标准**：
- [ ] 可正确加载 YAML 配置
- [ ] 支持 `${ENV_VAR}` 环境变量替换
- [ ] 缺少必填项时返回明确错误
- [ ] 提供默认配置模板 `config.yaml` 和 `models.yaml`

---

### Task 1.4 Transport 层

| 项 | 内容 |
|---|------|
| **产出** | `pkg/transport/http_client.go`, `auth.go`, `sse.go` |
| **依赖** | Task 1.2（引用 types.ProviderError） |
| **设计文档** | Section 10.1（Provider 认证策略），Section 9（SSE 解析） |

**三个文件职责**：

**http_client.go** — 统一 HTTP 客户端：

```go
type Client struct {
    http     *http.Client
    auth     AuthStrategy
    baseURL  string
    logger   *slog.Logger
}

// Do 发送请求，返回原始响应。自动添加认证 header。
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error)

// DoJSON 发送 JSON 请求，反序列化响应到 target。
func (c *Client) DoJSON(ctx context.Context, method, path string, body, target any) error

// DoStream 发送请求，返回 SSE 流读取器。
func (c *Client) DoStream(ctx context.Context, method, path string, body any) (*SSEReader, error)
```

**auth.go** — 认证策略（4 种）：

```go
type AuthStrategy interface {
    Apply(req *http.Request) error
}

type BearerAuth struct{ APIKey string }                     // OpenAI / 国内平台
type AnthropicAuth struct{ APIKey, Version string }         // x-api-key + anthropic-version
type GoogleAuth struct{ APIKey string }                      // ?key= query param
type DynamicAuth struct{ Static AuthStrategy; Creds map[string]string } // BYOK 动态凭证（实现 AuthStrategy 接口）
```

**sse.go** — SSE 通用解析器：

```go
type SSEReader struct {
    reader *bufio.Reader
}

type SSEEvent struct {
    Event string // 事件类型（Anthropic 用 event: 字段）
    Data  string // 数据（data: 字段）
}

func NewSSEReader(body io.Reader) *SSEReader
func (r *SSEReader) Read() (*SSEEvent, error)    // 读取下一个事件，EOF 时返回 io.EOF
func (r *SSEReader) Close() error
```

**验收标准**：
- [ ] `BearerAuth` 设置 `Authorization: Bearer xxx` header
- [ ] `AnthropicAuth` 设置 `x-api-key` + `anthropic-version` header
- [ ] `GoogleAuth` 追加 `?key=xxx` query param
- [ ] `DynamicAuth` 实现 `AuthStrategy` 接口（Apply 方法），优先使用 credentials，fallback 到 static
- [ ] SSEReader 能正确解析多行 `data:` 和 `event:` 字段
- [ ] SSEReader 遇到 `data: [DONE]` 正确处理
- [ ] HTTP Client 设置合理的默认超时（connect=5s, total=120s）

---

## 2. Sprint 2 — 第一条垂直切片：OpenAI Chat 全通

> 目标：从 Provider 接口定义 → OpenAI 适配器 → 最小 Manager → HTTP Handler，打通第一条完整链路。

### Task 2.1 Provider 接口定义

| 项 | 内容 |
|---|------|
| **产出** | `pkg/provider/interface.go`, `capability.go`, `registry.go` |
| **依赖** | Task 1.2 |
| **设计文档** | Section 5（能力接口设计） |

**interface.go**：定义所有能力接口（此时只需实现 `Provider` + `ChatProvider`，其余先占位）。

```go
// 基础接口
type Provider interface {
    Name() string
    Models() []types.ModelConfig
    Supports(cap Capability) bool
    Close() error
}

// P0 能力
type ChatProvider interface {
    Provider
    Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)
    ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error)
}

// P1+ 能力接口先定义签名，不需要实现
type EmbeddingProvider interface { ... }
type ImageGenProvider interface { ... }
// ...
```

**capability.go**：能力枚举 + `capInterfaceMap`（reflect 映射表）。

**registry.go**：能力分桶注册表。

- `Register(p Provider) error` — 自动检测接口实现 + Supports() 一致性校验
- `GetChatProvider(name string) (ChatProvider, bool)` — 类型安全获取
- `GetByModel(modelID string) (Provider, bool)` — 按模型名反查
- `GetForTier(tier ModelTier) []TierEntry` — 按分层获取

**验收标准**：
- [ ] Register 时自动检测 provider 实现了哪些接口
- [ ] Supports() 与实际接口实现不一致时 Register 返回 error
- [ ] GetChatProvider 返回类型安全的 ChatProvider，无需调用方断言
- [ ] 单元测试覆盖：注册/查询/一致性校验失败

---

### Task 2.2 OpenAI 适配器（Chat + Stream）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/adapter/openai/provider.go`, `chat.go`, `stream.go`, `mapper.go` |
| **依赖** | Task 1.4（Transport），Task 2.1（Provider 接口） |
| **设计文档** | Section 6.1（OpenAI 适配） |

**这是第一个 Provider 实现，也是最简单的**——因为内部标准就是 OpenAI 格式，几乎不需要转换。

**provider.go**：

```go
type OpenAIProvider struct {
    client *transport.Client
    models []types.ModelConfig
}

func New(cfg config.ProviderConfig, models []types.ModelConfig) (*OpenAIProvider, error)
func (p *OpenAIProvider) Name() string { return "openai" }
func (p *OpenAIProvider) Models() []types.ModelConfig { return p.models }
func (p *OpenAIProvider) Supports(cap provider.Capability) bool { ... }
func (p *OpenAIProvider) Close() error { return nil }
```

**chat.go** — 非流式对话：

```go
func (p *OpenAIProvider) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
    // 1. 构建 OpenAI 请求体（几乎透传，req.Stream 强制 false）
    // 2. POST /chat/completions
    // 3. 解析响应 → types.ChatResponse
    // 4. 错误处理：HTTP 状态码 → ProviderError
}
```

**stream.go** — 流式对话：

```go
func (p *OpenAIProvider) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
    // 1. 构建请求体（req.Stream = true）
    // 2. POST /chat/completions，获取 SSEReader
    // 3. 启动 goroutine 读取 SSE 事件
    // 4. 每个事件解析为 StreamEvent 发送到 channel
    // 5. 遇到 [DONE] 关闭 channel
    // 6. 遇到错误发送 error 类型的 StreamEvent
}
```

**验收标准**（需要真实 API Key 或 Mock Server）：
- [ ] 非流式：发送简单对话请求，收到完整响应
- [ ] 流式：发送流式请求，逐 chunk 收到 content_delta 事件，最后收到 done
- [ ] 错误处理：无效 API Key → ProviderError{StatusCode: 401}
- [ ] 错误处理：不存在的模型 → ProviderError{StatusCode: 404}
- [ ] Usage 字段正确填充 prompt_tokens/completion_tokens

---

### Task 2.3 最小 Manager（仅路由，无编排）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/manager.go`, `router.go` |
| **依赖** | Task 2.1（Registry），Task 2.2（OpenAI Provider） |
| **设计文档** | Section 7.1 + 7.2（Manager 核心 + 路由器） |

**首版 Manager 极简**——只做路由分发，不做熔断/缓存/限流：

```go
type Manager struct {
    registry *provider.Registry
    router   *Router
    config   *config.Config
}

func New(cfg *config.Config) (*Manager, error) {
    // 1. 创建 Registry
    // 2. 根据 config 注册各 provider（此时只有 OpenAI）
    // 3. 创建 Router
}

func (m *Manager) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
    cp, model, err := m.router.SelectChat(req)
    if err != nil { return nil, err }
    req.Model = model
    return cp.Chat(ctx, req)
}

func (m *Manager) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
    cp, model, err := m.router.SelectChat(req)
    if err != nil { return nil, err }
    req.Model = model
    return cp.ChatStream(ctx, req)
}
```

**Router 首版**：只支持按 model 名直接查找，暂不做 Tier 路由。

**验收标准**：
- [ ] `Manager.Chat()` → 通过 Router 找到 OpenAI → 调用成功
- [ ] 指定不存在的 model → 返回 ErrModelNotFound
- [ ] 指定不存在的 provider → 返回 ErrProviderNotFound

---

### Task 2.4 SDK 入口（最小版）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/gateway/client.go`, `options.go` |
| **依赖** | Task 2.3（Manager） |
| **设计文档** | Section 12.1（SDK 模式入口） |

**首版 Client 是 Manager 的薄壳**：

```go
type Client struct {
    manager *manager.Manager
}

func New(cfgPath string, opts ...Option) (*Client, error) {
    cfg, err := config.Load(cfgPath)
    // ...
    mgr, err := manager.New(cfg)
    return &Client{manager: mgr}, nil
}

func (c *Client) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
    return c.manager.Chat(ctx, req)
}

func (c *Client) ChatStream(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error) {
    return c.manager.ChatStream(ctx, req)
}

func (c *Client) Close() error { return c.manager.Close() }
```

**验收标准**：
- [ ] 可以通过 `gateway.New("config/models.yaml")` 创建 Client
- [ ] `client.Chat()` 端到端调通 OpenAI
- [ ] `client.Close()` 无 goroutine 泄漏

---

### Task 2.5 HTTP Handler + Server 薄壳

| 项 | 内容 |
|---|------|
| **产出** | `api/handler/chat.go`, `api/router.go`, `cmd/server/main.go` |
| **依赖** | Task 2.4（SDK Client） |
| **设计文档** | Section 12（对外 API 设计） |

**chat.go**：

```go
// POST /v1/chat/completions
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
    var req types.ChatRequest
    json.NewDecoder(r.Body).Decode(&req)

    if req.Stream {
        // SSE 响应
        w.Header().Set("Content-Type", "text/event-stream")
        ch, err := h.client.ChatStream(r.Context(), &req)
        // 循环写 SSE
    } else {
        resp, err := h.client.Chat(r.Context(), &req)
        json.NewEncoder(w).Encode(resp)
    }
}
```

**cmd/server/main.go**：

```go
func main() {
    client, _ := gateway.New(os.Getenv("CONFIG_PATH"))
    defer client.Close()
    handler := api.NewHandler(client)
    log.Fatal(http.ListenAndServe(":8080", handler))
}
```

**验收标准**（用 curl 验证）：
- [ ] `curl -X POST localhost:8080/v1/chat/completions -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'` → 返回 JSON 响应
- [ ] 同上加 `"stream": true` → 返回 SSE 流
- [ ] 无效请求体 → 400 错误
- [ ] 无效模型 → 404 错误

---

### Sprint 2 完成检查点

此时应该有一个可以运行的 HTTP 服务，可以：
- 接收 OpenAI 兼容格式的请求
- 通过 OpenAI API 完成对话（流式和非流式）
- 通过 SDK 直接调用（不走 HTTP）

```
curl → HTTP Handler → gateway.Client → Manager → Router → OpenAI Provider → Transport → api.openai.com
```

---

## 3. Sprint 3 — 横向扩展：Anthropic + 国内兼容平台

> 目标：新增 Anthropic 原生适配 + Compatible 通用适配器，让多平台 Chat 全通。

### Task 3.1 消息转换层

| 项 | 内容 |
|---|------|
| **产出** | `pkg/mapper/message.go`, `stream.go` |
| **依赖** | Task 1.2 |
| **设计文档** | Section 8（消息转换层） |

```go
// message.go
func ToAnthropic(messages []types.Message) (system string, converted []AnthropicMessage)
func ToGemini(messages []types.Message) (systemInstruction *GeminiContent, contents []GeminiContent)

// stream.go
func ParseOpenAIStream(event *transport.SSEEvent) (*types.StreamEvent, error)
func ParseAnthropicStream(event *transport.SSEEvent) (*types.StreamEvent, error)
func ParseGeminiStream(event *transport.SSEEvent) (*types.StreamEvent, error)
```

**验收标准**：
- [ ] system 消息正确从 messages 中抽取到顶层
- [ ] role "assistant" 在 Gemini 格式中映射为 "model"
- [ ] Content 双态（string / []ContentBlock）在各格式中正确转换
- [ ] 每种流式解析器有独立的单元测试（用固定 SSE 文本作为输入）

---

### Task 3.2 Anthropic 适配器

| 项 | 内容 |
|---|------|
| **产出** | `pkg/adapter/anthropic/provider.go`, `chat.go`, `stream.go`, `mapper.go` |
| **依赖** | Task 3.1（Mapper），Task 1.4（Transport） |
| **设计文档** | Section 6.2（Anthropic 适配） |

**关键映射点**（参照设计文档）：
- `system` 在 messages 中 → 顶层 `system` 字段
- `max_tokens` 可选 → Anthropic 必填（默认 4096）
- `stop` → `stop_sequences`
- `finish_reason` ← `stop_reason`
- `prompt_tokens / completion_tokens` ← `input_tokens / output_tokens`
- `tool_choice: "auto"` → `tool_choice: {"type":"auto"}`
- 流式 SSE 事件格式完全不同（`content_block_delta` 等）

**验收标准**：
- [ ] 非流式：简单对话请求成功
- [ ] 流式：逐 chunk 接收内容
- [ ] system prompt 正确传递
- [ ] Usage tokens 正确映射
- [ ] 注册到 Registry 后通过 `manager.Chat()` 调用成功

---

### Task 3.3 Compatible 通用适配器（国内平台）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/adapter/compatible/provider.go`, `chat.go`, `stream.go`, `platforms.go` |
| **依赖** | Task 1.4（Transport） |
| **设计文档** | Section 6.4（OpenAI Compatible） |

**核心思路**：代码结构与 OpenAI 适配器几乎相同，区别仅在 baseURL 和 PlatformQuirks。

**platforms.go**：

```go
var PlatformPresets = map[Platform]PlatformConfig{
    PlatformAlibaba:    {BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1", ...},
    PlatformBaidu:      {BaseURL: "https://qianfan.baidubce.com/v2", ...},
    PlatformVolcengine: {BaseURL: "https://ark.cn-beijing.volces.com/api/v3", ...},
    PlatformZhipu:      {BaseURL: "https://open.bigmodel.cn/api/paas/v4", ...},
    PlatformMiniMax:    {BaseURL: "https://api.minimax.io/v1", ...},
}
```

**验收标准**：
- [ ] 阿里百炼：Chat + Stream 全通
- [ ] 至少再通一个国内平台（火山或智谱）
- [ ] 各平台通过同一个 `CompatibleProvider` 代码，仅 config 不同

---

### Task 3.4 Router 增强（Tier 路由 + Fallback）

| 项 | 内容 |
|---|------|
| **产出** | 更新 `pkg/manager/router.go` |
| **依赖** | Task 3.2 + 3.3（多个 Provider 注册后才有意义） |
| **设计文档** | Section 7.2（路由器） |

增加：
- ModelTier 路由：`req.ModelTier = "large"` → 按优先级找可用 provider
- fallback 逻辑：首选 provider 失败时尝试下一个

**验收标准**：
- [ ] `ModelTier: "large"` → 优先选 OpenAI gpt-4o，失败 fallback 到阿里 qwen-max
- [ ] 显式指定 `Provider: "anthropic"` → 直接走 Anthropic
- [ ] 显式指定 `Model: "qwen-turbo"` → 反查到阿里平台

---

### Sprint 3 完成检查点

此时 HTTP 服务可以：
- 对话：OpenAI / Anthropic / 阿里百炼 / 火山 / 智谱（Chat + Stream）
- 路由：按 model 名 / provider 名 / ModelTier 三种方式选路
- 降级：首选失败时自动 fallback

---

## 4. Sprint 4 — 编排层核心能力

> 目标：给 Manager 装上熔断、重试、超时、缓存等"中间件"。

### Task 4.1 熔断器

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/circuit_breaker.go` |
| **设计文档** | Section 7.3 |

三态状态机：Closed → Open → HalfOpen。实现 `Execute(fn) (resp, error)` 包装函数。

**验收标准**：
- [ ] 连续 N 次失败后进入 Open 状态，拒绝请求
- [ ] Open 状态持续 recoveryTimeout 后进入 HalfOpen
- [ ] HalfOpen 状态成功一次回到 Closed
- [ ] 并发安全（加锁测试）

---

### Task 4.2 Per-Model 冷却

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/cooldown.go` |
| **设计文档** | Section 7.4 |

冷却键：`provider:keyHash:model`。退避序列：10s → 30s → 60s → 120s → 300s。

**验收标准**：
- [ ] 同一 key 不同 model 独立冷却
- [ ] 成功后清除冷却状态
- [ ] 退避级别正确递增
- [ ] 与 CircuitBreaker 共存不冲突

---

### Task 4.3 重试策略（四级分类 + Deadline + Budget）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/retry.go` |
| **设计文档** | Section 7.5 |

关键：
- `ClassifyForRetry()` 返回四级 ErrorAction
- `ExecuteWithDeadline()` 在全局 deadline 内重试
- `RetryBudgetTracker` 滑动窗口追踪重试比例，超限时停止重试

**验收标准**：
- [ ] 429 → ActionRetry 或 ActionRotateKey（区分 key 级和全局）
- [ ] 401/403 → ActionRotateKey
- [ ] 400/422 → ActionAbort
- [ ] 404 → ActionFallback
- [ ] Deadline 到期后不再重试
- [ ] RetryBudgetTracker.AllowRetry() 在超过 budget 比例后返回 false
- [ ] RetryBudgetTracker 滑动窗口定期重置

---

### Task 4.4 超时分级

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/timeout.go` |
| **设计文档** | Section 7.6 |

按 ModelTier 设置不同超时。包含 connect / firstToken / totalNonStream / totalStream / idleBetweenChunks。

---

### Task 4.5 DualCache

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/cache.go` |
| **设计文档** | Section 7.10 |

读：memory → redis（命中回填 memory）。写：同时写 memory + redis。保留 `IsSafeToCache` 安全检查。

**验收标准**：
- [ ] 纯内存模式正常工作（不配置 redis）
- [ ] Memory + Redis 模式：miss memory → hit redis → 回填 memory
- [ ] 不缓存 finish_reason=length / content_filter / 空响应
- [ ] TTL 过期后缓存失效

---

### Task 4.6 Manager 集成编排组件

| 项 | 内容 |
|---|------|
| **产出** | 更新 `pkg/manager/manager.go` |
| **依赖** | Task 4.1 ~ 4.5 |

将熔断器、冷却、重试、超时、缓存集成到 Manager 的 `Chat()` 和 `ChatStream()` 流程中。

```go
func (m *Manager) Chat(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error) {
    // 1. 缓存查询
    // 2. 路由选择 provider
    // 3. 冷却检查
    // 4. 熔断器包装
    // 5. 带 deadline 的重试
    // 6. 失败 fallback
    // 7. 缓存写入
    // 8. 指标上报
}
```

**验收标准**：
- [ ] 正常请求端到端成功
- [ ] 模拟 provider 故障：触发熔断 → fallback 到备选
- [ ] 重复相同请求：第二次走缓存
- [ ] 超时请求：正确返回超时错误

---

## 5. Sprint 5 — Hook 系统 + 配额 + 消费记录

### Task 5.1 Hook 系统

| 项 | 内容 |
|---|------|
| **产出** | `pkg/hook/hook.go`, `registry.go` |
| **设计文档** | Section 7.11 |

六个阶段：PreRoute / PostRoute / PreCall / PostCall / OnError / OnSuccess。

**阻塞语义区分**：
- **前置 Hook**（PreRoute / PreCall）：返回 error 时中止请求（拦截器语义），适用于内容过滤、权限检查
- **后置 Hook**（PostCall / OnSuccess / OnError 等）：返回 error 仅记录日志，不影响主流程

**验收标准**：
- [ ] 注册多个 Hook 到同一 Phase，全部按序执行
- [ ] PreCall Hook 返回 error → 请求被中止，不调用 Provider
- [ ] PostCall Hook 返回 error → 仅记录日志，请求正常完成
- [ ] Hook panic 不影响主流程（defer recover）
- [ ] HookEvent 在各阶段填充正确的字段

---

### Task 5.1.5 费用计算器（CostCalculator）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/cost.go` |
| **设计文档** | Section 7.1（Manager struct） |

**核心接口**：
```go
type CostCalculator struct {
    pricing map[string]ModelPricing
}

// Estimate 预估费用（用于 PreConsume）
func (c *CostCalculator) Estimate(model string, estimatedTokens int) float64

// Calculate 计算实际费用（用于 Settle）
func (c *CostCalculator) Calculate(model string, inputTokens, outputTokens int) float64
```

**验收标准**：
- [ ] 从 pricing.yaml 加载模型定价配置
- [ ] Estimate() 保守估算（假设 50% 输入 50% 输出）
- [ ] Calculate() 精确计算（区分 input/output 费率）
- [ ] 未知模型返回 0（不限制）

---

### Task 5.2 配额管理（预扣+结算）

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/quota.go` |
| **设计文档** | Section 7.13 |

**关键接口**：QuotaStore 需支持 token + cost 原子性预扣/结算：
```go
type QuotaStore interface {
    PreConsume(ctx, tenantID, tokens int, cost float64) (quotaID, error)
    Settle(ctx, quotaID, actualTokens int, actualCost float64) error
    Rollback(ctx, quotaID) error
}
```

**验收标准**：
- [ ] PreConsume 预扣额度成功（同时检查日 token 限额和月费用限额）
- [ ] DailyLimit 超额时返回 ErrDailyQuotaExceeded
- [ ] MonthlyLimit 超额时返回 ErrMonthlyQuotaExceeded
- [ ] Settle 按实际用量结算，退回 token 差额 + cost 差额（原子性）
- [ ] Rollback 请求失败时全额退回（token + cost）
- [ ] 并发预扣不超额（用 race detector 测试）

---

### Task 5.3 异步消费批量写入

| 项 | 内容 |
|---|------|
| **产出** | `pkg/manager/spend_writer.go` |
| **设计文档** | Section 7.12 |

**关键实现**：
- `flush()` 失败时写入 WAL 兜底，不仅记录日志
- `Close() error` 返回错误，且关闭 WAL 文件句柄

**验收标准**：
- [ ] 按 interval 定时 flush
- [ ] 按 batchSize 提前 flush
- [ ] 队列满时降级为同步写入，不丢失计费数据
- [ ] 同步写入也失败时，写入本地 WAL 文件兜底
- [ ] **flush() 批量写入失败时，遍历写入 WAL（而非仅记录日志）**
- [ ] Close() 时 flush 残余数据
- [ ] **Close() 返回 error，且调用 wal.Close() 关闭文件**
- [ ] WAL 文件在服务重启后可回放补录

---

## 6. Sprint 6 — Tool Calling + Embeddings

### Task 6.1 Tool 类型 + Mapper

| 项 | 内容 |
|---|------|
| **产出** | `pkg/types/tool.go`, `pkg/mapper/tool.go` |
| **设计文档** | Section 4, Section 8 |

OpenAI / Anthropic / Gemini 三种 tool 格式互转。

---

### Task 6.2 各适配器 Tool Calling 支持

更新 OpenAI / Anthropic / Compatible 适配器，支持 `req.Tools` 和 `resp.ToolCalls`。

---

### Task 6.3 Embedding

| 项 | 内容 |
|---|------|
| **产出** | 各适配器 `embedding.go`, `api/handler/embedding.go` |
| **设计文档** | Section 5.1 |

---

## 7. Sprint 7 — 多媒体 + 异步任务

### Task 7.1 异步任务管理器

### Task 7.2 图像生成（OpenAI 同步 + 国内异步）

### Task 7.3 视频生成（全异步）

### Task 7.4 TTS / STT

### Task 7.5 Google Gemini 完整适配

---

## 8. Sprint 8 — 安全与可观测性

### Task 8.1 多租户认证 + RBAC

### Task 8.2 密钥管理（SecretProvider）

### Task 8.3 请求日志脱敏

### Task 8.4 审计日志

### Task 8.5 OpenTelemetry Tracing

### Task 8.6 Prometheus 指标

---

## 9. Sprint 9 — 生产化

### Task 9.1 配置热更新

### Task 9.2 Dockerfile + Helm Chart

### Task 9.3 压测与调优

### Task 9.4 幂等性支持

### Task 9.5 对冲请求

### Task 9.6 流式中途失败策略

---

## 附录 A：每个 Sprint 的依赖与并行度

```
Sprint 1 ─── Types + Config + Transport（地基，必须串行完成）
    │
    ▼
Sprint 2 ─── OpenAI 垂直切片（第一条完整链路）
    │
    ├──────────────────────┐
    ▼                      ▼
Sprint 3                Sprint 4
Anthropic +             编排层组件
国内平台适配              (熔断/重试/缓存)
    │                      │
    └──────────┬───────────┘
               ▼
Sprint 5 ─── Hook + 配额 + 消费记录
    │
    ├────────────────┐
    ▼                ▼
Sprint 6          Sprint 7
Tool Calling      多媒体
+ Embeddings      + 异步任务
    │                │
    └────────┬───────┘
             ▼
Sprint 8 ─── 安全 + 可观测性
    │
    ▼
Sprint 9 ─── 生产化
```

**关键洞察**：Sprint 3（更多 Provider）和 Sprint 4（编排能力）可以并行开发——前者扩展宽度，后者扩展深度，互不阻塞。

---

## 附录 B：测试策略

| 层级 | 测试类型 | 工具 | 说明 |
|------|---------|------|------|
| Types | 单元测试 | `go test` | JSON 序列化/反序列化、边界值 |
| Transport | 单元测试 + httptest | `httptest.NewServer` | Mock HTTP 响应测试 SSE 解析 |
| Adapter | 集成测试 | 真实 API / Mock | 需 API Key，用 build tag `integration` 隔离 |
| Manager | 单元测试 | Mock Provider | 用 mock provider 测试路由/熔断/缓存逻辑 |
| API Handler | HTTP 测试 | `httptest` | 验证请求解析和响应格式 |
| 端到端 | E2E 测试 | `docker-compose` + curl | 完整服务 + 至少一个真实 provider |

**Mock Provider 示例**：

```go
// tests/mock_provider.go
type MockChatProvider struct {
    ChatFunc   func(ctx context.Context, req *types.ChatRequest) (*types.ChatResponse, error)
    StreamFunc func(ctx context.Context, req *types.ChatRequest) (<-chan types.StreamEvent, error)
}
```

---

## 附录 C：关键里程碑

| 里程碑 | 标志 | 对应 Sprint |
|--------|------|-------------|
| **M1: 单平台可用** | OpenAI Chat+Stream 端到端跑通（HTTP + SDK） | Sprint 2 |
| **M2: 多平台可用** | 3+ 平台 Chat+Stream 全通，Tier 路由 + Fallback 工作 | Sprint 3 |
| **M3: 生产可靠** | 熔断+重试+缓存+超时全部集成，故障场景验证通过 | Sprint 4-5 |
| **M4: 功能完整** | Tool Calling + Embeddings + 多媒体 全部可用 | Sprint 6-7 |
| **M5: 生产就绪** | 安全+可观测性+容器化+压测 全部完成 | Sprint 8-9 |

---

## 附录 D：技术选型参考

| 依赖 | 推荐 | 理由 |
|------|------|------|
| HTTP 框架 | `net/http` + `chi` | 轻量，chi 只加路由不加框架 |
| JSON | `encoding/json` （后续可换 `sonic`） | 先用标准库，性能瓶颈时再优化 |
| 日志 | `log/slog` | Go 1.21+ 标准库，结构化日志 |
| 配置 | `gopkg.in/yaml.v3` | YAML 解析 |
| 缓存 | `github.com/hashicorp/golang-lru/v2` + `github.com/redis/go-redis/v9` | LRU 内存缓存 + Redis |
| Metrics | `github.com/prometheus/client_golang` | Prometheus 指标 |
| Tracing | `go.opentelemetry.io/otel` | OpenTelemetry |
| Lint | `golangci-lint` | 代码质量 |
| 测试 | `github.com/stretchr/testify` | 断言和 mock |
