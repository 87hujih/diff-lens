package github

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// ErrInvalidPRURL 表示用户输入的 URL 不是合法的 GitHub PR URL。
var ErrInvalidPRURL = errors.New("invalid GitHub pull request URL")

// PRRef 是标准化后的仓库和 PR 标识。
type PRRef struct {
	Owner  string
	Repo   string
	Number int
}

// Client 后续负责真实分析中的 GitHub API 认证访问。
type Client struct {
	token string
}

// NewClient 保存可选 token，供后续 GitHub REST 请求使用。
func NewClient(token string) *Client {
	return &Client{token: token}
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
