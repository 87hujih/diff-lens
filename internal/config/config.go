package config

import (
	"bufio"
	"os"
	"strings"
	"time"
)

// Config 汇总 HTTP 服务和后续分析器需要的运行时配置。
type Config struct {
	Port          string
	GitHubToken   string
	LLMBaseURL    string
	LLMAPIKey     string
	LLMModel      string
	GitHubTimeout time.Duration
	LLMTimeout    time.Duration
}

// Load 读取进程环境变量，并应用本地开发默认值。
func Load() Config {
	fileEnv := loadDotEnv(".env")

	return Config{
		Port:          envOrDefault(fileEnv, "PORT", "8080"),
		GitHubToken:   envOrDefault(fileEnv, "GITHUB_TOKEN", ""),
		LLMBaseURL:    envOrDefault(fileEnv, "LLM_BASE_URL", "https://api.deepseek.com"),
		LLMAPIKey:     envOrDefault(fileEnv, "LLM_API_KEY", ""),
		LLMModel:      envOrDefault(fileEnv, "LLM_MODEL", "deepseek-chat"),
		GitHubTimeout: 15 * time.Second,
		LLMTimeout:    45 * time.Second,
	}
}

// envOrDefault 统一处理环境变量缺省值，避免重复兜底逻辑。
func envOrDefault(fileEnv map[string]string, key string, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		if fileValue := fileEnv[key]; fileValue != "" {
			return fileValue
		}
		return fallback
	}
	return value
}

func loadDotEnv(path string) map[string]string {
	fileEnv := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return fileEnv
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseDotEnvLine(scanner.Text())
		if ok {
			fileEnv[key] = value
		}
	}

	return fileEnv
}

func parseDotEnvLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimPrefix(line, "export ")

	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	value = strings.TrimSpace(value)
	value = strings.Trim(value, `"'`)
	return key, value, true
}
