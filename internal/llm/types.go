package llm

import (
	"errors"
	"net/http"
	"time"
)

var (
	ErrNotConfigured      = errors.New("llm analyzer not configured")
	ErrRequestFailed      = errors.New("llm request failed")
	ErrResponseInvalid    = errors.New("llm response invalid")
	ErrModelOutputInvalid = errors.New("llm model output invalid")
)

type AnalyzerError struct {
	Err    error
	Reason string
	Detail string
}

func (e AnalyzerError) Error() string {
	if e.Detail != "" {
		return e.Err.Error() + ": " + e.Detail
	}
	return e.Err.Error()
}

func (e AnalyzerError) Unwrap() error {
	return e.Err
}

func (e AnalyzerError) DegradedReason() string {
	return e.Reason
}

type AnalyzerOptions struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Timeout    time.Duration
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   *bool         `json:"stream,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []chatCompletionChoice `json:"choices"`
}

type chatCompletionChoice struct {
	Message chatMessage `json:"message"`
}
