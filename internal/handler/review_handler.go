package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"diff-lens/internal/github"
	"diff-lens/internal/review"
)

// ReviewHandler 将 HTTP 请求适配到 review 服务的流式契约。
type ReviewHandler struct {
	service *review.Service
}

// NewReviewHandler 注入 HTTP handler 需要的服务依赖。
func NewReviewHandler(service *review.Service) *ReviewHandler {
	return &ReviewHandler{service: service}
}

// AnalyzeStream 校验请求体、打开 SSE 响应，并转发分析事件。
func (h *ReviewHandler) AnalyzeStream(c *gin.Context) {
	var req review.AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求体无效"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	events, err := h.service.Analyze(c.Request.Context(), req)
	if err != nil {
		h.writeError(c, err)
		return
	}

	for event := range events {
		if err := WriteSSE(c.Writer, event); err != nil {
			return
		}
		c.Writer.Flush()
	}
}

// writeError 让服务错误也沿用成功流相同的 SSE 协议返回。
func (h *ReviewHandler) writeError(c *gin.Context, err error) {
	payload := errorPayloadFromError(err)
	_ = WriteSSE(c.Writer, review.ReviewEvent{Type: review.EventError, Data: payload})
	_ = WriteSSE(c.Writer, review.ReviewEvent{Type: review.EventDone, Data: review.DonePayload{OK: false}})
	c.Writer.Flush()
}

func errorPayloadFromError(err error) review.ErrorPayload {
	var analysisErr *review.AnalysisError
	if errors.As(err, &analysisErr) && analysisErr.Code != "" {
		return review.ErrorPayload{
			Code:        analysisErr.Code,
			Message:     messageForCode(analysisErr.Code, analysisErr.Message),
			Recoverable: analysisErr.Recoverable,
			Stage:       analysisErr.Stage,
		}
	}

	switch {
	case errors.Is(err, github.ErrInvalidPRURL):
		return mappedErrorPayload(
			"invalid_pr_url",
			"fetch_pr",
			"GitHub Pull Request URL 无效",
			true,
		)
	case errors.Is(err, github.ErrPRNotFound):
		return mappedErrorPayload(
			"github_pr_not_found",
			"fetch_pr",
			"未找到 GitHub Pull Request；请确认 URL 指向存在的 PR。",
			true,
		)
	case errors.Is(err, github.ErrGitHubUnauthorized):
		return mappedErrorPayload(
			"github_unauthorized",
			"fetch_pr",
			"GitHub 鉴权失败；请配置有效 token 后重试。",
			true,
		)
	case errors.Is(err, github.ErrGitHubRateLimited):
		return mappedErrorPayload(
			"github_rate_limited",
			"fetch_pr",
			"已达到 GitHub API 速率限制；请配置 token 或稍后重试。",
			true,
		)
	case errors.Is(err, github.ErrGitHubRequestFailed):
		return mappedErrorPayload(
			"github_request_failed",
			"fetch_pr",
			"GitHub 请求失败；请检查网络连接后重试。",
			true,
		)
	case errors.Is(err, github.ErrGitHubResponseInvalid):
		return mappedErrorPayload(
			"github_response_invalid",
			"fetch_pr",
			"GitHub 返回了无效响应；请在上游恢复后重试。",
			false,
		)
	default:
		return mappedErrorPayload(
			"analysis_failed",
			"",
			"生成报告前分析失败。",
			false,
		)
	}
}

func mappedErrorPayload(code string, stage string, message string, recoverable bool) review.ErrorPayload {
	return review.ErrorPayload{
		Code:        code,
		Message:     message,
		Recoverable: recoverable,
		Stage:       stage,
	}
}

func messageForCode(code string, fallback string) string {
	if fallback != "" {
		return fallback
	}

	switch code {
	case "invalid_pr_url":
		return "GitHub Pull Request URL 无效"
	case "github_pr_not_found":
		return "未找到 GitHub Pull Request；请确认 URL 指向存在的 PR。"
	case "github_unauthorized":
		return "GitHub 鉴权失败；请配置有效 token 后重试。"
	case "github_rate_limited":
		return "已达到 GitHub API 速率限制；请配置 token 或稍后重试。"
	case "github_request_failed":
		return "GitHub 请求失败；请检查网络连接后重试。"
	case "github_response_invalid":
		return "GitHub 返回了无效响应；请在上游恢复后重试。"
	default:
		return "生成报告前分析失败。"
	}
}
