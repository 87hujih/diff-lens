package llm

// Analyzer 保存 OpenAI 兼容模型配置，供后续 LLM review 调用使用。
type Analyzer struct {
	baseURL string
	apiKey  string
	model   string
}

// NewAnalyzer 将 LLM 传输配置集中到一个构造入口。
func NewAnalyzer(baseURL string, apiKey string, model string) *Analyzer {
	return &Analyzer{baseURL: baseURL, apiKey: apiKey, model: model}
}
