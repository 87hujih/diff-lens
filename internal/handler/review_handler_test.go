package handler_test

import (
	"context"
	"encoding/json"
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

func TestAnalyzeStreamDemoReturnsStructuredSSEContract(t *testing.T) {
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

	messages := parseSSEMessages(t, recorder.Body.String())
	for _, event := range []string{"step", "pr", "rules", "result", "done"} {
		if eventIndex(messages, event) == -1 {
			t.Fatalf("missing event %q in messages: %#v", event, messages)
		}
	}

	rulesIndex := eventIndex(messages, "rules")
	resultIndex := eventIndex(messages, "result")
	if rulesIndex >= resultIndex {
		t.Fatalf("rules event index = %d, want before result index %d", rulesIndex, resultIndex)
	}

	assertStepPayloadsMatchFrontendContract(t, eventDatas(t, messages, "step"))

	var pr review.PRInfo
	decodeEventData(t, eventData(t, messages, "pr"), &pr)
	assertPRMatchesFrontendContract(t, pr)

	var rules review.RulesPayload
	decodeEventData(t, eventData(t, messages, "rules"), &rules)
	assertRulesPayloadMatchesFrontendContract(t, rules)

	var done review.DonePayload
	decodeEventData(t, eventData(t, messages, "done"), &done)
	if !done.OK {
		t.Fatalf("done.ok = false, want true")
	}
	assertDegradedAbsentOrFalse(t, eventData(t, messages, "done"))

	var report review.Report
	resultData := eventData(t, messages, "result")
	decodeEventData(t, resultData, &report)
	assertDegradedAbsentOrFalse(t, resultData)
	assertResultMetaHasFrontendFields(t, resultData)
	assertReportMatchesFrontendContract(t, report)
	assertRiskSources(t, report.Risks, []string{"rule", "ai", "merged"})
}

type sseMessage struct {
	Event string
	Data  json.RawMessage
}

func parseSSEMessages(t *testing.T, body string) []sseMessage {
	t.Helper()

	normalized := strings.ReplaceAll(body, "\r\n", "\n")
	blocks := strings.Split(strings.TrimSpace(normalized), "\n\n")
	messages := make([]sseMessage, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(block) == "" {
			continue
		}

		var eventName string
		var dataLines []string
		for _, line := range strings.Split(block, "\n") {
			switch {
			case strings.HasPrefix(line, "event:"):
				eventName = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			case strings.HasPrefix(line, "data:"):
				dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}

		if eventName == "" {
			t.Fatalf("SSE message missing event line: %q", block)
		}
		if len(dataLines) == 0 {
			t.Fatalf("SSE message %q missing data line: %q", eventName, block)
		}

		data := json.RawMessage(strings.Join(dataLines, "\n"))
		if !json.Valid(data) {
			t.Fatalf("SSE message %q data is not valid JSON: %s", eventName, data)
		}
		messages = append(messages, sseMessage{Event: eventName, Data: data})
	}

	if len(messages) == 0 {
		t.Fatal("no SSE messages parsed")
	}
	return messages
}

func eventIndex(messages []sseMessage, event string) int {
	for i, message := range messages {
		if message.Event == event {
			return i
		}
	}
	return -1
}

func eventData(t *testing.T, messages []sseMessage, event string) json.RawMessage {
	t.Helper()

	for _, message := range messages {
		if message.Event == event {
			return message.Data
		}
	}
	t.Fatalf("missing event %q", event)
	return nil
}

func eventDatas(t *testing.T, messages []sseMessage, event string) []json.RawMessage {
	t.Helper()

	var data []json.RawMessage
	for _, message := range messages {
		if message.Event == event {
			data = append(data, message.Data)
		}
	}
	if len(data) == 0 {
		t.Fatalf("missing event %q", event)
	}
	return data
}

func decodeEventData(t *testing.T, data json.RawMessage, target any) {
	t.Helper()

	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("unmarshal event data into %T: %v; data=%s", target, err, data)
	}
}

func assertDegradedAbsentOrFalse(t *testing.T, data json.RawMessage) {
	t.Helper()

	var payload map[string]json.RawMessage
	decodeEventData(t, data, &payload)
	raw, ok := payload["degraded"]
	if !ok {
		return
	}

	var degraded bool
	decodeEventData(t, raw, &degraded)
	if degraded {
		t.Fatalf("degraded = true, want absent or false; data=%s", data)
	}
}

func assertStepPayloadsMatchFrontendContract(t *testing.T, data []json.RawMessage) {
	t.Helper()

	for _, raw := range data {
		var step review.StepPayload
		decodeEventData(t, raw, &step)
		if strings.TrimSpace(step.Step) == "" ||
			strings.TrimSpace(step.Status) == "" ||
			strings.TrimSpace(step.Message) == "" {
			t.Fatalf("step payload is not compatible with frontend StepPayload contract: %#v", step)
		}
	}
}

func assertPRMatchesFrontendContract(t *testing.T, pr review.PRInfo) {
	t.Helper()

	if strings.TrimSpace(pr.Title) == "" ||
		strings.TrimSpace(pr.Repo) == "" ||
		strings.TrimSpace(pr.SourceBranch) == "" ||
		strings.TrimSpace(pr.TargetBranch) == "" ||
		pr.Number <= 0 ||
		pr.ChangedFiles <= 0 ||
		pr.Additions <= 0 ||
		pr.Deletions <= 0 ||
		pr.Commits <= 0 {
		t.Fatalf("pr payload is not compatible with frontend PRInfo contract: %#v", pr)
	}
}

func assertRulesPayloadMatchesFrontendContract(t *testing.T, rules review.RulesPayload) {
	t.Helper()

	if len(rules.Risks) == 0 {
		t.Fatal("rules.risks is empty")
	}
	for _, risk := range rules.Risks {
		assertRiskMatchesFrontendContract(t, risk)
		if risk.Source != "rule" {
			t.Fatalf("rules risk source = %q, want rule: %#v", risk.Source, risk)
		}
	}
}

func assertReportMatchesFrontendContract(t *testing.T, report review.Report) {
	t.Helper()

	if strings.TrimSpace(report.Summary.RiskLevel) == "" {
		t.Fatalf("result.summary.risk_level is empty: %#v", report.Summary)
	}
	if strings.TrimSpace(report.Summary.Overview) == "" {
		t.Fatalf("result.summary.overview is empty: %#v", report.Summary)
	}
	if len(report.Summary.KeyChanges) == 0 {
		t.Fatalf("result.summary.key_changes is empty: %#v", report.Summary)
	}
	if len(report.Summary.ReviewFocus) == 0 {
		t.Fatalf("result.summary.review_focus is empty: %#v", report.Summary)
	}
	if len(report.Risks) == 0 {
		t.Fatal("result.risks is empty")
	}
	for _, risk := range report.Risks {
		assertRiskMatchesFrontendContract(t, risk)
	}
	if len(report.Evidence) == 0 {
		t.Fatal("result.evidence is empty")
	}
	for _, evidence := range report.Evidence {
		if strings.TrimSpace(evidence.ID) == "" || strings.TrimSpace(evidence.Snippet) == "" {
			t.Fatalf("evidence is not compatible with frontend EvidenceItem contract: %#v", evidence)
		}
	}
	if len(report.Comments) == 0 {
		t.Fatal("result.comments is empty")
	}
	for _, comment := range report.Comments {
		if strings.TrimSpace(comment.ID) == "" || strings.TrimSpace(comment.Body) == "" {
			t.Fatalf("comment is not compatible with frontend SuggestedComment contract: %#v", comment)
		}
	}
	if !report.Meta.RulesCompleted {
		t.Fatalf("result.meta.rules_completed = false, want true: %#v", report.Meta)
	}
	if !report.Meta.AICompleted {
		t.Fatalf("result.meta.ai_completed = false, want true: %#v", report.Meta)
	}
}

func assertRiskMatchesFrontendContract(t *testing.T, risk review.Risk) {
	t.Helper()

	if strings.TrimSpace(risk.ID) == "" ||
		strings.TrimSpace(risk.Source) == "" ||
		strings.TrimSpace(risk.Severity) == "" ||
		strings.TrimSpace(risk.Category) == "" ||
		strings.TrimSpace(risk.Title) == "" ||
		strings.TrimSpace(risk.Reason) == "" ||
		strings.TrimSpace(risk.Suggestion) == "" ||
		risk.Confidence <= 0 {
		t.Fatalf("risk is not compatible with frontend Risk contract: %#v", risk)
	}
}

func assertResultMetaHasFrontendFields(t *testing.T, data json.RawMessage) {
	t.Helper()

	var result map[string]json.RawMessage
	decodeEventData(t, data, &result)

	meta, ok := result["meta"]
	if !ok {
		t.Fatalf("result payload missing meta object: %s", data)
	}

	var metaFields map[string]json.RawMessage
	decodeEventData(t, meta, &metaFields)
	for _, field := range []string{
		"ai_completed",
		"rules_completed",
		"context_truncated",
		"omitted_files_count",
		"omitted_snippets_count",
	} {
		if _, ok := metaFields[field]; !ok {
			t.Fatalf("result.meta missing frontend-required field %q: %s", field, meta)
		}
	}
}

func assertRiskSources(t *testing.T, risks []review.Risk, want []string) {
	t.Helper()

	seen := map[string]bool{}
	for _, risk := range risks {
		seen[risk.Source] = true
	}
	for _, source := range want {
		if !seen[source] {
			t.Fatalf("missing risk source %q in %#v", source, risks)
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
			wantStage:   "fetch_pr",
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
