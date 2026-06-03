package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"diff-lens/internal/review"
)

// TestAnalyzeWithoutAPIKeyReturnsErrNotConfigured 验证对应场景的行为是否符合预期。
func TestAnalyzeWithoutAPIKeyReturnsErrNotConfigured(t *testing.T) {
	analyzer := NewAnalyzer("https://llm.example", "", "gpt-test")

	_, err := analyzer.Analyze(context.Background(), sampleReviewContext())
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("Analyze error = %v, want ErrNotConfigured", err)
	}
}

// TestAnalyzePostsOpenAICompatibleRequestAndParsesAnalysis 验证对应场景的行为是否符合预期。
func TestAnalyzePostsOpenAICompatibleRequestAndParsesAnalysis(t *testing.T) {
	var capturedPath string
	var capturedAuth string
	var capturedContentType string
	var capturedBody chatCompletionRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		capturedAuth = r.Header.Get("Authorization")
		capturedContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&capturedBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"choices": [{
				"message": {
					"content": "{\"summary\":\"Check auth handling.\",\"risks\":[{\"id\":\"ai-auth-1\",\"severity\":\"high\",\"confidence\":0.82,\"category\":\"auth\",\"title\":\"Auth bypass\",\"file\":\"internal/auth.go\",\"line\":12,\"evidence_refs\":[\"snippet-auth-1\"],\"reason\":\"The guard is removed.\",\"suggestion\":\"Keep authorization before returning data.\"}],\"comments\":[{\"id\":\"comment-auth-1\",\"file\":\"internal/auth.go\",\"line\":12,\"body\":\"Please keep authorization before returning user data.\",\"evidence_refs\":[\"snippet-auth-1\"]}],\"attention_items\":[\"Confirm auth tests cover anonymous users\"],\"meta\":{\"completed\":true}}"
				}
			}]
		}`))
	}))
	defer server.Close()

	analyzer := NewAnalyzerWithOptions(AnalyzerOptions{
		BaseURL:    server.URL + "/",
		APIKey:     "secret-token",
		Model:      "gpt-test",
		HTTPClient: server.Client(),
	})

	analysis, err := analyzer.Analyze(context.Background(), sampleReviewContext())
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	if capturedPath != "/v1/chat/completions" {
		t.Fatalf("request path = %q, want /v1/chat/completions", capturedPath)
	}
	if capturedAuth != "Bearer secret-token" {
		t.Fatalf("Authorization = %q, want bearer token", capturedAuth)
	}
	if !strings.HasPrefix(capturedContentType, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", capturedContentType)
	}
	if capturedBody.Model != "gpt-test" {
		t.Fatalf("model = %q, want gpt-test", capturedBody.Model)
	}
	if capturedBody.Stream != nil && *capturedBody.Stream {
		t.Fatalf("stream = true, want false or omitted")
	}
	if len(capturedBody.Messages) < 2 {
		t.Fatalf("messages length = %d, want system and user messages", len(capturedBody.Messages))
	}

	prompt := strings.Join(messageContents(capturedBody.Messages), "\n")
	for _, want := range []string{
		"Only output JSON",
		"untrusted user content",
		"Evidence refs",
		"Remove auth guard",
		"snippet-auth-1",
		"rule-auth-1",
		"@@ -10,7 +10,6 @@",
		"changed_files",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
	if strings.Contains(prompt, "FULL_RAW_DIFF_SHOULD_NOT_APPEAR") {
		t.Fatalf("prompt included full raw diff sentinel:\n%s", prompt)
	}

	if analysis.Summary != "Check auth handling." {
		t.Fatalf("summary = %q", analysis.Summary)
	}
	if len(analysis.Risks) != 1 || analysis.Risks[0].EvidenceRefs[0] != "snippet-auth-1" {
		t.Fatalf("risks = %#v, want parsed risk with evidence refs", analysis.Risks)
	}
	if len(analysis.Comments) != 1 || analysis.Comments[0].ID != "comment-auth-1" {
		t.Fatalf("comments = %#v, want parsed comment", analysis.Comments)
	}
	if !analysis.Meta.Completed {
		t.Fatalf("meta.completed = false, want true")
	}
}

func TestChatCompletionsURLNormalizesVersionedBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    string
	}{
		{
			name:    "root base url",
			baseURL: "https://llm.example.com",
			want:    "https://llm.example.com/v1/chat/completions",
		},
		{
			name:    "versioned base url",
			baseURL: "https://api.xiaomimimo.com/v1",
			want:    "https://api.xiaomimimo.com/v1/chat/completions",
		},
		{
			name:    "versioned base url with trailing slash",
			baseURL: "https://api.xiaomimimo.com/v1/",
			want:    "https://api.xiaomimimo.com/v1/chat/completions",
		},
		{
			name:    "nested proxy base url",
			baseURL: "https://llm.example.com/proxy/v1",
			want:    "https://llm.example.com/proxy/v1/chat/completions",
		},
		{
			name:    "full chat completions endpoint",
			baseURL: "https://llm.example.com/v1/chat/completions",
			want:    "https://llm.example.com/v1/chat/completions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := chatCompletionsURL(tt.baseURL)
			if err != nil {
				t.Fatalf("chatCompletionsURL returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("chatCompletionsURL(%q) = %q, want %q", tt.baseURL, got, tt.want)
			}
		})
	}
}

// TestAnalyzeClassifiesTransportAndResponseErrorsWithoutLeakingAPIKey 验证对应场景的行为是否符合预期。
func TestAnalyzeClassifiesTransportAndResponseErrorsWithoutLeakingAPIKey(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       error
	}{
		{name: "unauthorized", statusCode: http.StatusUnauthorized, body: `{"error":"bad key secret-token"}`, want: ErrRequestFailed},
		{name: "server error", statusCode: http.StatusInternalServerError, body: `{"error":"temporary failure"}`, want: ErrRequestFailed},
		{name: "invalid response json", statusCode: http.StatusOK, body: `{`, want: ErrResponseInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			analyzer := NewAnalyzerWithOptions(AnalyzerOptions{
				BaseURL:    server.URL,
				APIKey:     "secret-token",
				Model:      "gpt-test",
				HTTPClient: server.Client(),
			})

			_, err := analyzer.Analyze(context.Background(), sampleReviewContext())
			if !errors.Is(err, tt.want) {
				t.Fatalf("Analyze error = %v, want %v", err, tt.want)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("error leaked API key: %v", err)
			}
		})
	}
}

// TestAnalyzeRejectsInvalidModelOutput 验证对应场景的行为是否符合预期。
func TestAnalyzeRejectsInvalidModelOutput(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "invalid json", content: `not json`},
		{name: "trailing prose after json", content: `{"summary":"x","risks":[],"comments":[],"attention_items":[],"meta":{"completed":true}}
Here is why secret-token should not be accepted.`},
		{name: "missing risk evidence refs", content: `{"summary":"x","risks":[{"id":"ai-1","severity":"high","confidence":0.7,"category":"auth","title":"missing refs","reason":"x","suggestion":"y"}],"comments":[]}`},
		{name: "wrong evidence refs type", content: `{"summary":"x","risks":[{"id":"ai-1","severity":"high","confidence":0.7,"category":"auth","title":"bad refs","evidence_refs":"snippet-auth-1","reason":"x","suggestion":"y"}],"comments":[]}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(openAIResponseWithContent(tt.content))
			}))
			defer server.Close()

			analyzer := NewAnalyzerWithOptions(AnalyzerOptions{
				BaseURL:    server.URL,
				APIKey:     "secret-token",
				Model:      "gpt-test",
				HTTPClient: server.Client(),
			})

			_, err := analyzer.Analyze(context.Background(), sampleReviewContext())
			if !errors.Is(err, ErrModelOutputInvalid) {
				t.Fatalf("Analyze error = %v, want ErrModelOutputInvalid", err)
			}
			if strings.Contains(err.Error(), "secret-token") {
				t.Fatalf("error leaked API key: %v", err)
			}
		})
	}
}

// sampleReviewContext 构造测试使用的样例数据。
func sampleReviewContext() review.ReviewContext {
	return review.ReviewContext{
		SchemaVersion: review.ReviewContextSchemaVersion,
		ContextID:     "ctx-1",
		PR: review.PRInfo{
			Title:        "Remove auth guard",
			Author:       "dev",
			Repo:         "acme/app",
			Number:       42,
			SourceBranch: "feature/open-data",
			TargetBranch: "main",
			ChangedFiles: 1,
			Additions:    2,
			Deletions:    1,
			Commits:      1,
		},
		Stats: review.ContextStats{
			ChangedFiles: 1,
			Additions:    2,
			Deletions:    1,
			SourceFiles:  1,
		},
		RuleRisks: []review.Risk{{
			ID:           "rule-auth-1",
			Source:       "rules",
			Severity:     "high",
			Confidence:   0.9,
			Category:     "auth",
			Title:        "Authorization guard changed",
			File:         "internal/auth.go",
			Line:         12,
			EvidenceRefs: []string{"snippet-auth-1"},
			Evidence:     "bounded evidence only; FULL_RAW_DIFF_SHOULD_NOT_APPEAR",
			Reason:       "A guard near sensitive data access changed.",
			Suggestion:   "Review authorization behavior.",
		}},
		Files: []review.ContextFile{{
			Filename:  "internal/auth.go",
			Kind:      "source",
			Status:    "modified",
			Additions: 2,
			Deletions: 1,
			RiskIDs:   []string{"rule-auth-1"},
			Snippets: []review.ContextSnippet{{
				ID:        "snippet-auth-1",
				File:      "internal/auth.go",
				StartLine: 10,
				EndLine:   14,
				Patch:     "@@ -10,7 +10,6 @@\n- requireAuth(user)\n+ // auth removed in this snippet",
				Reason:    "Rule risk evidence",
			}},
		}},
		EvidenceRefs: []string{"rule-auth-1", "snippet-auth-1"},
	}
}

// messageContents 是测试辅助函数。
func messageContents(messages []chatMessage) []string {
	contents := make([]string, 0, len(messages))
	for _, message := range messages {
		contents = append(contents, message.Content)
	}
	return contents
}

// openAIResponseWithContent 是测试辅助函数。
func openAIResponseWithContent(content string) chatCompletionResponse {
	return chatCompletionResponse{
		Choices: []chatCompletionChoice{{
			Message: chatMessage{Role: "assistant", Content: content},
		}},
	}
}
