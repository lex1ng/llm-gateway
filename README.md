# LLM Gateway

统一的 LLM API 网关，支持 OpenAI、Anthropic、阿里百炼等多个平台，提供智能路由、熔断降级、缓存等企业级特性。

## 特性

- 🔌 **多平台支持**：OpenAI、Anthropic、阿里百炼、火山引擎等
- 🔄 **智能路由**：按模型名、Provider、模型层级(Tier)自动路由
- 📦 **双模式使用**：HTTP API 或 Go SDK 直接集成
- 🔐 **BYOK 支持**：请求级动态凭证，支持客户自带 API Key
- 🌊 **流式响应**：完整支持 SSE 流式输出
- 📊 **OpenAI 兼容**：API 格式与 OpenAI 完全兼容

## 快速开始

### 1. 安装

```bash
go get github.com/lex1ng/llm-gateway
```

### 2. 配置

创建 `config/config.yaml`：

```yaml
server:
  port: 8080

providers:
  openai:
    base_url: "https://api.openai.com/v1"
    api_key: "${OPENAI_API_KEY}"

model_catalog:
  - id: "gpt-4o"
    provider: "openai"
    tier: "large"
    context_window: 128000
    max_output: 16384
    input_price: 0.0025
    output_price: 0.01
    capabilities:
      chat: true
      vision: true
      tools: true
      streaming: true

  - id: "gpt-4o-mini"
    provider: "openai"
    tier: "small"
    context_window: 128000
    max_output: 16384
    input_price: 0.00015
    output_price: 0.0006
    capabilities:
      chat: true
      streaming: true

tier_routing:
  small:
    - provider: "openai"
      model: "gpt-4o-mini"
      priority: 1
  large:
    - provider: "openai"
      model: "gpt-4o"
      priority: 1
```

### 3. 启动服务器

```bash
# 设置 API Key
export OPENAI_API_KEY="sk-..."

# 启动服务
go run cmd/server/main.go -config config/config.yaml
```

### 4. 发送请求

**非流式请求：**

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

**流式请求：**

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "写一首关于编程的诗"}
    ],
    "stream": true
  }'
```

## SDK 使用

直接在 Go 项目中使用，无需启动 HTTP 服务：

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/lex1ng/llm-gateway/pkg/gateway"
    "github.com/lex1ng/llm-gateway/pkg/types"
)

func main() {
    // 创建客户端
    client, err := gateway.New("config/config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // 非流式调用
    resp, err := client.Chat(context.Background(), &types.ChatRequest{
        Model: "gpt-4o-mini",
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("Hello!")},
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(resp.Content)

    // 流式调用
    stream, err := client.ChatStream(context.Background(), &types.ChatRequest{
        Model:  "gpt-4o-mini",
        Stream: true,
        Messages: []types.Message{
            {Role: types.RoleUser, Content: types.NewTextContent("写一首诗")},
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    for event := range stream {
        switch event.Type {
        case types.StreamEventContentDelta:
            fmt.Print(event.Delta)
        case types.StreamEventDone:
            fmt.Println("\n--- Done ---")
        case types.StreamEventError:
            log.Printf("Error: %s", event.Error)
        }
    }
}
```

## 高级用法

### Tier 路由

不指定具体模型，按层级自动选择：

```go
resp, err := client.Chat(ctx, &types.ChatRequest{
    ModelTier: types.TierSmall,  // small / medium / large
    Messages:  messages,
})
```

### BYOK (自带 API Key)

请求级覆盖 API Key：

```go
resp, err := client.Chat(ctx, &types.ChatRequest{
    Model:    "gpt-4o",
    Messages: messages,
    Credentials: map[string]string{
        "api_key": "sk-user-provided-key",
    },
})
```

### 指定 Provider

强制使用特定 Provider：

```go
resp, err := client.Chat(ctx, &types.ChatRequest{
    Provider:  "openai",
    ModelTier: types.TierLarge,
    Messages:  messages,
})
```

## API 端点

| 端点 | 方法 | 描述 |
|------|------|------|
| `/v1/chat/completions` | POST | Chat 对话（OpenAI 兼容） |
| `/v1/models` | GET | 列出可用模型 |
| `/health` | GET | 健康检查 |

## 项目结构

```
.
├── api/                    # HTTP API 层
│   ├── handler/            # 请求处理器
│   └── router.go           # 路由注册
├── cmd/server/             # 服务入口
├── config/                 # 配置加载
├── pkg/
│   ├── adapter/            # Provider 适配器
│   │   └── openai/         # OpenAI 实现
│   ├── gateway/            # SDK 入口
│   ├── manager/            # 请求编排
│   ├── provider/           # Provider 接口
│   ├── transport/          # HTTP 客户端
│   └── types/              # 核心类型
└── examples/               # 使用示例
```

## 开发进度

- ✅ Sprint 1: 核心类型、配置、传输层
- ✅ Sprint 2: OpenAI Chat 完整链路
- 🚧 Sprint 3: Anthropic + 国内平台
- 🚧 Sprint 4: 熔断、重试、缓存
- 📋 Sprint 5-9: Hook 系统、配额、多媒体等

## License

MIT
