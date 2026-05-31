# ContextBuilder 与 LLM 分析实现计划

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本计划。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 在 GitHub 获取、diff 解析和规则扫描之后，构建受控的模型上下文，调用 OpenAI 兼容 Chat Completions 接口，并生成合并规则风险与 AI 风险的结构化 Review 报告。

**架构：** `internal/review` 拥有 `ReviewContext`、`ReviewAnalysis`、AI 风险类型、`ContextBuilder`、`ReportNormalizer` 和 service 内部接口。`internal/llm` 只作为 OpenAI 兼容 HTTP adapter，import `internal/review` 来实现 `review.AIAnalyzer` 接口；`review.Service` 只依赖接口，不 import `internal/llm`，避免 Go import cycle。LLM 只能消费裁剪、脱敏、带证据 ID 的 `ReviewContext`，不能直接读取完整 raw diff。`review.Service` 以真流式 channel 串联 GitHub -> diff -> rules -> context -> LLM -> normalize，并在 LLM 失败时保留已有 PR 信息和规则风险，返回带明确 meta 的 degraded 报告。

**技术栈：** Go 标准库 `net/http`、Go JSON 编解码、OpenAI 兼容 Chat Completions API、现有 SSE 事件契约。

---

## 前置条件与硬约束

- 必须先完成并合并 `docs/superpowers/plans/2026-05-30-diff-rules-scanner.md`。本计划依赖 `diff.Analysis`、结构化 hunks、文件分类和 `rules.Finding`；当前仓库中的 `internal/diff` 与 `internal/rules` 若仍是 stub，本计划只能实现接口和 `internal/llm` 单测，不能接入真实 pipeline。
- `internal/review` 不能 import `internal/llm`。`cmd/server/main.go` 负责同时 import `review` 和 `llm` 并完成依赖注入。
- `internal/llm` 可以 import `internal/review`，实现 `review.AIAnalyzer` 接口：输入 `review.ReviewContext`，输出 `review.ReviewAnalysis`。
- 真实模式 `Analyze` 必须立即返回事件 channel，pipeline 在 goroutine 内执行，不能等 GitHub、diff 或 LLM 全部完成后再用 `closedEventStream` 一次性返回。
- 模型输出中的风险必须引用 `ReviewContext` 内已有的 `evidence_refs`。无法匹配证据 ID 的高/中风险不得进入最终 `risks`，最多进入关注点或被丢弃。
- `Report` 需要保留兼容现有前端的 `degraded` 字段，同时增加 `meta` 说明 `ai_completed`、`rules_completed`、`context_truncated` 和 `degraded_reason`。

## 依赖方向

```text
cmd/server
  -> review.NewService(..., AIAnalyzer: llm.NewAnalyzer(...))

review.Service
  -> GitHubClient interface
  -> DiffParser interface
  -> RuleScanner interface
  -> ContextBuilder
  -> AIAnalyzer interface
  -> ReportNormalizer

internal/llm
  -> internal/review types
```

禁止出现：

```text
internal/review -> internal/llm
```

## 执行拆分与子 agent 派发记录

本计划按依赖拆成 5 条工作流执行：

| Lane | 范围 | 写入边界 | 状态 |
| --- | --- | --- | --- |
| A | 前置 diff/rules 核心 | `internal/diff/*`, `internal/rules/*` | 已由子 agent 完成，`go test ./internal/diff ./internal/rules` 通过 |
| B | review context/normalizer 合约 | `internal/review/context_*`, `internal/review/report_normalizer*`, `internal/review/types.go`, `frontend/src/types/review.ts` | 已由子 agent 完成，`go test ./internal/review -run 'ContextBuilder|ReportNormalizer'` 通过 |
| C | LLM adapter | `internal/llm/*` | 已由子 agent 完成，`go test ./internal/llm` 通过 |
| D | service/cmd/server 集成 | `internal/review/service.go`, `internal/review/service_test.go`, `cmd/server/main.go`, `internal/handler/review_handler_test.go` | 已集成真流式 pipeline 和 analyzer 注入 |
| E | 文档与最终验证 | `README.md`、全量验证命令 | 已完成 |

执行顺序：

```text
A + B 并行
  -> C 与 D 并行准备
  -> D 接入 C
  -> E 文档与全量验证
```

最终验证：

- `go test ./...`
- `npm --prefix frontend run build`
- `node scripts/check-pr-quality.test.mjs`
- `go list -deps ./internal/review | Select-String "diff-lens/internal/llm"` 无输出，确认 `internal/review` 没有依赖 `internal/llm`

## 背景

前置模块完成后，真实 PR 模式应已经具备：

- 通过 `internal/github` 获取 PR metadata、changed files、commits 和 raw patch。
- 通过 `internal/diff` 解析 patch，得到文件分类、hunk、行号和统计信息。
- 通过 `internal/rules` 输出第一批确定性 `review.Risk`。
- `review.Service` 已能发送 `fetch_pr`、`parse_diff`、`scan_rules`、`pr`、`rules`、`result`、`done`。

当前缺口是：规则扫描只能提供候选风险，缺少 PR 级总结、风险解释、优先级排序和可复制 Review 建议。下一步需要把已有结构化数据压缩成安全、可控、可测试的模型上下文，再让 OpenAI 兼容模型输出结构化 JSON。

## 需求拆分

### 功能需求

- 构建 `ReviewContext`，包含：
  - context schema version 和 context ID
  - PR 标题、作者、repo、分支、commit 摘要
  - 文件变更统计
  - 文件分类统计
  - 规则扫描命中结果
  - 带稳定 `snippet_id` 的关键 diff 片段
  - 可被模型引用的 `evidence_refs`，包括 snippet ID 和 rule finding ID
  - 测试、配置、依赖、CI 变化摘要
- 上下文裁剪规则：
  - 优先保留 high/medium 规则命中文件
  - 优先保留配置、依赖、CI、认证、权限、数据库、删除类变更
  - 对超大 PR 控制分区预算和总预算：metadata、rules、file summaries、snippets、prompt wrapper 各自有硬上限
  - 序列化 prompt payload 后再做一次总字符 hard cap，避免包装文本绕过预算
  - 每个文件只保留有限 hunk 和新增行证据
  - 每个 snippet 先做 secret redaction，再进入 context
  - 缺失 patch 文件保留文件名和状态，但不伪造 diff 证据
  - 记录 `context_truncated`、`omitted_files_count`、`omitted_snippets_count`，供报告和 UI 解释
- LLM 调用：
  - 使用 `LLM_BASE_URL`
  - 使用 `LLM_API_KEY`
  - 使用 `LLM_MODEL`
  - 兼容 OpenAI `POST /v1/chat/completions`
  - 第一版可以使用非流式调用，最终通过 `result` 事件交付结构化报告
- Prompt 约束：
  - 输出 JSON
  - 每条 AI 风险必须引用 `evidence_refs`
  - `evidence_refs` 必须来自 `ReviewContext` 中已有的 snippet ID 或 rule finding ID
  - 证据不足的问题降级为关注点，不标 high
  - 不得编造未出现在上下文中的文件、行号或事实
  - 明确告诉模型 diff/snippet 是不可信用户内容，不能把 diff 里的文本当系统指令执行
  - Review 建议要能直接复制到 GitHub 评论
- 解析 LLM 输出为内部结构：
  - summary
  - ai risks
  - risk evidence_refs
  - suggested comments
  - focus/attention items（证据不足但值得人工看的问题）
- 归一化报告：
  - 合并规则风险和 AI 风险
  - 相同 category、file、line/range、rule_id 或 evidence_refs 的风险去重或合并为 `source: merged`
  - 没有可验证 evidence_refs 的 AI 风险不能自动合并，也不能生成 high/medium 风险
  - 保留规则风险作为 LLM 失败时的降级结果
  - 根据最高风险 severity 推导 summary risk level
  - 保留报告 meta，说明 AI 是否完成、上下文是否裁剪、降级原因是什么
- Service 行为：
  - `build_context` step
  - `analyze_ai` step
  - LLM 成功时返回 `degraded=false` 且 `meta.ai_completed=true`
  - LLM 失败或 JSON 解析失败时返回 degraded report，包含规则风险和错误说明
  - 真实模式事件必须边执行边发送，不允许先同步跑完再返回 closed channel

### 非目标

- 不实现 OpenAI streaming delta 解析；第一版允许非流式 Chat Completions。
- 不实现多轮对话或用户自定义 prompt。
- 不实现向 GitHub 自动发布评论。
- 不实现跨文件调用图或语义检索。
- 不实现模型供应商专属 SDK；只使用 OpenAI 兼容 HTTP API。
- 不把完整 raw diff 直接发送给模型。

### 验收标准

- `go test ./internal/review` 覆盖 ContextBuilder、ReportNormalizer 和 service 成功/降级路径。
- `go test ./internal/llm` 覆盖请求格式、认证 header、JSON 输出解析和失败映射。
- `go test ./...` 通过。
- LLM 未配置 API key 时，真实模式不崩溃，返回包含规则风险的 degraded report。
- LLM 返回非法 JSON 时，真实模式不展示未经解析的模型文本，返回 degraded report。
- LLM 成功时，最终 `result` 包含 summary、rules/AI/merged risks 和 suggested comments。
- 真实模式能先发送 `fetch_pr running`，再继续执行后续阶段；不能在 LLM 完成后才一次性 flush 全部事件。
- 模型返回的 `evidence_refs` 必须能映射回 context snippets 或 rule findings；无法映射的风险不进入 high/medium risks。
- `result.meta` 能说明 AI 是否完成、规则扫描是否完成、上下文是否被裁剪和降级原因。
- README 准确说明模型配置、降级行为和当前限制。

## 文件分工

- 新建：`internal/review/context_builder.go`
  - 实现 `ContextBuilder`、预算控制、文件优先级和证据片段选择。
- 新建：`internal/review/context_builder_test.go`
  - 覆盖上下文裁剪、优先级、预算、缺失 patch、规则命中优先。
- 新建：`internal/review/context_types.go`
  - 定义 `ReviewContext`、`ContextFile`、`ContextSnippet`、`ContextStats`、`ReviewAnalysis`、`AIRisk`、`AnalysisMeta` 等 review 层结构。
- 新建：`internal/review/report_normalizer.go`
  - 合并规则风险和 AI 输出，生成最终 `review.Report`。
- 新建：`internal/review/report_normalizer_test.go`
  - 覆盖风险合并、去重、降级报告、risk level 推导。
- 修改：`internal/llm/analyzer.go`
  - 实现 OpenAI 兼容 Chat Completions 请求、prompt 生成和响应解析。
- 新建：`internal/llm/types.go`
  - 定义 `AnalyzerOptions`、`AnalyzerError`、OpenAI DTO；不要定义最终 review 领域类型，避免和 `internal/review` 互相依赖。
- 新建：`internal/llm/analyzer_test.go`
  - 使用 `httptest.Server` 覆盖成功、认证、HTTP 错误、非法 JSON、模型输出非法 JSON。
- 修改：`internal/review/service.go`
  - 注入 ContextBuilder、LLM Analyzer、ReportNormalizer，并接入真实模式。
- 修改：`internal/review/service_test.go`
  - 使用 fake analyzer 覆盖成功与降级事件流。
- 修改：`cmd/server/main.go`
  - 读取 config 中 LLM 配置，注入真实 analyzer。
- 修改：`frontend/src/types/review.ts`
  - 增加 `ReportMeta` 类型，对齐后端 `Report.Meta`。
- 修改：`frontend/src/state/reviewReducer.ts`
  - 继续兼容 `degraded`，同时保留 result meta 供 UI 展示。
- 修改：`README.md`
  - 更新 AI 分析能力、模型配置和降级说明。

## 任务 0：前置模块与依赖方向确认

**文件：**
- 只读检查：`internal/diff`
- 只读检查：`internal/rules`
- 只读检查：`internal/review/service.go`

- [ ] 确认 `docs/superpowers/plans/2026-05-30-diff-rules-scanner.md` 已完成，`internal/diff` 已提供 `diff.Analysis`、文件分类、hunks 和 warnings。
- [ ] 确认 `internal/rules` 已提供不依赖 `internal/review` 的 `rules.Finding`。
- [ ] 如果前置模块未完成，停止真实 pipeline 接入，只允许实现 `review`/`llm` 接口 scaffold 和单元测试。
- [ ] 写或更新架构测试/脚本，确认 `internal/review` 不 import `internal/llm`。
- [ ] 确认 `cmd/server/main.go` 是唯一负责把 `llm.NewAnalyzer` 注入 `review.Service` 的地方。

## 任务 1：ReviewContext 与 AI 输出数据结构

**文件：**
- 新建：`internal/review/context_types.go`
- 新建：`internal/review/context_builder_test.go`
- 修改：`internal/review/types.go`
- 修改：`frontend/src/types/review.ts`

- [ ] 定义 `ReviewContext`，包含 schema version、context ID、PR 元数据、commit 摘要、统计信息、规则风险和上下文文件。
- [ ] 定义 `ContextFile`，包含 filename、kind、status、additions、deletions、risk IDs、snippets。
- [ ] 定义 `ContextSnippet`，包含 stable `id`、file、start line、end line、patch 片段、reason。
- [ ] 定义 `ContextStats`，包含 changed files、additions、deletions、test/config/dependency/CI/source、omitted files、omitted snippets、truncated 统计。
- [ ] 定义 `ReviewAnalysis`、`AIRisk`、`AnalysisComment`，作为 LLM adapter 解析后的领域输出，放在 `internal/review` 而不是 `internal/llm`。
- [ ] `AIRisk` 必须包含 `EvidenceRefs []string`，后续 Normalizer 用它验证证据来源。
- [ ] 定义 `ReportMeta`/`AnalysisMeta`，至少包含 `ai_completed`、`rules_completed`、`context_truncated`、`degraded_reason`、`omitted_files_count`、`omitted_snippets_count`。
- [ ] 在 `review.Risk` 上增加 `EvidenceRefs []string`，JSON 字段为 `evidence_refs,omitempty`，用于最终报告追溯 AI/merged 风险证据。
- [ ] 在 `review.Report` 上增加 `Meta ReportMeta`，保留原有 `Degraded bool` 以兼容现有前端。
- [ ] 在 `frontend/src/types/review.ts` 上同步增加 `Risk.evidence_refs?: string[]` 和 `ReportMeta`，不要破坏现有 `degraded?: boolean`。
- [ ] 写失败测试：能表达 PR 信息、规则风险和 diff 片段。
- [ ] 写失败测试：每个 snippet 都有稳定 ID，并且同一输入重复 build ID 不变。
- [ ] 写失败测试：AI 风险必须能表达 evidence_refs。
- [ ] 写失败测试：缺失 patch 文件能进入 context，但 snippets 为空。
- [ ] 暂不实现 builder，只定义可测试的数据契约。
- [ ] 运行 `go test ./internal/review`，确认因 builder 未实现而只失败在预期测试。

## 任务 2：ContextBuilder 优先级与预算控制

**文件：**
- 新建：`internal/review/context_builder.go`
- 修改：`internal/review/context_builder_test.go`

- [ ] 定义 `ContextBuilderOptions`：
  - `MaxChars`
  - `MaxMetadataChars`
  - `MaxRuleChars`
  - `MaxFileSummaryChars`
  - `MaxFiles`
  - `MaxSnippetsPerFile`
  - `MaxSnippetChars`
- [ ] 实现默认预算，避免无配置时把完整 diff 送给模型。
- [ ] 实现分区预算：metadata、rules、file summaries、snippets 分别裁剪。
- [ ] 在序列化 prompt payload 后做最终总长度 hard cap，并设置 `ContextStats.Truncated=true`。
- [ ] 在 snippet 进入 context 前做 secret redaction，确保疑似 API key、password、private key 原文不会进入 LLM。
- [ ] 写失败测试：high/medium 规则命中文件优先进入 context。
- [ ] 写失败测试：配置、依赖、CI 文件优先于普通源码文件。
- [ ] 写失败测试：超过 `MaxChars` 时会裁剪，但仍保留 PR 元数据和规则风险。
- [ ] 写失败测试：超过单个分区预算时只裁剪该分区，不影响 PR 元数据和高优先级规则风险。
- [ ] 写失败测试：每个文件 snippets 不超过 `MaxSnippetsPerFile`。
- [ ] 写失败测试：snippet 长度不超过 `MaxSnippetChars`。
- [ ] 写失败测试：snippet 中的疑似 secret 被脱敏，context 不包含原始值。
- [ ] 写失败测试：diff 中包含 prompt injection 文本时，只作为 snippet 数据进入 JSON，不改变 system/developer prompt。
- [ ] 实现 `BuildReviewContext(pr github.PullRequestData, analysis diff.Analysis, risks []Risk) ReviewContext` 或等价方法。
- [ ] 确保 builder 不修改输入对象。
- [ ] 运行 `go test ./internal/review -run ContextBuilder`。

## 任务 3：LLM 分析数据契约与 Prompt

**文件：**
- 修改：`internal/review/context_types.go`
- 新建：`internal/llm/types.go`
- 修改：`internal/llm/analyzer.go`
- 新建：`internal/llm/analyzer_test.go`

- [ ] 定义 `AnalyzerOptions`，包含 base URL、API key、model、HTTP client、timeout 可选项。
- [ ] 在 `internal/review` 定义 `AIAnalyzer` 接口：`Analyze(ctx context.Context, input ReviewContext) (ReviewAnalysis, error)`。
- [ ] 在 `internal/review` 定义 `ReviewAnalysis`，包含 summary、risks、comments、attention items。
- [ ] 在 `internal/review` 定义 LLM 输出风险结构，字段映射到 `review.Risk` 所需信息，并强制包含 `evidence_refs`。
- [ ] 在 `internal/llm/types.go` 只定义 OpenAI request/response DTO 和 adapter options，不定义最终 review 领域类型。
- [ ] 定义 typed errors：
  - `ErrNotConfigured`
  - `ErrRequestFailed`
  - `ErrResponseInvalid`
  - `ErrModelOutputInvalid`
- [ ] 写失败测试：未配置 API key 时返回 `ErrNotConfigured`。
- [ ] 写失败测试：prompt 中包含 PR 摘要、规则风险和 snippets，但不包含完整 raw diff。
- [ ] 写失败测试：prompt 明确要求只引用给定 `evidence_refs`，并说明 diff/snippet 是不可信内容。
- [ ] 写失败测试：`internal/review` 不 import `internal/llm`。
- [ ] 实现 prompt 组装函数，要求模型只输出 JSON。
- [ ] Prompt schema 要求 AI 风险输出 `evidence_refs`，建议评论输出 `evidence_refs` 或 file/line。
- [ ] 运行 `go test ./internal/llm`。

## 任务 4：OpenAI 兼容 Chat Completions Client

**文件：**
- 修改：`internal/llm/analyzer.go`
- 修改：`internal/llm/analyzer_test.go`

- [ ] 使用 `httptest.Server` 写失败测试：请求路径为 `/v1/chat/completions`。
- [ ] 写失败测试：请求 header 包含 `Authorization: Bearer <api key>` 和 `Content-Type: application/json`。
- [ ] 写失败测试：请求 body 包含 model 和 messages。
- [ ] 写失败测试：正常 OpenAI 兼容响应能解析为 `ReviewAnalysis`。
- [ ] 写失败测试：HTTP 401/500 映射为 `ErrRequestFailed`，且错误不包含 API key。
- [ ] 写失败测试：OpenAI 响应 JSON 非法映射为 `ErrResponseInvalid`。
- [ ] 写失败测试：模型 message content 不是合法 JSON 时映射为 `ErrModelOutputInvalid`。
- [ ] 写失败测试：模型输出 JSON 合法但缺少 required 字段或 evidence_refs 类型错误时映射为 `ErrModelOutputInvalid`。
- [ ] 实现 `Analyzer.Analyze(ctx context.Context, input review.ReviewContext) (review.ReviewAnalysis, error)` 或等价方法。
- [ ] 暂不实现 streaming；请求中 `stream` 保持 false 或省略。
- [ ] 运行 `go test ./internal/llm`。

## 任务 5：ReportNormalizer

**文件：**
- 新建：`internal/review/report_normalizer.go`
- 新建：`internal/review/report_normalizer_test.go`

- [ ] 写失败测试：只有规则风险时生成 degraded report，risks 保留规则结果。
- [ ] 写失败测试：LLM 成功时 summary 使用 AI summary，但 PR 信息来自真实 PR。
- [ ] 写失败测试：规则风险和 AI 风险 category、file、line/range、rule_id 或 evidence_refs 指向同一证据时会合并为 `source: merged`。
- [ ] 写失败测试：相同 file/line/category 但 evidence_refs 不同的风险不会被误合并。
- [ ] 写失败测试：AI 风险缺少 evidence_refs、file 或 line 时降级 severity 或丢弃为 comment/focus，不生成高风险。
- [ ] 写失败测试：AI 风险引用不存在的 evidence_refs 时不得进入 high/medium risks。
- [ ] 写失败测试：risk level 根据最高 severity 推导，high > medium > low。
- [ ] 写失败测试：comments 为空时仍返回空数组，不返回 nil 导致前端歧义。
- [ ] 写失败测试：`Report.Meta` 正确反映 `ai_completed`、`rules_completed`、`context_truncated`、`degraded_reason`。
- [ ] 实现 `NormalizeReport(pr PRInfo, ruleRisks []Risk, ai ReviewAnalysis, context ReviewContext, options)` 或等价函数。
- [ ] 实现 LLM 失败降级报告 builder，summary 明确说明 AI 分析未完成。
- [ ] 运行 `go test ./internal/review -run ReportNormalizer`。

## 任务 6：接入 Review Service 真实模式

**文件：**
- 修改：`internal/review/service.go`
- 修改：`internal/review/service_test.go`
- 修改：`cmd/server/main.go`

- [ ] 给 `review.ServiceOptions` 增加 ContextBuilder、LLM Analyzer、ReportNormalizer 依赖或 factory。
- [ ] 定义 service 内部接口，避免 review service 直接依赖 llm 具体 HTTP 实现：
  - analyzer 输入 `ReviewContext`
  - analyzer 输出 `review.ReviewAnalysis`
- [ ] `review.Service` 不 import `internal/llm`；真实 analyzer 只在 `cmd/server/main.go` 中注入。
- [ ] 重构真实模式为真流式：`Analyze` 创建 channel 后立即返回，goroutine 内执行 fetch、parse、scan、build context、analyze 和 normalize。
- [ ] channel 创建后的 pipeline fatal error 在 goroutine 内发送 `error` + `done{ok:false}`；PR URL 无效、demo provider 缺失等构造期错误仍可直接返回 error。
- [ ] 移除或避免真实模式继续使用 `closedEventStream` 一次性发送事件。
- [ ] 写失败测试：真实模式成功路径事件顺序包含 `build_context` 和 `analyze_ai`。
- [ ] 写失败测试：LLM 成功时最终 `result.Degraded=false` 或明确表示 AI 分析完成。
- [ ] 写失败测试：LLM 未配置时，最终 `result.Degraded=true`，但保留 rules 风险。
- [ ] 写失败测试：LLM JSON 解析失败时，最终 `result.Degraded=true`，不泄露原始模型文本。
- [ ] 写失败测试：context builder 不应在空 diff 或空规则风险下 panic。
- [ ] 写失败测试：第一个 `fetch_pr running` 事件在 GitHub fake client 阻塞期间已经可被读取，证明不是同步跑完后才返回。
- [ ] 实现真实模式串联：GitHub fetch -> diff parse -> rules scan -> context build -> llm analyze -> normalize report。
- [ ] 在 LLM 调用前发送 `build_context` running/completed。
- [ ] 在 LLM 调用前后发送 `analyze_ai` running/completed 或 failed/degraded step。
- [ ] 在 `cmd/server/main.go` 中使用 `config.Load()` 的 LLM 配置创建 analyzer。
- [ ] 运行 `go test ./internal/review`。

## 任务 7：错误映射与降级策略

**文件：**
- 修改：`internal/handler/review_handler.go`（仅当 service 新增 typed error 需要 handler 映射）
- 修改：`internal/handler/review_handler_test.go`（仅当 handler 映射变化）
- 修改：`internal/review/service_test.go`

- [ ] 确认 LLM 未配置、请求失败、响应非法、模型输出非法都不会让整个 SSE 失败。
- [ ] 确认这类 LLM 错误会转成 degraded `result`，而不是 SSE `error`。
- [ ] 确认 degraded `result.meta.degraded_reason` 使用稳定枚举或短 code，例如 `llm_not_configured`、`llm_request_failed`、`llm_response_invalid`、`llm_output_invalid`。
- [ ] 确认模型原始输出不进入 SSE、日志、错误响应或测试 snapshot。
- [ ] 确认 GitHub 获取失败、diff parse 不可恢复错误仍走 SSE `error`。
- [ ] 如果 handler 需要新增 code，补充测试：
  - `llm_not_configured` 不应作为 fatal SSE error
  - `github_*` 错误仍按原映射返回
- [ ] 确认任何响应都不包含 API key。
- [ ] 运行 `go test ./internal/handler`。

## 任务 8：README 与模型配置文档

**文件：**
- 修改：`README.md`

- [ ] 更新进度表：
  - ContextBuilder 标为已完成第一阶段
  - OpenAI 兼容 LLM 分析标为已完成第一阶段
  - ReportNormalizer 标为已完成第一阶段
- [ ] 增加环境变量说明：
  - `LLM_BASE_URL`
  - `LLM_API_KEY`
  - `LLM_MODEL`
- [ ] 说明支持 OpenAI 兼容 Chat Completions 服务，例如 DeepSeek、Qwen 或 OpenAI 兼容服务。
- [ ] 说明 LLM 不直接消费完整 raw diff，只消费裁剪后的 `ReviewContext`。
- [ ] 说明 AI 风险必须引用已有 context/rule 证据，证据无法验证时不会作为高/中风险展示。
- [ ] 说明 LLM 失败时仍返回规则扫描降级报告。
- [ ] 说明 `result.meta` 中的 `ai_completed`、`context_truncated`、`degraded_reason` 含义。
- [ ] 更新真实 PR curl 验证说明，补充需要配置 LLM API key 才能得到 AI summary/comments。
- [ ] 不要承诺模型结果一定发现所有问题。

## 任务 9：最终验证

**文件：**
- 不新增源代码改动，只执行前面任务产生的验证。

- [ ] 运行 `go test ./internal/llm`。
- [ ] 运行 `go test ./internal/review`。
- [ ] 运行 `go test ./internal/handler`。
- [ ] 运行 `go test ./...`。
- [ ] 运行 `node scripts/check-pr-quality.test.mjs`。
- [ ] 可选：启动服务 `go run ./cmd/server`。
- [ ] 未配置 LLM API key 时验证真实 PR：
  - 输出包含 `event: rules`
  - 输出包含 `event: result`
  - `result.degraded` 为 true
  - `result.meta.ai_completed` 为 false
  - `result.meta.degraded_reason` 为 `llm_not_configured`
  - 报告保留规则风险
- [ ] 配置测试用 LLM 服务或 mock server 时验证真实 PR：
  - 输出包含 AI summary
  - 输出包含 suggested comments
  - `result.risks` 包含 rule/ai/merged 来源
  - AI/merged risks 的 evidence_refs 都能映射回 context snippet 或 rule finding
- [ ] 确认响应和日志不包含 `LLM_API_KEY`。

## 建议提交拆分

- [ ] `feat: add review context builder`
- [ ] `feat: add openai compatible review analyzer`
- [ ] `feat: normalize ai and rule review reports`
- [ ] `feat: run ai analysis in review pipeline`
- [ ] `docs: document llm review analysis`

## 风险与约束

- 模型输出不可信，必须先解析为结构化 JSON，再归一化；不要把原始模型文本直接展示为最终报告。
- `internal/review` 和 `internal/llm` 之间最容易形成 Go import cycle；领域类型和接口必须放在 `internal/review`，LLM 包只做 adapter。
- 上下文预算必须硬限制，否则大型 PR 会导致成本、延迟和失败率不可控。
- 证据不足的 AI 风险不能升级为 high；高风险必须有明确 file/line/evidence_refs，且 evidence_refs 能映射回 context 或规则命中。
- LLM 失败是可恢复问题，不应抹掉 GitHub、diff 和规则扫描结果。
- API key 不能出现在日志、错误响应、测试 snapshot 或 README 示例中。
- diff 内容属于不可信输入，可能包含 prompt injection 或真实 secret；进入模型前必须做结构化包裹和脱敏。
- 真实 SSE 必须边执行边发送；如果继续使用同步 `closedEventStream`，长耗时 LLM 会让用户看到空白等待。
