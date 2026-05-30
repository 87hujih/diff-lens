# AI PR Review 助手设计文档

## 1. 项目定位

项目名为 **diff-lens**。它是一个本地 Web 版 AI Pull Request Review 助手，面向需要快速理解 PR、识别潜在风险、生成 Review 建议的开发者。

用户输入 GitHub PR 链接后，系统自动获取 PR 元数据、变更文件和 diff 内容，通过确定性规则扫描与 LLM 上下文分析，流式生成结构化 Review 报告。工具的目标不是替代 reviewer，而是帮助 reviewer 更快进入上下文，提前发现高风险点，并减少重复性检查成本。

## 2. 目标与非目标

### 目标

- 支持公开 GitHub PR 分析，并可选使用 GitHub Token 提高 API 额度或访问私有仓库。
- 支持 PR 变更总结、风险代码识别、Review 建议生成。
- 使用 SSE 流式返回分析进度和结果，提升演示效果与用户体验。
- 使用 OpenAI 兼容 LLM 配置，便于切换 DeepSeek、Qwen、OpenAI 兼容服务或未来七牛云模型。
- 内置示例 PR 模式，保证比赛 Demo 稳定。
- 生成可复制到 GitHub 的 Review 建议。
- 在 README 和作品说明中解释模型选择、上下文获取方式、误报/漏报控制和未来扩展。

### 非目标

- 第一版不实现 GitHub App 自动评论。
- 第一版不实现 GitHub OAuth 登录。
- 第一版不做复杂分析历史、团队管理或多用户权限系统。
- 第一版不做语言专属深度静态分析，优先支持语言无关的通用风险规则。

## 3. 用户流程

1. 用户打开本地 Web 分析台。
2. 用户输入 GitHub PR URL，可选填写 GitHub Token。
3. 用户点击开始分析，或点击“使用示例 PR”进入稳定演示流程。
4. 前端通过 `POST /api/reviews/analyze/stream` 发起 SSE 流式分析请求。
5. 页面左侧展示分析步骤状态，右侧逐步展示阶段结果。
6. 系统依次完成 PR 获取、diff 解析、规则扫描、上下文构建、AI 分析、报告生成。
7. 用户查看最终报告，包括 PR 总结、风险卡片、证据片段和建议评论。
8. 用户可复制单条 Review 建议，也可一键复制完整 Review 评论。

## 4. 技术架构

后端使用 Go + Gin，前端使用 React + Vite。

```text
.
├── cmd/server
├── internal/config
├── internal/github
├── internal/diff
├── internal/rules
├── internal/llm
├── internal/review
├── internal/demo
├── internal/handler
└── frontend
```

第一版采用单体后端和请求内分析 pipeline，不引入数据库、任务队列或后台 worker。一次分析请求在同一个 HTTP 连接内完成，服务端通过 SSE 持续返回阶段事件和最终报告。

### 后端模块

- `config`：读取端口、GitHub Token、LLM Base URL、API Key、模型名等环境变量。
- `github`：解析 GitHub PR URL，拉取 PR 元数据、文件列表、patch、commit 信息。
- `diff`：解析 patch，统计文件类型、增删行数、测试文件和配置文件变化。
- `rules`：执行语言无关的通用风险规则扫描。
- `llm`：封装 OpenAI 兼容聊天补全接口。
- `review`：定义核心类型、接口和编排逻辑，生成内部 `ReviewEvent` 和最终报告。
- `demo`：提供稳定演示数据和演示事件流，复用真实模式的 SSE 事件契约。
- `handler`：实现 Gin HTTP 接口、请求校验和 SSE 编码，不直接编排 GitHub、diff、规则或 LLM 逻辑。

后端依赖方向：

```text
handler
  -> review.Service
       -> github.Client
       -> diff.Parser
       -> rules.Scanner
       -> review.ContextBuilder
       -> llm.Analyzer
       -> review.ReportNormalizer
```

`review` 是核心编排层。核心类型和接口放在 `internal/review`，`github`、`llm`、`demo` 是适配器。`handler` 只负责 HTTP 与 SSE 协议适配。这样可以单独测试分析流程，也方便后续加入 CLI、GitHub App 自动评论或任务队列。

真实模式的数据流：

```text
POST /api/reviews/analyze/stream
      |
      v
handler 校验请求并建立 SSE 响应
      |
      v
review.Service 产出 ReviewEvent
      |
      v
github.Client 获取 PR 元数据、文件、patch、commits
      |
      v
diff.Parser 解析 patch 并生成文件统计
      |
      v
rules.Scanner 扫描确定性候选风险
      |
      v
ContextBuilder 选择证据片段并压缩模型上下文
      |
      v
llm.Analyzer 分析 ReviewContext
      |
      v
ReportNormalizer 合并规则风险和 AI 输出
      |
      v
handler 编码 step/pr/rules/ai_delta/result/error/done 事件
```

示例模式放在后端。`demo` 模块直接产出与真实模式一致的 `ReviewEvent` 流，保证比赛现场、视频录制和本地调试都走同一套前端展示逻辑。

### 架构决策

- 第一版使用单体 Go 服务，请求内直接完成分析并通过 SSE 返回；不引入数据库、任务队列或后台 worker。
- demo 模式由后端提供，复用同一套 SSE 事件契约，不在前端硬编码完整假报告。
- 前端最终状态只信 `result` 事件，`ai_delta` 只用于展示 AI 分析过程。
- LLM 只能消费 `ContextBuilder` 生成的 `ReviewContext`，不能直接消费完整原始 diff。
- 只有 GitHub 获取失败等“没有输入数据”的错误终止真实分析；LLM、规则扫描、部分 diff 解析、JSON 解析失败都走降级报告。
- 核心类型、接口和编排放在 `internal/review`；`github`、`llm`、`demo` 是适配器；`handler` 只做 HTTP/SSE 适配。
- 前端使用简单 reducer 管理 SSE 状态，不引入 Redux 或 Zustand；业务判断全部放在后端。
- 测试重点覆盖后端 pipeline、降级路径和 SSE 契约；前端重点测试 reducer、SSE parser 和复制格式。

### 前端模块

- PR 输入区：URL 输入、Token 折叠输入、示例 PR 按钮。
- 步骤栏：展示获取 PR、解析 diff、规则扫描、上下文构建、AI 分析、报告生成。
- 报告区：展示 Review Brief、Risk Radar、Suggested Comments、Evidence Drawer。
- SSE 客户端：使用 `fetch` 读取 `text/event-stream`，支持 POST 请求体。

前端推荐结构：

```text
frontend/src/
├── api/
│   └── reviewStream.ts
├── types/
│   └── review.ts
├── state/
│   └── reviewReducer.ts
├── components/
│   ├── PrInputPanel.tsx
│   ├── StepTimeline.tsx
│   ├── ReviewBrief.tsx
│   ├── RiskRadar.tsx
│   ├── SuggestedComments.tsx
│   └── EvidenceDrawer.tsx
└── App.tsx
```

前端数据流保持单向：

```text
User submits PR URL
      |
      v
reviewStream 使用 fetch POST 建立 SSE
      |
      v
SSE parser 解析 step/pr/rules/ai_delta/result/error/done
      |
      v
reviewReducer 更新状态
      |
      v
UI 从状态渲染
```

前端状态建议：

```text
ReviewState
├── status: idle | running | completed | failed
├── steps
├── pr
├── ruleRisks
├── aiText
├── result
├── error
└── degraded
```

前端只负责请求发起、SSE 解析、状态机更新、展示、筛选和复制。diff 解析、风险判断、LLM 输出修复和报告归一化都放在后端。

## 5. 核心接口

### `POST /api/reviews/analyze/stream`

请求头：

```http
Accept: text/event-stream
Content-Type: application/json
```

请求体：

```json
{
  "pr_url": "https://github.com/owner/repo/pull/123",
  "github_token": "optional",
  "demo": false
}
```

服务端返回 `text/event-stream`。

事件类型：

```text
step        分析阶段状态
pr          PR 基本信息
rules       规则扫描结果
ai_delta    AI 分析流式内容
result      完整结构化报告
error       错误信息
done        分析结束
```

示例事件：

```text
event: step
data: {"step":"fetch_pr","status":"running","message":"正在获取 PR 信息"}

event: rules
data: {"risks":[{"severity":"medium","title":"核心代码变化但未检测到测试文件变更"}]}

event: result
data: {"summary":{"risk_level":"medium","overview":"..."},"risks":[],"comments":[]}

event: done
data: {"ok":true}
```

内部统一使用 `ReviewEvent` 表示事件，`handler` 负责把事件编码为 SSE：

```text
ReviewEvent
├── type: step | pr | rules | ai_delta | result | error | done
├── data: 对应事件 payload
└── request_id / timestamp: 可选调试字段
```

`ai_delta` 不驱动最终报告 UI，只用于展示模型正在分析。最终报告必须来自 `result` 事件。

## 6. 报告数据模型

```json
{
  "pr": {
    "title": "string",
    "author": "string",
    "repo": "owner/repo",
    "number": 123,
    "source_branch": "feature/x",
    "target_branch": "main",
    "changed_files": 8,
    "additions": 120,
    "deletions": 40,
    "commits": 3
  },
  "summary": {
    "risk_level": "low | medium | high",
    "overview": "string",
    "key_changes": ["string"],
    "review_focus": ["string"]
  },
  "risks": [
    {
      "id": "risk-1",
      "source": "rule | ai | merged",
      "severity": "low | medium | high",
      "confidence": 0.82,
      "category": "secret | dangerous_operation | injection | error_handling | test_gap | config | size",
      "title": "string",
      "file": "src/example.go",
      "line": 42,
      "evidence": "string",
      "reason": "string",
      "suggestion": "string"
    }
  ],
  "comments": [
    {
      "id": "comment-1",
      "file": "src/example.go",
      "line": 42,
      "body": "string"
    }
  ]
}
```

`source` 用于区分风险来源：

- `rule`：确定性规则命中。
- `ai`：LLM 基于上下文和证据提出。
- `merged`：规则和 LLM 指向同一风险后合并。

最终 UI 以 `result` 中的归一化报告为准。规则风险和 AI 风险在后端合并、去重和降级，前端不重新推导业务结论。

## 7. 规则扫描策略

第一版规则扫描聚焦语言无关风险：

- 敏感信息风险：疑似 token、secret、password、private key。
- 危险操作风险：删除、清空、重置、强制覆盖、批量更新等危险行为。
- SQL/命令拼接风险：疑似 SQL 或 shell 命令拼接用户输入。
- 错误处理风险：空 catch、吞异常、忽略错误返回。
- 测试缺口：核心代码变化但没有测试文件变化。
- 变更规模风险：单 PR 文件数或 diff 行数过大。
- 配置依赖风险：CI、Docker、锁文件、环境变量、权限配置变化。

规则扫描的定位是稳定召回候选风险。LLM 负责基于候选风险和 diff 证据做解释、排序和 Review 建议生成。

## 8. LLM 分析策略

LLM 使用 OpenAI 兼容接口，通过环境变量配置：

```env
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=replace-me
LLM_MODEL=deepseek-chat
```

上下文构建不直接把完整 PR diff 全量塞给模型，而是压缩为：

- PR 标题、描述、作者、分支和 commit 信息。
- 文件变更统计。
- 关键 diff 片段。
- 规则扫描命中结果。
- 测试文件、配置文件、依赖文件变化情况。

Prompt 约束：

- 输出结构化 JSON。
- 每条风险必须包含证据、严重级别、置信度、原因和建议动作。
- 没有 diff 证据的问题不能上升为明确风险。
- 不确定的问题标记为 `needs_attention` 或降低严重级别。
- Review 建议应可直接复制到 GitHub PR 评论中。

## 9. 误报与漏报控制

误报控制：

- 高风险必须有明确 diff 证据，最好同时有规则命中。
- UI 展示置信度，避免把不确定判断伪装成确定 bug。
- 对证据不足的问题降级为关注点。
- 模型输出解析失败时返回规则扫描结果和降级提示，而不是生成不可信报告。

漏报控制：

- 确定性规则先扫全量 diff。
- 文件分类确保配置、依赖、CI、权限、测试缺口不被忽略。
- 大型 PR 优先抽取高风险文件和规则命中片段给模型。
- 未来可加入语言专属规则和跨文件调用关系检索。

## 10. 安全与降级策略

Token 处理：

- 用户输入的 GitHub Token 只用于当前分析请求，不写入数据库或本地文件。
- 服务端日志不打印 token、Authorization header 或完整请求体。
- 前端 Token 输入默认折叠和密码显示，刷新页面后不保留。
- README 明确建议使用最小权限 token。

异常降级：

- GitHub API 限额或网络失败时，返回明确错误事件，并提示用户配置 token 或使用示例模式。
- LLM 请求失败时，保留 PR 信息和规则扫描结果，报告标记为“AI 分析未完成”。
- LLM JSON 解析失败时，返回原始规则风险和可读错误，不展示未经解析的模型输出。
- PR diff 过大时，后端优先分析规则命中文件、配置文件、依赖文件和测试相关文件，并在报告中提示上下文被裁剪。

流式响应约束：

- 第一版使用 `fetch` 读取 `text/event-stream`，不是原生 `EventSource`，因为请求需要 POST body。
- 每个 SSE 事件必须是可独立解析的 JSON。
- 服务端在每个阶段 flush 输出，避免用户长时间看到空白等待。
- 错误通过 `event: error` 返回，最后仍发送 `done` 或关闭连接。

## 11. 前端体验

页面采用工作流向导型分析台。

未开始状态：

- PR URL 输入框。
- 可折叠 GitHub Token 输入。
- 使用示例 PR 按钮。

分析中状态：

- 左侧展示步骤进度。
- 右侧展示当前阶段结果。
- PR 信息、规则风险和 AI 内容逐步出现。

完成状态：

- `Review Brief`：PR 总结、关键变更、整体风险等级。
- `Risk Radar`：风险卡片，支持按 High / Medium / Low 筛选。
- `Suggested Comments`：单条复制和一键复制完整 Review。
- `Evidence Drawer`：展示风险关联的 diff 证据片段。

视觉方向是专业、清晰、开发工具感。避免营销页式大图和空泛介绍，第一屏就是可操作分析台。

## 12. 示例模式

示例模式使用内置 mock PR 数据和 mock 分析结果，但仍走同样的 SSE 事件流。

示例模式用途：

- 保证比赛现场和视频演示稳定。
- 在 GitHub API 限额、网络或模型响应异常时仍能展示完整产品流程。
- 帮评委快速理解最终体验。

README 中明确说明示例模式用于演示，真实 PR 模式仍会调用 GitHub API 和 LLM。

## 13. 测试策略

后端测试：

- GitHub PR URL 解析测试。
- diff 文件分类和统计测试。
- 通用风险规则扫描测试。
- SSE 事件编码测试。
- LLM JSON 输出解析和失败降级测试。

前端测试：

- 输入校验。
- SSE 流解析。
- 步骤状态更新。
- 风险卡片过滤。
- Review 建议复制。

手动验收：

- 示例 PR 模式完整跑通。
- 真实公开 PR 分析成功。
- 无 GitHub Token 时能分析公开 PR。
- GitHub API 或 LLM 出错时能展示清晰错误。

## 14. 验收标准

项目完成时应满足：

- 示例 PR 模式可以稳定完成完整 SSE 流程。
- 至少一个公开 GitHub PR 可以在真实模式下完成分析。
- 规则扫描至少覆盖敏感信息、危险操作、测试缺口、配置依赖变化和大 PR 风险。
- 最终报告包含摘要、风险卡片、证据片段和可复制 Review 建议。
- README 说明运行方式、环境变量、模型配置、Demo 步骤、设计思路和已知限制。
- 后端核心逻辑有单元测试，前端关键交互经过手动验收。

## 15. 三天开发与 PR 拆分

第 1 天目标：后端主链路可跑通。

- `chore: initialize go gin and react vite workspace`
- `feat: fetch github pull request metadata and files`
- `feat: add diff parser and rule-based risk scanner`
- `feat: stream review analysis progress with sse`

第 2 天目标：AI 分析和前端报告成型。

- `feat: add openai compatible llm review analyzer`
- `feat: build workflow review dashboard`
- `feat: add demo pr mode`

第 3 天目标：打磨、测试、文档和视频。

- `feat: add copyable review comments`
- `test: cover github parser rule scanner and stream events`
- `docs: complete competition readme and demo guide`

## 16. 未来扩展

- GitHub App 自动评论。
- GitHub OAuth 登录和私有仓库授权。
- 多语言专属规则。
- 跨文件调用图和上下文检索。
- 团队 Review 规范配置。
- 历史分析缓存和风险趋势。
- 接入七牛云模型或对象存储保存报告。
- 与 CI 集成，在 PR 创建后自动生成 Review 报告。
