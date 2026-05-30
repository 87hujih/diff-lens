package config

import "os"

// Config 汇总 HTTP 服务和后续分析器需要的运行时配置。
type Config struct {
	Port        string
	GitHubToken string
	LLMBaseURL  string
	LLMAPIKey   string
	LLMModel    string
}

// Load 读取进程环境变量，并应用本地开发默认值。
func Load() Config {
	return Config{
		Port:        envOrDefault("PORT", "8080"),
		GitHubToken: os.Getenv("GITHUB_TOKEN"),
		LLMBaseURL:  envOrDefault("LLM_BASE_URL", "https://api.deepseek.com"),
		LLMAPIKey:   os.Getenv("LLM_API_KEY"),
		LLMModel:    envOrDefault("LLM_MODEL", "deepseek-chat"),
	}
}

// envOrDefault 统一处理环境变量缺省值，避免重复 fallback 逻辑。
func envOrDefault(key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
