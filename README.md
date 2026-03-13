# LLM Gateway

统一的 LLM API 网关，支持 OpenAI、Anthropic、阿里百炼、火山引擎、智谱、DeepSeek 等多个平台，提供智能路由、熔断降级、缓存等企业级特性。

## 特性

- **多平台支持**：OpenAI、Anthropic、阿里百炼、火山引擎、智谱、DeepSeek，以及任何 OpenAI 兼容接口
- **智能路由**：按模型名、Provider、模型层级(Tier)自动路由
- **双模式使用**：HTTP API 或 Go SDK 直接集成
- **BYOK 支持**：请求级动态凭证，支持客户自带 API Key
- **流式响应**：完整支持 SSE 流式输出
- **OpenAI 兼容**：API 格式与 OpenAI `/v1/chat/completions` 完全兼容
- **Responses API**：支持 OpenAI Responses API（`/v1/responses`），适合推理模型
- **Embeddings API**：支持文本向量化（`/v1/embeddings`），兼容 OpenAI Embeddings 格式

## 快速开始

### 1. 编译

```bash
git clone https://github.com/lex1ng/llm-gateway.git
cd llm-gateway
go build -o llm-gateway cmd/server/main.go
```

### 2. 配置文件说明

LLM Gateway 使用三个配置文件，项目提供了 `*.example` 模板，复制后按需修改即可：

```bash
cp config/config.example.yaml config/config.yaml
cp config/models.example.yaml config/models.yaml
cp config/.env.example config/.env
# 编辑 config/.env，填入你的 API Key
```

| 文件 | 作用 | 是否必须 |
|------|------|---------|
| `config/config.yaml` | 厂商端点、API Key 引用、代理、超时等运行时配置 | **必须** |
| `config/models.yaml` | 模型目录（注册后可按 model 名自动路由到厂商）+ Tier 路由表 | **可选**（不注册的模型通过 `provider` 字段直通） |
| `config/.env` | 环境变量（API Key 等敏感信息），通过 `--env` 加载 | **可选**（也可直接 `export` 环境变量） |

> **models.yaml 不是必须的。** 未在 catalog 中注册的模型，只要指定 `provider` 字段就能直通调用：
> ```json
> {"provider": "alibaba", "model": "任意厂商支持的模型名", "messages": [...]}
> ```
> 注册的好处：可以只传 `model` 不传 `provider`，gateway 自动路由到正确厂商；还可以使用 Tier 路由和成本计算。

> 配置文件中使用 `${VAR_NAME}` 引用环境变量，支持默认值 `${VAR:-default}`。

### 3. 两种部署模式

#### 模式一：HTTP Server（独立服务）

适合团队共用、跨语言调用。需要准备的文件：

```
config/
├── config.yaml    # 厂商配置（必须）
├── models.yaml    # 模型目录（可选）
└── .env           # API Key（可选，也可 export）
```

```bash
# 使用 .env 文件
go run cmd/server/main.go --env config/.env

# 或直接 export 环境变量
export DASHSCOPE_API_KEY=sk-...
go run cmd/server/main.go

# 自定义配置路径
go run cmd/server/main.go --config config/config.yaml --env config/.env
```

启动后默认监听 `0.0.0.0:8080`。

```bash
# 验证
curl http://localhost:8080/health
curl http://localhost:8080/v1/models
```

#### 模式二：Go SDK（嵌入集成）

适合 Go 项目直接集成，无需启动 HTTP 服务。支持两种初始化方式：

**方式 A：Builder 模式（推荐，零配置文件）**

无需任何 YAML 文件，纯代码配置：

```go
import "github.com/lex1ng/llm-gateway/pkg/gateway"

client, err := gateway.NewBuilder().
    AddProvider("alibaba", gateway.ProviderOpts{
        BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
        APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
    }).
    Build()
if err != nil {
    log.Fatal(err)
}
defer client.Close()

resp, err := client.Chat(ctx, &types.ChatRequest{
    Provider: "alibaba",
    Model:    "qwen-plus",
    Messages: messages,
})
```

> 完整示例见 [`examples/basic/main.go`](examples/basic/main.go)，包含 Chat、Stream、Embedding 的用法。

**方式 B：配置文件模式**

需要准备配置文件：

```
你的项目/
├── config/
│   ├── config.yaml    # 厂商配置（必须）
│   └── models.yaml    # 模型目录（可选）
└── main.go
```

```go
import "github.com/lex1ng/llm-gateway/pkg/gateway"

client, err := gateway.New("config/config.yaml")
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

> 配置文件模式支持模型目录（models.yaml）自动路由，适合需要 Tier 路由、多厂商模型注册的场景。
> API Key 通过进程环境变量读取（config.yaml 中的 `${VAR}` 会从 `os.Getenv` 解析）。

### 4. 最小化配置示例

只用一个厂商（阿里百炼）+ 一个模型，3 步即可：

**config.yaml:**
```yaml
server:
  host: "0.0.0.0"
  port: 8080

providers:
  alibaba:
    base_url: "https://dashscope.aliyuncs.com/compatible-mode/v1"
    api_key: "${DASHSCOPE_API_KEY}"
```

**models.yaml（可选）:**
```yaml
model_catalog:
  - id: "qwen-plus"
    provider: "alibaba"
    capabilities:
      chat: true
      streaming: true
```

**启动：**
```bash
export DASHSCOPE_API_KEY=sk-xxx
go run cmd/server/main.go
```

不写 models.yaml 也行，只是调用时必须指定 provider：
```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"provider": "alibaba", "model": "qwen-plus", "messages": [{"role": "user", "content": "你好"}]}'
```

---

## 请求方式

### 方式一：curl 请求

#### 非流式请求

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

#### 流式请求

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -N \
  -d '{
    "model": "qwen-turbo",
    "messages": [{"role": "user", "content": "写一首关于编程的诗"}],
    "stream": true,
    "max_tokens": 200
  }'
```

#### 指定厂商

当模型名在 catalog 中有注册时，gateway 自动解析对应的厂商。如果需要调用**未注册的模型**（或同名模型在多厂商都有），通过 `provider` 字段显式指定：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "alibaba",
    "model": "qwen3-0.6b",
    "messages": [{"role": "user", "content": "你好"}],
    "max_tokens": 50
  }'
```

#### BYOK（自带 API Key）

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hello"}],
    "credentials": {"api_key": "sk-your-own-key"},
    "max_tokens": 100
  }'
```

#### Tier 路由

不指定具体模型，按层级自动选择最优可用模型：

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model_tier": "small",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }'
```

`model_tier` 可选值：`small`、`medium`、`large`。路由优先级在 `config/models.yaml` 的 `tier_routing` 中配置。

#### Responses API

OpenAI Responses API，适合推理模型：

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o",
    "input": "What is 2+2?",
    "max_output_tokens": 100
  }'
```

#### Embeddings

文本向量化，支持任何 OpenAI 兼容的 Embedding 模型：

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "model": "text-embedding-v3",
    "input": ["你好世界", "Hello world"]
  }'
```

指定厂商：

```bash
curl http://localhost:8080/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{
    "provider": "alibaba",
    "model": "text-embedding-v3",
    "input": ["搜索文本"]
  }'
```

### 方式二：Go SDK 调用

无需启动 HTTP 服务，直接在 Go 项目中集成。

#### Builder 模式（零配置文件）

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/lex1ng/llm-gateway/pkg/gateway"
    "github.com/lex1ng/llm-gateway/pkg/types"
)

func main() {
    client, err := gateway.NewBuilder().
        AddProvider("alibaba", gateway.ProviderOpts{
            BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
            APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
        }).
        Build()
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 非流式调用
    resp, err := client.Chat(context.Background(), &types.ChatRequest{
        Provider: "alibaba",
        Model:    "qwen-plus",
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("Hello!")},
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Content)
}
```

> 完整示例（含 Chat、Stream、Embedding）见 [`examples/basic/main.go`](examples/basic/main.go)。

Builder 支持注册多个厂商：

```go
client, err := gateway.NewBuilder().
    AddProvider("alibaba", gateway.ProviderOpts{
        BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
        APIKey:  os.Getenv("DASHSCOPE_API_KEY"),
    }).
    AddProvider("openai", gateway.ProviderOpts{
        BaseURL: "https://api.openai.com/v1",
        APIKey:  os.Getenv("OPENAI_API_KEY"),
    }).
    SetTimeout(60 * time.Second).  // 可选：自定义超时
    Build()
```

#### 配置文件模式

```go
client, err := gateway.New("config/config.yaml")
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

> 配置文件模式额外支持模型目录（models.yaml）自动路由和 Tier 路由。

#### SDK 路由控制

```go
// 按 Tier 路由
resp, _ := client.Chat(ctx, &types.ChatRequest{
    ModelTier: types.TierSmall,
    Messages:  messages,
})

// 指定厂商 + 模型（可以是 catalog 外的模型）
resp, _ := client.Chat(ctx, &types.ChatRequest{
    Provider: "alibaba",
    Model:    "qwen3-0.6b",
    Messages: messages,
})

// BYOK
resp, _ := client.Chat(ctx, &types.ChatRequest{
    Model:       "gpt-4o",
    Messages:    messages,
    Credentials: map[string]string{"api_key": "sk-user-key"},
})

// Embedding
embedResp, _ := client.Embed(ctx, &types.EmbedRequest{
    Provider: "alibaba",
    Model:    "text-embedding-v3",
    Input:    []string{"你好世界", "Hello world"},
})
```

### 方式三：测试脚本

项目自带测试脚本，方便快速验证：

```bash
# 健康检查 + 列出模型
./scripts/test-api.sh

# 非流式 Chat（默认模型 gpt-4o-mini）
./scripts/test-api.sh chat

# 指定 catalog 中的模型
./scripts/test-api.sh chat qwen-turbo

# 指定厂商:模型（catalog 外的模型也可以）
./scripts/test-api.sh chat alibaba:qwen3-0.6b

# 带自定义 prompt
./scripts/test-api.sh chat "Claude Opus 4.6" "你好"

# 流式请求
./scripts/test-api.sh stream qwen-turbo "写一首诗"

# Responses API
./scripts/test-api.sh responses gpt-4o

# Embedding
./scripts/test-api.sh embed alibaba:text-embedding-v3

# 错误场景测试
./scripts/test-api.sh errors

# 全部测试
./scripts/test-api.sh all qwen-turbo
```

---

## 添加新厂商

只需修改两个配置文件，无需写代码。

### 第一步：在 `config/config.yaml` 添加 Provider

```yaml
providers:
  # ... 已有的 providers ...

  my-new-provider:
    base_url: "https://api.example.com/v1"    # 厂商 API 地址
    api_key: "${MY_PROVIDER_API_KEY}"          # 从环境变量读取
    rate_limit: 300
    # extra:
    #   chat_path: "/chat/completions"         # 自定义 endpoint 路径（默认 /chat/completions）
```

> 所有走 **OpenAI 兼容协议**的厂商（绝大多数国内平台），gateway 都能直接接入，无需额外开发。

### 第二步：在 `config/models.yaml` 注册模型

```yaml
model_catalog:
  # ... 已有模型 ...

  - id: "example-model-pro"                   # 模型 ID（需要和厂商实际模型名一致）
    provider: "my-new-provider"               # 对应 config.yaml 中的 provider 名
    tier: "large"                             # small / medium / large
    context_window: 128000
    max_output: 8192
    input_price: 0.002                        # 每 1K token 价格（用于成本计算）
    output_price: 0.006
    capabilities:
      chat: true
      streaming: true
      tools: true                             # 是否支持 function calling
      vision: true                            # 是否支持图片输入
      json_mode: true                         # 是否支持 JSON mode
      # embedding: true                       # Embedding 模型设此项，其余 capabilities 不填

# 如需 Tier 路由，将模型加入路由表
tier_routing:
  large:
    - provider: "my-new-provider"
      model: "example-model-pro"
      priority: 3                             # 优先级数字越小越优先
```

### 第三步：设置 API Key 并重启

```bash
export MY_PROVIDER_API_KEY="your-api-key"
go run cmd/server/main.go --env config/.env
```

启动日志会显示哪些 provider 加载成功，哪些因缺少 API Key 被跳过。

### 快速接入示例

以 **Moonshot（月之暗面）** 为例：

**config.yaml:**
```yaml
providers:
  moonshot:
    base_url: "https://api.moonshot.cn/v1"
    api_key: "${MOONSHOT_API_KEY}"
    rate_limit: 300
```

**models.yaml:**
```yaml
model_catalog:
  - id: "moonshot-v1-8k"
    provider: "moonshot"
    tier: "small"
    context_window: 8192
    max_output: 4096
    input_price: 0.012
    output_price: 0.012
    capabilities:
      chat: true
      streaming: true

tier_routing:
  small:
    - provider: "moonshot"
      model: "moonshot-v1-8k"
      priority: 5
```

```bash
export MOONSHOT_API_KEY="sk-..."
# 重启服务后即可调用
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "moonshot-v1-8k", "messages": [{"role": "user", "content": "你好"}]}'
```

### 接入非 OpenAI 协议的厂商

如果厂商**不是** OpenAI 兼容接口（如 Anthropic 原生 API），但你有一个 OpenAI 兼容的代理（如 OneAPI），可以通过 `api_format` 配置：

```yaml
providers:
  anthropic:
    base_url: "https://your-oneapi-proxy.com/v1"   # OneAPI 代理地址
    api_key: "${ANTHROPIC_API_KEY}"
    extra:
      api_format: "openai"                          # 走 OpenAI 兼容格式
```

直连 Anthropic 原生 API：

```yaml
providers:
  anthropic:
    base_url: "https://api.anthropic.com"
    api_key: "${ANTHROPIC_API_KEY}"
    extra:
      anthropic_version: "2023-06-01"               # API 版本
      default_max_tokens: 4096                      # 默认 max_tokens
```

---

## 配置参考

### 可配置的 Extra 字段

通过 `extra` 字段可以自定义厂商的行为：

| 字段 | 适用于 | 默认值 | 说明 |
|------|--------|--------|------|
| `chat_path` | OpenAI 兼容 | `/chat/completions` | Chat 接口路径 |
| `responses_path` | OpenAI 兼容 | `/responses` | Responses 接口路径 |
| `models_path` | OpenAI 兼容 | `/models` | 模型列表接口路径 |
| `embeddings_path` | OpenAI 兼容 | `/embeddings` | Embeddings 接口路径 |
| `api_format` | Anthropic | `anthropic` | 设为 `"openai"` 时走 OpenAI 兼容协议 |
| `anthropic_version` | Anthropic 原生 | `2023-06-01` | Anthropic API 版本 |
| `default_max_tokens` | Anthropic 原生 | `4096` | 默认 max_tokens |
| `messages_path` | Anthropic 原生 | `/v1/messages` | Messages 接口路径 |

---

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/v1/chat/completions` | POST | Chat 对话（OpenAI 兼容格式） |
| `/v1/responses` | POST | Responses API（OpenAI 推理模型） |
| `/v1/embeddings` | POST | 文本向量化（OpenAI 兼容格式） |
| `/v1/models` | GET | 列出所有可用模型 |
| `/health` | GET | 健康检查 |
| `/healthz` | GET | 健康检查（K8s 探针） |

## 项目结构

```
llm-gateway/
├── cmd/server/             # HTTP 服务入口
├── config/
│   ├── config.yaml         # 主配置（providers、server、manager 等）
│   ├── config.example.yaml # 最小化配置模板
│   ├── models.yaml         # 模型目录 + Tier 路由配置
│   ├── models.example.yaml # 最小化模型目录模板
│   └── .env.example        # 环境变量模板
├── pkg/
│   ├── adapter/
│   │   ├── openai/         # OpenAI 及所有兼容接口的适配器
│   │   └── anthropic/      # Anthropic 原生接口适配器
│   ├── gateway/            # SDK 入口（Go 项目直接集成）
│   ├── manager/            # 请求编排（路由、重试、熔断）
│   ├── provider/           # Provider 接口定义
│   ├── transport/          # HTTP 客户端（认证）
│   └── types/              # 核心类型定义
├── api/
│   ├── handler/            # HTTP 请求处理器
│   └── router.go           # 路由注册
└── scripts/
    └── test-api.sh         # API 测试脚本
```

## 已支持的厂商

| 厂商 | Provider 名称 | 协议 | 状态 |
|------|--------------|------|------|
| OpenAI | `openai` | OpenAI 原生 | ✅ |
| Anthropic | `anthropic` | OpenAI 兼容 / 原生 | ✅ |
| 阿里百炼 (DashScope) | `alibaba` | OpenAI 兼容 | ✅ |
| 火山引擎 (Doubao) | `volcengine` | OpenAI 兼容 | ✅ |
| 智谱 (GLM) | `zhipu` | OpenAI 兼容 | ✅ |
| DeepSeek | `deepseek` | OpenAI 兼容 | ✅ |
| 任意 OpenAI 兼容平台 | 自定义名称 | OpenAI 兼容 | ✅ |

## License

MIT
