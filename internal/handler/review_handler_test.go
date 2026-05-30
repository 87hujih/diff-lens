package handler_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"diff-lens/internal/demo"
	"diff-lens/internal/diff"
	"diff-lens/internal/github"
	"diff-lens/internal/handler"
	"diff-lens/internal/review"
	"diff-lens/internal/rules"
)

func TestAnalyzeStreamDemoReturnsSSEEvents(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})
	router := handler.NewRouter(service)

	req := httptest.NewRequest(http.MethodPost, "/api/reviews/analyze/stream", strings.NewReader(`{"demo":true}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, expected := range []string{"event: step", "event: pr", "event: rules", "event: result", "event: done"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}
}

func TestAnalyzeStreamInvalidJSONReturnsHTTP400JSON(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})
	router := handler.NewRouter(service)

	recorder := postAnalyzeStream(t, router, `{`)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "event:") {
		t.Fatalf("body contains SSE event for invalid JSON:\n%s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", recorder.Header().Get("Content-Type"))
	}
}

func TestAnalyzeStreamInvalidPRURLReturnsStructuredSSEError(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			t.Fatalf("GitHub client factory should not be called for invalid PR URL")
			return nil
		},
	})
	router := handler.NewRouter(service)

	recorder := postAnalyzeStream(t, router, `{"pr_url":"https://example.com/openai/example/pull/123"}`)

	assertSSEError(t, recorder, "invalid_pr_url", "fetch_pr", true)
	assertSSEDoneFailed(t, recorder.Body.String())
}

func TestAnalyzeStreamMapsGitHubErrorsToStructuredSSEErrors(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantCode    string
		wantStage   string
		recoverable bool
	}{
		{
			name:        "PR not found",
			err:         fmt.Errorf("fetch pull request: %w", github.ErrPRNotFound),
			wantCode:    "github_pr_not_found",
			wantStage:   "fetch_pr",
			recoverable: true,
		},
		{
			name:        "unauthorized",
			err:         fmt.Errorf("fetch pull request: %w", github.ErrGitHubUnauthorized),
			wantCode:    "github_unauthorized",
			wantStage:   "fetch_pr",
			recoverable: true,
		},
		{
			name:        "rate limited",
			err:         fmt.Errorf("fetch pull request: %w", github.ErrGitHubRateLimited),
			wantCode:    "github_rate_limited",
			wantStage:   "fetch_pr",
			recoverable: true,
		},
		{
			name:        "request failed",
			err:         fmt.Errorf("fetch pull request: %w", github.ErrGitHubRequestFailed),
			wantCode:    "github_request_failed",
			wantStage:   "fetch_pr",
			recoverable: true,
		},
		{
			name:        "response invalid",
			err:         fmt.Errorf("fetch pull request: %w", github.ErrGitHubResponseInvalid),
			wantCode:    "github_response_invalid",
			wantStage:   "fetch_pr",
			recoverable: false,
		},
		{
			name:        "unknown",
			err:         errors.New("unexpected analysis failure"),
			wantCode:    "analysis_failed",
			wantStage:   "",
			recoverable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := review.NewService(review.ServiceOptions{
				GitHubClientFactory: func(token string) review.GitHubClient {
					return fakeGitHubClient{err: tt.err}
				},
			})
			router := handler.NewRouter(service)

			recorder := postAnalyzeStream(t, router, `{"pr_url":"https://github.com/openai/example/pull/123"}`)

			assertSSEError(t, recorder, tt.wantCode, tt.wantStage, tt.recoverable)
			assertSSEDoneFailed(t, recorder.Body.String())
		})
	}
}

func TestAnalyzeStreamErrorDoesNotExposeGitHubToken(t *testing.T) {
	const token = "test-secret-token"
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(gotToken string) review.GitHubClient {
			if gotToken != token {
				t.Fatalf("factory token = %q, want request token", gotToken)
			}
			return fakeGitHubClient{
				err: fmt.Errorf("request failed with token %s: %w", token, github.ErrGitHubRequestFailed),
			}
		},
	})
	router := handler.NewRouter(service)

	recorder := postAnalyzeStream(t, router, `{"pr_url":"https://github.com/openai/example/pull/123","github_token":"`+token+`"}`)

	assertSSEError(t, recorder, "github_request_failed", "fetch_pr", true)
	if strings.Contains(recorder.Body.String(), token) {
		t.Fatalf("SSE error body exposed token %q:\n%s", token, recorder.Body.String())
	}
}

func TestAnalyzeStreamRealModeSuccessReturnsPRResultAndDone(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		GitHubClientFactory: func(token string) review.GitHubClient {
			return fakeGitHubClient{data: samplePullRequestData()}
		},
		DiffParser:   diff.NewParser(),
		RulesScanner: rules.NewScanner(),
	})
	router := handler.NewRouter(service)

	recorder := postAnalyzeStream(t, router, `{"pr_url":"https://github.com/openai/example/pull/123"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, expected := range []string{"event: pr", "event: rules", "event: result", "event: done", `"degraded":true`, `"ok":true`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}
}

type fakeGitHubClient struct {
	data github.PullRequestData
	err  error
}

func (c fakeGitHubClient) FetchPullRequest(ctx context.Context, ref github.PRRef) (github.PullRequestData, error) {
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
	}
}

type analyzeRouter interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func postAnalyzeStream(t *testing.T, router analyzeRouter, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/reviews/analyze/stream", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)
	return recorder
}

func assertSSEError(t *testing.T, recorder *httptest.ResponseRecorder, code string, stage string, recoverable bool) {
	t.Helper()

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", recorder.Header().Get("Content-Type"))
	}

	body := recorder.Body.String()
	for _, expected := range []string{
		"event: error",
		`"code":"` + code + `"`,
		`"recoverable":` + fmt.Sprint(recoverable),
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}

	if stage != "" && !strings.Contains(body, `"stage":"`+stage+`"`) {
		t.Fatalf("body missing stage %q:\n%s", stage, body)
	}
	if stage == "" && strings.Contains(body, `"stage":`) {
		t.Fatalf("body contains unexpected stage:\n%s", body)
	}
}

func assertSSEDoneFailed(t *testing.T, body string) {
	t.Helper()

	for _, expected := range []string{"event: done", `"ok":false`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}
}
