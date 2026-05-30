package config_test

import (
	"testing"

	"diff-lens/internal/config"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("LLM_BASE_URL", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	cfg := config.Load()

	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
	}
	if cfg.LLMBaseURL != "https://api.deepseek.com" {
		t.Fatalf("LLMBaseURL = %q, want default DeepSeek endpoint", cfg.LLMBaseURL)
	}
	if cfg.LLMModel != "deepseek-chat" {
		t.Fatalf("LLMModel = %q, want %q", cfg.LLMModel, "deepseek-chat")
	}
}

func TestLoadReadsEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("GITHUB_TOKEN", "ghp_test")
	t.Setenv("LLM_BASE_URL", "https://llm.example.com")
	t.Setenv("LLM_API_KEY", "sk_test")
	t.Setenv("LLM_MODEL", "qwen-plus")

	cfg := config.Load()

	if cfg.Port != "9090" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.GitHubToken != "ghp_test" {
		t.Fatalf("GitHubToken was not loaded")
	}
	if cfg.LLMBaseURL != "https://llm.example.com" {
		t.Fatalf("LLMBaseURL = %q, want env value", cfg.LLMBaseURL)
	}
	if cfg.LLMAPIKey != "sk_test" {
		t.Fatalf("LLMAPIKey was not loaded")
	}
	if cfg.LLMModel != "qwen-plus" {
		t.Fatalf("LLMModel = %q, want %q", cfg.LLMModel, "qwen-plus")
	}
}
