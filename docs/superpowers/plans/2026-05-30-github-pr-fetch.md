# GitHub PR 获取实现计划

> **给 agentic workers：** 必须使用 `superpowers:subagent-driven-development`（如果可用）或 `superpowers:executing-plans` 来执行本计划。任务使用 checkbox（`- [ ]`）格式跟踪进度。

**目标：** 实现真实分析链路的第一个后端增量：获取 GitHub PR 元数据、变更文件和 commit 列表，并通过现有 SSE 接口流式返回标准化 PR 信息。

**架构：** `internal/github` 负责 GitHub REST 传输、URL 解析、API DTO、分页、Token 处理和 API 错误归一化。`internal/review.Service` 通过注入的 GitHub client factory 负责编排真实模式并产出事件；`internal/handler` 仍只做 HTTP/SSE 协议适配。本阶段刻意不实现 diff 解析、规则扫描、上下文构建和 LLM 分析；真实模式在成功获取 PR 数据后返回明确的 degraded 最终报告。

**技术栈：** Go 标准库 `net/http`、基于 `httptest` 的 Go 单元测试、现有 Gin handler 边界。

---

## 背景

当前仓库已经完成架构脚手架：

- `internal/review/service.go` 会分发 demo 请求，非 demo 请求目前返回 `ErrRealAnalysisNotImplemented`。
- `internal/github/client.go` 已支持 GitHub PR URL 解析，但尚未调用 GitHub API。
- `internal/diff`、`internal/rules`、`internal/llm` 仍是占位实现。
- `internal/handler/review_handler.go` 已能把 `ReviewEvent` 作为 SSE 流式返回。
- 前端已经能消费 `step`、`pr`、`rules`、`result`、`error` 和 `done` 事件。

下一步最有价值的模块是 GitHub PR 获取，因为后续 diff 解析、规则扫描、上下文构建和 LLM 分析都依赖真实 PR 元数据和文件 patch。

## 需求拆分

### 功能需求

- 接受标准 GitHub PR URL：`https://github.com/{owner}/{repo}/pull/{number}`。
- 通过 `GET /repos/{owner}/{repo}/pulls/{number}` 获取 PR 元数据。
- 通过 `GET /repos/{owner}/{repo}/pulls/{number}/files` 获取变更文件。
- 通过 `GET /repos/{owner}/{repo}/pulls/{number}/commits` 获取 commit 列表。
- 支持 files 和 commits 的 GitHub 分页。
- 将获取到的数据归一化为 `internal/github` 包内的数据模型，供 `review.Service` 转换成 `review.PRInfo`。
- 真实模式下依次发送：
  - `fetch_pr` 阶段的 `step` running/completed 事件
  - 携带真实 PR 元数据的 `pr` 事件
  - `degraded: true` 的 `result` 事件，说明 diff/rules/AI 分析仍待后续模块实现
  - `ok: true` 且 `degraded: true` 的 `done` 事件
- 当请求体包含 `github_token` 时，优先使用请求级 Token。
- 请求级 Token 为空时，回退使用配置中的 `GITHUB_TOKEN`。
- 不在日志、错误消息或响应中暴露 Token 值。

### 错误处理需求

- 非法 PR URL 返回 SSE `error`，code 为 `invalid_pr_url`，stage 为 `fetch_pr`，`recoverable: true`。
- GitHub `401` 或 `403` 返回明确的认证失败或限流类错误。
- GitHub `404` 返回 `github_pr_not_found`。
- 网络失败返回 `github_request_failed`。
- GitHub JSON 响应格式异常返回 `github_response_invalid`。
- 错误流最后仍必须发送 `done`，且 `ok: false`。

### 本计划不做

- 不实现 diff 解析，除了保存原始文件 patch 字符串。
- 不实现规则扫描逻辑。
- 不调用 LLM。
- 不实现 GitHub App 安装鉴权、OAuth 或自动 PR 评论。
- 不持久化 Token 或获取到的 PR 数据。

### 验收标准

- demo 模式行为保持不变。
- 合法公开 PR URL 不再返回 `real_analysis_not_implemented`。
- 合法公开 PR URL 会通过 `pr` 事件流式返回真实 PR 元数据。
- 真实模式最终报告必须明确标记 degraded，不能假装完整 Review 分析已经完成。
- 单元测试覆盖 GitHub client 成功路径、分页、Token header 和错误映射。
- Service 测试覆盖真实模式事件顺序和非法 URL 处理。
- Handler 测试覆盖 SSE 错误格式。
- `go test ./...` 通过。

## 文件分工

- 修改：`internal/github/client.go`
  - 增加 GitHub REST client、请求构造、分页和 JSON 解码。
- 新建：`internal/github/types.go`
  - 定义 `PullRequestData`、`PullRequestFile`、`PullRequestCommit`、`ClientOptions` 和 API 错误类型。
- 修改：`internal/github/client_test.go`
  - 使用 `httptest.Server` 覆盖成功路径、分页、认证 header 和 API 错误。
- 修改：`internal/review/types.go`
  - 如现有类型不足，只增加 service/handler 需要的最小 typed error 或 degraded summary 字段。
- 修改：`internal/review/service.go`
  - 增加 GitHub client factory 依赖和真实模式编排。
- 修改：`internal/review/service_test.go`
  - 使用 fake GitHub client 测试真实模式事件顺序和 degraded result。
- 修改：`internal/handler/review_handler.go`
  - 将 typed service error 映射为结构化 SSE `error` payload。
- 修改：`internal/handler/review_handler_test.go`
  - 验证非法 PR URL 和上游拉取失败都会返回 `error` 加 `done`。
- 修改：`cmd/server/main.go`
  - 将配置中的 GitHub Token 接入真实 service client factory。
- 修改：`README.md`
  - 更新当前进度和真实 PR 手动验证说明。

## 任务 1：GitHub 数据契约

**文件：**
- 新建：`internal/github/types.go`
- 修改：`internal/github/client.go`
- 修改：`internal/github/client_test.go`

- [ ] 增加测试，描述 review pipeline 需要的标准化 GitHub 数据字段。
- [ ] 定义 `PullRequestData`，包含 PR 元数据、变更文件、commit 列表和原始 patch。
- [ ] 定义 `PullRequestFile`，包含 filename、status、additions、deletions、changes 和可选 patch。
- [ ] 定义 `PullRequestCommit`，包含 SHA、message、作者 login/name，以及可用时的 timestamp。
- [ ] 定义 typed errors：非法 URL、未找到、认证/限流、请求失败、响应格式异常。
- [ ] 保持 `ParsePRURL` 行为兼容现有测试。
- [ ] 运行 `go test ./internal/github`，确认现有测试仍通过。

## 任务 2：GitHub REST Client

**文件：**
- 修改：`internal/github/client.go`
- 修改：`internal/github/client_test.go`

- [ ] 写一个失败测试，验证 `FetchPullRequest` 会调用 PR、files 和 commits 三类 endpoint。
- [ ] 写一个失败测试，验证配置 Token 会发送 `Authorization: Bearer <token>`。
- [ ] 写一个失败测试，验证 files/commits 分页会跟随 GitHub `Link` header 或等价的下一页检测。
- [ ] 为 `404`、`401/403`、畸形 JSON 和网络失败写失败测试。
- [ ] 增加 `ClientOptions` 或等价测试 seam，用于注入 base API URL 和 `*http.Client`。
- [ ] 使用 Go 标准库 `net/http` 实现 `FetchPullRequest(ctx, ref)`。
- [ ] 只解码产品需要的 GitHub 字段，忽略无关 API 字段。
- [ ] 即使 GitHub 因二进制文件或超大文件省略 `patch`，也保留文件记录。
- [ ] 运行 `go test ./internal/github`，确认 GitHub client 测试全部通过。

## 任务 3：Review Service 真实模式

**文件：**
- 修改：`internal/review/service.go`
- 修改：`internal/review/service_test.go`
- 修改：`cmd/server/main.go`

- [ ] 给 `review.ServiceOptions` 增加 GitHub client factory 依赖。
- [ ] 使用 fake GitHub client 编写 service 测试，避免测试发起真实网络请求。
- [ ] 写一个失败测试，验证非 demo 分析对合法 PR 请求会发送 `step`、`pr`、`result` 和 `done`。
- [ ] 写一个失败测试，验证请求级 Token 优先于配置默认 Token。
- [ ] 写一个失败测试，验证非法 PR URL 会产生 typed recoverable analysis error。
- [ ] 实现真实模式编排：
  - 校验并解析 PR URL
  - 发送 `fetch_pr` running step
  - 获取 GitHub PR 数据
  - 发送 `fetch_pr` completed step
  - 发送标准化 `pr`
  - 发送 degraded `result`
  - 发送 degraded `done`
- [ ] 编写 degraded summary 文案，明确说明当前只完成 PR 元数据获取，后续分析阶段尚未实现。
- [ ] 在 `cmd/server/main.go` 中将配置的 `GITHUB_TOKEN` 传给 GitHub client factory。
- [ ] 运行 `go test ./internal/review ./cmd/server`（如适用），再运行 `go test ./...`。

## 任务 4：Handler 错误映射

**文件：**
- 修改：`internal/handler/review_handler.go`
- 修改：`internal/handler/review_handler_test.go`

- [ ] 增加 handler 测试，覆盖非法 PR URL 和 GitHub 获取失败时返回 SSE `error` 事件。
- [ ] 确保 typed analysis error 会填充 `ErrorPayload.Code`、`Stage`、`Message` 和 `Recoverable`。
- [ ] 保持请求体 JSON 格式错误为 HTTP `400` JSON 响应，因为此时 SSE 流尚未建立。
- [ ] 确保请求校验后的 service 错误都通过 SSE `error` 加 `done` 返回。
- [ ] 运行 `go test ./internal/handler`。

## 任务 5：README 与手动验证

**文件：**
- 修改：`README.md`

- [ ] 更新进度表：GitHub PR 获取从“预留接口与数据结构”改为“已完成第一阶段真实获取”。
- [ ] 说明真实模式当前会获取 PR metadata/files/commits，并在 diff/rules/LLM 实现前返回 degraded 报告。
- [ ] 增加公开 PR URL 的手动验证 curl 示例。
- [ ] 说明可选 `GITHUB_TOKEN` 和请求级 `github_token` 的优先级。
- [ ] 不要宣称完整 AI Review 已可用。

## 任务 6：最终验证

**文件：**
- 不新增源代码改动，只执行前面任务产生的验证。

- [ ] 运行 `go test ./...`。
- [ ] 运行 `node scripts/check-pr-quality.test.mjs`。
- [ ] 可选：用 `go run ./cmd/server` 启动服务。
- [ ] 验证 demo 模式仍可用：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"demo\":true}"`
- [ ] 验证真实模式能到达 degraded 报告：
  - `curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"`
- [ ] 确认真实模式输出包含 `event: pr`、`event: result` 和 `event: done`。
- [ ] 确认非法 PR URL 输出包含 `event: error` 和 code `invalid_pr_url`。

## 建议提交拆分

- [ ] `feat: add github pull request data contracts`
- [ ] `feat: fetch github pull request metadata`
- [ ] `feat: stream real pull request metadata`
- [ ] `fix: map review analysis errors to sse payloads`
- [ ] `docs: document github fetch increment`

## 风险与约束

- GitHub API rate limit 可能导致无 Token 的手动验证不稳定；自动测试必须使用 `httptest.Server`。
- GitHub 可能对二进制文件或过大文件省略 `patch`；后续模块必须能容忍缺失 patch。
- 小 PR 测试容易漏掉分页问题；现在就要加入多页 fixture，确保后续 diff/rules 能看到完整数据。
- Token 优先级必须明确，避免请求已经提供 Token 时仍意外使用旧的环境变量 Token。
- degraded 报告必须诚实。本阶段只证明真实数据访问能力，不暗示 AI Review 已完成。
