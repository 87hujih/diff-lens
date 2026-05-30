package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

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
	status := review.DonePayload{OK: false}
	payload := review.ErrorPayload{
		Code:        "analysis_failed",
		Message:     err.Error(),
		Recoverable: false,
	}

	if errors.Is(err, review.ErrRealAnalysisNotImplemented) {
		payload.Code = "real_analysis_not_implemented"
		payload.Recoverable = true
	}

	_ = WriteSSE(c.Writer, review.ReviewEvent{Type: review.EventError, Data: payload})
	_ = WriteSSE(c.Writer, review.ReviewEvent{Type: review.EventDone, Data: status})
	c.Writer.Flush()
}
