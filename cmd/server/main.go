package main

import (
	"log"

	"diff-lens/internal/config"
	"diff-lens/internal/demo"
	"diff-lens/internal/github"
	"diff-lens/internal/handler"
	"diff-lens/internal/review"
)

func main() {
	// 将支持演示模式的 review 服务接入 HTTP 路由。
	cfg := config.Load()
	service := review.NewService(review.ServiceOptions{
		DemoProvider:       demo.NewProvider(),
		DefaultGitHubToken: cfg.GitHubToken,
		GitHubClientFactory: func(token string) review.GitHubClient {
			return github.NewClient(token)
		},
	})
	router := handler.NewRouter(service)

	// Run 在配置端口开始监听后，请求生命周期由 Gin 接管。
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
