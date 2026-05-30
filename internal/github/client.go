package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.github.com"
	perPage        = 100
)

// PRRef 是标准化后的仓库和 PR 标识。
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// Client fetches pull request data from the GitHub REST API.
type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a GitHub client using the default API base URL.
func NewClient(token string) *Client {
	return NewClientWithOptions(ClientOptions{Token: token})
}

// NewClientWithOptions creates a GitHub client with injectable transport settings.
func NewClientWithOptions(options ClientOptions) *Client {
	baseURL := strings.TrimRight(options.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Client{
		token:      options.Token,
		baseURL:    baseURL,
		httpClient: httpClient,
	}
}

// FetchPullRequest fetches PR metadata, changed files, and commits from GitHub.
func (c *Client) FetchPullRequest(ctx context.Context, ref PRRef) (PullRequestData, error) {
	pr, err := c.fetchPullRequestMetadata(ctx, ref)
	if err != nil {
		return PullRequestData{}, err
	}

	files, err := c.fetchPullRequestFiles(ctx, ref)
	if err != nil {
		return PullRequestData{}, err
	}

	commits, err := c.fetchPullRequestCommits(ctx, ref)
	if err != nil {
		return PullRequestData{}, err
	}

	number := pr.Number
	if number == 0 {
		number = ref.Number
	}

	return PullRequestData{
		Ref:          ref,
		Title:        pr.Title,
		Author:       pr.User.Login,
		Repo:         ref.Owner + "/" + ref.Repo,
		Number:       number,
		SourceBranch: pr.Head.Ref,
		TargetBranch: pr.Base.Ref,
		ChangedFiles: pr.ChangedFiles,
		Additions:    pr.Additions,
		Deletions:    pr.Deletions,
		CommitsCount: pr.Commits,
		Files:        files,
		Commits:      commits,
	}, nil
}

// ParsePRURL 解析标准 GitHub PR URL，并提取 owner、repo 和 PR 编号。
func ParsePRURL(rawURL string) (PRRef, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return PRRef{}, ErrInvalidPRURL
	}

	if parsed.Host != "github.com" {
		return PRRef{}, ErrInvalidPRURL
	}

	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) != 4 || parts[2] != "pull" {
		return PRRef{}, ErrInvalidPRURL
	}

	number, err := strconv.Atoi(parts[3])
	if err != nil || number <= 0 {
		return PRRef{}, ErrInvalidPRURL
	}

	if parts[0] == "" || parts[1] == "" {
		return PRRef{}, ErrInvalidPRURL
	}

	return PRRef{Owner: parts[0], Repo: parts[1], Number: number}, nil
}

type pullRequestResponse struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	User         user   `json:"user"`
	Head         branch `json:"head"`
	Base         branch `json:"base"`
	ChangedFiles int    `json:"changed_files"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Commits      int    `json:"commits"`
}

type user struct {
	Login string `json:"login"`
}

type branch struct {
	Ref string `json:"ref"`
}

type pullRequestFileResponse struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
	Patch     string `json:"patch"`
}

type pullRequestCommitResponse struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
			Date string `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	Author user `json:"author"`
}

func (c *Client) fetchPullRequestMetadata(ctx context.Context, ref PRRef) (pullRequestResponse, error) {
	var pr pullRequestResponse
	_, err := c.getJSON(ctx, c.endpoint(ref, ""), nil, &pr)
	if err != nil {
		return pullRequestResponse{}, err
	}
	return pr, nil
}

func (c *Client) fetchPullRequestFiles(ctx context.Context, ref PRRef) ([]PullRequestFile, error) {
	rawFiles, err := fetchPaginated[pullRequestFileResponse](ctx, c, c.endpoint(ref, "files"))
	if err != nil {
		return nil, err
	}

	files := make([]PullRequestFile, 0, len(rawFiles))
	for _, file := range rawFiles {
		files = append(files, PullRequestFile{
			Filename:  file.Filename,
			Status:    file.Status,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Changes:   file.Changes,
			Patch:     file.Patch,
		})
	}
	return files, nil
}

func (c *Client) fetchPullRequestCommits(ctx context.Context, ref PRRef) ([]PullRequestCommit, error) {
	rawCommits, err := fetchPaginated[pullRequestCommitResponse](ctx, c, c.endpoint(ref, "commits"))
	if err != nil {
		return nil, err
	}

	commits := make([]PullRequestCommit, 0, len(rawCommits))
	for _, commit := range rawCommits {
		timestamp, err := parseGitHubTimestamp(commit.Commit.Author.Date)
		if err != nil {
			return nil, err
		}

		commits = append(commits, PullRequestCommit{
			SHA:         commit.SHA,
			Message:     commit.Commit.Message,
			AuthorName:  commit.Commit.Author.Name,
			AuthorLogin: commit.Author.Login,
			Timestamp:   timestamp,
		})
	}
	return commits, nil
}

func fetchPaginated[T any](ctx context.Context, c *Client, firstURL string) ([]T, error) {
	var all []T
	nextURL := firstURL

	for nextURL != "" {
		var page []T
		headers, err := c.getJSON(ctx, nextURL, paginationQuery(1), &page)
		if err != nil {
			return nil, err
		}

		all = append(all, page...)

		if next := nextLink(headers.Get("Link")); next != "" {
			nextURL = next
			continue
		}

		if len(page) < perPage {
			break
		}

		next, err := nextPageURL(nextURL)
		if err != nil {
			return nil, err
		}
		nextURL = next
	}

	return all, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string, defaultQuery url.Values, target any) (http.Header, error) {
	requestURL, err := applyDefaultQuery(rawURL, defaultQuery)
	if err != nil {
		return nil, fmt.Errorf("%w: create request", ErrGitHubRequestFailed)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: create request", ErrGitHubRequestFailed)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrGitHubRequestFailed, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, mapGitHubStatus(resp)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return nil, fmt.Errorf("%w: decode GitHub response", ErrGitHubResponseInvalid)
	}

	return resp.Header, nil
}

func (c *Client) endpoint(ref PRRef, suffix string) string {
	apiPath := path.Join(
		"/repos",
		url.PathEscape(ref.Owner),
		url.PathEscape(ref.Repo),
		"pulls",
		strconv.Itoa(ref.Number),
		suffix,
	)

	return c.baseURL + apiPath
}

func paginationQuery(page int) url.Values {
	query := url.Values{}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", strconv.Itoa(perPage))
	return query
}

func applyDefaultQuery(rawURL string, defaults url.Values) (string, error) {
	if len(defaults) == 0 {
		return rawURL, nil
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	query := parsed.Query()
	for key, values := range defaults {
		if query.Get(key) != "" {
			continue
		}
		for _, value := range values {
			query.Add(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func nextPageURL(rawURL string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("%w: parse pagination URL", ErrGitHubRequestFailed)
	}

	query := parsed.Query()
	page, err := strconv.Atoi(query.Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	query.Set("page", strconv.Itoa(page+1))
	query.Set("per_page", strconv.Itoa(perPage))
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func nextLink(linkHeader string) string {
	for _, part := range strings.Split(linkHeader, ",") {
		sections := strings.Split(part, ";")
		if len(sections) < 2 {
			continue
		}

		rawURL := strings.TrimSpace(sections[0])
		if !strings.HasPrefix(rawURL, "<") || !strings.HasSuffix(rawURL, ">") {
			continue
		}

		for _, section := range sections[1:] {
			if strings.TrimSpace(section) == `rel="next"` {
				return strings.TrimSuffix(strings.TrimPrefix(rawURL, "<"), ">")
			}
		}
	}

	return ""
}

func mapGitHubStatus(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusNotFound:
		return fmt.Errorf("%w: status %d", ErrPRNotFound, resp.StatusCode)
	case http.StatusUnauthorized:
		return fmt.Errorf("%w: status %d", ErrGitHubUnauthorized, resp.StatusCode)
	case http.StatusForbidden:
		if resp.Header.Get("X-RateLimit-Remaining") == "0" || resp.Header.Get("Retry-After") != "" {
			return fmt.Errorf("%w: status %d", ErrGitHubRateLimited, resp.StatusCode)
		}
		return fmt.Errorf("%w: status %d", ErrGitHubUnauthorized, resp.StatusCode)
	default:
		return fmt.Errorf("%w: status %d", ErrGitHubRequestFailed, resp.StatusCode)
	}
}

func parseGitHubTimestamp(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}

	timestamp, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: decode GitHub commit timestamp", ErrGitHubResponseInvalid)
	}
	return timestamp, nil
}
