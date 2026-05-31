# Demo、QA 与 Release 交付验证计划

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本计划。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 在核心功能已基本串通后，补齐 demo 契约、端到端 QA、安全降级检查和比赛交付文档，让 diff-lens 能稳定演示、可独立运行、可提交。

**架构：** `internal/demo` 继续作为无网络、无 token、无 LLM key 的稳定演示数据源，但事件阶段必须对齐真实 pipeline，并用测试锁定 demo 的报告形状。QA 通过后端单测、前端 build、PR 质量脚本、curl 手动验证、可选 mock LLM 服务和浏览器手动验收覆盖核心路径。Release 文档统一落在 README 与比赛提交材料中，避免产品能力描述超过真实实现。

**技术栈：** Go 测试、React/Vite build、curl SSE 验证、现有 Node PR 质量脚本、可选 OpenAI-compatible mock 服务、README/比赛文档。

---

## 当前仓库校准（2026-05-31）

这份计划已经不是从零实现计划，而是 release 前的补洞和验收计划。当前代码状态需要按以下事实执行：

- 真实模式已经接入 GitHub PR 获取、diff 解析、规则扫描、ContextBuilder、OpenAI 兼容 LLM analyzer 和 ReportNormalizer。
- `go test ./...`、`node scripts/check-pr-quality.test.mjs`、`npm --prefix frontend run build` 在最近一次审阅时通过。
- `internal/demo/demo_test.go` 当前不存在；`internal/demo/demo.go` 当前只有 1 条 `merged` 风险，尚未覆盖 `rule` / `ai` / `merged` 三类 demo 风险。
- `internal/handler/review_handler_test.go` 已覆盖 demo SSE 基础事件，但断言较浅，缺少事件顺序、`done.ok`、`result.degraded`、risk source 和 comments 形状断言。
- `README.md` 已经覆盖大部分运行和降级说明，但还需要补 `PORT`、架构图中的 frontend 节点，并把 token 示例从 `ghp_xxx` 调整为更安全的占位符。
- `competition_README_TEMPLATE.md` 仍有大量 `待补充`，这是 release 文档最高优先级缺口。

## 执行状态（2026-05-31）

已完成并通过独立复核：

- A. Demo 契约：`internal/demo` 增加 provider 测试，demo stream 输出 `running` / `completed` 阶段，报告覆盖 `rule` / `ai` / `merged` 风险。
- B. Handler SSE 契约：handler demo 测试按 SSE message 解析并 typed-unmarshal `step`、`pr`、`rules`、`result`、`done`，锁定 raw `result.meta` 必需字段。
- C. Release 文档：README、比赛 README、PR 模板已补齐 release 口径，去掉通用占位符和危险 token 示例。
- D. Mock LLM 与 checklist：新增 mock OpenAI-compatible 服务和 release checklist。
- E. 自动 QA 与安全扫描：`go test -count=1 ./...`、`npm --prefix frontend run build`、`node scripts/check-pr-quality.test.mjs`、mock 脚本编译、`git diff --check`、secret scan、sensitive field scan、placeholder scan 均通过。

仍需手动验收：

- 启动后端并 curl demo stream。
- 启动前端并浏览器验证 demo、复制、响应式布局。
- 选择公开真实 PR 做 curl 验证。
- 验证非法 PR URL、无 LLM key degraded path。
- 使用 mock LLM 跑真实 PR 的 AI success path。

## 背景

前置计划覆盖了主要功能开发：

- `2026-05-30-github-pr-fetch.md`：真实 GitHub PR metadata/files/commits 获取。
- `2026-05-30-diff-rules-scanner.md`：diff 解析与确定性规则扫描。
- `2026-05-30-context-builder-llm-analyzer.md`：ContextBuilder、OpenAI 兼容 LLM、ReportNormalizer。
- `2026-05-30-frontend-review-dashboard.md`：前端 Review Dashboard、风险筛选、证据和复制体验。

完成这些后，剩余重点不是继续扩展功能，而是确保项目能在比赛现场和本地环境稳定跑通。demo 模式必须不依赖外部网络和密钥；真实模式必须能清楚处理 GitHub/LLM 失败；README 和提交材料必须准确表达能力、限制和运行方式。

## 需求拆分

### 功能需求

- demo 模式事件阶段对齐真实链路：
  - `fetch_pr`
  - `parse_diff`
  - `scan_rules`
  - `build_context`
  - `analyze_ai`
  - `result`
  - `done`
- demo 状态口径必须明确：
  - 推荐：每个阶段输出 `running` 和 `completed`，让前端时间线更接近真实模式。
  - 可接受：只输出 `completed`，但测试名称必须说明这是 demo 的有意简化。
- demo 报告包含：
  - PR summary
  - key changes
  - review focus
  - `rule` / `ai` / `merged` 三类风险
  - evidence
  - suggested comments
  - degraded=false 或缺省 false
- demo 模式不调用 GitHub API、不调用 LLM、不读取 token。
- 后端验证覆盖：
  - GitHub client
  - diff parser
  - rules scanner
  - context builder
  - LLM analyzer 降级
  - review service pipeline
  - handler SSE 契约
  - demo provider 契约
- 前端验证覆盖：
  - build 通过
  - demo 流程可展示
  - 真实 degraded 报告可展示
  - 错误状态可展示
  - 复制功能可用
  - token 不落 localStorage/sessionStorage
- 安全与降级检查：
  - GitHub token 不出现在日志、错误响应、测试 snapshot
  - LLM API key 不出现在日志、错误响应、测试 snapshot
  - GitHub 失败走 SSE error
  - LLM 失败保留规则结果并返回 degraded report
  - 缺失 patch、大 PR、空规则结果不崩溃
  - 大 PR 或上下文裁剪通过 `result.meta` 和前端提示表达
- Release 文档：
  - README 准确说明运行方式、环境变量、demo、真实 PR、模型配置、降级策略、已知限制。
  - 比赛 README 模板必须补齐项目亮点、架构、演示步骤、测试方式、限制和未来扩展，不能保留 `待补充`。

### 非目标

- 不增加新核心功能。
- 不实现 GitHub App 自动评论。
- 不实现 OAuth、多用户、数据库或历史记录。
- 不做复杂浏览器自动化测试框架，除非当前工具链已经稳定可用。
- 不将 demo 数据硬编码到前端；demo 仍由后端事件流提供。
- 不把 mock LLM 当作真实 AI 能力写进产品文档；mock 只用于 release 验收。

### 验收标准

- `go test ./...` 通过。
- `node scripts/check-pr-quality.test.mjs` 通过。
- `npm --prefix frontend run build` 通过。
- `internal/demo` 不再是 `[no test files]`，demo 契约由测试覆盖。
- demo curl 能完整返回真实 pipeline 形状的 SSE 事件。
- 前端 demo 模式可完整展示 summary、risks、evidence、comments。
- 至少一个公开 GitHub PR 能在真实模式下跑到最终报告或清晰降级报告。
- README 能让新用户独立启动后端、前端并完成 demo 验收。
- `competition_README_TEMPLATE.md` 不再有通用占位符，比赛提交者只需补视频链接、参赛批次等外部信息。
- 文档不夸大能力，不把 demo 或 mock LLM 当真实 AI 分析结果。

## 文件分工

- 修改：`internal/demo/demo.go`
  - 升级 demo 事件流和报告数据。
- 新建：`internal/demo/demo_test.go`
  - 当前不存在。必须覆盖 demo 事件顺序、报告完整性、不依赖外部服务。
- 修改：`internal/review/service_test.go`
  - 如 demo provider 契约变化，补充 service demo 模式测试。
- 修改：`internal/handler/review_handler_test.go`
  - 确认 demo SSE 契约完整返回，并解析事件顺序和 payload。
- 修改：`README.md`
  - 小幅补齐 `PORT`、frontend 架构节点、安全占位符、FAQ，不重写已经准确的内容。
- 修改：`competition_README_TEMPLATE.md`
  - 必须补齐项目说明、亮点、演示步骤、测试方式、限制和未来扩展。
- 修改：`competition_PR_TEMPLATE.md`
  - 如果当前功能完成后 PR 描述模板需要更贴合实际提交，做小幅更新。
- 可选新建：`scripts/mock-openai-compatible.py`
  - 如果没有真实 LLM key，用最小 mock 服务验证 AI success 路径。
- 可选新建：`docs/demo-guide.md`
  - 如果 README 过长，将演示脚本和检查清单拆出。
- 可选新建：`docs/release-checklist.md`
  - 如果比赛提交需要独立 checklist。

## 执行拆分与派发

本阶段按文件边界拆成 5 个工作流，避免多个 agent 同时修改同一批文件：

| 工作流 | 主要任务 | 写入范围 | 依赖 |
| --- | --- | --- | --- |
| A. Demo 契约 | 任务 1，升级 demo 报告和 provider 测试 | `internal/demo/*`、必要时 `internal/review/service_test.go` | 无 |
| B. Handler SSE 契约 | 任务 2，强化 demo SSE handler 测试 | `internal/handler/review_handler_test.go` | 依赖 A 的 demo risk/source 形状 |
| C. Release 文档 | 任务 8、9，补 README 和比赛模板 | `README.md`、`competition_README_TEMPLATE.md`、必要时 `competition_PR_TEMPLATE.md`、`docs/demo-guide.md` | 无 |
| D. Mock LLM 与 checklist | 任务 5.5、10，提供可复现 AI success 验收入口 | `scripts/mock-openai-compatible.py`、`docs/release-checklist.md` | 无 |
| E. QA 与安全验证 | 任务 3、4、5、6、7、10，运行命令并记录缺口 | 不优先写源码；只在发现真实缺陷后最小修复 | 依赖 A-D 合并后的状态 |

派发策略：

- 先并行派发 A、C、D。
- A 返回并整合后，再派发 B，避免 handler 测试对 demo payload 做错假设。
- A-D 整合后，派发或本地执行 E 做最终 QA、安全检查和 release 记录。
- 所有 agent 必须声明自己修改的文件，不能回滚其他 agent 或用户已有改动。

## 任务 1：Demo 事件流与报告升级

**文件：**
- 修改：`internal/demo/demo.go`
- 新建：`internal/demo/demo_test.go`

- [ ] 写失败测试：直接调用 `demo.NewProvider().Stream`，收集完整事件流。
- [ ] 写失败测试：demo 模式事件阶段顺序包含 `fetch_pr`、`parse_diff`、`scan_rules`、`build_context`、`analyze_ai`、`result`，并包含 `pr`、`rules`、`result`、`done` 事件。
- [ ] 写失败测试：如果决定模拟真实状态，则每个阶段应包含 `running` 和 `completed`；如果保持 completed-only，测试名称必须说明这是有意简化。
- [ ] 写失败测试：demo `result.degraded` 为 false 或缺省 false。
- [ ] 写失败测试：demo `result.risks` 包含 `rule`、`ai`、`merged` 三类来源。
- [ ] 写失败测试：每条 demo risk 包含 title、severity、confidence、category、reason、suggestion。
- [ ] 写失败测试：至少一条 demo risk 包含 file、line、evidence。
- [ ] 写失败测试：demo `comments` 非空，且 body 可直接复制到 GitHub。
- [ ] 写失败测试：设置假的 `GITHUB_TOKEN`、`LLM_API_KEY` 后 demo 输出仍然完全一致，证明 demo 不读取外部密钥。
- [ ] 实现新的 demo PR、risks、comments、summary 数据。
- [ ] 确保 demo provider 不读取环境变量、不创建 GitHub client、不创建 LLM analyzer。
- [ ] 运行 `go test ./internal/demo`。

## 任务 2：Demo SSE 与前端契约验证

**文件：**
- 修改：`internal/handler/review_handler_test.go`
- 修改：`frontend/src/types/review.ts`（仅当 demo 字段需要前端兼容）
- 修改：`frontend/src/state/reviewReducer.ts`（仅当事件状态处理需要调整）

- [ ] 写或更新 handler 测试：demo 请求返回所有关键 `event:`，并按 SSE message 粒度解析，而不是只做字符串包含。
- [ ] 确认 `rules` event 在 demo 中先于 `result`。
- [ ] 确认 `done` payload 为 `ok: true`，且 demo 不带 degraded 或 `degraded:false`。
- [ ] 确认 `result` payload 中 summary、risks、comments、meta 字段与前端 `Report` 类型兼容。
- [ ] 确认 demo risk source 包含 `rule`、`ai`、`merged`，防止前端筛选和来源徽标只在真实模式才被覆盖。
- [ ] 运行 `go test ./internal/handler`。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 3：后端全链路 QA

**文件：**
- 不优先改源码；发现缺口后只做最小修复。

- [ ] 运行 `go test ./internal/github`。
- [ ] 运行 `go test ./internal/diff`。
- [ ] 运行 `go test ./internal/rules`。
- [ ] 运行 `go test ./internal/llm`。
- [ ] 运行 `go test ./internal/review`。
- [ ] 运行 `go test ./internal/handler`。
- [ ] 运行 `go test ./...`。
- [ ] 若任何测试失败，先判断是否是测试环境问题还是真实缺陷。
- [ ] 对真实缺陷做最小修复，并重新运行失败包测试。
- [ ] 记录最终通过命令，后续写入 release checklist 或最终汇报。

## 任务 4：前端 QA

**文件：**
- 不优先改源码；发现缺口后只做最小修复。

- [ ] 运行 `npm --prefix frontend run build`。
- [ ] 如果依赖缺失，先运行 `npm --prefix frontend install`，再 build。
- [ ] 检查 TypeScript 错误，优先修复类型契约不一致问题。
- [ ] 启动前端：优先 `npm --prefix frontend run dev`。
- [ ] 如果 Windows 环境遇到 Vite/esbuild `spawn EPERM`，改用 `npm --prefix frontend run build` 后启动 `npm --prefix frontend run preview:static`。
- [ ] 启动后端：`go run ./cmd/server`。
- [ ] 在浏览器手动运行 demo 模式。
- [ ] 验证步骤栏完整展示所有阶段。
- [ ] 验证 Review Brief、Risk Radar、Evidence Drawer、Suggested Comments 都有内容。
- [ ] 验证单条复制和完整 Review 复制。
- [ ] 缩小浏览器宽度，检查移动端无明显重叠或横向溢出。
- [ ] 检查 token 输入只保存在 React state 中，不写入 localStorage/sessionStorage。

## 任务 5：真实模式手动验收

**文件：**
- 不优先改源码；发现缺口后只做最小修复。

- [ ] 选择一个公开 GitHub PR，最好文件数量适中、包含测试或配置变化。
- [ ] 启动后端：`go run ./cmd/server`。
- [ ] 无 token 验证真实模式：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"`
- [ ] 如果遇到 rate limit，使用请求级 `github_token` 重新验证，不要把真实 token 写入命令记录或文档。
- [ ] 验证成功路径至少包含 `event: pr`、`event: rules`、`event: result`、`event: done`。
- [ ] 未配置 LLM key 时，验证返回 degraded report，且保留规则风险。
- [ ] 没有真实 LLM key 时，先用可选 mock OpenAI-compatible 服务验证 AI success 路径。
- [ ] 配置测试 LLM key 或 mock LLM 服务时，验证最终报告包含 AI summary、AI comments，并能产生 `ai` 或 `merged` 风险。
- [ ] 验证真实模式前端展示不因 degraded 状态中断。
- [ ] 记录真实 PR URL、是否使用 token、是否使用 mock LLM、最终是完整报告还是 degraded report。

## 任务 5.5：可选 mock LLM 验收入口

**文件：**
- 可选新建：`scripts/mock-openai-compatible.py`
- 或只在 `docs/release-checklist.md` 中记录外部 mock 启动方式。

- [ ] 如果没有可安全使用的测试 LLM key，新建一个最小 mock 服务，提供 `POST /v1/chat/completions`。
- [ ] mock 响应必须返回符合 analyzer 期望的 JSON 字符串，包含 summary、risks、comments。
- [ ] mock risk 的 `evidence_refs` 必须引用真实 context 中存在的 evidence，否则会被 ReportNormalizer 丢弃。
- [ ] 用 `LLM_BASE_URL=http://127.0.0.1:{port}`、`LLM_API_KEY=test-key`、`LLM_MODEL=mock-model` 启动后端。
- [ ] 跑一个公开 PR，确认非 degraded result 可以展示 AI summary、comments、`ai` 或 `merged` 风险。
- [ ] 不把 mock 当成真实模型能力写进 README，只作为 release 验收工具。

## 任务 6：错误与降级路径 QA

**文件：**
- 不优先改源码；发现缺口后只做最小修复。

- [ ] 验证非法 PR URL 返回 SSE `error`，code 为 `invalid_pr_url`。
- [ ] 验证 GitHub 404 返回 `github_pr_not_found` 或当前约定 code。
- [ ] 验证 GitHub 401/403 返回认证或限流类错误。
- [ ] 验证 GitHub 网络失败不会 panic。
- [ ] 验证缺失 patch 或二进制文件不会让 diff parser 崩溃。
- [ ] 验证 LLM 未配置不作为 fatal SSE error，而是 degraded report。
- [ ] 验证 LLM 返回非法 JSON 时不会展示原始模型输出。
- [ ] 验证空规则结果也能生成可读报告。
- [ ] 验证大 PR 或上下文裁剪会通过 `result.meta.context_truncated`、`omitted_files_count`、`omitted_snippets_count` 表达，并在前端显示 Context limited 提示。

## 任务 7：安全检查

**文件：**
- 不优先改源码；发现泄露风险后做最小修复。

- [ ] 搜索日志和错误格式，确认不会打印 `github_token`。
- [ ] 搜索日志和错误格式，确认不会打印 `GITHUB_TOKEN`。
- [ ] 搜索日志和错误格式，确认不会打印 `LLM_API_KEY`。
- [ ] 检查 README 示例，确认所有 token/API key 都是占位符。
- [ ] 检查测试 snapshot 或断言输出，确认不包含真实 secret。
- [ ] 检查前端代码，确认 token 不写入 localStorage/sessionStorage。
- [ ] 检查 curl 手动验证记录，最终文档中不保留真实 token。
- [ ] 运行源码密钥泄露检查：
  - `rg -n "ghp_[A-Za-z0-9_]+|github_pat_|sk-[A-Za-z0-9_-]{20,}" cmd internal frontend scripts README.md competition_README_TEMPLATE.md competition_PR_TEMPLATE.md competition_COMMIT_CONVENTION.md --glob "!frontend/node_modules/**" --glob "!frontend/dist/**"`
- [ ] 运行敏感字段使用点审查：
  - `rg -n "LLM_API_KEY|GITHUB_TOKEN|github_token|Authorization|localStorage|sessionStorage" cmd internal frontend README.md competition_README_TEMPLATE.md competition_PR_TEMPLATE.md competition_COMMIT_CONVENTION.md --glob "!frontend/node_modules/**" --glob "!frontend/dist/**"`
- [ ] 运行全仓补扫，排除明显 vendored/构建产物：
  - `rg -n "ghp_|github_pat_|sk-|LLM_API_KEY|GITHUB_TOKEN|github_token|Authorization" . --glob "!frontend/node_modules/**" --glob "!frontend/dist/**" --glob "!ui-ux-pro-max-skill/**"`
- [ ] 将测试 fixture、README 占位符、设计文档中的 schema 示例列为误报 allowlist。
- [ ] 对真实泄露立即移除；如果 README 仍使用 `ghp_xxx`，改成 `<github-token>`。

## 任务 8：README 更新

**文件：**
- 修改：`README.md`

- [ ] 检查当前进度表，确保不要把已完成模块写成“待实现”。
- [ ] 更新架构图，展示完整链路：GitHub -> diff -> rules -> context -> LLM -> report -> SSE -> frontend。
- [ ] 增加环境变量说明：
  - `PORT`
  - `GITHUB_TOKEN`
  - `LLM_BASE_URL`
  - `LLM_API_KEY`
  - `LLM_MODEL`
- [ ] 增加 demo 模式说明，强调不需要网络或密钥。
- [ ] 增加真实 PR 验证说明。
- [ ] 增加 LLM 失败降级说明。
- [ ] 把 README 中 token 示例从 `ghp_xxx` 改成 `<github-token>` 或 `replace-with-token`。
- [ ] 增加已知限制：
  - 不自动评论 GitHub
  - 不做 OAuth
  - 规则扫描是启发式
  - 模型输出需人工复核
- [ ] 增加常见问题：
  - GitHub rate limit
  - LLM key 未配置
  - binary/large patch 缺失
- [ ] 如果 README 已经足够长，FAQ 保持短小，不展开内部实现细节。
- [ ] 保持 README 可读，避免堆砌内部实现细节。

## 任务 9：比赛交付材料

**文件：**
- 修改：`competition_README_TEMPLATE.md`
- 修改：`competition_PR_TEMPLATE.md`（如需要）
- 可选新建：`docs/demo-guide.md`
- 可选新建：`docs/release-checklist.md`

- [ ] 在比赛 README 中补充项目一句话介绍，不保留“请在这里”类占位文案。
- [ ] 补充核心功能列表。
- [ ] 补充技术架构说明。
- [ ] 补充模型与上下文策略：
  - 不直接发送完整 raw diff
  - 规则先扫、LLM 后解释
  - JSON 输出解析失败降级
- [ ] 补充误报/漏报控制。
- [ ] 补充演示步骤：
  - 启动后端
  - 启动前端
  - 点击 Demo PR
  - 输入真实 PR
- [ ] 补充测试方式：`go test ./...`、`node scripts/check-pr-quality.test.mjs`、`npm --prefix frontend run build`、demo curl、真实 PR curl。
- [ ] 补充已知限制和未来扩展。
- [ ] 删除或替换 `待补充`、`功能一`、`功能二`、`npm run test` 等通用模板占位内容。
- [ ] 如果新建 `docs/demo-guide.md`，README 中链接它。
- [ ] 如果新建 `docs/release-checklist.md`，列出最终提交前命令清单。

## 任务 10：最终发布检查

**文件：**
- 不新增源码改动，只执行验证和必要文档补齐。

- [ ] 运行 `go test ./...`。
- [ ] 确认 `go test ./...` 输出中 `internal/demo` 不再是 `[no test files]`。
- [ ] 运行 `npm --prefix frontend run build`。
- [ ] 运行 `node scripts/check-pr-quality.test.mjs`。
- [ ] 启动后端并验证 demo curl：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"demo\":true}"`
- [ ] 启动前端并手动验证 demo。
- [ ] 验证一个公开真实 PR。
- [ ] 验证非法 PR URL 错误。
- [ ] 验证无 LLM key 降级路径。
- [ ] 如果使用 mock LLM，验证 AI success 路径并记录 mock 启动命令。
- [ ] 检查 `git status --short`，确认只包含本次预期改动。
- [ ] 检查 README 和比赛文档没有夸大功能，没有把 demo 或 mock LLM 当作真实模型分析能力。
- [ ] 检查 `competition_README_TEMPLATE.md` 没有残留通用占位符。
- [ ] 记录最终验证结果，作为 PR 描述中的测试方式。

## 建议提交拆分

- [ ] `test: cover demo stream contract`
- [ ] `feat: enrich demo review report`
- [ ] `test: cover release validation paths`
- [ ] `docs: update readme for release`
- [ ] `docs: complete competition submission guide`
- [ ] `chore: finalize release checklist`

## 风险与约束

- demo 模式必须稳定，不能依赖真实 GitHub、真实 LLM 或比赛现场网络。
- 文档必须诚实：如果某项能力仍是降级、启发式或 mock 验证，必须明确说明。
- 安全检查必须覆盖 token/API key，不允许为了调试把密钥写进日志或文档。
- QA 发现问题时先做最小修复，不在 release 阶段引入新的大功能。
- 最终提交前必须保证 demo 和真实模式至少各有一条可复现验证路径。
- README 已经有大量有效内容，后续只补缺口，不做无意义重写。
