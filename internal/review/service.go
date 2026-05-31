package review

import (
	"context"
	"errors"
	"fmt"

	"diff-lens/internal/diff"
	"diff-lens/internal/github"
	"diff-lens/internal/rules"
)

// AnalysisError 携带 service 阶段可被 handler 映射的结构化错误信息。
type AnalysisError struct {
	Code        string
	Message     string
	Recoverable bool
	Stage       string
	Err         error
}

func (e *AnalysisError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Code
}

func (e *AnalysisError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// DemoProvider 输出本地演示和冒烟测试使用的确定性事件流。
type DemoProvider interface {
	Stream(ctx context.Context) (<-chan ReviewEvent, error)
}

// GitHubClient 是 review service 真实模式需要的 GitHub 数据获取接口。
type GitHubClient interface {
	FetchPullRequest(ctx context.Context, ref github.PRRef) (github.PullRequestData, error)
}

// GitHubClientFactory 允许 service 按请求 token 创建 GitHub client。
type GitHubClientFactory func(token string) GitHubClient

// DiffParser converts provider-neutral changed files into structured diff data.
type DiffParser interface {
	ParseFiles(files []diff.FileInput) (diff.Analysis, error)
}

// RuleScanner runs deterministic checks over parsed diff data.
type RuleScanner interface {
	Scan(analysis diff.Analysis) []rules.Finding
}

// ReviewContextBuilder creates the bounded model context from PR data and rule risks.
type ReviewContextBuilder interface {
	Build(pr github.PullRequestData, ruleRisks []Risk) ReviewContext
}

// ReportGenerator creates final and degraded review reports.
type ReportGenerator interface {
	Normalize(pr PRInfo, ruleRisks []Risk, ai ReviewAnalysis, ctx ReviewContext, options ReportNormalizerOptions) Report
	Degraded(pr PRInfo, ruleRisks []Risk, ctx ReviewContext, reason string) Report
}

// ServiceOptions 聚合依赖，便于 service 保持可测试。
type ServiceOptions struct {
	DemoProvider        DemoProvider
	GitHubClientFactory GitHubClientFactory
	DefaultGitHubToken  string
	DiffParser          DiffParser
	RuleScanner         RuleScanner
	RulesScanner        RuleScanner
	ContextBuilder      ReviewContextBuilder
	AIAnalyzer          AIAnalyzer
	ReportGenerator     ReportGenerator
}

// Service 协调 review 分析模式，并向 handler 输出领域事件流。
type Service struct {
	demoProvider        DemoProvider
	githubClientFactory GitHubClientFactory
	defaultGitHubToken  string
	diffParser          DiffParser
	ruleScanner         RuleScanner
	contextBuilder      ReviewContextBuilder
	aiAnalyzer          AIAnalyzer
	reportGenerator     ReportGenerator
}

// NewService 使用注入的 provider 构造应用服务。
func NewService(options ServiceOptions) *Service {
	parser := options.DiffParser
	if parser == nil {
		parser = diff.NewParser()
	}
	scanner := options.RuleScanner
	if scanner == nil {
		scanner = options.RulesScanner
	}
	if scanner == nil {
		scanner = rules.NewScanner()
	}
	builder := options.ContextBuilder
	if builder == nil {
		builder = NewContextBuilder(ContextBuilderOptions{})
	}
	reportGenerator := options.ReportGenerator
	if reportGenerator == nil {
		reportGenerator = NewReportNormalizer()
	}

	return &Service{
		demoProvider:        options.DemoProvider,
		githubClientFactory: options.GitHubClientFactory,
		defaultGitHubToken:  options.DefaultGitHubToken,
		diffParser:          parser,
		ruleScanner:         scanner,
		contextBuilder:      builder,
		aiAnalyzer:          options.AIAnalyzer,
		reportGenerator:     reportGenerator,
	}
}

// Analyze 将请求分派给 demo provider 或真实 PR 获取链路。
func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (<-chan ReviewEvent, error) {
	if req.Demo {
		if s.demoProvider == nil {
			return nil, errors.New("demo provider is not configured")
		}
		return s.demoProvider.Stream(ctx)
	}

	return s.analyzeReal(ctx, req)
}

func (s *Service) analyzeReal(ctx context.Context, req AnalyzeRequest) (<-chan ReviewEvent, error) {
	ref, err := github.ParsePRURL(req.PRURL)
	if err != nil {
		return nil, &AnalysisError{
			Code:        "invalid_pr_url",
			Message:     "invalid GitHub pull request URL",
			Recoverable: true,
			Stage:       "fetch_pr",
			Err:         err,
		}
	}

	if s.githubClientFactory == nil {
		return nil, &AnalysisError{
			Code:    "github_client_not_configured",
			Message: "github client factory is not configured",
			Stage:   "fetch_pr",
		}
	}

	token := req.GitHubToken
	if token == "" {
		token = s.defaultGitHubToken
	}

	client := s.githubClientFactory(token)
	if client == nil {
		return nil, &AnalysisError{
			Code:    "github_client_not_configured",
			Message: "github client factory returned nil client",
			Stage:   "fetch_pr",
		}
	}

	out := make(chan ReviewEvent)
	go s.runRealPipeline(ctx, out, client, ref)
	return out, nil
}

func (s *Service) runRealPipeline(ctx context.Context, out chan<- ReviewEvent, client GitHubClient, ref github.PRRef) {
	defer close(out)

	send := func(event ReviewEvent) bool {
		select {
		case <-ctx.Done():
			return false
		case out <- event:
			return true
		}
	}

	if !send(stepEvent("fetch_pr", "running", "正在获取 PR 信息")) {
		return
	}
	data, err := client.FetchPullRequest(ctx, ref)
	if err != nil {
		sendErrorAndDone(send, errorPayloadForStage(err, "fetch_pr"))
		return
	}
	if !send(stepEvent("fetch_pr", "completed", "已获取 PR 元数据、文件列表和 commits")) {
		return
	}

	pr := prInfoFromPullRequest(data)
	if !send(ReviewEvent{Type: EventPR, Data: pr}) {
		return
	}

	if !send(stepEvent("parse_diff", "running", "正在解析 PR diff")) {
		return
	}
	analysis, err := s.diffParser.ParseFiles(diffInputsFromPullRequest(data))
	if err != nil {
		sendErrorAndDone(send, ErrorPayload{
			Code:        "diff_parse_failed",
			Message:     "Diff parsing failed before a report could be produced.",
			Recoverable: true,
			Stage:       "parse_diff",
		})
		return
	}
	if !send(stepEvent("parse_diff", "completed", "已解析 PR diff")) {
		return
	}

	if !send(stepEvent("scan_rules", "running", "正在执行确定性规则扫描")) {
		return
	}
	ruleRisks := risksFromFindings(s.ruleScanner.Scan(analysis))
	if !send(stepEvent("scan_rules", "completed", fmt.Sprintf("规则扫描完成，发现 %d 条风险", len(ruleRisks)))) {
		return
	}
	if !send(ReviewEvent{Type: EventRules, Data: RulesPayload{Risks: ruleRisks}}) {
		return
	}

	if !send(stepEvent("build_context", "running", "正在构建受控 AI 上下文")) {
		return
	}
	reviewContext := s.contextBuilder.Build(data, ruleRisks)
	if !send(stepEvent("build_context", "completed", "已构建裁剪后的 ReviewContext")) {
		return
	}

	if !send(stepEvent("analyze_ai", "running", "正在调用 AI 分析")) {
		return
	}
	if s.aiAnalyzer == nil {
		report := s.reportGenerator.Degraded(pr, ruleRisks, reviewContext, "llm_not_configured")
		if !send(stepEvent("analyze_ai", "failed", "LLM 未配置，返回规则扫描降级报告")) {
			return
		}
		if !send(stepEvent("result", "completed", "已生成规则扫描降级报告")) {
			return
		}
		send(ReviewEvent{Type: EventResult, Data: report})
		send(ReviewEvent{Type: EventDone, Data: DonePayload{OK: true, Degraded: true}})
		return
	}

	analysisResult, err := s.aiAnalyzer.Analyze(ctx, reviewContext)
	if err != nil {
		reason := degradedReasonFromAnalyzerError(err)
		report := s.reportGenerator.Degraded(pr, ruleRisks, reviewContext, reason)
		if !send(stepEvent("analyze_ai", "failed", "AI 分析失败，返回规则扫描降级报告")) {
			return
		}
		if !send(stepEvent("result", "completed", "已生成规则扫描降级报告")) {
			return
		}
		send(ReviewEvent{Type: EventResult, Data: report})
		send(ReviewEvent{Type: EventDone, Data: DonePayload{OK: true, Degraded: true}})
		return
	}
	if !send(stepEvent("analyze_ai", "completed", "AI 分析完成")) {
		return
	}
	report := s.reportGenerator.Normalize(pr, ruleRisks, analysisResult, reviewContext, ReportNormalizerOptions{
		AICompleted:    true,
		RulesCompleted: true,
	})
	if !send(stepEvent("result", "completed", "已生成最终 Review 报告")) {
		return
	}
	if !send(ReviewEvent{Type: EventResult, Data: report}) {
		return
	}
	send(ReviewEvent{Type: EventDone, Data: DonePayload{OK: true, Degraded: report.Degraded}})
}

func prInfoFromPullRequest(data github.PullRequestData) PRInfo {
	repo := data.Repo
	if repo == "" && data.Ref.Owner != "" && data.Ref.Repo != "" {
		repo = data.Ref.Owner + "/" + data.Ref.Repo
	}

	number := data.Number
	if number == 0 {
		number = data.Ref.Number
	}

	changedFiles := data.ChangedFiles
	if changedFiles == 0 {
		changedFiles = len(data.Files)
	}

	commits := data.CommitsCount
	if commits == 0 {
		commits = len(data.Commits)
	}

	return PRInfo{
		Title:        data.Title,
		Author:       data.Author,
		Repo:         repo,
		Number:       number,
		SourceBranch: data.SourceBranch,
		TargetBranch: data.TargetBranch,
		ChangedFiles: changedFiles,
		Additions:    data.Additions,
		Deletions:    data.Deletions,
		Commits:      commits,
	}
}

func degradedReport(pr PRInfo) Report {
	return Report{
		PR: pr,
		Summary: Summary{
			RiskLevel: "low",
			Overview:  "已获取 PR 元数据、文件列表和 commits；diff 解析、规则扫描和 AI 分析尚未实现。本报告是降级结果，不代表完整 review 结论。",
			KeyChanges: []string{
				fmt.Sprintf("变更 %d 个文件，新增 %d 行、删除 %d 行。", pr.ChangedFiles, pr.Additions, pr.Deletions),
				fmt.Sprintf("包含 %d 个 commit，源分支 %q 合入目标分支 %q。", pr.Commits, pr.SourceBranch, pr.TargetBranch),
			},
			ReviewFocus: []string{
				"后续阶段会重点分析测试覆盖、配置变更、危险操作和敏感信息风险。",
				"在完整 diff 解析与规则扫描实现前，请人工复核具体代码变更。",
			},
		},
		Risks:    []Risk{},
		Comments: []SuggestedComment{},
		Degraded: true,
	}
}

func stepEvent(step string, status string, message string) ReviewEvent {
	return ReviewEvent{
		Type: EventStep,
		Data: StepPayload{Step: step, Status: status, Message: message},
	}
}

func sendErrorAndDone(send func(ReviewEvent) bool, payload ErrorPayload) {
	if !send(ReviewEvent{Type: EventError, Data: payload}) {
		return
	}
	send(ReviewEvent{Type: EventDone, Data: DonePayload{OK: false}})
}

func errorPayloadForStage(err error, stage string) ErrorPayload {
	switch {
	case errors.Is(err, github.ErrPRNotFound):
		return ErrorPayload{
			Code:        "github_pr_not_found",
			Message:     "GitHub pull request was not found; check that the URL points to an existing PR.",
			Recoverable: true,
			Stage:       stage,
		}
	case errors.Is(err, github.ErrGitHubUnauthorized):
		return ErrorPayload{
			Code:        "github_unauthorized",
			Message:     "GitHub authentication failed; configure a valid token and retry.",
			Recoverable: true,
			Stage:       stage,
		}
	case errors.Is(err, github.ErrGitHubRateLimited):
		return ErrorPayload{
			Code:        "github_rate_limited",
			Message:     "GitHub API rate limit was reached; configure a token or retry later.",
			Recoverable: true,
			Stage:       stage,
		}
	case errors.Is(err, github.ErrGitHubRequestFailed):
		return ErrorPayload{
			Code:        "github_request_failed",
			Message:     "GitHub request failed; check the network connection and retry.",
			Recoverable: true,
			Stage:       stage,
		}
	case errors.Is(err, github.ErrGitHubResponseInvalid):
		return ErrorPayload{
			Code:        "github_response_invalid",
			Message:     "GitHub returned an invalid response; retry after the upstream response is healthy.",
			Recoverable: false,
			Stage:       stage,
		}
	default:
		return ErrorPayload{
			Code:        "analysis_failed",
			Message:     "Analysis failed before a report could be produced.",
			Recoverable: false,
			Stage:       stage,
		}
	}
}

func diffInputsFromPullRequest(data github.PullRequestData) []diff.FileInput {
	inputs := make([]diff.FileInput, 0, len(data.Files))
	for _, file := range data.Files {
		inputs = append(inputs, diff.FileInput{
			Filename:  file.Filename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Changes:   file.Changes,
			Patch:     file.Patch,
			// github.PullRequestFile does not currently expose whether a
			// missing patch was binary or omitted, so keep this false.
			PatchBinaryOrOmitted: false,
		})
	}
	return inputs
}

func risksFromFindings(findings []rules.Finding) []Risk {
	if len(findings) == 0 {
		return []Risk{}
	}
	risks := make([]Risk, 0, len(findings))
	for _, finding := range findings {
		risk := Risk{
			ID:           finding.ID,
			Source:       "rule",
			Severity:     finding.Severity,
			Confidence:   finding.Confidence,
			Category:     finding.Category,
			Title:        finding.Title,
			File:         finding.File,
			Line:         finding.Line,
			RuleID:       finding.RuleID,
			EvidenceRefs: []string{finding.ID},
			Evidence:     finding.MaskedEvidence,
			Reason:       finding.Reason,
			Suggestion:   finding.Suggestion,
		}
		risks = append(risks, risk)
	}
	return risks
}

func degradedReasonFromAnalyzerError(err error) string {
	var classified interface {
		DegradedReason() string
	}
	if errors.As(err, &classified) {
		if reason := classified.DegradedReason(); reason != "" {
			return reason
		}
	}
	return "llm_failed"
}

func closedEventStream(events ...ReviewEvent) <-chan ReviewEvent {
	out := make(chan ReviewEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out
}
