# diff-lens

diff-lens 是一个本地 Web 版 AI Pull Request Review 助手。完整目标是：开发者输入 GitHub PR 链接后，系统获取 PR 变更，结合规则扫描和大模型分析，生成 PR 总结、风险提示和可复制的 Review 建议。

项目目标不是替代 reviewer，而是帮助 reviewer 更快进入上下文，减少重复检查成本，并把高风险变更提前暴露出来。

> 当前真实模式已接入 GitHub PR 获取、diff 解析、确定性规则扫描、受控上下文构建和 OpenAI 兼容 LLM 分析。未配置 `LLM_API_KEY` 或模型调用失败时，真实模式会返回包含规则风险的 degraded report；demo 模式仍用于展示稳定完整流程。

## 功能目标

- 分析公开 GitHub PR，并可选使用 GitHub Token 提高 API 额度或访问私有仓库。
- 汇总 PR 背景、文件变更、测试覆盖和潜在风险。
- 使用确定性规则召回敏感信息、危险操作、配置变更、依赖变更和大 PR 风险。
- 使用 OpenAI 兼容模型生成结构化 Review 报告。
- 通过 SSE 流式展示分析进度。
- 提供可复制到 GitHub 的 Review 建议。
- 提供示例 PR 模式，保证本地演示稳定。

## 前端分析台

React/Vite 前端是面向 reviewer 的数据密集型分析台，而不是营销页。主要区域包括：

- `Review Brief`：展示 PR 标题、作者、基础统计、风险等级、关键变更、review focus 和降级原因。
- `Risk Radar`：按 `all`、`high`、`medium`、`low` 过滤风险，点击风险后联动右侧证据区。
- `Evidence Drawer`：展示当前风险的严重级别、来源、置信度、证据片段、证据引用 ID 和建议修复方式，并提供复制按钮。
- `Suggested Comments`：把后端归一化后的 review 建议整理为可复制到 GitHub 的评论。
- `AI Trace`：只展示 SSE `ai_delta` 中的模型分析过程文本，用于解释分析进度；该事件可能缺席，且不作为最终报告来源。
- `StatusBanner`：在请求错误、LLM 未配置、LLM 调用失败或其他 degraded 状态下提示当前结果来源，规则扫描结果仍会保留。

最终可审阅报告来自 SSE `result` 事件。`ai_delta` 只用于展示分析过程，可能因为未配置 `LLM_API_KEY`、模型失败或后端降级而不存在。

## 当前进度

| 模块 | 状态 |
| --- | --- |
| PR 模板 | 已完成 |
| PR 标题与描述质量检查 | 已完成 |
| 项目设计文档 | 已完成 |
| Go/Gin 后端 | 已完成基础框架 |
| GitHub PR URL 解析 | 已完成基础校验 |
| GitHub PR 获取 | 已完成真实获取：metadata、files、patch、commits |
| diff 解析 | 已完成第一阶段：文件分类、hunk 解析、统计、缺失 patch 降级 |
| 规则风险扫描 | 已完成第一批通用规则：secret、危险操作、测试缺口、大 PR、配置/依赖/CI |
| ContextBuilder | 已完成第一阶段：裁剪、脱敏、证据 ID、预算控制 |
| OpenAI 兼容 LLM 分析 | 已完成第一阶段：非流式 Chat Completions、JSON 解析、失败降级 |
| ReportNormalizer | 已完成第一阶段：规则/AI 风险合并、证据校验、degraded meta |
| React/Vite 前端分析台 | 已完成 Review Brief、Risk Radar、Evidence Drawer、AI Trace、StatusBanner 与 Suggested Comments |
| 示例 PR 模式与复制 Review 建议 | 已完成 demo 流程 |

## 技术栈

- 后端：Go、Gin
- 前端：React、Vite、TypeScript
- 数据源：GitHub REST API
- AI 接入：OpenAI 兼容 Chat Completions API
- 流式响应：Server-Sent Events
- CI：GitHub Actions

## 设计概览

真实模式链路如下。LLM 只消费裁剪和脱敏后的 `ReviewContext`，不会直接消费完整 raw diff。

```text
GitHub PR URL
      |
      v
GitHub Client 获取 PR metadata、files、commits
      |
      v
Diff Parser 统计文件、测试、配置和依赖变化
      |
      v
Rule Scanner 召回确定性风险
      |
      v
Context Builder 整理关键证据
      |
      v
LLM Analyzer 生成结构化 Review 报告
      |
      v
SSE Stream 返回进度和结果
```

详细设计见 [AI PR Review 助手设计文档](docs/superpowers/specs/2026-05-29-ai-pr-review-design.md)。

## 本地启动与验证

后端默认监听 `8080`：

```bash
go run ./cmd/server
```

前端开发服务默认监听 `5173`，并把 `/api` 代理到后端：

```bash
npm --prefix frontend install
npm --prefix frontend run dev
```

### 环境变量

`GITHUB_TOKEN` 是可选配置，用于提高 GitHub API rate limit，或访问 token 有权限读取的私有仓库。

```bash
GITHUB_TOKEN=ghp_xxx go run ./cmd/server
```

请求体中的 `github_token` 优先级高于环境变量 `GITHUB_TOKEN`。如果两者都提供，当前请求会使用 `github_token`；如果请求体没有提供 token，后端会回退使用 `GITHUB_TOKEN`。

Token 只用于当前 GitHub API 请求，不会写入本地文件或数据库。不要把真实 token 提交到仓库或写进 README。

LLM 配置使用 OpenAI 兼容 Chat Completions API：

```bash
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=replace-me
LLM_MODEL=deepseek-chat
```

`LLM_BASE_URL` 和 `LLM_MODEL` 有本地默认值；只有配置 `LLM_API_KEY` 后才会得到 AI summary、AI risks 和 suggested comments。支持 DeepSeek、Qwen、OpenAI 兼容网关等实现 `POST /v1/chat/completions` 的服务。

模型输出不会被直接展示。后端先解析为结构化 JSON，再由 `ReportNormalizer` 校验证据引用并合并规则风险。AI 风险必须引用已有 context snippet 或 rule finding 的 `evidence_refs`；证据无法验证时不会作为 high/medium risk 展示。

可运行的验证命令：

```bash
go test ./...
node scripts/check-pr-quality.test.mjs
npm --prefix frontend run build
```

### 前端启动和 demo 验证

启动后端和前端：

```bash
go run ./cmd/server
npm --prefix frontend run dev
```

打开 Vite 输出的本地地址，通常是 `http://localhost:5173`。在页面中点击 `Run demo`，用于验证：

- Pipeline 时间线持续接收 SSE 事件。
- `Review Brief` 使用 `result` 中的归一化报告，而不是直接展示模型原始输出。
- `Risk Radar` 可以筛选风险，点击风险会打开 `Evidence Drawer`。
- `Suggested Comments` 的复制按钮可以复制单条评论或完整 review。
- `AI Trace` 只显示 `ai_delta` 分析过程；demo 或真实模式中该事件缺席时，页面应保持可用。
- `StatusBanner` 在 degraded report 中展示降级原因，例如未配置 `LLM_API_KEY`。
- 在 375px、768px 和 1440px 宽度下检查单列/双列布局、按钮换行、长文件名、证据代码块和评论文本没有溢出。

演示流接口：

```bash
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data '{"demo":true}'
```

真实公开 PR 手动验证：

```bash
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

需要请求级 token 时：

```bash
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"github_token\":\"ghp_xxx\",\"demo\":false}"
```

未配置 `LLM_API_KEY` 时，真实模式仍应输出：

- `event: rules`
- `event: result`
- `"degraded":true`
- `"meta":{"ai_completed":false,...,"degraded_reason":"llm_not_configured"}`
- 规则扫描保留下来的风险

配置可用模型后，`result` 会包含 AI summary、suggested comments，以及 `rule` / `ai` / `merged` 来源的 risks。模型结果是辅助 review 的候选信号，不保证发现所有问题。

前端只把 `result` 作为最终 report 渲染；`ai_delta` 只进入 `AI Trace`，用于说明分析过程和模型阶段状态。真实模式下如果模型未配置、调用失败或后端提前降级，`ai_delta` 可以完全不存在。

## 当前限制

- 规则扫描是语言无关的启发式检查，第一阶段不覆盖完整 AST 语义、跨文件调用图、复杂 SQL/命令拼接或所有错误处理缺陷。
- GitHub 可能省略大型文件或二进制文件 patch；这类文件会保留文件名和状态，但不会伪造 diff 证据。
- ContextBuilder 会裁剪大型 PR。`result.meta.context_truncated`、`omitted_files_count` 和 `omitted_snippets_count` 会说明上下文是否被裁剪。
- LLM 未配置、请求失败、响应非法或模型输出 JSON 不合法时，真实模式返回 degraded report，并通过 `result.meta.degraded_reason` 说明原因。
- AI 风险必须有可验证证据引用；证据不足的问题会被降级或丢弃。
- demo 模式仍会输出完整演示报告，适合比赛演示和本地体验。

PR 质量检查脚本会校验：

- PR 标题是否符合 `feat: add xxx`、`fix: handle xxx` 等格式。
- PR 描述是否包含 `功能描述`、`实现思路`、`测试方式`。
- 是否勾选 `本 PR 只做一件事`。
- PR 是否超过默认规模限制；必要时可使用 `allow-large-pr` 标签放行。

## 项目结构

```text
diff-lens/
├── .github/
│   ├── pull_request_template.md
│   └── workflows/
│       └── pr-quality.yml
├── cmd/
│   └── server/
├── docs/
│   └── superpowers/
│       ├── plans/
│       └── specs/
├── frontend/
│   └── src/
├── internal/
│   ├── config/
│   ├── demo/
│   ├── diff/
│   ├── github/
│   ├── handler/
│   ├── llm/
│   ├── rules/
│   └── review/
├── scripts/
│   ├── check-pr-quality.mjs
│   └── check-pr-quality.test.mjs
├── competition_COMMIT_CONVENTION.md
├── competition_PR_TEMPLATE.md
├── competition_README_TEMPLATE.md
└── README.md
```

## 许可证

仅用于 XEngineer 新工科计划项目比赛作品提交。是否开源按比赛要求和个人选择确认。
