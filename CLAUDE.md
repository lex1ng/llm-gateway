# LLM Gateway 开发规范

## 开发流程

### 1. 任务选择
- **必须**按照 `docs/dev-checklist.md` 的任务顺序开发
- 选择下一个 `⬜` 状态且**无未完成前置依赖**的任务
- 开工前将状态改为 `🔨`

### 2. 开发参考
- **实现指南**：`docs/development-plan.md` — 每个 Task 的详细实现步骤和验收标准
- **设计参考**：`docs/architecture-design.md` — 接口定义、数据结构、代码示例

### 3. 冲突处理
| 冲突类型 | 处理方式 |
|---------|---------|
| 字段名、返回类型等小问题 | 直接修改 `architecture-design.md`，以实际代码为准 |
| 功能定义不清、设计缺陷 | 暂停开发，询问用户意见 |

### 4. 完成任务
1. 代码完成后，运行测试确保通过
2. 使用 `git commit` 保存代码
3. 更新 `docs/dev-checklist.md`：
   - 状态改为 `✅`
   - 备注中记录 commit hash（如 `fc47b02`）
   - 简要说明产出

### 5. Git 提交规范
```bash
# 提交格式
git commit -m "feat(module): description

- bullet point 1
- bullet point 2

Co-Authored-By: Claude <noreply@anthropic.com>"
```

## 目录结构
```
llm-gateway/
├── cmd/server/          # HTTP 服务入口
├── pkg/
│   ├── types/           # 统一类型定义
│   ├── config/          # 配置加载
│   ├── transport/       # HTTP 客户端
│   ├── adapter/         # Provider 适配器
│   ├── manager/         # 编排层
│   ├── gateway/         # SDK 入口
│   └── hook/            # Hook 系统
├── api/handler/         # HTTP Handler
└── config/              # 配置文件
```

## 当前进度
> 每次开发前先检查 `docs/dev-checklist.md` 获取最新进度

Sprint 1 — 地基层（Types + Config + Transport）待开始
