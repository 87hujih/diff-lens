package review_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"diff-lens/internal/demo"
	"diff-lens/internal/github"
	"diff-lens/internal/review"
)

func TestAnalyzeDemoEmitsStableEventOrder(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{Demo: true})
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
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventResult,
		review.EventDone,
	})
	assertStepSequence(t, got, []string{
		"fetch_pr",
		"fetch_pr",
		"parse_diff",
		"parse_diff",
		"scan_rules",
		"scan_rules",
		"build_context",
		"build_context",
		"analyze_ai",
		"analyze_ai",
		"result",
		"result",
	})
}

func TestAnalyzeRealEmitsPRMetadataAndDegradedResult(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
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
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
		review.EventStep,
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

	report := got[len(got)-2].Data.(review.Report)
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

	if report.Meta.AICompleted {
		t.Fatalf("result meta ai_completed = true, want false")
	}
	if report.Meta.DegradedReason != "llm_not_configured" {
		t.Fatalf("degraded_reason = %q, want llm_not_configured", report.Meta.DegradedReason)
	}

	done := got[len(got)-1].Data.(review.DonePayload)
	if !done.OK || !done.Degraded {
		t.Fatalf("done = %#v, want ok=true degraded=true", done)
	}
}

func TestAnalyzeRealReturnsBeforeFetchCompletes(t *testing.T) {
	client := &blockingGitHubClient{
		data:    samplePullRequestData(),
		entered: make(chan struct{}),
		release: make(chan struct{}),
	}
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return client
		},
	})

	type result struct {
		events <-chan review.ReviewEvent
		err    error
	}
	returned := make(chan result, 1)
	go func() {
		events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
			PRURL: "https://github.com/openai/example/pull/123",
		})
		returned <- result{events: events, err: err}
	}()

	var got result
	select {
	case got = <-returned:
	case <-time.After(100 * time.Millisecond):
		close(client.release)
		t.Fatal("Analyze did not return before GitHub fetch completed")
	}
	if got.err != nil {
		close(client.release)
		t.Fatalf("Analyze returned error: %v", got.err)
	}

	select {
	case first := <-got.events:
		step := first.Data.(review.StepPayload)
		if first.Type != review.EventStep || step.Step != "fetch_pr" || step.Status != "running" {
			t.Fatalf("first event = %#v, want fetch_pr running", first)
		}
	case <-time.After(100 * time.Millisecond):
		close(client.release)
		t.Fatal("timed out waiting for first event")
	}

	close(client.release)
	collectEvents(t, got.events)
}

func TestAnalyzeRealLLMSuccessProducesCompletedReport(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		AIAnalyzer: fakeAIAnalyzer{
			analysis: review.ReviewAnalysis{Summary: "AI summary"},
		},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	got := collectEvents(t, events)
	report := got[len(got)-2].Data.(review.Report)
	if report.Degraded {
		t.Fatalf("result degraded = true, want false")
	}
	if !report.Meta.AICompleted {
		t.Fatalf("result meta ai_completed = false, want true")
	}
	if report.Summary.Overview != "AI summary" {
		t.Fatalf("overview = %q, want AI summary", report.Summary.Overview)
	}
}

func TestAnalyzeRealLLMRecoverableErrorProducesDegradedResult(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{data: samplePullRequestData()}
		},
		AIAnalyzer: fakeAIAnalyzer{err: fakeAnalyzerError{reason: "llm_output_invalid"}},
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{
		PRURL: "https://github.com/openai/example/pull/123",
	})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	got := collectEvents(t, events)
	report := got[len(got)-2].Data.(review.Report)
	if !report.Degraded {
		t.Fatalf("result degraded = false, want true")
	}
	if report.Meta.DegradedReason != "llm_output_invalid" {
		t.Fatalf("degraded_reason = %q, want llm_output_invalid", report.Meta.DegradedReason)
	}
	done := got[len(got)-1].Data.(review.DonePayload)
	if !done.OK || !done.Degraded {
		t.Fatalf("done = %#v, want ok=true degraded=true", done)
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

func TestAnalyzeRealEmitsGitHubClientErrorEvent(t *testing.T) {
	clientErr := github.ErrGitHubUnauthorized
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return &fakeGitHubClient{err: clientErr}
		},
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
		review.EventError,
		review.EventDone,
	})
	payload := got[1].Data.(review.ErrorPayload)
	if payload.Code != "github_unauthorized" || payload.Stage != "fetch_pr" {
		t.Fatalf("error payload = %#v, want github_unauthorized at fetch_pr", payload)
	}
	done := got[2].Data.(review.DonePayload)
	if done.OK {
		t.Fatalf("done ok = true, want false")
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
	if !strings.Contains(err.Error(), "GitHub client factory 未配置") {
		t.Fatalf("error = %q, want clear missing factory message", err.Error())
	}
}

type fakeGitHubClient struct {
	data github.PullRequestData
	err  error
	refs []github.PRRef
}

type blockingGitHubClient struct {
	data    github.PullRequestData
	entered chan struct{}
	release chan struct{}
}

func (c *blockingGitHubClient) FetchPullRequest(ctx context.Context, ref github.PRRef) (github.PullRequestData, error) {
	close(c.entered)
	select {
	case <-ctx.Done():
		return github.PullRequestData{}, ctx.Err()
	case <-c.release:
		return c.data, nil
	}
}

type fakeAIAnalyzer struct {
	analysis review.ReviewAnalysis
	err      error
}

func (a fakeAIAnalyzer) Analyze(ctx context.Context, input review.ReviewContext) (review.ReviewAnalysis, error) {
	if a.err != nil {
		return review.ReviewAnalysis{}, a.err
	}
	return a.analysis, nil
}

type fakeAnalyzerError struct {
	reason string
}

func (e fakeAnalyzerError) Error() string {
	return e.reason
}

func (e fakeAnalyzerError) DegradedReason() string {
	return e.reason
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

func assertStepSequence(t *testing.T, events []review.ReviewEvent, want []string) {
	t.Helper()

	got := make([]string, 0, len(events))
	for _, event := range events {
		if event.Type != review.EventStep {
			continue
		}
		step := event.Data.(review.StepPayload)
		got = append(got, step.Step)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("step sequence = %v, want %v", got, want)
	}
}
