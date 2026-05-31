package github_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"diff-lens/internal/github"
)

// TestParsePRURLAcceptsGitHubPullRequestURL 验证对应场景的行为是否符合预期。
func TestParsePRURLAcceptsGitHubPullRequestURL(t *testing.T) {
	ref, err := github.ParsePRURL("https://github.com/openai/example/pull/123?tab=files#discussion")
	if err != nil {
		t.Fatalf("ParsePRURL returned error: %v", err)
	}

	if ref.Owner != "openai" {
		t.Fatalf("Owner = %q, want %q", ref.Owner, "openai")
	}
	if ref.Repo != "example" {
		t.Fatalf("Repo = %q, want %q", ref.Repo, "example")
	}
	if ref.Number != 123 {
		t.Fatalf("Number = %d, want %d", ref.Number, 123)
	}
}

// TestParsePRURLRejectsNonPullRequestURL 验证对应场景的行为是否符合预期。
func TestParsePRURLRejectsNonPullRequestURL(t *testing.T) {
	_, err := github.ParsePRURL("https://github.com/openai/example/issues/123")
	if err == nil {
		t.Fatal("ParsePRURL returned nil error for issue URL")
	}
}

// TestPullRequestDataRepresentsMetadataFilesAndCommits 验证对应场景的行为是否符合预期。
func TestPullRequestDataRepresentsMetadataFilesAndCommits(t *testing.T) {
	timestamp := time.Date(2026, 5, 30, 10, 15, 0, 0, time.UTC)

	data := github.PullRequestData{
		Ref: github.PRRef{
			Owner:  "openai",
			Repo:   "example",
			Number: 123,
		},
		Title:        "Add review pipeline",
		Author:       "octocat",
		Repo:         "openai/example",
		Number:       123,
		SourceBranch: "feature/review-pipeline",
		TargetBranch: "main",
		ChangedFiles: 2,
		Additions:    42,
		Deletions:    7,
		CommitsCount: 1,
		Files:        []github.PullRequestFile{{Filename: "internal/review/service.go", Status: "modified", Additions: 40, Deletions: 7, Changes: 47, Patch: "@@ -1 +1 @@"}},
		Commits:      []github.PullRequestCommit{{SHA: "abc123", Message: "Add review pipeline", AuthorName: "Ada Lovelace", AuthorLogin: "ada", Timestamp: timestamp}},
	}

	if data.Ref.Owner != "openai" || data.Ref.Repo != "example" || data.Ref.Number != 123 {
		t.Fatalf("Ref = %+v, want openai/example#123", data.Ref)
	}
	if data.Title != "Add review pipeline" || data.Author != "octocat" || data.Repo != "openai/example" || data.Number != 123 {
		t.Fatalf("metadata = %+v, want populated PR metadata", data)
	}
	if data.SourceBranch != "feature/review-pipeline" || data.TargetBranch != "main" {
		t.Fatalf("branches = %q -> %q, want feature/review-pipeline -> main", data.SourceBranch, data.TargetBranch)
	}
	if data.ChangedFiles != 2 || data.Additions != 42 || data.Deletions != 7 || data.CommitsCount != 1 {
		t.Fatalf("stats = files:%d additions:%d deletions:%d commits:%d, want 2/42/7/1", data.ChangedFiles, data.Additions, data.Deletions, data.CommitsCount)
	}
	if len(data.Files) != 1 || data.Files[0].Filename != "internal/review/service.go" || data.Files[0].Patch == "" {
		t.Fatalf("Files = %+v, want populated file patch data", data.Files)
	}
	if len(data.Commits) != 1 || data.Commits[0].SHA != "abc123" || !data.Commits[0].Timestamp.Equal(timestamp) {
		t.Fatalf("Commits = %+v, want populated commit data", data.Commits)
	}
}

// TestPullRequestFileAllowsEmptyPatch 验证对应场景的行为是否符合预期。
func TestPullRequestFileAllowsEmptyPatch(t *testing.T) {
	file := github.PullRequestFile{
		Filename:  "assets/logo.png",
		Status:    "modified",
		Additions: 0,
		Deletions: 0,
		Changes:   0,
		Patch:     "",
	}

	if file.Patch != "" {
		t.Fatalf("Patch = %q, want empty patch for binary or oversized file", file.Patch)
	}
}

// TestGitHubErrorsSupportErrorsIs 验证对应场景的行为是否符合预期。
func TestGitHubErrorsSupportErrorsIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "invalid PR URL", err: github.ErrInvalidPRURL},
		{name: "PR not found", err: github.ErrPRNotFound},
		{name: "unauthorized", err: github.ErrGitHubUnauthorized},
		{name: "rate limited", err: github.ErrGitHubRateLimited},
		{name: "request failed", err: github.ErrGitHubRequestFailed},
		{name: "response invalid", err: github.ErrGitHubResponseInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := fmt.Errorf("github client: %w", tt.err)

			if !errors.Is(wrapped, tt.err) {
				t.Fatalf("errors.Is(%v, %v) = false, want true", wrapped, tt.err)
			}
		})
	}
}

// TestNewClientWithOptionsUsesTimeoutClientByDefault 验证对应场景的行为是否符合预期。
func TestNewClientWithOptionsUsesTimeoutClientByDefault(t *testing.T) {
	client := github.NewClientWithOptions(github.ClientOptions{})

	timeout := time.Duration(reflect.ValueOf(client).Elem().FieldByName("httpClient").Elem().FieldByName("Timeout").Int())
	if timeout == 0 {
		t.Fatalf("default HTTP client timeout = %v, want non-zero timeout", timeout)
	}
}

// TestNewClientWithOptionsPreservesInjectedHTTPClient 验证对应场景的行为是否符合预期。
func TestNewClientWithOptionsPreservesInjectedHTTPClient(t *testing.T) {
	injected := &http.Client{Timeout: 2 * time.Second}

	client := github.NewClientWithOptions(github.ClientOptions{HTTPClient: injected})

	got := reflect.ValueOf(client).Elem().FieldByName("httpClient").Pointer()
	want := reflect.ValueOf(injected).Pointer()
	if got != want {
		t.Fatalf("HTTP client pointer = %x, want injected pointer %x", got, want)
	}
}

// TestFetchPullRequestRequestsEndpointsAndReturnsData 验证对应场景的行为是否符合预期。
func TestFetchPullRequestRequestsEndpointsAndReturnsData(t *testing.T) {
	var paths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)

		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("Accept header was not set to GitHub JSON media type")
		}
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
			t.Errorf("GitHub API version header was not set")
		}

		switch r.URL.Path {
		case "/repos/openai/example/pulls/123":
			writeJSON(w, `{
				"number": 123,
				"title": "Add REST client",
				"user": {"login": "octocat"},
				"head": {"ref": "feature/rest-client"},
				"base": {"ref": "main"},
				"changed_files": 2,
				"additions": 42,
				"deletions": 7,
				"commits": 1
			}`)
		case "/repos/openai/example/pulls/123/files":
			writeJSON(w, `[
				{
					"filename": "internal/github/client.go",
					"status": "modified",
					"additions": 40,
					"deletions": 7,
					"changes": 47,
					"patch": "@@ -1 +1 @@"
				},
				{
					"filename": "internal/github/types.go",
					"status": "added",
					"additions": 2,
					"deletions": 0,
					"changes": 2,
					"patch": "@@ -0,0 +1 @@"
				}
			]`)
		case "/repos/openai/example/pulls/123/commits":
			writeJSON(w, `[
				{
					"sha": "abc123",
					"commit": {
						"message": "Add GitHub REST client",
						"author": {
							"name": "Ada Lovelace",
							"date": "2026-05-30T10:15:00Z"
						}
					},
					"author": {"login": "ada"}
				}
			]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	data, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}

	wantPaths := []string{
		"/repos/openai/example/pulls/123",
		"/repos/openai/example/pulls/123/files",
		"/repos/openai/example/pulls/123/commits",
	}
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("requested paths = %v, want %v", paths, wantPaths)
	}

	if data.Ref.Owner != "openai" || data.Ref.Repo != "example" || data.Ref.Number != 123 {
		t.Fatalf("Ref = %+v, want openai/example#123", data.Ref)
	}
	if data.Title != "Add REST client" || data.Author != "octocat" || data.Repo != "openai/example" || data.Number != 123 {
		t.Fatalf("metadata = %+v, want normalized PR metadata", data)
	}
	if data.SourceBranch != "feature/rest-client" || data.TargetBranch != "main" {
		t.Fatalf("branches = %q -> %q, want feature/rest-client -> main", data.SourceBranch, data.TargetBranch)
	}
	if data.ChangedFiles != 2 || data.Additions != 42 || data.Deletions != 7 || data.CommitsCount != 1 {
		t.Fatalf("stats = files:%d additions:%d deletions:%d commits:%d, want 2/42/7/1", data.ChangedFiles, data.Additions, data.Deletions, data.CommitsCount)
	}
	if len(data.Files) != 2 || data.Files[0].Filename != "internal/github/client.go" || data.Files[1].Filename != "internal/github/types.go" {
		t.Fatalf("Files = %+v, want two normalized files", data.Files)
	}
	if len(data.Commits) != 1 {
		t.Fatalf("Commits length = %d, want 1", len(data.Commits))
	}
	wantTimestamp := time.Date(2026, 5, 30, 10, 15, 0, 0, time.UTC)
	if data.Commits[0].SHA != "abc123" || data.Commits[0].Message != "Add GitHub REST client" || data.Commits[0].AuthorName != "Ada Lovelace" || data.Commits[0].AuthorLogin != "ada" || !data.Commits[0].Timestamp.Equal(wantTimestamp) {
		t.Fatalf("Commit = %+v, want normalized commit data", data.Commits[0])
	}
}

// TestFetchPullRequestSendsAuthorizationHeaderWhenTokenConfigured 验证对应场景的行为是否符合预期。
func TestFetchPullRequestSendsAuthorizationHeaderWhenTokenConfigured(t *testing.T) {
	const token = "test-token"
	var sawBearerToken bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer "+token {
			sawBearerToken = true
		} else {
			t.Errorf("Authorization header was not set as a bearer token")
		}

		switch r.URL.Path {
		case "/repos/openai/example/pulls/123":
			writeMinimalPR(w)
		case "/repos/openai/example/pulls/123/files", "/repos/openai/example/pulls/123/commits":
			writeJSON(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		Token:      token,
		HTTPClient: server.Client(),
	})

	_, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if !sawBearerToken {
		t.Fatalf("Authorization bearer token was not observed")
	}
}

// TestFetchPullRequestMergesPaginatedFiles 验证对应场景的行为是否符合预期。
func TestFetchPullRequestMergesPaginatedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openai/example/pulls/123":
			writeMinimalPR(w)
		case "/repos/openai/example/pulls/123/files":
			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("Link", fmt.Sprintf(`<%s/repos/openai/example/pulls/123/files?page=2&per_page=100>; rel="next"`, serverURL(r)))
				writeJSON(w, `[{"filename":"first.go","status":"modified","additions":1,"deletions":0,"changes":1,"patch":"@@ first @@"}]`)
			case "2":
				writeJSON(w, `[{"filename":"second.go","status":"added","additions":2,"deletions":0,"changes":2,"patch":"@@ second @@"}]`)
			default:
				t.Errorf("unexpected files page")
				http.Error(w, "unexpected page", http.StatusInternalServerError)
			}
		case "/repos/openai/example/pulls/123/commits":
			writeJSON(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	data, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if len(data.Files) != 2 || data.Files[0].Filename != "first.go" || data.Files[1].Filename != "second.go" {
		t.Fatalf("Files = %+v, want files from both pages", data.Files)
	}
}

// TestFetchPullRequestMergesPaginatedCommits 验证对应场景的行为是否符合预期。
func TestFetchPullRequestMergesPaginatedCommits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openai/example/pulls/123":
			writeMinimalPR(w)
		case "/repos/openai/example/pulls/123/files":
			writeJSON(w, `[]`)
		case "/repos/openai/example/pulls/123/commits":
			switch r.URL.Query().Get("page") {
			case "", "1":
				w.Header().Set("Link", fmt.Sprintf(`<%s/repos/openai/example/pulls/123/commits?page=2&per_page=100>; rel="next"`, serverURL(r)))
				writeJSON(w, `[{"sha":"first","commit":{"message":"first commit","author":{"name":"Ada","date":"2026-05-30T10:15:00Z"}},"author":{"login":"ada"}}]`)
			case "2":
				writeJSON(w, `[{"sha":"second","commit":{"message":"second commit","author":{"name":"Grace","date":"2026-05-30T10:20:00Z"}},"author":{"login":"grace"}}]`)
			default:
				t.Errorf("unexpected commits page")
				http.Error(w, "unexpected page", http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	data, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if len(data.Commits) != 2 || data.Commits[0].SHA != "first" || data.Commits[1].SHA != "second" {
		t.Fatalf("Commits = %+v, want commits from both pages", data.Commits)
	}
}

// TestFetchPullRequestKeepsFileWhenPatchMissing 验证对应场景的行为是否符合预期。
func TestFetchPullRequestKeepsFileWhenPatchMissing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openai/example/pulls/123":
			writeMinimalPR(w)
		case "/repos/openai/example/pulls/123/files":
			writeJSON(w, `[{"filename":"assets/logo.png","status":"modified","additions":0,"deletions":0,"changes":0}]`)
		case "/repos/openai/example/pulls/123/commits":
			writeJSON(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	data, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if err != nil {
		t.Fatalf("FetchPullRequest returned error: %v", err)
	}
	if len(data.Files) != 1 || data.Files[0].Filename != "assets/logo.png" || data.Files[0].Patch != "" {
		t.Fatalf("Files = %+v, want file retained with empty patch", data.Files)
	}
}

// TestFetchPullRequestMapsHTTPStatusErrors 验证对应场景的行为是否符合预期。
func TestFetchPullRequestMapsHTTPStatusErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		headers map[string]string
		wantErr error
	}{
		{name: "not found", status: http.StatusNotFound, wantErr: github.ErrPRNotFound},
		{name: "unauthorized", status: http.StatusUnauthorized, wantErr: github.ErrGitHubUnauthorized},
		{name: "forbidden without rate limit", status: http.StatusForbidden, wantErr: github.ErrGitHubUnauthorized},
		{name: "rate limited", status: http.StatusForbidden, headers: map[string]string{"X-RateLimit-Remaining": "0"}, wantErr: github.ErrGitHubRateLimited},
		{name: "server error", status: http.StatusInternalServerError, wantErr: github.ErrGitHubRequestFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for key, value := range tt.headers {
					w.Header().Set(key, value)
				}
				http.Error(w, `{"message":"request failed"}`, tt.status)
			}))
			defer server.Close()

			client := github.NewClientWithOptions(github.ClientOptions{
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			})

			_, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("FetchPullRequest error = %v, want errors.Is(..., %v)", err, tt.wantErr)
			}
		})
	}
}

// TestFetchPullRequestMapsMalformedJSONToResponseInvalid 验证对应场景的行为是否符合预期。
func TestFetchPullRequestMapsMalformedJSONToResponseInvalid(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, `{`)
	}))
	defer server.Close()

	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	})

	_, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if !errors.Is(err, github.ErrGitHubResponseInvalid) {
		t.Fatalf("FetchPullRequest error = %v, want ErrGitHubResponseInvalid", err)
	}
}

// TestFetchPullRequestMapsHTTPClientFailureToRequestFailed 验证对应场景的行为是否符合预期。
func TestFetchPullRequestMapsHTTPClientFailureToRequestFailed(t *testing.T) {
	client := github.NewClientWithOptions(github.ClientOptions{
		BaseURL: "http://example.invalid",
		HTTPClient: &http.Client{
			Transport: failingRoundTripper{},
		},
	})

	_, err := client.FetchPullRequest(context.Background(), github.PRRef{Owner: "openai", Repo: "example", Number: 123})
	if !errors.Is(err, github.ErrGitHubRequestFailed) {
		t.Fatalf("FetchPullRequest error = %v, want ErrGitHubRequestFailed", err)
	}
}

// failingRoundTripper 是测试辅助数据结构或替身类型。
type failingRoundTripper struct{}

// RoundTrip 是测试辅助函数。
func (failingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("request failed")
}

// writeMinimalPR 写入测试 HTTP 响应。
func writeMinimalPR(w http.ResponseWriter) {
	writeJSON(w, `{
		"number": 123,
		"title": "Minimal PR",
		"user": {"login": "octocat"},
		"head": {"ref": "feature"},
		"base": {"ref": "main"},
		"changed_files": 0,
		"additions": 0,
		"deletions": 0,
		"commits": 0
	}`)
}

// writeJSON 写入测试 HTTP 响应。
func writeJSON(w http.ResponseWriter, body string) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

// serverURL 是测试辅助函数。
func serverURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}
