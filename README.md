# diff-lens

diff-lens 是一个本地 Web 版 AI Pull Request Review 助手。开发者输入 GitHub PR 链接后，系统会获取 PR 变更，结合规则扫描和大模型分析，生成 PR 总结、风险提示和可复制的 Review 建议。

项目目标不是替代 reviewer，而是帮助 reviewer 更快进入上下文，减少重复检查成本，并把高风险变更提前暴露出来。

> 当前仓库处于初始化阶段，已完成 PR 模板、PR 质量检查脚本和设计文档。后端、前端和 AI 分析流程会在后续 PR 中逐步实现。

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
| Go/Gin 后端 | 计划中 |
| GitHub PR 获取与 diff 解析 | 计划中 |
| 规则风险扫描 | 计划中 |
| OpenAI 兼容 LLM 分析 | 计划中 |
| React/Vite 前端分析台 | 计划中 |
| 示例 PR 模式与复制 Review 建议 | 计划中 |

## 技术栈

- 后端：Go、Gin
- 前端：React、Vite、TypeScript
- 数据源：GitHub REST API
- AI 接入：OpenAI 兼容 Chat Completions API
- 流式响应：Server-Sent Events
- CI：GitHub Actions

## 设计概览

```text
GitHub PR URL
      |
      v
GitHub Client 获取 PR 元数据和 patch
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

## 本地验证

当前可运行的是 PR 质量检查脚本测试：

```bash
node scripts/check-pr-quality.test.mjs
```

PR 质量检查脚本会校验：

- PR 标题是否符合 `feat: add xxx`、`fix: handle xxx` 等格式。
- PR 描述是否包含 `功能描述`、`实现思路`、`测试方式`。
- 是否勾选 `本 PR 只做一件事`。
- PR 是否超过默认规模限制；必要时可使用 `allow-large-pr` 标签放行。

后续后端和前端落地后，会补充完整的本地启动方式。

## 项目结构

```text
diff-lens/
├── .github/
│   ├── pull_request_template.md
│   └── workflows/
│       └── pr-quality.yml
├── docs/
│   └── superpowers/
│       └── specs/
│           └── 2026-05-29-ai-pr-review-design.md
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
