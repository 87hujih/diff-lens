# MVP 执行路线图

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本路线图。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 将 diff-lens 已拆分的功能计划串成一条可执行的 MVP 交付路线，明确每个阶段的进入条件、完成标准、验证命令、PR 拆分和最终验收口径。

**架构：** MVP 按“后端真实数据 -> 后端分析能力 -> AI 报告 -> 前端呈现 -> Demo/QA/Release”顺序推进。每个阶段都必须产出可运行、可测试的增量，不能把未验证的下游功能堆到同一个大 PR。后端事件契约是前后端集成主线，任何 `ReviewEvent` 或 `Report` 字段变化都必须同步类型、测试和 README。

**技术栈：** Go/Gin、React/Vite/TypeScript、GitHub REST API、OpenAI 兼容 Chat Completions、SSE、Go tests、TypeScript build、Node PR 质量脚本。

---

## 背景

当前已经形成 5 份模块计划：

- `docs/superpowers/plans/2026-05-30-github-pr-fetch.md`
- `docs/superpowers/plans/2026-05-30-diff-rules-scanner.md`
- `docs/superpowers/plans/2026-05-30-context-builder-llm-analyzer.md`
- `docs/superpowers/plans/2026-05-30-frontend-review-dashboard.md`
- `docs/superpowers/plans/2026-05-30-demo-qa-release.md`

这些计划分别解决单个模块，但还需要一份总控文档回答：

- 先做哪一块，后做哪一块。
- 每一块做到什么程度才能进入下一阶段。
- 每一阶段如何测试和提交。
- 哪些变更会影响前后端契约。
- MVP 最终怎样判断“可以提交/演示”。

## 总体执行顺序

严格按以下顺序执行，除非发现前置计划本身需要修正：

1. **GitHub PR 获取**
   - 计划：`2026-05-30-github-pr-fetch.md`
   - 目标：真实模式可以获取 PR metadata、files、commits。

2. **Diff 解析 + 规则扫描**
   - 计划：`2026-05-30-diff-rules-scanner.md`
   - 目标：真实模式可以基于 patch 输出确定性风险。

3. **ContextBuilder + LLM Analyzer + ReportNormalizer**
   - 计划：`2026-05-30-context-builder-llm-analyzer.md`
   - 目标：真实模式可以构建受控模型上下文，调用 OpenAI 兼容模型，并输出结构化报告；LLM 失败时可降级。

4. **前端 Review Dashboard**
   - 计划：`2026-05-30-frontend-review-dashboard.md`
   - 目标：前端完整展示 summary、risks、evidence、comments、AI trace、错误和降级状态。

5. **Demo、QA 与 Release**
   - 计划：`2026-05-30-demo-qa-release.md`
   - 目标：demo 稳定、真实模式可验收、文档完整、比赛提交材料就绪。

## 阶段 1：GitHub PR 获取

**执行文档：**
- `docs/superpowers/plans/2026-05-30-github-pr-fetch.md`

**进入条件：**
- 架构脚手架已存在。
- `internal/github`、`internal/review`、`internal/handler` 基础测试可运行。
- 没有未理解的同文件冲突。

**完成标准：**
- [ ] `internal/github` 定义 PR 数据契约和 typed errors。
- [ ] GitHub client 能获取 PR metadata、files、commits。
- [ ] files 和 commits 支持分页。
- [ ] Token header 正确，且不会泄露 token。
- [ ] `review.Service` 非 demo 请求不再直接返回 `real_analysis_not_implemented`。
- [ ] 真实模式返回 `step`、`pr`、degraded `result`、`done`。
- [ ] handler 能把 GitHub/URL 错误映射为 SSE `error`。
- [ ] README 准确说明当前真实模式只完成 PR 数据获取。

**验证命令：**

```bash
go test ./internal/github
go test ./internal/review
go test ./internal/handler
go test ./...
node scripts/check-pr-quality.test.mjs
```

**手动验收：**

```bash
go run ./cmd/server
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

预期至少包含：

```text
event: step
event: pr
event: result
event: done
```

**建议 PR/提交：**
- [ ] `feat: add github pull request data contracts`
- [ ] `feat: fetch github pull request metadata`
- [ ] `feat: stream real pull request metadata`
- [ ] `fix: map github fetch errors to sse payloads`
- [ ] `docs: document github fetch increment`

**不得进入下一阶段的情况：**
- GitHub client 测试访问真实 GitHub。
- 请求级 token 和环境 token 优先级不明确。
- 真实模式仍只返回 `real_analysis_not_implemented`。
- GitHub 错误无法被前端识别为结构化 SSE error。

## 阶段 2：Diff 解析 + 规则扫描

**执行文档：**
- `docs/superpowers/plans/2026-05-30-diff-rules-scanner.md`

**进入条件：**
- 阶段 1 验证通过。
- `github.PullRequestData.Files` 中包含 filename/status/additions/deletions/changes/patch。
- 真实模式已能稳定拿到 PR 文件数据。

**完成标准：**
- [ ] `internal/diff` 能解析 GitHub unified patch。
- [ ] 能计算新增/删除/上下文行号。
- [ ] 能分类 source/test/config/dependency/ci/docs/missing patch 文件。
- [ ] 能汇总 PR 级统计。
- [ ] `internal/rules` 能输出第一批确定性 `review.Risk`。
- [ ] 规则至少覆盖 secret、dangerous operation、test gap、size、config/dependency。
- [ ] `review.Service` 真实模式新增 `parse_diff`、`scan_rules` 阶段。
- [ ] 真实模式返回 `rules` 事件。
- [ ] 最终 degraded report 包含规则风险。
- [ ] README 说明真实模式已能输出规则扫描结果，但 AI 分析仍未完成。

**验证命令：**

```bash
go test ./internal/diff
go test ./internal/rules
go test ./internal/review
go test ./internal/handler
go test ./...
node scripts/check-pr-quality.test.mjs
```

**手动验收：**

真实 PR 请求预期至少包含：

```text
event: step
event: pr
event: rules
event: result
event: done
```

**建议 PR/提交：**
- [ ] `feat: add diff analysis data model`
- [ ] `feat: parse github pull request patches`
- [ ] `feat: add rule based risk scanner`
- [ ] `feat: stream rule scan results`
- [ ] `docs: document diff and rule scan increment`

**不得进入下一阶段的情况：**
- 缺失 patch 或 binary file 会 panic。
- 规则扫描直接依赖 GitHub API DTO，而不是 diff 分析结果。
- 测试缺口规则对纯文档变更误报。
- `rules` 事件和最终 `result.risks` 不一致且没有解释。

## 阶段 3：ContextBuilder + LLM Analyzer

**执行文档：**
- `docs/superpowers/plans/2026-05-30-context-builder-llm-analyzer.md`

**进入条件：**
- 阶段 2 验证通过。
- diff analysis 和 rule risks 结构稳定。
- 后端已有可测试的降级报告路径。

**完成标准：**
- [ ] `ContextBuilder` 只输出裁剪后的 `ReviewContext`。
- [ ] LLM 不能直接消费完整 raw diff。
- [ ] ContextBuilder 有字符预算、文件预算、snippet 预算。
- [ ] 规则命中文件和高风险文件优先进入上下文。
- [ ] `internal/llm` 使用 OpenAI 兼容 `/v1/chat/completions`。
- [ ] LLM 请求 header、body、错误映射有测试。
- [ ] LLM 输出必须先解析为结构化 JSON。
- [ ] `ReportNormalizer` 合并 rule/ai/merged 风险。
- [ ] LLM 未配置、请求失败、输出非法时，保留规则风险并返回 degraded report。
- [ ] README 说明 LLM 配置、上下文裁剪和降级策略。

**验证命令：**

```bash
go test ./internal/llm
go test ./internal/review
go test ./internal/handler
go test ./...
node scripts/check-pr-quality.test.mjs
```

**手动验收：**

- 未配置 `LLM_API_KEY`：
  - 真实模式应返回 degraded report。
  - report 保留 PR 信息和规则风险。

- 配置测试 LLM 或 mock LLM：
  - 真实模式应返回 summary、risks、comments。
  - `result.risks` 至少能体现 rule/ai/merged 来源。

**建议 PR/提交：**
- [ ] `feat: add review context builder`
- [ ] `feat: add openai compatible review analyzer`
- [ ] `feat: normalize ai and rule review reports`
- [ ] `feat: run ai analysis in review pipeline`
- [ ] `docs: document llm review analysis`

**不得进入下一阶段的情况：**
- LLM 原始文本被直接展示为最终报告。
- 模型输出非法 JSON 会导致整个真实模式失败。
- API key 出现在错误、日志或测试输出中。
- 上下文预算没有硬限制。

## 阶段 4：前端 Review Dashboard

**执行文档：**
- `docs/superpowers/plans/2026-05-30-frontend-review-dashboard.md`

**进入条件：**
- 后端 `ReviewEvent` 和 `Report` 契约基本稳定。
- 阶段 3 至少能提供 demo/真实两类 result。
- 前端基础 SSE 流可用。

**完成标准：**
- [ ] `App.tsx` 拆成清晰组件。
- [ ] 前端能展示完整步骤栏。
- [ ] Review Brief 展示 PR 元数据和 summary。
- [ ] Risk Radar 支持 severity 筛选。
- [ ] Evidence Drawer 展示 evidence、file、line、suggestion。
- [ ] Suggested Comments 支持单条复制和完整 Review 复制。
- [ ] AI Trace 展示 `ai_delta`，但不驱动最终报告结论。
- [ ] 错误和 degraded 状态清晰展示。
- [ ] 移动端无明显重叠或溢出。
- [ ] README 更新前端功能和 demo 验收步骤。

**验证命令：**

```bash
npm --prefix frontend run build
go test ./...
node scripts/check-pr-quality.test.mjs
```

**手动验收：**

```bash
go run ./cmd/server
npm --prefix frontend run dev
```

浏览器验收：
- [ ] Demo PR 可完整展示。
- [ ] 真实 PR degraded report 可展示。
- [ ] 错误状态可展示。
- [ ] 复制功能可用。
- [ ] 窄屏布局可读。

**建议 PR/提交：**
- [ ] `feat: split review dashboard components`
- [ ] `feat: add risk filtering and evidence drawer`
- [ ] `feat: add copyable review summary`
- [ ] `feat: polish review dashboard states`
- [ ] `docs: document frontend review dashboard`

**不得进入下一阶段的情况：**
- 前端重新推导后端风险结论。
- `ai_delta` 被当作最终报告来源。
- Token 被写入 localStorage/sessionStorage。
- build 不通过。

## 阶段 5：Demo、QA 与 Release

**执行文档：**
- `docs/superpowers/plans/2026-05-30-demo-qa-release.md`

**进入条件：**
- 阶段 1-4 均已完成并通过对应验证。
- demo 和真实模式的前后端契约稳定。
- README 已能描述当前能力但仍需要最终收尾。

**完成标准：**
- [ ] Demo 事件流对齐真实 pipeline。
- [ ] Demo 不依赖 GitHub、LLM、token 或网络。
- [ ] Demo 报告包含 summary、risks、evidence、comments。
- [ ] 后端全量测试通过。
- [ ] 前端 build 通过。
- [ ] PR 质量脚本通过。
- [ ] 真实公开 PR 可跑通或清晰降级。
- [ ] 安全检查确认无 token/API key 泄露。
- [ ] README 和比赛材料完整。
- [ ] Release checklist 可复现。

**验证命令：**

```bash
go test ./...
npm --prefix frontend run build
node scripts/check-pr-quality.test.mjs
```

**最终手动验收：**

```bash
go run ./cmd/server
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data "{\"demo\":true}"
```

```bash
npm --prefix frontend run dev
```

浏览器验收：
- [ ] Demo PR 完整可演示。
- [ ] 真实公开 PR 可分析。
- [ ] LLM 未配置时可降级。
- [ ] 非法 PR URL 错误清晰。
- [ ] 复制 Review 可用。

**建议 PR/提交：**
- [ ] `feat: align demo stream with review pipeline`
- [ ] `test: cover demo and release validation paths`
- [ ] `docs: update readme for release`
- [ ] `docs: add competition demo guide`
- [ ] `chore: finalize release checklist`

## 跨阶段集成风险

### SSE 契约漂移

- 风险：后端新增或改名事件字段，前端 reducer 未同步。
- 控制：
  - 每次改 `review.EventType`、`ReviewEvent`、`Report`、`Risk`，同步 `frontend/src/types/review.ts`。
  - 阶段 4 前不要大量改前端 UI，但阶段 1-3 必须保持类型文档清晰。

### 降级状态混乱

- 风险：GitHub 失败、diff 失败、rules 空结果、LLM 失败都混成同一种错误。
- 控制：
  - GitHub 获取失败是 fatal SSE error。
  - LLM 失败是 degraded result。
  - 缺失 patch 是可恢复降级。
  - README 必须解释这些差异。

### Demo 与真实 pipeline 偏离

- 风险：demo 很漂亮，但事件顺序和真实模式不同，前端只适配 demo。
- 控制：
  - Demo 必须经过同一套 SSE event type。
  - Demo 阶段名必须覆盖真实 pipeline 的主要阶段。
  - Handler 测试覆盖 demo 事件顺序。

### 规则和 AI 风险重复

- 风险：同一问题在 UI 出现多次，降低可信度。
- 控制：
  - ReportNormalizer 按 file/line/category 合并。
  - `source` 使用 rule/ai/merged。
  - 前端不做二次合并。

### 文档夸大能力

- 风险：README 或比赛材料声称“自动发现所有问题”或“完整替代 reviewer”。
- 控制：
  - 统一表述：辅助 reviewer，更快进入上下文。
  - 明确启发式规则和模型输出都需要人工复核。
  - 已知限制必须保留。

## PR 拆分原则

- 每个 PR 只做一个阶段或一个阶段内的一个清晰子任务。
- 后端契约变更 PR 必须包含：
  - 后端测试
  - 必要的前端类型同步
  - README 当前能力更新
- 前端 UI PR 不应混入后端分析逻辑。
- Release PR 不应引入大功能，只做 demo、QA、文档和小修。
- 每个 PR 描述必须包含：
  - 功能描述
  - 实现思路
  - 测试方式
  - 是否影响 SSE/Report 契约

## 最终 MVP 验收清单

- [ ] `go test ./...` 通过。
- [ ] `npm --prefix frontend run build` 通过。
- [ ] `node scripts/check-pr-quality.test.mjs` 通过。
- [ ] Demo 模式无网络可完整演示。
- [ ] 至少一个公开 GitHub PR 可在真实模式下跑通。
- [ ] 无 GitHub token 时，公开 PR 能工作或给出清晰 rate limit/认证提示。
- [ ] 无 LLM API key 时，保留规则扫描 degraded report。
- [ ] 配置 LLM API key 后，能生成 AI summary 和 suggested comments。
- [ ] 最终报告包含 summary、risks、evidence、comments。
- [ ] 前端可复制单条评论和完整 Review。
- [ ] token 和 API key 不出现在日志、响应、测试输出或文档示例中。
- [ ] README 能让新用户独立启动后端、前端、demo 和真实 PR 验证。
- [ ] 比赛提交材料说明技术亮点、降级策略、误报/漏报控制和未来扩展。

## 执行记录建议

每完成一个阶段，在对应 PR 或 release checklist 中记录：

- 完成阶段名称。
- 关键文件变更。
- 运行过的命令和结果。
- 手动验收的 PR URL 或 demo 路径。
- 是否改变 SSE/Report 契约。
- 是否有已知限制或后续 TODO。

## 当前建议下一步

从阶段 1 开始执行，不要跳到前端或 LLM：

- [ ] 完成 `2026-05-30-github-pr-fetch.md` 的剩余验证和收尾。
- [ ] 确认 `go test ./...` 通过。
- [ ] 确认真实 PR curl 至少返回 `pr`、`result`、`done`。
- [ ] 提交 GitHub PR 获取阶段。
- [ ] 再进入 diff/rules 阶段。
