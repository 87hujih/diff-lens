# diff-lens

diff-lens 是一个本地 Web 版 AI Pull Request Review 助手。开发者输入 GitHub PR 链接后，系统会获取 PR 变更，结合确定性规则扫描和 OpenAI 兼容模型分析，生成 PR 摘要、风险提示、证据片段和可复制的 Review 建议。

## 议题方向

- 参赛批次：提交前填写
- 所选议题：提交前填写
- 作品类型：本地 Web 应用 / AI 开发者工具

## Demo 视频

- 视频链接：提交前填写
- 说明：提交前确认链接公开可访问，视频建议展示 demo 模式和真实公开 PR 模式各一次。

## 核心功能

1. GitHub PR 分析：解析公开 GitHub PR URL，获取 PR metadata、files、patch 和 commits；可选使用 GitHub token 提高额度或访问有权限的私有仓库。
2. 确定性规则扫描：对 diff 做启发式检查，召回 secret、危险操作、配置/依赖/CI 变更、测试缺口和大 PR 风险。
3. 受控上下文构建：把 raw diff 裁剪、脱敏并转成带证据 ID 的 `ReviewContext`，避免直接把完整 diff 丢给模型。
4. OpenAI 兼容模型分析：调用兼容 `POST /v1/chat/completions` 的模型服务，要求模型输出结构化 JSON。
5. 报告归一化与降级：校验 AI 证据引用，合并 rule / ai / merged 风险；LLM 未配置或失败时返回包含规则风险的 degraded report。
6. SSE 前端分析台：通过 Server-Sent Events 展示 pipeline 进度，并在 React/Vite 前端呈现 Review Brief、Risk Radar、Evidence Drawer、AI Trace 和 Suggested Comments。
7. 稳定 demo 模式：不需要网络、GitHub token 或 LLM key，即可展示完整本地演示流程。demo 数据用于演示，不代表真实 AI 分析结果。

## 功能演示

建议视频或截图覆盖以下流程：

1. 启动后端和前端，打开 `http://localhost:5173`。
2. 点击 `Demo PR`，查看 pipeline 时间线、PR 摘要、风险列表、证据和建议评论。
3. 输入一个公开 GitHub PR URL，运行真实模式。
4. 未配置 `LLM_API_KEY` 时，展示 degraded report 如何保留规则扫描结果。
5. 配置可用模型后，展示 AI summary、suggested comments，以及 rule / ai / merged 来源的风险。
6. 复制单条 Suggested Comment 或完整 Review，人工粘贴到 GitHub。

## 技术栈与依赖

### 主要技术栈

- 前端：React、Vite、TypeScript
- 后端：Go、Gin
- 数据库/存储：无数据库，当前版本不保存历史记录
- 流式通信：Server-Sent Events
- 数据源：GitHub REST API
- AI 接入：OpenAI 兼容 Chat Completions API

### 第三方依赖

| 依赖 | 用途 | 是否核心功能 |
| --- | --- | --- |
| Gin | 后端 HTTP server 和 API 路由 | 是 |
| React | 前端 Review Dashboard | 是 |
| Vite / esbuild / TypeScript | 前端开发、构建和类型检查 | 是 |
| GitHub REST API | 获取 PR metadata、files、patch、commits | 是 |
| OpenAI-compatible Chat Completions API | 生成结构化 AI review 信号 | 是 |

原创实现部分包括：PR URL 解析、GitHub client 封装、diff parser、规则扫描器、ContextBuilder、LLM analyzer、ReportNormalizer、SSE review pipeline、demo provider、React 分析台和 PR 质量检查脚本。

## 技术架构

真实模式链路：

```text
GitHub PR URL
      |
      v
GitHub Client 获取 metadata / files / commits
      |
      v
Diff Parser 解析 patch、文件类型、测试和统计信息
      |
      v
Rule Scanner 先召回确定性风险
      |
      v
Context Builder 裁剪、脱敏、生成证据 ID
      |
      v
LLM Analyzer 请求 OpenAI 兼容模型并解析 JSON
      |
      v
Report Normalizer 校验证据、合并风险、生成 comments
      |
      v
SSE Stream
      |
      v
Frontend Review Dashboard
```

demo 模式使用后端内置演示数据源，不调用 GitHub API，不读取 token，也不调用 LLM。它用于比赛和本地体验的稳定演示。

## 模型与上下文策略

- 不直接发送完整 raw diff。后端先解析 diff，再通过 ContextBuilder 选择关键文件、hunks、规则发现和证据片段。
- 规则先扫、LLM 后解释。规则扫描提供确定性召回，模型负责总结、补充候选风险和生成建议评论。
- 模型输出必须是结构化 JSON。解析失败、请求失败、模型未配置或证据引用不合法时，不展示原始模型输出。
- AI risk 必须引用已有 context snippet 或 rule finding 的 `evidence_refs`。证据无法验证时会被降级或丢弃。
- 大 PR 会触发上下文裁剪，并通过 `result.meta.context_truncated`、`omitted_files_count`、`omitted_snippets_count` 告知前端。

## 误报与漏报控制

- 规则扫描偏召回，适合提前暴露 secret、危险命令、配置/依赖/CI 改动和测试缺口，但它是启发式，不保证语义完整。
- ReportNormalizer 会合并重复风险，区分 `rule`、`ai`、`merged` 来源，避免把模型无证据猜测直接升为高风险。
- LLM 输出需要 reviewer 人工复核。diff-lens 的定位是辅助 reviewer 更快进入上下文，不替代代码审查结论。
- GitHub 省略的大 patch 或二进制文件不会伪造代码证据，只展示文件状态和降级说明。
- 当前不自动向 GitHub 写评论，所有 Suggested Comments 都需要人工复制和发布。

## 原创说明

本项目由参赛期间自主完成。

如存在以下情况，请如实说明：

- 复用历史代码：提交前按实际情况填写；当前模板未声明复用历史业务代码。
- 使用开源模板：提交前按实际情况填写；当前实现为本仓库内自建结构。
- 使用第三方服务/API：GitHub REST API；可选 OpenAI 兼容 Chat Completions 服务。
- 使用 AI 辅助生成代码或内容：提交前按实际情况填写使用范围。

## 本地运行方式

### 环境要求

- Go：用于运行后端和测试
- Node.js / npm：用于前端依赖、构建和开发服务
- Python：仅在使用 `npm --prefix frontend run preview:static` 静态预览代理时需要
- 可选：GitHub token、OpenAI 兼容模型 API key

### 安装依赖

```bash
npm --prefix frontend install
```

### 启动项目

启动后端，默认监听 `8080`，可通过 `PORT` 覆盖：

```bash
go run ./cmd/server
```

启动前端开发服务，默认监听 `5173`：

```bash
npm --prefix frontend run dev
```

如果 Windows 环境中 Vite/esbuild 遇到 `spawn EPERM`，可改用静态预览代理：

```bash
npm --prefix frontend run build
npm --prefix frontend run preview:static
```

### 访问地址

```text
http://localhost:5173
```

### 环境变量

```bash
PORT=8080
GITHUB_TOKEN=<github-token>
LLM_BASE_URL=https://api.deepseek.com
LLM_API_KEY=replace-with-token
LLM_MODEL=deepseek-chat
```

`GITHUB_TOKEN` 和 `LLM_API_KEY` 都是可选项。没有 `LLM_API_KEY` 时，真实模式会返回 degraded report，并保留规则扫描结果。

## 项目结构

```text
diff-lens/
├── .github/
│   ├── pull_request_template.md
│   └── workflows/
├── cmd/
│   └── server/
├── docs/
│   └── superpowers/
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
├── competition_COMMIT_CONVENTION.md
├── competition_PR_TEMPLATE.md
├── competition_README_TEMPLATE.md
└── README.md
```

## PR 与提交记录

本项目按照比赛要求，通过多个 PR 持续迭代完成。每个 PR 尽量只包含一个独立功能或变更点。

建议 PR 拆分记录：

| PR | 内容 | 状态 |
| --- | --- | --- |
| #1 | 项目初始化、README、比赛提交模板和 PR 质量检查 | 提交前填写 PR 链接 |
| #2 | Go/Gin 后端基础框架、GitHub PR URL 解析和 SSE 接口 | 提交前填写 PR 链接 |
| #3 | GitHub PR 获取、diff parser、规则扫描器 | 提交前填写 PR 链接 |
| #4 | ContextBuilder、OpenAI 兼容 LLM analyzer、ReportNormalizer 与降级报告 | 提交前填写 PR 链接 |
| #5 | React/Vite Review Dashboard、风险筛选、证据抽屉和复制体验 | 提交前填写 PR 链接 |
| #6 | demo 契约、QA 验证、README 和比赛交付材料 | 提交前填写 PR 链接 |

## 测试方式

```bash
go test ./...
node scripts/check-pr-quality.test.mjs
npm --prefix frontend run build
```

手动测试流程：

1. 启动后端：`go run ./cmd/server`。
2. 启动前端：`npm --prefix frontend run dev`，或先 build 再运行 `npm --prefix frontend run preview:static`。
3. 打开 `http://localhost:5173`，点击 `Demo PR`，确认 timeline、Review Brief、Risk Radar、Evidence Drawer 和 Suggested Comments 都有内容。
4. 用 curl 验证 demo SSE：

```bash
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data '{"demo":true}'
```

5. 用公开 PR 验证真实模式：

```bash
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream \
  -H "Content-Type: application/json" \
  --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

## 已知限制

- 不自动评论 GitHub PR，不做 OAuth、多用户、数据库历史记录或 GitHub App 安装流程。
- 规则扫描是启发式检查，不覆盖完整 AST 语义、跨文件调用图和所有语言特定问题。
- 模型输出需要人工复核，不能直接当作最终 review 结论。
- 未配置或调用失败的 LLM 会产生 degraded report；这不是失败，而是保留规则扫描结果的降级路径。
- GitHub 可能省略大型 patch 或二进制文件内容，diff-lens 不会伪造不存在的代码证据。
- demo 模式是稳定演示数据，不是真实 GitHub PR 或真实 AI 分析结果。

## 未来扩展

- 接入 GitHub App 或 OAuth，让用户授权后选择仓库和 PR。
- 支持自动创建 draft review，但仍保留人工确认。
- 增加语言级 AST 规则、跨文件调用图和更细的测试覆盖分析。
- 增加历史报告、团队规则配置和组织级风险策略。
- 增加浏览器自动化 E2E 测试和更多真实 PR 回归样例。

## 许可证

仅用于本次训练营比赛作品提交。是否开源请按比赛要求和个人选择确认。
