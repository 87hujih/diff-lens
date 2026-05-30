package review_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"diff-lens/internal/demo"
	"diff-lens/internal/diff"
	"diff-lens/internal/github"
	"diff-lens/internal/review"
	"diff-lens/internal/rules"
)

func TestAnalyzeDemoEmitsStableEventOrder(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{Demo: true})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	var got []review.EventType
	for event := range events {
		got = append(got, event.Type)
	}

	want := []review.EventType{
		review.EventStep,
		review.EventPR,
		review.EventRules,
		review.EventResult,
		review.EventDone,
	}

	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d; events=%v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %q, want %q; events=%v", i, got[i], want[i], got)
		}
	}
}

func TestAnalyzeRealEmitsPRMetadataAndDegradedResult(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		DiffParser:   fakeDiffParser{},
		RulesScanner: fakeRulesScanner{},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	got := collectEvents(t, events)
	assertEventTypes(t, got, []review.EventType{
		review.EventStep,
		review.EventStep,
		review.EventPR,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventRules,
		review.EventResult,
		review.EventDone,
	})

	firstStep := got[0].Data.(review.StepPayload)
	if firstStep.Step != "fetch_pr" || firstStep.Status != "running" {
		t.Fatalf("first step = %#v, want fetch_pr running", firstStep)
	}

	secondStep := got[1].Data.(review.StepPayload)
	if secondStep.Step != "fetch_pr" || secondStep.Status != "completed" {
		t.Fatalf("second step = %#v, want fetch_pr completed", secondStep)
	}

	pr := got[2].Data.(review.PRInfo)
	wantPR := review.PRInfo{
		Title:        "feat: stream real PR metadata",
		Author:       "octocat",
		Repo:         "openai/example",
		Number:       123,
		SourceBranch: "feature/review-service",
		TargetBranch: "main",
		ChangedFiles: 3,
		Additions:    27,
		Deletions:    4,
		Commits:      2,
	}
	if !reflect.DeepEqual(pr, wantPR) {
		t.Fatalf("pr event = %#v, want %#v", pr, wantPR)
	}

	report := got[8].Data.(review.Report)
	if !report.Degraded {
		t.Fatalf("result degraded = false, want true")
	}
	if report.PR != wantPR {
		t.Fatalf("result PR = %#v, want %#v", report.PR, wantPR)
	}
	if len(report.Risks) != 0 {
		t.Fatalf("result risks = %d, want 0", len(report.Risks))
	}
	if len(report.Comments) != 0 {
		t.Fatalf("result comments = %d, want 0", len(report.Comments))
	}

	done := got[9].Data.(review.DonePayload)
	if !done.OK || !done.Degraded {
		t.Fatalf("done = %#v, want ok=true degraded=true", done)
	}
}

func TestAnalyzeRealParsesGitHubFilesScansRulesAndMapsRisks(t *testing.T) {
	parser := &capturingDiffParser{
		analysis: diff.Analysis{
			Stats: diff.FileStats{ChangedFiles: 2, Additions: 12, Deletions: 3},
			Files: []diff.FileDiff{{Filename: "internal/service.go"}},
		},
	}
	scanner := &capturingRulesScanner{
		findings: []rules.Finding{{
			ID:             "security.sensitive-information:internal/service.go:42:abc123",
			RuleID:         "security.sensitive-information",
			Severity:       "high",
			Confidence:     0.91,
			Category:       "security",
			Title:          "Sensitive information added",
			File:           "internal/service.go",
			Line:           42,
			MaskedEvidence: `api_token="<masked>"`,
			Reason:         "Added code appears to include a credential.",
			Suggestion:     "Move secrets to managed storage.",
		}},
	}
	data := samplePullRequestData()
	data.Files = []github.PullRequestFile{
		{
			Filename:  "internal/service.go",
			Status:    "modified",
			Additions: 12,
			Deletions: 3,
			Changes:   15,
			Patch:     "@@ -1 +1 @@\n-old\n+api_token=\"secret\"\n",
		},
		{
			Filename:  "assets/logo.png",
			Status:    "added",
			Additions: 0,
			Deletions: 0,
			Changes:   0,
		},
	}
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: data}
		},
		DiffParser:   parser,
		RulesScanner: scanner,
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	got := collectEvents(t, events)
	assertEventTypes(t, got, []review.EventType{
		review.EventStep,
		review.EventStep,
		review.EventPR,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventRules,
		review.EventResult,
		review.EventDone,
	})

	if len(parser.inputs) != 2 {
		t.Fatalf("parser inputs = %d, want 2", len(parser.inputs))
	}
	wantFirstInput := diff.FileInput{
		Filename:             "internal/service.go",
		Status:               "modified",
		Additions:            12,
		Deletions:            3,
		Changes:              15,
		Patch:                "@@ -1 +1 @@\n-old\n+api_token=\"secret\"\n",
		PatchBinaryOrOmitted: false,
	}
	if !reflect.DeepEqual(parser.inputs[0], wantFirstInput) {
		t.Fatalf("first parser input = %#v, want %#v", parser.inputs[0], wantFirstInput)
	}
	if parser.inputs[1].PatchBinaryOrOmitted {
		t.Fatalf("second parser input PatchBinaryOrOmitted = true, want false because GitHub data cannot express it")
	}

	if !reflect.DeepEqual(scanner.analysis, parser.analysis) {
		t.Fatalf("scanner analysis = %#v, want parser analysis %#v", scanner.analysis, parser.analysis)
	}

	rulesPayload := got[7].Data.(review.RulesPayload)
	wantRisk := review.Risk{
		ID:         "security.sensitive-information:internal/service.go:42:abc123",
		Source:     "rules",
		Severity:   "high",
		Confidence: 0.91,
		Category:   "security",
		Title:      "Sensitive information added",
		File:       "internal/service.go",
		Line:       42,
		Evidence:   `api_token="<masked>"`,
		Reason:     "Added code appears to include a credential.",
		Suggestion: "Move secrets to managed storage.",
	}
	if !reflect.DeepEqual(rulesPayload.Risks, []review.Risk{wantRisk}) {
		t.Fatalf("rules risks = %#v, want %#v", rulesPayload.Risks, []review.Risk{wantRisk})
	}

	report := got[8].Data.(review.Report)
	if !report.Degraded {
		t.Fatalf("report degraded = false, want true")
	}
	if !reflect.DeepEqual(report.Risks, []review.Risk{wantRisk}) {
		t.Fatalf("report risks = %#v, want %#v", report.Risks, []review.Risk{wantRisk})
	}
}

func TestAnalyzeRealReturnsRecoverableParseDiffError(t *testing.T) {
	parseErr := errors.New("parse failed")
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		DiffParser:   fakeDiffParser{err: parseErr},
		RulesScanner: fakeRulesScanner{},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err == nil {
		t.Fatal("Analyze returned nil error, want parse_diff error")
	}
	if events != nil {
		t.Fatalf("events = %v, want nil on parse error", events)
	}

	var analysisErr *review.AnalysisError
	if !errors.As(err, &analysisErr) {
		t.Fatalf("error type = %T, want *review.AnalysisError", err)
	}
	if analysisErr.Stage != "parse_diff" {
		t.Fatalf("stage = %q, want parse_diff", analysisErr.Stage)
	}
	if !analysisErr.Recoverable {
		t.Fatalf("recoverable = false, want true")
	}
	if !errors.Is(err, parseErr) {
		t.Fatalf("wrapped error = %v, want %v", err, parseErr)
	}
}

func TestAnalyzeRealUsesRequestGitHubTokenBeforeDefault(t *testing.T) {
	var gotToken string
	service := review.NewService(review.ServiceOptions{
		DefaultGitHubToken: "default-token",
		GitHubClientFactory: func(token string) review.GitHubClient {
			gotToken = token
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		DiffParser:   fakeDiffParser{},
		RulesScanner: fakeRulesScanner{},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL:       "https://github.com/openai/example/pull/123",
		GitHubToken: "request-token",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	collectEvents(t, events)

	if gotToken != "request-token" {
		t.Fatalf("factory token = %q, want request token", gotToken)
	}
}

func TestAnalyzeRealUsesDefaultGitHubTokenWhenRequestTokenEmpty(t *testing.T) {
	var gotToken string
	service := review.NewService(review.ServiceOptions{
		DefaultGitHubToken: "default-token",
		GitHubClientFactory: func(token string) review.GitHubClient {
			gotToken = token
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		DiffParser:   fakeDiffParser{},
		RulesScanner: fakeRulesScanner{},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	collectEvents(t, events)

	if gotToken != "default-token" {
		t.Fatalf("factory token = %q, want default token", gotToken)
	}
}

func TestAnalyzeRealInvalidPRURLReturnsRecoverableAnalysisError(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			t.Fatalf("GitHub client factory should not be called for invalid PR URL")
			return nil
		},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://example.com/openai/example/pull/123",
	})
	if err == nil {
		t.Fatal("Analyze returned nil error, want invalid PR URL error")
	}
	if events != nil {
		t.Fatalf("events = %v, want nil on invalid PR URL", events)
	}

	var analysisErr *review.AnalysisError
	if !errors.As(err, &analysisErr) {
		t.Fatalf("error type = %T, want *review.AnalysisError", err)
	}
	if analysisErr.Code != "invalid_pr_url" {
		t.Fatalf("code = %q, want invalid_pr_url", analysisErr.Code)
	}
	if analysisErr.Stage != "fetch_pr" {
		t.Fatalf("stage = %q, want fetch_pr", analysisErr.Stage)
	}
	if !analysisErr.Recoverable {
		t.Fatalf("recoverable = false, want true")
	}
}

func TestAnalyzeRealReturnsGitHubClientError(t *testing.T) {
	clientErr := errors.New("github unavailable")
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{err: clientErr}
		},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if !errors.Is(err, clientErr) {
		t.Fatalf("Analyze error = %v, want github client error", err)
	}
	if events != nil {
		t.Fatalf("events = %v, want nil on github client error", events)
	}
}

func TestAnalyzeRealReturnsClearErrorWhenGitHubClientFactoryMissing(t *testing.T) {
	service := review.NewService(review.ServiceOptions{})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err == nil {
		t.Fatal("Analyze returned nil error, want missing factory error")
	}
	if events != nil {
		t.Fatalf("events = %v, want nil on missing factory", events)
	}
	if !strings.Contains(err.Error(), "github client factory is not configured") {
		t.Fatalf("error = %q, want clear missing factory message", err.Error())
	}
}

type fakeGitHubClient struct {
	data github.PullRequestData
	err  error
	refs []github.PRRef
}

type fakeDiffParser struct {
	analysis diff.Analysis
	err      error
}

func (p fakeDiffParser) ParseFiles(files []diff.FileInput) (diff.Analysis, error) {
	if p.err != nil {
		return diff.Analysis{}, p.err
	}
	return p.analysis, nil
}

type capturingDiffParser struct {
	inputs   []diff.FileInput
	analysis diff.Analysis
}

func (p *capturingDiffParser) ParseFiles(files []diff.FileInput) (diff.Analysis, error) {
	p.inputs = append([]diff.FileInput(nil), files...)
	return p.analysis, nil
}

type fakeRulesScanner struct {
	findings []rules.Finding
}

func (s fakeRulesScanner) Scan(analysis diff.Analysis) []rules.Finding {
	return s.findings
}

type capturingRulesScanner struct {
	analysis diff.Analysis
	findings []rules.Finding
}

func (s *capturingRulesScanner) Scan(analysis diff.Analysis) []rules.Finding {
	s.analysis = analysis
	return s.findings
}

func (c *fakeGitHubClient) FetchPullRequest(ctx context.Context, ref github.PRRef) (github.PullRequestData, error) {
	c.refs = append(c.refs, ref)
	if c.err != nil {
		return github.PullRequestData{}, c.err
	}
	return c.data, nil
}

func samplePullRequestData() github.PullRequestData {
	return github.PullRequestData{
		Ref: github.PRRef{
			Owner:  "openai",
			Repo:   "example",
			Number: 123,
		},
		Title:        "feat: stream real PR metadata",
		Author:       "octocat",
		Repo:         "openai/example",
		Number:       123,
		SourceBranch: "feature/review-service",
		TargetBranch: "main",
		ChangedFiles: 3,
		Additions:    27,
		Deletions:    4,
		CommitsCount: 2,
		Files: []github.PullRequestFile{
			{Filename: "internal/review/service.go", Status: "modified", Additions: 20, Deletions: 2, Changes: 22},
			{Filename: "internal/review/service_test.go", Status: "modified", Additions: 6, Deletions: 1, Changes: 7},
			{Filename: "cmd/server/main.go", Status: "modified", Additions: 1, Deletions: 1, Changes: 2},
		},
		Commits: []github.PullRequestCommit{
			{SHA: "abc123", Message: "add real review service mode", AuthorLogin: "octocat"},
			{SHA: "def456", Message: "wire github client factory", AuthorLogin: "octocat"},
		},
	}
}

func collectEvents(t *testing.T, events <-chan review.ReviewEvent) []review.ReviewEvent {
	t.Helper()

	var got []review.ReviewEvent
	for event := range events {
		got = append(got, event)
	}
	return got
}

func assertEventTypes(t *testing.T, events []review.ReviewEvent, want []review.EventType) {
	t.Helper()

	got := make([]review.EventType, 0, len(events))
	for _, event := range events {
		got = append(got, event.Type)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("event types = %v, want %v", got, want)
	}
}
