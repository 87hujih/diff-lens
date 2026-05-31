# Diff 解析与规则扫描实现计划

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本计划。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 在真实 PR 模式中解析 GitHub file patch，提取可分析的 diff 结构和文件统计，并基于确定性规则输出第一批风险结果。

**架构：** `internal/review.Service` 负责把 GitHub DTO 转换成 `diff.FileInput`，再串联 diff parser 和 rules scanner。`internal/diff` 只依赖自身的语言无关输入，输出结构化 diff、文件标签、patch 状态、统计和解析 warning，不 import `internal/github`。`internal/rules` 只消费 `diff.Analysis` 并输出自身领域模型 `rules.Finding`，不 import `internal/review`；`review.Service` 最后把 finding 映射为 `review.Risk`，通过 SSE 增加 `parse_diff`、`scan_rules` 阶段与 `rules` 事件，最终仍返回 degraded 报告，因为 LLM 分析尚未实现。

**技术栈：** Go 标准库、Go 单元测试、现有 Gin/SSE 事件契约。

---

## 数据流

```text
github.PullRequestData
  -> review.Service converts Files to []diff.FileInput
  -> diff.Parser parses hunks, tags files, records patch status and warnings
  -> rules.Scanner scans diff.Analysis and returns []rules.Finding
  -> review.Service maps findings to review.Risk
  -> SSE emits pr, rules, degraded result, done
```

## 背景

当前项目已经完成或正在完成真实 GitHub PR 获取模块：

- `internal/github.Client` 能获取 PR metadata、changed files 和 commits。
- `internal/review.Service` 真实模式已经能发送 `step`、`pr`、`result` 和 `done`，但最终报告仍是 degraded。
- `internal/diff/parser.go` 目前只有 `FileStats` 和空 `Parser`。
- `internal/rules/scanner.go` 目前 `Scan()` 返回 `nil`。
- 设计文档要求第一版规则扫描覆盖敏感信息、危险操作、测试缺口、配置依赖变化和大 PR 风险。

这个模块的目标是让真实 PR 模式从“只拉数据”推进到“能基于 patch 输出稳定的确定性风险”。LLM、上下文构建和 AI 评论生成仍是后续模块。

## 需求拆分

### 功能需求

- 在 `review.Service` 中从 `github.PullRequestData.Files` 转换出 `[]diff.FileInput`，包含文件名、状态、增删行、changes 和 raw patch。
- `internal/diff` 不直接接收或 import GitHub DTO，便于后续支持 GitLab、本地 diff 或测试 fixture。
- 解析 GitHub unified diff patch：
  - 识别 hunk header：`@@ -oldStart,oldCount +newStart,newCount @@`
  - 识别新增行、删除行和上下文行
  - 为新增行计算新文件行号
  - 为删除行计算旧文件行号
  - 保留每个 hunk 的简短上下文
- 对文件做标签化分类：
  - 测试文件
  - 配置文件
  - 依赖/锁文件
  - CI 文件
  - 文档文件
  - 源码文件
  - 同一个文件允许多个标签，例如 `.github/workflows/ci.yml` 同时是 CI 和配置文件
- 单独记录 patch 状态，不把二进制或缺失 patch 当作文件类型：
  - 有 patch
  - 空 patch
  - 缺失 patch
  - 二进制或 GitHub 省略 patch
- 汇总 PR 级统计：
  - 变更文件数
  - 新增/删除行数
  - 测试文件数量
  - 配置文件数量
  - 依赖文件数量
  - CI 文件数量
  - 文档文件数量
  - 源码文件数量
  - 缺失 patch 文件数量
  - 二进制或被 GitHub 省略 patch 文件数量
  - 是否有源码变更
  - 是否有测试变更
- 实现第一批确定性规则：
  - 敏感信息风险：token、secret、password、private key、API key 等疑似内容，证据必须脱敏
  - 危险操作风险：rm -rf、DROP TABLE、TRUNCATE、DELETE without WHERE、chmod 777、force push 等
  - 测试缺口：源码变更但没有测试文件变更
  - 变更规模风险：文件数或 diff 行数超过阈值
  - 配置/依赖风险：CI、Docker、env、权限、锁文件或依赖文件变化
- 将规则结果作为 `rules.Finding` 返回，包含 id、rule_id、severity、confidence、category、title、file、line、masked_evidence、reason 和 suggestion。
- `review.Service` 负责把 `rules.Finding` 映射成 `review.Risk`，避免 `internal/rules` 依赖应用层报告模型。
- `review.Service` 真实模式新增阶段：
  - fetch 完成后尽早发送 `pr` 事件
  - `parse_diff` running/completed
  - `scan_rules` running/completed
  - `rules` 事件
  - 最终 degraded `result` 中包含规则风险和可复制建议草稿（可选）

### 非目标

- 不调用 LLM。
- 不实现 ContextBuilder。
- 不做语言专属 AST 分析。
- 不在第一批实现 SQL/命令拼接、空 catch、忽略 error 等语言相关或上下文不足时高误报的规则；这些规则后续进入语言感知 scanner 或低置信度实验规则。
- 不自动评论 GitHub。
- 不修改前端交互，除非现有 SSE 契约无法消费 `rules` 事件。
- 不把所有规则做成可配置系统；第一版先使用代码内固定阈值和 pattern。
- 不在 SSE 或最终报告中暴露疑似密钥原文。

### 验收标准

- `go test ./internal/diff` 通过。
- `go test ./internal/rules` 通过。
- `go test ./internal/review` 覆盖真实模式中的 diff/rules 事件顺序。
- `go test ./...` 通过。
- demo 模式事件顺序保持不变。
- 真实模式成功路径至少返回：
  - `step: fetch_pr`
  - `pr`
  - `step: parse_diff`
  - `step: scan_rules`
  - `rules`
  - `result`
  - `done`
- 最终报告仍 `degraded: true`，但 `risks` 来自规则扫描结果。
- 缺失 patch、二进制文件、空 patch 不会导致 panic。
- `internal/diff` 不 import `internal/github`。
- `internal/rules` 不 import `internal/review`。
- 敏感信息 finding 的 evidence 必须脱敏，测试覆盖不能回传原始 secret。
- malformed hunk 只影响对应文件或 hunk，记录 warning 并继续；只有整体输入非法才返回 parser error。

## 文件分工

- 修改：`internal/diff/parser.go`
  - 实现 parser 主入口、文件分类、patch 解析和统计。
- 新建：`internal/diff/types.go`
  - 定义 `FileInput`、`Analysis`、`FileDiff`、`DiffHunk`、`DiffLine`、`FileKind`、`PatchStatus`、`Warning` 等结构。
- 新建：`internal/diff/parser_test.go`
  - 覆盖 hunk 解析、行号计算、文件分类、统计和缺失 patch。
- 新建：`internal/rules/types.go`
  - 定义 `Finding` 和规则相关常量，避免 rules 包依赖 review 包。
- 修改：`internal/rules/scanner.go`
  - 让 `Scanner.Scan` 接收 `diff.Analysis` 并输出 `[]rules.Finding`。
- 新建：`internal/rules/scanner_test.go`
  - 覆盖各类确定性规则和去重行为。
- 修改：`internal/review/service.go`
  - 注入 diff parser 与 rules scanner，真实模式串联 GitHub -> `diff.FileInput` -> diff -> rules -> review risk。
  - 在 service 内完成 `rules.Finding` 到 `review.Risk` 的映射。
- 修改：`internal/review/service_test.go`
  - 使用 fake GitHub client、fake parser 或真实轻量 parser 测试事件顺序和降级报告。
- 修改：`cmd/server/main.go`
  - 注入真实 `diff.Parser` 和 `rules.Scanner`。
- 修改：`README.md`
  - 更新当前进度与真实模式能力说明。

## 任务 1：Diff 数据结构与文件分类

**文件：**
- 新建：`internal/diff/types.go`
- 修改：`internal/diff/parser.go`
- 新建：`internal/diff/parser_test.go`

- [ ] 写失败测试：`ClassifyFile("src/app_test.go")` 包含测试文件标签。
- [ ] 写失败测试：`ClassifyFile(".github/workflows/ci.yml")` 同时包含 CI 和配置文件标签。
- [ ] 写失败测试：`ClassifyFile("package-lock.json")` 同时包含依赖和锁文件标签。
- [ ] 写失败测试：`ClassifyFile("README.md")` 包含文档标签。
- [ ] 写失败测试：普通 `.go`、`.ts`、`.tsx`、`.js` 文件包含源码标签。
- [ ] 定义 `FileKind`，至少覆盖 `source`、`test`、`config`、`dependency`、`lockfile`、`ci`、`docs`。
- [ ] 定义 `PatchStatus`，至少覆盖 `present`、`empty`、`missing`、`binary_or_omitted`。
- [ ] 定义 `FileInput`，包含 filename、status、additions、deletions、changes、patch。
- [ ] 定义 `Analysis`，包含 `Files []FileDiff`、`Stats FileStats` 和 `Warnings []Warning`。
- [ ] 定义 `FileDiff`，包含 filename、status、kinds、additions、deletions、changes、patch、hunks、hasPatch、patchStatus。
- [ ] 保留并扩展现有 `FileStats`，增加 CI、docs、source、missing patch 等统计字段。
- [ ] 实现文件分类逻辑。
- [ ] 运行 `go test ./internal/diff`，确认分类测试通过。

## 任务 2：GitHub Patch 解析器

**文件：**
- 修改：`internal/diff/parser.go`
- 修改：`internal/diff/parser_test.go`

- [ ] 写失败测试：解析单 hunk patch，能得到新增行、删除行和上下文行。
- [ ] 写失败测试：新增行的新文件行号正确。
- [ ] 写失败测试：删除行的旧文件行号正确。
- [ ] 写失败测试：多 hunk patch 都能解析。
- [ ] 写失败测试：`\ No newline at end of file` 不会作为正常 diff 行。
- [ ] 写失败测试：空 patch 或缺失 patch 文件仍进入 `Analysis.Files`，并设置对应 `PatchStatus`。
- [ ] 实现 `Parser.ParseFiles(files []FileInput) (Analysis, error)` 或等价入口。
- [ ] 对 malformed hunk 做温和降级：保留文件统计和可解析 hunk，记录 `Analysis.Warnings`，不因单个 hunk 失败直接中断整个 PR。
- [ ] 写测试验证 `internal/diff` 不 import `internal/github`。
- [ ] 运行 `go test ./internal/diff`。

## 任务 3：Diff 汇总统计

**文件：**
- 修改：`internal/diff/parser.go`
- 修改：`internal/diff/parser_test.go`

- [ ] 写失败测试：`FileStats.ChangedFiles`、`Additions`、`Deletions` 来自 `diff.FileInput`。
- [ ] 写失败测试：测试文件数量、配置文件数量、依赖文件数量、锁文件数量、CI 文件数量统计正确，允许同一文件贡献多个统计。
- [ ] 写失败测试：源码变更但无测试变更时，`HasSourceChanges=true` 且 `HasTestChanges=false`。
- [ ] 写失败测试：缺失 patch 文件数量、二进制或 omitted patch 文件数量统计正确。
- [ ] 实现 PR 级统计聚合。
- [ ] 确保统计逻辑不依赖 patch 一定存在。
- [ ] 运行 `go test ./internal/diff`。

## 任务 4：规则扫描器接口与基础设施

**文件：**
- 新建：`internal/rules/types.go`
- 修改：`internal/rules/scanner.go`
- 新建：`internal/rules/scanner_test.go`

- [ ] 定义 `rules.Finding`，字段包含 id、rule_id、severity、confidence、category、title、file、line、masked_evidence、reason、suggestion。
- [ ] 将 `Scanner.Scan()` 改为 `Scanner.Scan(analysis diff.Analysis) []Finding` 或等价签名。
- [ ] 写测试验证 `internal/rules` 不 import `internal/review`。
- [ ] 为每条 finding 生成稳定 ID，使用 `ruleID + file + line + hash(normalizedEvidence)`，不能使用扫描顺序 index。
- [ ] 实现基础 helper：
  - 只扫描新增行作为主要风险来源
  - 根据 file+line+category 去重
  - severity 与 confidence 使用固定规则
  - evidence 做长度裁剪，避免过长响应
  - 敏感信息 evidence 必须脱敏后再进入 finding
- [ ] 写测试验证空 analysis 返回空风险。
- [ ] 写测试验证 finding ID 稳定且不为空，新增一条无关 finding 不会改变已有 finding ID。
- [ ] 运行 `go test ./internal/rules`。

## 任务 5：实现第一批确定性规则

**文件：**
- 修改：`internal/rules/scanner.go`
- 修改：`internal/rules/scanner_test.go`

- [ ] 写并实现敏感信息规则测试：
  - `password = "..."`
  - `api_key`
  - `secret`
  - `private key`
- [ ] 写并实现敏感信息 evidence 脱敏测试：
  - finding 不包含原始密钥值
  - finding 保留足够定位问题的 key 名和文件行号
- [ ] 写并实现危险操作规则测试：
  - `rm -rf`
  - `DROP TABLE`
  - `TRUNCATE`
  - 明显无 WHERE 的 `DELETE FROM`
- [ ] 写并实现测试缺口规则测试：
  - 有源码变更且无测试变更时产生 medium risk
  - 只有文档或测试变更时不产生测试缺口风险
- [ ] 写并实现大 PR 规则测试：
  - 文件数超过阈值
  - 增删总行数超过阈值
- [ ] 写并实现配置/依赖风险测试：
  - CI 文件变化
  - Dockerfile 变化
  - `.env` 或 env 示例变化
  - lockfile 或依赖文件变化
- [ ] 明确延期 SQL/命令拼接和错误处理类规则，并在 README 或代码注释中避免宣称已覆盖这类语义风险。
- [ ] 运行 `go test ./internal/rules`。

## 任务 6：接入 Review Service 真实模式

**文件：**
- 修改：`internal/review/service.go`
- 修改：`internal/review/service_test.go`
- 修改：`cmd/server/main.go`

- [ ] 给 `review.ServiceOptions` 增加 diff parser 和 rules scanner 依赖。
- [ ] 定义 service 内部接口，避免 `review` 层依赖具体实现细节过重：
  - parser 输入 `[]diff.FileInput`，输出 `diff.Analysis`
  - scanner 输入 `diff.Analysis`，输出 `[]rules.Finding`
- [ ] 在 `review.Service` 中实现 `github.PullRequestData.Files` 到 `[]diff.FileInput` 的转换。
- [ ] 在 `review.Service` 中实现 `rules.Finding` 到 `review.Risk` 的映射。
- [ ] 写失败测试：真实模式事件顺序包含 `fetch_pr`、`pr`、`parse_diff`、`scan_rules`、`rules`、`result`、`done`，其中 `pr` 在 parse 和 scan 前发出。
- [ ] 写失败测试：`rules` event 中包含 scanner finding 映射后的风险。
- [ ] 写失败测试：最终 `Report.Risks` 包含规则风险，`Report.Degraded=true`。
- [ ] 写失败测试：parser 整体失败时返回 typed recoverable error，stage 为 `parse_diff`。
- [ ] 写失败测试：parser warning 不阻断扫描，最终报告保持 degraded 并可包含可解析文件的规则风险。
- [ ] 写失败测试：scanner 空输入安全；如未来 scanner 返回 error，再补 typed recoverable error。
- [ ] 实现真实模式串联：GitHub fetch -> PR event -> diff input conversion -> diff parse -> rules scan -> report normalize。
- [ ] 在 `cmd/server/main.go` 注入 `diff.NewParser()` 和 `rules.NewScanner()`。
- [ ] 运行 `go test ./internal/review`。

## 任务 7：README 与当前能力说明

**文件：**
- 修改：`README.md`

- [ ] 更新进度表：
  - diff 解析标为已完成第一阶段
  - 规则风险扫描标为已完成第一批通用规则
  - LLM 分析仍保持预留或未完成
- [ ] 说明真实模式现在会获取 GitHub PR，并输出规则扫描风险。
- [ ] 说明最终报告仍是 degraded，因为 AI 分析尚未实现。
- [ ] 说明第一批规则不覆盖 SQL/命令拼接、空 catch、忽略 error 等需要更多上下文的语义风险。
- [ ] 更新 curl 验证示例的预期输出，加入 `event: rules`。
- [ ] 不要宣称完整 AI Review 已完成。

## 任务 8：最终验证

**文件：**
- 不新增源代码改动，只执行前面任务产生的验证。

- [ ] 运行 `go test ./internal/diff`。
- [ ] 运行 `go test ./internal/rules`。
- [ ] 运行 `go test ./internal/review`。
- [ ] 运行 `go test ./internal/handler`，确认 SSE 契约未被破坏。
- [ ] 运行 `go test ./...`。
- [ ] 运行 `node scripts/check-pr-quality.test.mjs`。
- [ ] 可选：启动服务 `go run ./cmd/server`。
- [ ] 验证 demo 模式仍可用：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"demo\":true}"`
- [ ] 验证真实模式包含规则扫描：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"`
- [ ] 确认真实模式输出包含 `event: rules`。
- [ ] 确认真实模式先输出 `event: pr`，再进入 `parse_diff` 和 `scan_rules` 阶段。
- [ ] 确认最终 `result` 仍包含 `"degraded":true`。
- [ ] 使用包含疑似 secret 的 fixture 验证 `rules` 和 `result` 中不包含原始 secret 值。

## 建议提交拆分

- [ ] `feat: add diff analysis data model`
- [ ] `feat: parse github pull request patches`
- [ ] `feat: add rules finding model`
- [ ] `feat: add rule based risk scanner`
- [ ] `feat: stream rule scan results`
- [ ] `docs: document diff and rule scan increment`

## 风险与约束

- GitHub patch 不是完整文件内容，只能做基于变更行的启发式扫描，规则文案必须避免过度确定。
- GitHub 可能省略 binary/large file patch，parser 必须保留文件记录并降级处理。
- 规则扫描容易误报；第一版要保守设置 severity 和 confidence，证据不足时不要标 high。
- 敏感信息规则可能扫到真实 secret，任何 evidence、SSE payload 和最终 report 都必须使用脱敏值。
- SQL/命令拼接和错误处理类规则依赖语言上下文，第一版若用正则硬扫会产生大量误报，因此明确延期。
- 测试缺口规则不能对纯文档、纯配置或纯测试变更误报。
- `review.Service` 仍应保持 LLM 未实现的 degraded 状态，避免前端或 README 误导用户。
