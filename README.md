# diff-lens

diff-lens 是一个本地 Web 版 AI Pull Request Review 助手。完整目标是：开发者输入 GitHub PR 链接后，系统获取 PR 变更，结合规则扫描和大模型分析，生成 PR 总结、风险提示和可复制的 Review 建议。

项目目标不是替代 reviewer，而是帮助 reviewer 更快进入上下文，减少重复检查成本，并把高风险变更提前暴露出来。

> 当前真实模式已完成 GitHub PR 第一阶段获取：PR metadata、变更文件和 commit 列表。diff 解析、规则扫描和 LLM 分析仍在后续阶段；真实模式目前会返回 degraded report，demo 模式用于展示完整产品流程。

## 功能目标

- 分析公开 GitHub PR，并可选使用 GitHub Token 提高 API 额度或访问私有仓库。
- 汇总 PR 背景、文件变更、测试覆盖和潜在风险。
- 使用确定性规则召回敏感信息、危险操作、配置变更、依赖变更和大 PR 风险。
- 使用 OpenAI 兼容模型生成结构化 Review 报告。
- 通过 SSE 流式展示分析进度。
- 提供可复制到 GitHub 的 Review 建议。
- 提供示例 PR 模式，保证本地演示稳定。

## 当前进度

| 模块 | 状态 |
| --- | --- |
| PR 模板 | 已完成 |
| PR 标题与描述质量检查 | 已完成 |
| 项目设计文档 | 已完成 |
| Go/Gin 后端 | 已完成基础框架 |
| GitHub PR URL 解析 | 已完成基础校验 |
| GitHub PR 获取 | 已完成第一阶段真实获取：metadata、files、commits |
| diff 解析 | 已预留接口与数据结构 |
| 规则风险扫描 | 已预留 scanner 框架，真实模式尚未运行 |
| OpenAI 兼容 LLM 分析 | 已预留 analyzer 配置入口，真实模式尚未调用 |
| React/Vite 前端分析台 | 已完成基础页面与 SSE 状态流 |
| 示例 PR 模式与复制 Review 建议 | 已完成 demo 流程 |

## 技术栈

- 后端：Go、Gin
- 前端：React、Vite、TypeScript
- 数据源：GitHub REST API
- AI 接入：OpenAI 兼容 Chat Completions API
- 流式响应：Server-Sent Events
- CI：GitHub Actions

## 设计概览

完整目标链路如下。当前真实模式只完成到 GitHub Client 获取 PR metadata、变更文件和 commit 列表；diff/rules/LLM 仍未接入真实分析。

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

可运行的验证命令：

```bash
go test ./...
node scripts/check-pr-quality.test.mjs
npm --prefix frontend run build
```

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

## 当前限制

- 真实模式当前只完成 PR 数据获取，会调用 GitHub API 获取 PR metadata、变更文件和 commit 列表。
- diff 解析、规则扫描、上下文构建和 AI 分析仍在后续模块完成，当前不会基于真实 diff 生成完整 Review 结论。
- 因此真实模式当前返回 degraded report 是预期行为；它证明真实 PR 获取链路可用，但不代表完整 AI Review 已完成。
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
│   ├── github/
│   ├── handler/
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
