package handler

import (
	"github.com/gin-gonic/gin"

	"diff-lens/internal/review"
)

// NewRouter 基于 review 服务依赖注册 HTTP 路由。
func NewRouter(service *review.Service) *gin.Engine {
	router := gin.Default()
	reviewHandler := NewReviewHandler(service)

	// 这个流式端点是浏览器侧渐进式分析的接口契约。
	router.POST("/api/reviews/analyze/stream", reviewHandler.AnalyzeStream)

	return router
}
