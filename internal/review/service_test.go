package review_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

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

	report := got[3].Data.(review.Report)
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

	done := got[4].Data.(review.DonePayload)
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
