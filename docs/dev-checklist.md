# LLM Gateway — 开发进度清单

> **本文件是多人/多 Agent 协作的唯一进度源。**
>
> 配套文档：
> - [architecture-design.md](./architecture-design.md) — 架构设计参考（只读）
> - [development-plan.md](./development-plan.md) — 每个 Task 的详细实现指南（只读）

---

## 协作规则

### 给人类开发者

1. 开工前：找到下一个 `⬜` 任务，在备注写上你的名字，状态改为 `🔨`
2. 完工后：状态改为 `✅`，备注写产出摘要
3. 被阻塞：状态改为 `⏸️`，备注写阻塞原因和依赖的任务 ID

### 给 AI Agent

1. **开工前必读本文件**，了解当前进度和哪些任务已完成
2. 按任务 ID 顺序找到下一个 `⬜` 状态且**无未完成前置依赖**的任务
3. 开工时将状态改为 `🔨`，完工后改为 `✅` 并在备注中写明：
   - 创建/修改了哪些文件
   - 测试是否通过
   - 是否有遗留问题
4. **不要跳过依赖**：如果前置任务是 `⬜`，不要开始当前任务
5. **不要重复工作**：如果状态是 `🔨` 或 `✅`，不要再做这个任务
6. 每个任务的详细实现指南见 [development-plan.md](./development-plan.md) 对应章节

### 状态说明

| 状态 | 含义 |
|------|------|
| ⬜ | 待开发 |
| 🔨 | 开发中（备注里标注由谁/哪个 agent 在做） |
| ✅ | 已完成 |
| ⏸️ | 被阻塞（备注里说明阻塞原因） |
| 🔄 | 需要返工（备注里说明原因） |

---

## 总览

| Sprint | 主题 | 进度 |
|--------|------|------|
| Sprint 1 | 地基层：Types + Config + Transport | ✅ 12/12 |
| Sprint 2 | 第一条垂直切片：OpenAI Chat 全通 | ✅ 16/16 |
| Sprint 3 | 横向扩展：Anthropic + 国内平台 | 🔨 8/13 |
| Sprint 4 | 编排层核心能力 | ⬜ 0/18 |
| Sprint 5 | Hook 系统 + 配额 + 消费记录 | ⬜ 0/13 |
| Sprint 6 | Tool Calling + Embeddings | ⬜ 0/9 |
| Sprint 7 | 多媒体 + 异步任务 | ⬜ 0/11 |
| Sprint 8 | 安全与可观测性 | ⬜ 0/14 |
| Sprint 9 | 生产化 | ⬜ 0/10 |
| Sprint 10 | 管理后台与可观测性 | ⬜ 0/23 |

---

## Sprint 1 — 地基层：Types + Config + Transport

> 参考：[development-plan.md § Sprint 1](./development-plan.md#1-sprint-1--地基层types--config--transport)
>
> 前置依赖：无

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 1.1.1 | 初始化 go.mod | `go.mod` | — | §3 | ✅ | `f517292` |
| 1.1.2 | 创建完整目录骨架 | 全部目录 + `.gitkeep` | — | §3 | ✅ | `f517292` 19个目录 |
| 1.1.3 | Makefile（build/test/lint） | `Makefile` | — | — | ✅ | `f517292` |
| 1.2.1 | Message / Content / ContentBlock 类型 | `pkg/types/message.go` | 1.1.1 | §4.1 | ✅ | `6ebabff` JSON双态序列化 |
| 1.2.2 | ChatRequest（含 Credentials）/ EmbedRequest 等请求类型 | `pkg/types/request.go` | 1.2.1 | §4.2 | ✅ | `6ebabff` |
| 1.2.3 | ChatResponse / StreamEvent / EmbedResponse 等响应类型 | `pkg/types/response.go` | 1.2.1 | §4.2 | ✅ | `6ebabff` |
| 1.2.4 | ModelTier / ModelConfig / ModelCapabilities | `pkg/types/model.go` | 1.1.1 | §4.3 | ✅ | `6ebabff` |
| 1.2.5 | ErrorCode / ProviderError / ErrorAction 四级分类 | `pkg/types/error.go` | 1.1.1 | §4.4 + §7.5 | ✅ | `6ebabff` |
| 1.2.6 | TokenUsage / Cost | `pkg/types/usage.go` | 1.1.1 | §4.2 | ✅ | `6ebabff` |
| 1.3.1 | Config 结构定义 + YAML 加载 + 环境变量替换 | `config/config.go` | 1.2.4 | §11 | ✅ | `fcb252f` 支持 ${ENV_VAR} |
| 1.3.2 | 默认配置模板 | `config/config.yaml`, `config/models.yaml` | 1.3.1 | §11 | ✅ | `fcb252f` |
| 1.4.1 | 统一 HTTP Client（Do / DoJSON / DoStream） | `pkg/transport/http_client.go` | 1.2.5 | §10.1 | ✅ | `fcb252f` |
| 1.4.2 | AuthStrategy 接口 + BearerAuth / AnthropicAuth / GoogleAuth / DynamicAuth | `pkg/transport/auth.go` | 1.4.1 | §10.1 | ✅ | `fcb252f` DynamicAuth 实现 BYOK |
| 1.4.3 | SSE 通用解析器 | `pkg/transport/sse.go` | 1.4.1 | §9 | ✅ | `fcb252f` SSEReader/SSEWriter |

---

## Sprint 2 — 第一条垂直切片：OpenAI Chat 全通

> 参考：[development-plan.md § Sprint 2](./development-plan.md#2-sprint-2--第一条垂直切片openai-chat-全通)
>
> 前置依赖：Sprint 1 全部完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 2.1.1 | Provider 基础接口 + ChatProvider 接口 | `pkg/provider/interface.go` | 1.2.* | §5.1 | ✅ | `0674a11` 含所有能力接口 |
| 2.1.2 | Capability 枚举 + capInterfaceMap | `pkg/provider/capability.go` | 2.1.1 | §5.2 | ✅ | `0674a11` reflect.TypeFor |
| 2.1.3 | Registry（注册/查询/一致性校验） | `pkg/provider/registry.go` | 2.1.1, 2.1.2 | §5.3 | ✅ | `0674a11` |
| 2.2.1 | OpenAI Provider 结构体 + Name/Models/Supports/Close | `pkg/adapter/openai/provider.go` | 2.1.1, 1.4.1 | §6.1 | ✅ | `0674a11` |
| 2.2.2 | OpenAI Chat 非流式 | `pkg/adapter/openai/chat.go` | 2.2.1 | §6.1 | ✅ | `0674a11` |
| 2.2.3 | OpenAI Chat 流式 | `pkg/adapter/openai/stream.go` | 2.2.1, 1.4.3 | §6.1 | ✅ | `0674a11` |
| 2.2.4 | OpenAI Mapper（请求/响应映射） | `pkg/adapter/openai/mapper.go` | 2.2.1 | §6.1 | ✅ | `0674a11` 内含在 chat.go |
| 2.3.1 | Manager 结构体 + New() 初始化 | `pkg/manager/manager.go` | 2.1.3, 1.3.1 | §7.1 | ✅ | `0674a11` |
| 2.3.2 | Router（按 model 名查找，暂不做 Tier） | `pkg/manager/router.go` | 2.1.3 | §7.2 | ✅ | `0674a11` 已含 Tier 路由 |
| 2.3.3 | Manager.Chat() 非流式入口 | `pkg/manager/manager.go` | 2.3.1, 2.3.2 | §7.1 | ✅ | `0674a11` |
| 2.3.4 | Manager.ChatStream() 流式入口 | `pkg/manager/manager.go` | 2.3.1, 2.3.2 | §7.1 | ✅ | `0674a11` |
| 2.4.1 | SDK Client 结构体 + New() / Close() | `pkg/gateway/client.go` | 2.3.1 | §12.1 | ✅ | `0674a11` |
| 2.4.2 | SDK Options（WithCache / WithHook / WithLogger） | `pkg/gateway/options.go` | 2.4.1 | §12.1 | ✅ | `0674a11` WithHook 待 Sprint 5 |
| 2.5.1 | Chat HTTP Handler（非流式 + 流式 SSE） | `api/handler/chat.go` | 2.4.1 | §12 | ✅ | `0674a11` |
| 2.5.2 | API 路由注册 | `api/router.go` | 2.5.1 | §12 | ✅ | `0674a11` |
| 2.5.3 | cmd/server/main.go 薄壳 | `cmd/server/main.go` | 2.4.1, 2.5.2 | §12.1 | ✅ | `0674a11` |

### Sprint 2 里程碑验证

- [ ] `curl POST /v1/chat/completions` 非流式 → 返回 JSON 响应
- [ ] `curl POST /v1/chat/completions` 流式 → 返回 SSE 流
- [ ] SDK 直接调用 `client.Chat()` → 成功
- [ ] 无效模型 → 404 错误
- [ ] 无效 API Key → 401 错误

---

## Sprint 3 — 横向扩展：Anthropic + 国内平台

> 参考：[development-plan.md § Sprint 3](./development-plan.md#3-sprint-3--横向扩展anthropic--国内兼容平台)
>
> 前置依赖：Sprint 2 全部完成
>
> **可与 Sprint 4 并行开发**（Sprint 3 扩展宽度，Sprint 4 扩展深度）

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 3.1.1 | Anthropic 消息格式转换 ToAnthropic() | `pkg/adapter/anthropic/chat.go` | 1.2.1 | §8 | ✅ | 内联在 adapter 中，extractSystemAndConvert() |
| 3.1.2 | Gemini 消息格式转换 ToGemini() | `pkg/mapper/message.go` | 1.2.1 | §8 | ⬜ | contents/parts 结构 |
| 3.1.3 | 各平台流式解析器（ParseAnthropicStream / ParseGeminiStream） | `pkg/adapter/anthropic/stream.go` | 1.4.3 | §9 | ✅ | Anthropic 部分完成，内联在 adapter |
| 3.2.1 | Anthropic Provider + Mapper | `pkg/adapter/anthropic/provider.go`, `chat.go` | 3.1.1, 1.4.2 | §6.2 | ✅ | AnthropicAuth + 11 个单元测试 |
| 3.2.2 | Anthropic Chat 非流式 | `pkg/adapter/anthropic/chat.go` | 3.2.1 | §6.2 | ✅ | system 抽取、max_tokens 默认 4096、stop→stop_sequences |
| 3.2.3 | Anthropic Chat 流式 | `pkg/adapter/anthropic/stream.go` | 3.2.1, 3.1.3 | §6.2 | ✅ | content_block_delta + message_delta 解析 |
| 3.3.1 | Compatible Provider + PlatformPresets | `pkg/adapter/openai/provider.go` (复用) | 1.4.1 | §6.4 | ✅ | 复用 OpenAI 适配器 + `NewWithName()`，无需独立 adapter |
| 3.3.2 | Compatible Chat 非流式 | `pkg/adapter/openai/chat.go` (复用) | 3.3.1 | §6.4 | ✅ | OpenAI compatible，直接复用 |
| 3.3.3 | Compatible Chat 流式 | `pkg/adapter/openai/stream.go` (复用) | 3.3.1 | §6.4 | ✅ | OpenAI compatible，直接复用 |
| 3.4.1 | Router 增加 Tier 路由 | `pkg/manager/router.go` | 3.2.*, 3.3.* | §7.2 | ⬜ | 按优先级选 provider |
| 3.4.2 | Router 增加 Fallback 逻辑 | `pkg/manager/router.go` | 3.4.1 | §7.2 | ⬜ | 首选失败自动切换 |
| 3.5.1 | 阿里百炼集成测试 | `tests/integration/alibaba_test.go` | 3.3.2 | — | ⬜ | 需要 DASHSCOPE_API_KEY |
| 3.5.2 | Anthropic 集成测试 | `tests/integration/anthropic_test.go` | 3.2.2 | — | ⬜ | 需要 ANTHROPIC_API_KEY |

### Sprint 3 里程碑验证

- [ ] Anthropic Chat + Stream 全通
- [ ] 阿里百炼 Chat + Stream 全通
- [ ] 至少再通一个国内平台（火山或智谱）
- [ ] `ModelTier: "large"` → 按优先级选 provider
- [ ] 首选 provider 超时 → 自动 fallback 到备选

---

## Sprint 4 — 编排层核心能力

> 参考：[development-plan.md § Sprint 4](./development-plan.md#4-sprint-4--编排层核心能力)
>
> 前置依赖：Sprint 2 完成（不依赖 Sprint 3）
>
> **可与 Sprint 3 并行开发**

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 4.1.1 | CircuitBreaker 三态状态机 | `pkg/manager/circuit_breaker.go` | 2.3.1 | §7.3 | ⬜ | Closed/Open/HalfOpen |
| 4.1.2 | CircuitBreaker.Execute() 包装函数 | `pkg/manager/circuit_breaker.go` | 4.1.1 | §7.3 | ⬜ | 并发安全 |
| 4.2.1 | CooldownManager 冷却管理器 | `pkg/manager/cooldown.go` | 2.3.1 | §7.4 | ⬜ | 键：provider:keyHash:model |
| 4.2.2 | 退避序列（10s→30s→60s→120s→300s） | `pkg/manager/cooldown.go` | 4.2.1 | §7.4 | ⬜ | 成功清除，失败递增 |
| 4.3.1 | ErrorAction 四级分类 + ClassifyForRetry() | `pkg/manager/retry.go` | 1.2.5 | §7.5 | ⬜ | Retry/RotateKey/Fallback/Abort |
| 4.3.2 | RetryBudgetTracker 滑动窗口预算跟踪 | `pkg/manager/retry.go` | 4.3.1 | §7.5 | ⬜ | RecordRequest/AllowRetry/RecordRetry |
| 4.3.3 | ExecuteWithDeadline() 重试执行器 | `pkg/manager/retry.go` | 4.3.1, 4.3.2 | §7.5 | ⬜ | 全局 deadline + budget 双重限制 |
| 4.4.1 | TimeoutConfig + 按 ModelTier 差异化超时 | `pkg/manager/timeout.go` | 1.2.4 | §7.6 | ⬜ | connect/firstToken/total/idle |
| 4.5.1 | MemoryCache（LRU + TTL） | `pkg/manager/cache.go` | 2.3.1 | §7.10 | ⬜ | 基于 golang-lru/v2 |
| 4.5.2 | RedisCache（可选） | `pkg/manager/cache.go` | 4.5.1 | §7.10 | ⬜ | 基于 go-redis/v9 |
| 4.5.3 | DualCache（memory → redis 读，双写） | `pkg/manager/cache.go` | 4.5.1, 4.5.2 | §7.10 | ⬜ | Redis miss → 回填 memory |
| 4.5.4 | IsSafeToCache() 安全检查 | `pkg/manager/cache.go` | 4.5.1 | §7.10 | ⬜ | 不缓存 length/content_filter/空 |
| 4.6.1 | Manager.Chat() 集成熔断器 | `pkg/manager/manager.go` | 4.1.2 | §7.1 | ⬜ | |
| 4.6.2 | Manager.Chat() 集成冷却检查 | `pkg/manager/manager.go` | 4.2.1 | §7.1 | ⬜ | |
| 4.6.3 | Manager.Chat() 集成重试+Deadline | `pkg/manager/manager.go` | 4.3.3 | §7.1 | ⬜ | |
| 4.6.4 | Manager.Chat() 集成缓存 | `pkg/manager/manager.go` | 4.5.3 | §7.1 | ⬜ | |
| 4.6.5 | Manager.Chat() 集成超时 | `pkg/manager/manager.go` | 4.4.1 | §7.1 | ⬜ | |
| 4.7.1 | 编排层单元测试（Mock Provider） | `pkg/manager/*_test.go` | 4.6.* | — | ⬜ | 覆盖：正常/熔断/缓存命中/超时/fallback |

### Sprint 4 里程碑验证

- [ ] 模拟 provider 故障：触发熔断 → 自动 fallback
- [ ] 同一请求第二次 → 缓存命中
- [ ] 超时请求 → 正确返回超时错误
- [ ] Deadline 到期 → 不再重试

---

## Sprint 5 — Hook 系统 + 配额 + 消费记录

> 参考：[development-plan.md § Sprint 5](./development-plan.md#5-sprint-5--hook-系统--配额--消费记录)
>
> 前置依赖：Sprint 4 完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 5.1.1 | Phase 枚举 + Hook 接口 + HookEvent | `pkg/hook/hook.go` | 1.2.* | §7.11 | ⬜ | 6 个阶段 |
| 5.1.2 | Registry（注册 + Dispatch 阻塞/非阻塞语义） | `pkg/hook/registry.go` | 5.1.1 | §7.11 | ⬜ | PreRoute/PreCall 可拦截，其余仅记录 |
| 5.1.3 | Manager 集成 Hook 调度 | `pkg/manager/manager.go` | 5.1.2, 4.6.* | §7.11 | ⬜ | Chat() 16 Phase 完整执行链 |
| 5.1.4 | SDK Client 支持 WithHook() | `pkg/gateway/client.go` | 5.1.2, 2.4.1 | §12.1 | ⬜ | |
| 5.1.5 | CostCalculator（费用计算器） | `pkg/manager/cost.go` | 1.2.* | §7.1 | ⬜ | Estimate()+Calculate()，加载 pricing.yaml。→ 并入 10.2.3 |
| 5.2.1 | QuotaStore 接口定义（支持 token+cost 原子性） | `pkg/manager/quota.go` | 1.2.* | §7.13 | ⬜ | PreConsume(tokens,cost)/Settle(tokens,cost) |
| 5.2.2 | QuotaManager.PreConsume() / Settle() / Rollback() | `pkg/manager/quota.go` | 5.2.1 | §7.13 | ⬜ | 并发安全；检查日token和月费用双限额 |
| 5.2.3 | Manager 集成配额（估算token+费用→预扣→调用→结算） | `pkg/manager/manager.go` | 5.1.5, 5.2.2 | §7.13 | ⬜ | 依赖 CostCalculator |
| 5.3.1 | SpendWriter + SpendUpdate + SpendStorage + WAL 接口 | `pkg/manager/spend_writer.go` | 1.2.* | §7.12 | ⬜ | → 并入 10.2.2 |
| 5.3.2 | 批量合并 + 定时 flush + 关闭 flush | `pkg/manager/spend_writer.go` | 5.3.1 | §7.12 | ⬜ | → 并入 10.2.2 |
| 5.3.3 | 队列满降级同步写入 + WAL 兜底 | `pkg/manager/spend_writer.go` | 5.3.1 | §7.12 | ⬜ | → 并入 10.2.2 |
| 5.3.4 | Close() 返回 error + 关闭 WAL | `pkg/manager/spend_writer.go` | 5.3.1 | §7.12 | ⬜ | → 并入 10.2.2 |
| 5.3.5 | Manager 集成 SpendWriter | `pkg/manager/manager.go` | 5.3.2 | §7.12 | ⬜ | → 并入 10.2.5 |

---

## Sprint 6 — Tool Calling + Embeddings

> 参考：[development-plan.md § Sprint 6](./development-plan.md#6-sprint-6--tool-calling--embeddings)
>
> 前置依赖：Sprint 3 完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 6.1.1 | Tool / ToolCall / ToolResult 类型定义 | `pkg/types/tool.go` | 1.2.1 | §4 | ⬜ | OpenAI function calling 格式 |
| 6.1.2 | Tool Mapper：OpenAI ↔ Anthropic 格式互转 | `pkg/mapper/tool.go` | 6.1.1, 3.1.1 | §8 | ⬜ | parameters→input_schema |
| 6.1.3 | Tool Mapper：OpenAI ↔ Gemini 格式互转 | `pkg/mapper/tool.go` | 6.1.1, 3.1.2 | §8 | ⬜ | functionDeclarations |
| 6.2.1 | OpenAI 适配器支持 tools 参数 | `pkg/adapter/openai/chat.go` | 6.1.1 | §6.1 | ⬜ | |
| 6.2.2 | Anthropic 适配器支持 tools 参数 | `pkg/adapter/anthropic/chat.go` | 6.1.2 | §6.2 | ⬜ | tool_choice 格式转换 |
| 6.2.3 | Compatible 适配器支持 tools 参数 | `pkg/adapter/compatible/chat.go` | 6.1.1 | §6.4 | ⬜ | 与 OpenAI 相同 |
| 6.3.1 | EmbeddingProvider 接口实现（OpenAI + Compatible） | `pkg/adapter/openai/embedding.go`, `compatible/embedding.go` | 2.1.1 | §5.1 | ⬜ | |
| 6.3.2 | Google Embeddings 适配 | `pkg/adapter/google/embedding.go` | 3.1.2 | §6.3 | ⬜ | embedContent 格式 |
| 6.3.3 | Embedding HTTP Handler | `api/handler/embedding.go` | 6.3.1 | §12 | ⬜ | POST /v1/embeddings |

---

## Sprint 7 — 多媒体 + 异步任务

> 参考：[development-plan.md § Sprint 7](./development-plan.md#7-sprint-7--多媒体--异步任务)
>
> 前置依赖：Sprint 3 完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 7.1.1 | AsyncTask 类型定义 | `pkg/types/async_task.go` | 1.2.* | §4.5 | ⬜ | TaskStatus 枚举 |
| 7.1.2 | 异步任务管理器（Submit + Poll） | `pkg/manager/async_task.go` | 7.1.1 | §7 | ⬜ | 内存 + 可选持久化 |
| 7.2.1 | ImageGenProvider 接口实现（OpenAI 同步） | `pkg/adapter/openai/image.go` | 2.1.1 | §5.1 | ⬜ | DALL-E / GPT Image |
| 7.2.2 | ImageGenProvider 接口实现（国内平台异步） | `pkg/adapter/compatible/image.go` | 7.1.2 | §6.4 | ⬜ | Submit + Poll 模式 |
| 7.3.1 | VideoGenProvider 接口实现（全异步） | `pkg/adapter/compatible/video.go` | 7.1.2 | §5.1 | ⬜ | |
| 7.4.1 | TTSProvider 实现（OpenAI + 国内） | `pkg/adapter/openai/audio.go`, `compatible/audio.go` | 2.1.1 | §5.1 | ⬜ | 返回 io.ReadCloser |
| 7.4.2 | STTProvider 实现（OpenAI + 国内） | `pkg/adapter/openai/audio.go`, `compatible/audio.go` | 2.1.1 | §5.1 | ⬜ | multipart/form-data 上传 |
| 7.5.1 | Google Gemini 完整适配 | `pkg/adapter/google/` | 3.1.2 | §6.3 | ⬜ | Chat + Stream + Embed + Image |
| 7.6.1 | Image HTTP Handler | `api/handler/image.go` | 7.2.1 | §12 | ⬜ | POST /v1/images/generations |
| 7.6.2 | Video / Audio HTTP Handler | `api/handler/video.go`, `audio.go` | 7.3.1, 7.4.* | §12 | ⬜ | |
| 7.6.3 | 异步任务查询 Handler | `api/handler/task.go` | 7.1.2 | §12 | ⬜ | GET /v1/tasks/{id} |

---

## Sprint 8 — 安全与可观测性

> 参考：[development-plan.md § Sprint 8](./development-plan.md#8-sprint-8--安全与可观测性)
>
> 前置依赖：Sprint 4 完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 8.1.1 | Tenant 结构 + HashedKey + TenantBudget | `pkg/auth/tenant.go` | 1.3.1 | §10.2 | ⬜ | |
| 8.1.2 | Authorizer + 前缀索引 Authenticate | `pkg/auth/rbac.go` | 8.1.1 | §10.2 | ⬜ | O(1) 前缀定位 + bcrypt |
| 8.1.3 | Authorize 模型/Provider 白名单检查 | `pkg/auth/rbac.go` | 8.1.2 | §10.2 | ⬜ | |
| 8.1.4 | Auth 中间件 | `api/middleware/auth.go` | 8.1.2 | §10.2 | ⬜ | 从 header 提取 key → Authenticate → ctx 注入 tenant |
| 8.2.1 | SecretProvider 接口 + EnvSecretProvider | `pkg/secret/provider.go` | 1.3.1 | §10.3 | ⬜ | |
| 8.2.2 | KMS / Vault SecretProvider（可选） | `pkg/secret/provider.go` | 8.2.1 | §10.3 | ⬜ | |
| 8.3.1 | SanitizeConfig + SanitizeForLog() | `api/middleware/sanitizer.go` | — | §10.4 | ⬜ | hash/truncate/mask 策略。→ 并入 10.2.1 |
| 8.4.1 | AuditEvent + AuditLogger 接口 | `pkg/audit/logger.go` | — | §10.5 | ⬜ | → 并入 10.2.1 统一请求日志 |
| 8.4.2 | FileAuditLogger / StdoutAuditLogger 实现 | `pkg/audit/logger.go` | 8.4.1 | §10.5 | ⬜ | → 并入 10.2.2 异步写入器 |
| 8.4.3 | Audit 中间件 | `api/middleware/audit.go` | 8.4.1 | §10.5 | ⬜ | → 并入 10.2.1 Manager 内置 |
| 8.5.1 | OpenTelemetry Tracing 初始化 + Span 层级 | `pkg/observability/tracing.go` | — | §7.14.1 | ⬜ | |
| 8.5.2 | Tracing 中间件 | `api/middleware/tracing.go` | 8.5.1 | §7.14.1 | ⬜ | 注入 trace_id |
| 8.6.1 | Prometheus 指标定义（全局，不含 tenant_id） | `pkg/observability/metrics.go` | — | §7.14.2 | ⬜ | 避免高基数 |
| 8.6.2 | SLO 告警规则定义 | `pkg/observability/slo.go` | 8.6.1 | §7.14.3 | ⬜ | |

---

## Sprint 9 — 生产化

> 参考：[development-plan.md § Sprint 9](./development-plan.md#9-sprint-9--生产化)
>
> 前置依赖：Sprint 5 ~ 8 完成

| ID | 任务 | 产出文件 | 前置依赖 | 设计文档 | 状态 | 备注 |
|----|------|---------|---------|---------|------|------|
| 9.1.1 | 配置热更新（watch 文件变更 → 重新加载） | `config/config.go` | 1.3.1 | §11 | ⬜ | fsnotify 或 ticker |
| 9.2.1 | IdempotencyStore 接口 + 内存实现 | `pkg/manager/idempotency.go` | 2.3.1 | §7.8 | ⬜ | GetOrSet/Complete/Fail |
| 9.2.2 | IdempotencyStore Redis 实现 | `pkg/manager/idempotency.go` | 9.2.1 | §7.8 | ⬜ | |
| 9.3.1 | Hedge 对冲请求 | `pkg/manager/hedger.go` | 2.3.1 | §7.9 | ⬜ | 双 goroutine 竞争 |
| 9.4.1 | StreamWatcher + 流式中途失败策略 | `pkg/manager/stream_failover.go` | 2.3.4 | §7.7 | ⬜ | abort/fallback/switch |
| 9.5.1 | RateLimiter（令牌桶） | `pkg/manager/rate_limiter.go` | 2.3.1 | §7 | ⬜ | 按 provider + 按 tenant |
| 9.5.2 | RateLimit 中间件 | `api/middleware/ratelimit.go` | 9.5.1 | §12 | ⬜ | |
| 9.6.1 | TokenCounter（估算 + headroom） | `pkg/manager/token_counter.go` | 1.2.4 | §7 | ⬜ | tiktoken 或近似算法 |
| 9.7.1 | Dockerfile | `Dockerfile` | — | — | ⬜ | 多阶段构建 |
| 9.7.2 | Helm Chart（可选） | `deploy/helm/` | 9.7.1 | — | ⬜ | |

---

## Sprint 10 — 管理后台与可观测性

> 前置依赖：Sprint 2 完成（核心功能）
>
> 可选增强依赖：Sprint 4（熔断状态采集）、Sprint 8.1（租户鉴权）
>
> **可与 Sprint 3~9 并行开发**（数据采集在 Manager 层，不依赖编排层完成）
>
> **设计要点**：数据采集嵌入 Manager 层（非 HTTP 中间件），确保 HTTP 服务和 SDK 直接调用两种模式都能被监控到。

### 10.1 数据存储层

| ID | 任务 | 产出文件 | 前置依赖 | 状态 | 备注 |
|----|------|---------|---------|------|------|
| 10.1.1 | Storage 接口定义（RequestLogStore + UsageStore） | `pkg/admin/store.go` | — | ⬜ | CRUD + 聚合查询接口 |
| 10.1.2 | SQLite 实现 + 自动建表迁移 | `pkg/admin/sqlite.go` | 10.1.1 | ⬜ | 开发/单机场景，零依赖 |
| 10.1.3 | PostgreSQL 实现（可选） | `pkg/admin/postgres.go` | 10.1.1 | ⬜ | 生产场景，高并发 |

### 10.2 数据采集层

> **关键设计**：采集器嵌入 Manager 层，SDK 和 HTTP 共享同一采集路径。
>
> ```
> HTTP 模式:  Router → Handler → Manager.Chat() → [记录器] → Provider
> SDK 模式:   client.Chat() → Manager.Chat() → [记录器] → Provider
> ```

| ID | 任务 | 产出文件 | 前置依赖 | 状态 | 备注 |
|----|------|---------|---------|------|------|
| 10.2.1 | 请求日志记录器（Manager 内置，含脱敏） | `pkg/admin/recorder.go` | 10.1.1 | ⬜ | 记录 model/provider/tokens/latency/status/input_preview/output_preview，吸收 8.3.1+8.4.1+8.4.3 |
| 10.2.2 | 异步批量写入器（chan + batch flush + WAL 兜底） | `pkg/admin/writer.go` | 10.1.2 | ⬜ | 不阻塞请求，吸收 5.3.1~5.3.4+8.4.2 |
| 10.2.3 | 费用计算引擎（基于 models.yaml 价格） | `pkg/admin/cost.go` | — | ⬜ | 吸收 5.1.5，Calculate(usage, model) → Cost |
| 10.2.4 | Provider 健康采集器（成功率/延迟/熔断状态） | `pkg/admin/health.go` | 2.3.1 | ⬜ | 定时采样 + 请求级更新 |
| 10.2.5 | Manager 集成记录器 | `pkg/manager/manager.go` | 10.2.1, 10.2.2 | ⬜ | Chat()/ChatStream() 中嵌入记录逻辑，吸收 5.3.5 |
| 10.2.6 | SDK WithRequestLog() + WithAdmin() 选项 | `pkg/gateway/options.go`, `client.go` | 10.2.5, 10.3.1 | ⬜ | SDK 用户可选开启记录和管理后台 |

### 10.3 Admin REST API

| ID | 任务 | 产出文件 | 前置依赖 | 状态 | 备注 |
|----|------|---------|---------|------|------|
| 10.3.1 | Admin Server + 路由注册 + 简易鉴权 | `api/admin/router.go` | 10.1.2 | ⬜ | 支持挂载到主 Server 或独立启动（SDK 场景） |
| 10.3.2 | GET /admin/api/dashboard — 概览统计 | `api/admin/handler_dashboard.go` | 10.3.1 | ⬜ | 总请求数/成功率/token 消耗/费用/活跃 provider |
| 10.3.3 | GET /admin/api/requests — 日志查询（分页/过滤/搜索） | `api/admin/handler_requests.go` | 10.3.1 | ⬜ | 按 model/provider/status/时间范围过滤 |
| 10.3.4 | GET /admin/api/requests/:id — 请求详情 | `api/admin/handler_requests.go` | 10.3.3 | ⬜ | 含完整 input/output（脱敏后）、token 明细、耗时 |
| 10.3.5 | GET /admin/api/usage/summary — Token 消耗汇总 | `api/admin/handler_usage.go` | 10.3.1, 10.2.3 | ⬜ | 按模型/Provider/租户维度聚合 |
| 10.3.6 | GET /admin/api/usage/trend — 消耗趋势（时间序列） | `api/admin/handler_usage.go` | 10.3.5 | ⬜ | 按小时/天/周粒度，支持对比 |
| 10.3.7 | GET /admin/api/providers — Provider 状态列表 | `api/admin/handler_providers.go` | 10.2.4 | ⬜ | 在线状态/成功率/平均延迟/熔断状态 |
| 10.3.8 | GET/PUT /admin/api/models — 模型目录管理 | `api/admin/handler_models.go` | 10.3.1 | ⬜ | 查看 + 编辑模型目录和路由规则 |

### 10.4 前端

| ID | 任务 | 产出文件 | 前置依赖 | 状态 | 备注 |
|----|------|---------|---------|------|------|
| 10.4.1 | 前端项目初始化（React + Ant Design + Vite） | `web/` | — | ⬜ | Layout + 路由 + API Client + 主题 |
| 10.4.2 | Dashboard 概览页 | `web/src/pages/Dashboard/` | 10.3.2 | ⬜ | 请求量/Token/费用/成功率卡片 + 趋势图(ECharts) |
| 10.4.3 | 请求日志查看器 | `web/src/pages/Requests/` | 10.3.3, 10.3.4 | ⬜ | 表格 + 搜索过滤 + 详情抽屉（完整输入输出） |
| 10.4.4 | Token 消耗分析页 | `web/src/pages/Usage/` | 10.3.5, 10.3.6 | ⬜ | 多维度图表：按模型/Provider/时间 |
| 10.4.5 | Provider 监控页 | `web/src/pages/Providers/` | 10.3.7 | ⬜ | 状态卡片 + 延迟/成功率折线图 |
| 10.4.6 | 模型管理页 | `web/src/pages/Models/` | 10.3.8 | ⬜ | 模型列表 + 编辑表单 + 路由规则配置 |
| 10.4.7 | 前端 embed 打包（go:embed 嵌入二进制） | `api/admin/static.go` | 10.4.2~10.4.6 | ⬜ | 单二进制部署，无需独立前端服务器 |

### Sprint 10 里程碑验证

- [ ] HTTP 模式：请求后在管理后台看到完整日志（model/tokens/latency/status）
- [ ] SDK 模式：`WithRequestLog() + WithAdmin()` 后同样能看到管理后台
- [ ] Dashboard 页：总请求数、Token 消耗、费用、成功率实时更新
- [ ] 请求详情：点开能看到完整的 input/output 摘要和 token 明细
- [ ] Token 分析：按模型/Provider/时间维度查看消耗趋势
- [ ] Provider 监控：能看到各 provider 在线状态和成功率
- [ ] 单二进制部署：`go build` 产出包含前端的单个可执行文件

---

## 变更日志

> 每次修改本文件时，在此记录时间、操作人/Agent、变更内容。

| 日期 | 操作者 | 变更 |
|------|--------|------|
| 2026-03-11 | Claude | 新增 Sprint 10 管理后台与可观测性（23 项任务），数据采集在 Manager 层（HTTP+SDK 双模式支持），吸收 5.1.5/5.3.1-5.3.5/8.3.1/8.4.1-8.4.3 |
| 2026-03-11 | Claude | architecture-design.md 全面同步：S6/S8/S9/S10/S11/S12/S13/S14 对齐实际实现，commit `6a7f721` |
| 2026-03-11 | Claude | README 全面重写，涵盖部署/请求方式/添加厂商指南/配置参考 |
| 2026-03-11 | Claude | Per-provider HTTP 代理：transport 层 + provider 构造器 + config.yaml |
| 2026-03-11 | Claude | OpenAI adapter 增强：configurable endpoints、BearerAuth 修复、enable_thinking scope 修复 |
| 2026-03-11 | Claude | ProviderConfig.Extra map + GetExtra/GetExtraInt helpers，支持厂商自定义参数 |
| 2026-03-11 | Claude | Anthropic adapter：api_format switch 支持 OpenAI 兼容代理 |
| 2026-03-11 | Claude | Responses API 支持（provider 接口 + handler + router） |
| 2026-03-11 | Claude | Sprint 3 Anthropic adapter (3.1.1, 3.1.3, 3.2.1-3.2.3) 完成，11 个单元测试通过 |
| 2026-03-11 | Claude | Sprint 3 Compatible Provider (3.3.1-3.3.3) 完成，复用 OpenAI adapter + NewWithName() |
| 2026-02-23 | Claude | Sprint 2 完成 (2.1.1-2.5.3)，commit `0674a11` |
| 2026-02-23 | Claude | Sprint 1 完成 (1.3.1-1.4.3)，commit `fcb252f` |
| 2026-02-22 | Claude (初始创建) | 创建开发清单，全部任务状态为 ⬜ |
