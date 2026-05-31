package github

import (
	"errors"
	"net/http"
	"time"
)

// GitHub 领域错误集中声明，便于服务层用 errors.Is 做分类。
var (
	// ErrInvalidPRURL 表示输入不是有效的 GitHub PR URL。
	ErrInvalidPRURL = errors.New("invalid GitHub pull request URL")

	// ErrPRNotFound 表示 GitHub 没有返回请求标识对应的 PR。
	ErrPRNotFound = errors.New("github pull request not found")

	// ErrGitHubUnauthorized 表示 GitHub 拒绝了请求凭据。
	ErrGitHubUnauthorized = errors.New("github unauthorized")

	// ErrGitHubRateLimited 表示 GitHub 因限流拒绝了请求。
	ErrGitHubRateLimited = errors.New("github rate limited")

	// ErrGitHubRequestFailed 表示客户端未能完成 GitHub 请求。
	ErrGitHubRequestFailed = errors.New("github request failed")

	// ErrGitHubResponseInvalid 表示 GitHub 返回了客户端无法解码的响应。
	ErrGitHubResponseInvalid = errors.New("github response invalid")
)

// ClientOptions 包含 GitHub 客户端发起 REST 调用时使用的可选配置。
type ClientOptions struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

// PullRequestData 是评审分析所需的标准化 GitHub PR 载荷。
type PullRequestData struct {
	Ref          PRRef
	Title        string
	Author       string
	Repo         string
	Number       int
	SourceBranch string
	TargetBranch string
	ChangedFiles int
	Additions    int
	Deletions    int
	CommitsCount int
	Files        []PullRequestFile
	Commits      []PullRequestCommit
}

// PullRequestFile 描述 PR 中的一个变更文件。
type PullRequestFile struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Changes   int
	Patch     string
}

// PullRequestCommit 描述 PR 包含的一个提交。
type PullRequestCommit struct {
	SHA         string
	Message     string
	AuthorName  string
	AuthorLogin string
	Timestamp   time.Time
}
