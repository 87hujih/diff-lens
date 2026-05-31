package main

import (
	"log"

	"diff-lens/internal/config"
	"diff-lens/internal/demo"
	"diff-lens/internal/diff"
	"diff-lens/internal/github"
	"diff-lens/internal/handler"
	"diff-lens/internal/llm"
	"diff-lens/internal/review"
	"diff-lens/internal/rules"
)

// main 组装配置、服务依赖和 HTTP 路由后启动进程。
func main() {
	// 将支持演示模式的评审服务接入 HTTP 路由。
	cfg := config.Load()
	service := review.NewService(review.ServiceOptions{
		DemoProvider:       demo.NewProvider(),
		DefaultGitHubToken: cfg.GitHubToken,
		GitHubClientFactory: func(token string) review.GitHubClient {
			return github.NewClientWithOptions(github.ClientOptions{
				Token:   token,
				Timeout: cfg.GitHubTimeout,
			})
		},
		DiffParser:     diff.NewParser(),
		RuleScanner:    rules.NewScanner(),
		ContextBuilder: review.NewContextBuilder(review.ContextBuilderOptions{}),
		AIAnalyzer: llm.NewAnalyzerWithOptions(llm.AnalyzerOptions{
			BaseURL: cfg.LLMBaseURL,
			APIKey:  cfg.LLMAPIKey,
			Model:   cfg.LLMModel,
			Timeout: cfg.LLMTimeout,
		}),
		ReportGenerator: review.NewReportNormalizer(),
	})
	router := handler.NewRouter(service)

	// Run 在配置端口开始监听后，请求生命周期由 Gin 接管。
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
