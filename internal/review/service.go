package review

import (
	"context"
	"errors"
	"fmt"

	"diff-lens/internal/github"
)

// ErrRealAnalysisNotImplemented 标记非 demo PR 分析尚未实现的脚手架边界。
var ErrRealAnalysisNotImplemented = errors.New("real PR analysis is not implemented yet")

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

// ServiceOptions 聚合依赖，便于 service 保持可测试。
type ServiceOptions struct {
	DemoProvider        DemoProvider
	GitHubClientFactory GitHubClientFactory
	DefaultGitHubToken  string
}

// Service 协调 review 分析模式，并向 handler 输出领域事件流。
type Service struct {
	demoProvider        DemoProvider
	githubClientFactory GitHubClientFactory
	defaultGitHubToken  string
}

// NewService 使用注入的 provider 构造应用服务。
func NewService(options ServiceOptions) *Service {
	return &Service{
		demoProvider:        options.DemoProvider,
		githubClientFactory: options.GitHubClientFactory,
		defaultGitHubToken:  options.DefaultGitHubToken,
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

	running := ReviewEvent{
		Type: EventStep,
		Data: StepPayload{Step: "fetch_pr", Status: "running", Message: "正在获取 PR 信息"},
	}

	data, err := client.FetchPullRequest(ctx, ref)
	if err != nil {
		return nil, err
	}

	pr := prInfoFromPullRequest(data)
	report := degradedReport(pr)

	return closedEventStream(
		running,
		ReviewEvent{
			Type: EventStep,
			Data: StepPayload{Step: "fetch_pr", Status: "completed", Message: "已获取 PR 元数据、文件列表和 commits"},
		},
		ReviewEvent{Type: EventPR, Data: pr},
		ReviewEvent{Type: EventResult, Data: report},
		ReviewEvent{Type: EventDone, Data: DonePayload{OK: true, Degraded: true}},
	), nil
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

func closedEventStream(events ...ReviewEvent) <-chan ReviewEvent {
	out := make(chan ReviewEvent, len(events))
	for _, event := range events {
		out <- event
	}
	close(out)
	return out
}
