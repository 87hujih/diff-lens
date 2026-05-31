# 前端 Review Dashboard 实现计划

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本计划。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 将当前单文件 React 页面升级为可演示、可扫描、可复制 Review 建议的专业 PR 分析台，在当前后端契约之上完整消费 `step`、`pr`、`rules`、`ai_delta`、`result`、`error`、`done` 事件。

**架构：** 前端继续保持简单单向数据流：`reviewStream` 解析 SSE，`reviewReducer` 维护状态，`App` 组合页面，组件只负责展示和局部交互。本地 UI action（例如 reset、abort、选中风险）不混入后端 `ReviewEvent` 契约。业务判断仍以后端 `result` 为准，前端只做筛选、复制、展开证据和错误/降级状态展示，不重新推导风险结论。

**技术栈：** React 19、TypeScript、Vite/esbuild、现有 CSS，不引入 Redux/Zustand 或大型 UI 库。

---

## 背景

当前前端已经具备基础能力：

- `frontend/src/api/reviewStream.ts` 使用 `fetch` POST 读取 `text/event-stream`。
- `frontend/src/state/reviewReducer.ts` 能处理 `step`、`pr`、`rules`、`ai_delta`、`result`、`error`、`done`。
- `frontend/src/types/review.ts` 已对齐后端核心事件和报告模型。
- `frontend/src/App.tsx` 已有 PR URL 输入、GitHub Token 输入、Demo PR 按钮、步骤栏、基础风险卡片和单条复制。

当前缺口：

- 大部分 UI 仍集中在 `App.tsx`，组件边界不清晰。
- 没有完整的 Review Brief、Risk Radar、Suggested Comments、Evidence Drawer。
- 没有风险筛选、Evidence Drawer、完整 Review 一键复制。
- `ai_delta` 只进入状态，没有展示模型分析过程。
- 错误、降级、空状态和移动端体验还比较粗糙。

## 当前契约校准

实现前必须按以下事实校准，不要按理想 UI 自行发明字段或事件：

- 后端当前 step id 是 `fetch_pr`、`parse_diff`、`scan_rules`、`build_context`、`analyze_ai`；`result` 和 `done` 是流生命周期事件，不是 step id。
- 后端 step status 当前主要是 `running`、`completed`、`failed`；`degraded` 来自 `Report.degraded` 或 `DonePayload.degraded`，前端可以在视觉上标记降级，但不要假设会收到 `status: "degraded"`。
- `ai_delta` 已在类型中预留，但当前真实 pipeline 不发送该事件；UI 必须能在没有 `ai_delta` 的情况下展示 `analyze_ai` 运行中、完成、失败和降级状态。
- `Risk.evidence` 是当前可直接展示的证据文本；`evidence_refs` 目前只是引用 ID，最终 `Report` 里没有 snippet map，Evidence Drawer 第一版不能承诺从 ref 反查完整 diff 片段或做精确高亮。
- 当前 demo provider 只发送一个 completed step、`pr`、`rules`、`result`、`done`。前端必须能优雅展示这种较短事件流；如果验收需要覆盖所有 step/status 视觉状态，应在本计划中显式加入 demo fixture 更新。
- Go 端 `Summary.RiskLevel` 和 `Risk.Severity` 都是 string，前端需要容忍未知值并降级到中性样式。

## 需求拆分

### 功能需求

- 拆分组件：
  - `PrInputPanel`
  - `StepTimeline`
  - `ReviewBrief`
  - `RiskRadar`
  - `SuggestedComments`
  - `EvidenceDrawer`
  - `AITracePanel` 或等价 AI 分析过程展示组件
  - `StatusBanner` 或等价错误/降级提示组件
- 输入区：
  - PR URL 输入
  - GitHub Token 折叠/密码输入
  - Analyze PR 按钮
  - Demo PR 按钮
  - running 时禁用重复提交或清晰处理 abort
- 步骤栏：
  - 展示 `fetch_pr`、`parse_diff`、`scan_rules`、`build_context`、`analyze_ai` 阶段；缺失阶段不强行补假数据
  - running/completed/failed 状态视觉可区分
  - 根据 `state.degraded` 或 `result.degraded` 展示全局降级提示，而不是依赖 step status
  - 没有事件时展示空状态
- Review Brief：
  - 展示 repo、PR number、title、author、source/target branch
  - 展示 changed files/additions/deletions/commits
  - 展示 summary overview、key changes、review focus
  - 展示整体 risk level 和 degraded 状态
- Risk Radar：
  - 展示来自 `result.risks` 的最终风险；没有 result 时展示 `ruleRisks`
  - 支持 High / Medium / Low / All 筛选
  - 展示 source、category、confidence、file、line、reason、suggestion
  - 点击风险时打开 Evidence Drawer
- Evidence Drawer：
  - 展示选中风险的 `evidence`、`evidence_refs`、file、line、suggestion
  - 允许复制 evidence 或建议文本
  - 缺少 evidence 时展示清晰空状态
  - 如果只有 `evidence_refs`，展示引用 ID，不伪造 diff 片段
- Suggested Comments：
  - 展示每条可复制 GitHub Review 评论
  - 支持单条复制
  - 支持一键复制完整 Review
  - 复制格式包含文件/行号（如果有）和评论正文
- AI 分析过程：
  - 如果有 `ai_delta`，展示模型分析文本或“AI 正在分析”的面板
  - 最终 UI 仍以 `result` 为准，不用 `ai_delta` 推导报告
- 错误与降级：
  - `error` 事件展示 code、message、stage、recoverable
  - `degraded` 报告展示非阻塞警告
  - LLM 未配置或失败时说明“规则结果仍可用”
  - 用户主动 abort 或新请求替换旧请求时，不展示为失败

### 非目标

- 不实现前端业务风险判断。
- 不新增路由系统。
- 不引入图表库、大型 UI 框架或复杂状态库。
- 不做 GitHub OAuth。
- 不实现真实 GitHub 评论发布。
- 不在前端保存 token。
- 不把前端本地 `reset` action 加入后端 SSE 事件类型。
- 不在本计划内新增后端 evidence snippet map；Evidence Drawer 只消费现有 `Risk` 字段。

### 验收标准

- `npm --prefix frontend run build` 通过。
- `go test ./...` 不因前端类型契约变化而失败。
- demo 模式页面能完整展示已有 demo 数据对应的 steps、brief、risk、comments 和 evidence；如果要求覆盖所有 step/status 视觉状态，需同步更新 `internal/demo/demo.go` fixture。
- 真实模式在 degraded report 下仍能清晰展示已完成内容。
- 风险筛选不会改变后端结果，只改变当前展示列表。
- 单条复制和完整 Review 复制可用。
- 移动端宽度下无明显文字重叠、按钮溢出或布局错位。
- Drawer 可通过关闭按钮和 `Esc` 关闭；移动端不遮挡不可恢复的主要内容。
- SSE 多行 data、残留 buffer、JSON parse error、AbortError、非 2xx 响应都有明确行为。

## 文件分工

- 修改：`frontend/src/App.tsx`
  - 只保留页面级状态、请求动作和组件组合。
- 修改：`frontend/src/types/review.ts`
  - 按后端最终契约补充可选字段，避免前端因字段缺失崩溃。
- 修改：`frontend/src/state/reviewReducer.ts`
  - 修正新请求重置、error/done/degraded 状态和 `ai_delta` 累积行为。
- 修改：`frontend/src/api/reviewStream.ts`
  - 增强 SSE 解析容错，处理多行 data、最后残留 buffer 和 JSON parse error。
- 新建：`frontend/src/components/PrInputPanel.tsx`
- 新建：`frontend/src/components/StepTimeline.tsx`
- 新建：`frontend/src/components/ReviewBrief.tsx`
- 新建：`frontend/src/components/RiskRadar.tsx`
- 新建：`frontend/src/components/SuggestedComments.tsx`
- 新建：`frontend/src/components/EvidenceDrawer.tsx`
- 新建：`frontend/src/components/StatusBanner.tsx`
- 新建：`frontend/src/components/AITracePanel.tsx`
- 新建：`frontend/src/utils/copyReview.ts`
  - 生成单条评论和完整 Review 文本。
- 新建：`frontend/src/utils/clipboard.ts`
  - 封装 Clipboard API 和不可用时的温和失败。
- 新建：`frontend/src/utils/riskFilters.ts`
  - 风险筛选和排序，不做业务结论推导。
- 修改：`frontend/src/styles.css`
  - 重构布局、状态、卡片、抽屉、按钮、移动端样式。
- 可选修改：`internal/demo/demo.go`
  - 仅当验收需要完整覆盖所有 step/status/AI Trace 视觉状态时，扩充 demo 事件流 fixture。

## 任务 1：类型与状态契约整理

**文件：**
- 修改：`frontend/src/types/review.ts`
- 修改：`frontend/src/state/reviewReducer.ts`

- [ ] 检查后端 `review.Report`、`Risk`、`SuggestedComment` 当前字段，确保前端类型兼容。
- [ ] 将后端输入字段 `Summary.risk_level`、`Risk.severity` 保持为可接收任意 string，并新增 `normalizeRiskLevel` / `normalizeSeverity` helper，把渲染层输出收敛为 `low | medium | high | unknown`。
- [ ] 将后端输入字段 `StepPayload.status` 保持为可接收任意 string，并新增 `normalizeStepStatus` helper，把渲染层输出收敛为 `running | completed | failed | unknown`。
- [ ] 为 `ReviewState` 增加 `activeRiskId` 或由 `App` 局部 state 管理选中风险，二选一并保持清晰；不要把选中状态塞进后端事件。
- [ ] 增加本地 reducer action（推荐 `{ type: "reset" }` 或 `{ type: "stream_event"; event }` action union），在请求开始前显式 reset，避免新请求沿用旧 result/error。
- [ ] 修正 `ai_delta`：如果后端 data 是对象、字符串或空值，都能安全转成展示文本；对象建议 `JSON.stringify(data, null, 2)`。
- [ ] 增加 request generation id 或等价保护，确保旧请求 abort 后的迟到事件不会污染新请求状态。
- [ ] 增加 reducer 测试/验证条件说明：reset 清空旧状态、error 保留已收到 PR/rules、done(degraded) 标记降级、ai_delta 累积对象和字符串。
- [ ] 运行 `npm --prefix frontend run build`，确认类型通过。

## 任务 2：增强 SSE Client 容错

**文件：**
- 修改：`frontend/src/api/reviewStream.ts`

- [ ] 支持 SSE 多行 `data:` 合并。
- [ ] 在 reader 结束后处理最后残留 buffer。
- [ ] 兼容 `\n\n` 和 `\r\n\r\n` 分隔。
- [ ] 忽略 comment 行（以 `:` 开头）和空行。
- [ ] JSON parse 失败时抛出包含 event type 的错误，但不打印敏感请求体、token 或完整 data。
- [ ] AbortError 不应被 UI 当作普通失败显示；调用方可选择忽略。
- [ ] 对非 2xx 响应保留明确错误。
- [ ] 如果 `response.body` 为空，抛出明确错误。
- [ ] 增加解析器验证样例：单帧、多行 data、两个 chunk 拼接、结束时残留 buffer、非法 JSON。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 3：拆分输入区和步骤栏组件

**文件：**
- 新建：`frontend/src/components/PrInputPanel.tsx`
- 新建：`frontend/src/components/StepTimeline.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] 将 PR URL、Token、Analyze、Demo 控件移入 `PrInputPanel`。
- [ ] Token 使用 password 输入，默认不保留刷新状态。
- [ ] running 时 Analyze 按钮展示进行中状态，并禁用重复提交或明确显示“重新开始会取消当前分析”。
- [ ] 新请求开始时 abort 旧请求、dispatch reset，并创建新的 request generation id。
- [ ] 将步骤展示移入 `StepTimeline`。
- [ ] `StepTimeline` 对 running/completed/failed 使用不同视觉状态；degraded 用全局 banner 或 timeline 注记展示。
- [ ] `StepTimeline` 使用后端 step id 原文，同时通过 label map 显示友好名称；未知 step id 使用原文。
- [ ] 保持第一屏就是可操作分析台，不做营销 hero。
- [ ] 输入框使用明确 label，不只依赖 placeholder。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 4：实现 Review Brief

**文件：**
- 新建：`frontend/src/components/ReviewBrief.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] 展示 PR repo、number、title、author。
- [ ] 展示 source branch -> target branch。
- [ ] 展示 changed files、additions、deletions、commits。
- [ ] 展示 summary overview。
- [ ] 展示 key changes。
- [ ] 展示 review focus。
- [ ] 展示 risk level badge。
- [ ] risk level 未知时显示 neutral badge，不让 CSS class 变成未定义样式。
- [ ] degraded report 显示清晰提示，不遮挡报告内容。
- [ ] `meta.context_truncated`、`omitted_files_count`、`omitted_snippets_count` 有值时用小型提示说明上下文裁剪。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 5：实现 Risk Radar 与筛选

**文件：**
- 新建：`frontend/src/components/RiskRadar.tsx`
- 新建：`frontend/src/utils/riskFilters.ts`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] 实现 All / High / Medium / Low 筛选。
- [ ] 排序规则：high 优先，其次 medium、low，未知 severity 放在 low 后；同级保持输入顺序。
- [ ] 展示 severity、source、category、confidence。
- [ ] 展示 file 和 line；缺失时展示“未绑定具体行”之类的中性文本。
- [ ] 展示 reason 和 suggestion。
- [ ] 点击卡片触发选中风险，用于 Evidence Drawer。
- [ ] 选中风险必须有清晰 active 状态；键盘 focus 状态可见。
- [ ] 无风险时展示空状态，不渲染空白卡片。
- [ ] 为 `riskFilters.ts` 记录验证样例：筛选不改变原数组、排序稳定、未知 severity 不崩溃。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 6：实现 Evidence Drawer

**文件：**
- 新建：`frontend/src/components/EvidenceDrawer.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] Drawer 展示选中风险 title、severity、source、file、line。
- [ ] 展示 evidence，使用等宽块并允许换行。
- [ ] 展示 evidence_refs；没有 evidence 但有 refs 时说明“只有引用 ID，当前报告未包含完整片段”。
- [ ] 展示 suggestion。
- [ ] 支持关闭 Drawer。
- [ ] 支持 `Esc` 关闭，并在关闭后把焦点返回触发的风险卡片或安全位置。
- [ ] 支持复制 evidence。
- [ ] 支持复制 suggestion。
- [ ] 没有 evidence 时展示“该风险没有可展示证据片段”，不要空白。
- [ ] 移动端 Drawer 改为全宽底部或内联详情，避免遮挡主要内容。
- [ ] Drawer 使用合适的 `role`、`aria-label` 或标题关联，关闭按钮有可读 label。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 7：实现 Suggested Comments 与复制格式

**文件：**
- 新建：`frontend/src/components/SuggestedComments.tsx`
- 新建：`frontend/src/utils/copyReview.ts`
- 新建：`frontend/src/utils/clipboard.ts`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] 实现 `formatSingleComment(comment)`。
- [ ] 实现 `formatFullReview(report)`。
- [ ] 单条复制包含 file/line（如果有）和 body。
- [ ] 完整 Review 复制包含 summary、risk level、重点风险和 suggested comments。
- [ ] 复制成功后给按钮短暂状态反馈，例如 “Copied”。
- [ ] Clipboard API 不可用、权限拒绝或非 HTTPS 环境时，降级为显示可选中文本/错误提示，至少不崩溃。
- [ ] 没有 comments 时展示空状态。
- [ ] 为 `copyReview.ts` 记录验证样例：无 file/line、有 file 无 line、完整 report 无 comments、完整 report 有多条 risks/comments。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 8：AI Trace、错误和降级状态

**文件：**
- 新建：`frontend/src/components/AITracePanel.tsx`
- 新建：`frontend/src/components/StatusBanner.tsx`
- 修改：`frontend/src/App.tsx`
- 修改：`frontend/src/styles.css`

- [ ] `AITracePanel` 展示 `state.aiText`，为空但 analyze_ai running 时展示 AI 分析中状态。
- [ ] 如果后端始终没有发送 `ai_delta`，`AITracePanel` 不应显得像数据丢失；文案应说明“等待模型结果”或“AI 阶段无流式文本”。
- [ ] `StatusBanner` 展示 error code、message、stage、recoverable。
- [ ] degraded 状态展示非阻塞提示，并优先显示 `result.meta.degraded_reason`。
- [ ] failed 状态下仍保留已经收到的 PR 或规则信息，方便用户判断失败阶段。
- [ ] AbortError 不展示为失败。
- [ ] 新请求主动 abort 旧请求时，旧请求 Promise reject 不应覆盖新请求状态。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 9：整体布局与响应式打磨

**文件：**
- 修改：`frontend/src/styles.css`
- 修改：`frontend/src/App.tsx`

- [ ] 桌面端保持左侧步骤、右侧报告的工作台布局。
- [ ] 报告区使用清晰分区：Brief、Risk Radar、Suggested Comments、Evidence/AI Trace。
- [ ] 移动端改为单列布局。
- [ ] 所有按钮文字不溢出。
- [ ] 风险卡片长文件名可换行或截断。
- [ ] evidence 代码块不撑破容器。
- [ ] 主要交互目标尺寸不小于 44px；键盘 focus 可见。
- [ ] 在 375px、768px、1440px 三个宽度检查布局。
- [ ] 避免页面使用营销式 hero；第一屏保留实际工具。
- [ ] 运行 `npm --prefix frontend run build`。

## 任务 10：测试、手动验收与文档更新

**文件：**
- 修改：`README.md`
- 可选修改：`internal/demo/demo.go`

- [ ] 更新 README 前端功能说明，列出 Review Brief、Risk Radar、Suggested Comments、Evidence Drawer。
- [ ] 添加前端启动命令和 demo 验收步骤。
- [ ] 说明完整报告来自 `result` 事件，`ai_delta` 只展示分析过程。
- [ ] 如果要求 demo 覆盖所有 step/status/AI Trace 视觉状态，扩充 `internal/demo/demo.go`：发送 `fetch_pr`、`parse_diff`、`scan_rules`、`build_context`、`analyze_ai` 的 running/completed 样例，至少一条 `ai_delta`，并保持现有 demo report 字段。
- [ ] 启动后端：`go run ./cmd/server`。
- [ ] 启动前端：`npm --prefix frontend run dev`。
- [ ] 用 demo 模式手动验收已有 fixture 对应的完整流程。
- [ ] 用真实公开 PR 手动验收 degraded 和完整报告两类状态；如果本地没有 LLM 配置，至少覆盖 rules-only degraded report。
- [ ] 手动验收 invalid PR URL，确认展示结构化错误且不泄露 token。
- [ ] 手动验收连续点击 Analyze/Demo，确认旧请求不会污染新结果。
- [ ] 手动验收复制功能成功和失败路径；失败路径可以通过临时禁用 `navigator.clipboard` 或非安全上下文模拟。
- [ ] 手动验收 Drawer 的关闭按钮、`Esc` 关闭、移动端底部/内联展示。
- [ ] 在 375px、768px、1440px 宽度检查无文字重叠、按钮溢出、evidence 横向撑破。
- [ ] 运行 `npm --prefix frontend run build`。
- [ ] 运行 `go test ./...`。
- [ ] 运行 `node scripts/check-pr-quality.test.mjs`。

## 建议提交拆分

- [ ] `feat: split review dashboard components`
- [ ] `feat: add risk filtering and evidence drawer`
- [ ] `feat: add copyable review summary`
- [ ] `feat: polish review dashboard states`
- [ ] `docs: document frontend review dashboard`
- [ ] 可选：`test: expand review dashboard demo fixture`

## 风险与约束

- 前端不能重新判断风险结论，最终报告必须以后端 `result` 为准。
- `ai_delta` 不能驱动最终 UI 结论，只能展示分析过程。
- Token 不能持久化，不能写入 localStorage。
- Clipboard API 可能不可用，复制功能需要温和失败。
- 响应式布局要优先保证可读和不重叠，而不是追求复杂视觉效果。
- Abort 和快速重试容易产生迟到事件，必须通过 abort signal 加 request generation id 或等价机制隔离。
- 任何新增 CSS class 基于后端 string 值时都要先 normalize，不能直接把未知后端值拼进关键布局样式。

## 测试矩阵

实现完成后至少覆盖以下路径；没有前端测试框架时，必须在 PR 描述或 README 验收记录中逐项说明手动验证结果。

| 区域 | 场景 | 期望 |
| --- | --- | --- |
| SSE parser | 多行 `data:`、`\r\n`、残留 buffer、非法 JSON | 正确解析或给出不含敏感数据的错误 |
| reducer | reset、error、done degraded、ai_delta 对象/字符串 | 状态清空/保留/累积行为符合任务 1 |
| request lifecycle | 新请求 abort 旧请求、旧 Promise reject | 新请求状态不被覆盖，AbortError 不显示失败 |
| Risk Radar | All/High/Medium/Low、未知 severity、稳定排序 | 只改变展示列表，不修改源数据 |
| Copy | 单条、完整 Review、Clipboard 不可用 | 文本格式正确，失败不崩溃 |
| Evidence Drawer | 有 evidence、只有 refs、无 evidence、Esc 关闭 | 信息明确，焦点和移动端行为可用 |
| Degraded/Error | LLM 未配置、AI 失败、invalid URL、GitHub rate limit | 已完成内容保留，错误/降级提示可读 |
| Responsive | 375px、768px、1440px | 无明显重叠、溢出或不可点击控件 |
