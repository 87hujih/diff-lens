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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
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
			"invalid GitHub pull request URL",
			true,
		)
	case errors.Is(err, github.ErrPRNotFound):
		return mappedErrorPayload(
			"github_pr_not_found",
			"fetch_pr",
			"GitHub pull request was not found; check that the URL points to an existing PR.",
			true,
		)
	case errors.Is(err, github.ErrGitHubUnauthorized):
		return mappedErrorPayload(
			"github_unauthorized",
			"fetch_pr",
			"GitHub authentication failed; configure a valid token and retry.",
			true,
		)
	case errors.Is(err, github.ErrGitHubRateLimited):
		return mappedErrorPayload(
			"github_rate_limited",
			"fetch_pr",
			"GitHub API rate limit was reached; configure a token or retry later.",
			true,
		)
	case errors.Is(err, github.ErrGitHubRequestFailed):
		return mappedErrorPayload(
			"github_request_failed",
			"fetch_pr",
			"GitHub request failed; check the network connection and retry.",
			true,
		)
	case errors.Is(err, github.ErrGitHubResponseInvalid):
		return mappedErrorPayload(
			"github_response_invalid",
			"fetch_pr",
			"GitHub returned an invalid response; retry after the upstream response is healthy.",
			false,
		)
	default:
		return mappedErrorPayload(
			"analysis_failed",
			"",
			"Analysis failed before a report could be produced.",
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
		return "invalid GitHub pull request URL"
	case "github_pr_not_found":
		return "GitHub pull request was not found; check that the URL points to an existing PR."
	case "github_unauthorized":
		return "GitHub authentication failed; configure a valid token and retry."
	case "github_rate_limited":
		return "GitHub API rate limit was reached; configure a token or retry later."
	case "github_request_failed":
		return "GitHub request failed; check the network connection and retry."
	case "github_response_invalid":
		return "GitHub returned an invalid response; retry after the upstream response is healthy."
	default:
		return "Analysis failed before a report could be produced."
	}
}
