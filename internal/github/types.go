package github

import (
	"errors"
	"net/http"
	"time"
)

var (
	// ErrInvalidPRURL indicates the input is not a valid GitHub pull request URL.
	ErrInvalidPRURL = errors.New("invalid GitHub pull request URL")

	// ErrPRNotFound indicates GitHub returned no pull request for the requested ref.
	ErrPRNotFound = errors.New("github pull request not found")

	// ErrGitHubUnauthorized indicates GitHub rejected the request credentials.
	ErrGitHubUnauthorized = errors.New("github unauthorized")

	// ErrGitHubRateLimited indicates GitHub rejected the request because of rate limiting.
	ErrGitHubRateLimited = errors.New("github rate limited")

	// ErrGitHubRequestFailed indicates the client could not complete the GitHub request.
	ErrGitHubRequestFailed = errors.New("github request failed")

	// ErrGitHubResponseInvalid indicates GitHub returned a response the client could not decode.
	ErrGitHubResponseInvalid = errors.New("github response invalid")
)

// ClientOptions contains optional GitHub client configuration for future REST calls.
type ClientOptions struct {
	BaseURL    string
	Token      string
	HTTPClient *http.Client
}

// PullRequestData is the normalized GitHub pull request payload needed by review analysis.
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

// PullRequestFile describes one changed file in a pull request.
type PullRequestFile struct {
	Filename  string
	Status    string
	Additions int
	Deletions int
	Changes   int
	Patch     string
}

// PullRequestCommit describes one commit included in a pull request.
type PullRequestCommit struct {
	SHA         string
	Message     string
	AuthorName  string
	AuthorLogin string
	Timestamp   time.Time
}
