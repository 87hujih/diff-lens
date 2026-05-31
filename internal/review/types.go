package review

// EventType 定义前端 reducer 消费的 SSE 事件类型。
type EventType string

const (
	// EventStep 表示分析流水线的粗粒度进度。
	EventStep EventType = "step"
	// EventPR 携带标准化后的 PR 元数据。
	EventPR EventType = "pr"
	// EventRules 携带确定性规则扫描结果。
	EventRules EventType = "rules"
	// EventAIDelta 预留给 LLM 增量文本输出。
	EventAIDelta EventType = "ai_delta"
	// EventResult 携带最终结构化 review 报告。
	EventResult EventType = "result"
	// EventError 携带可恢复或终止性的分析错误。
	EventError EventType = "error"
	// EventDone 用成功或降级状态结束事件流。
	EventDone EventType = "done"
)

// AnalyzeRequest 是流式 review 端点接收的 JSON 请求体。
type AnalyzeRequest struct {
	PRURL       string `json:"pr_url"`
	GitHubToken string `json:"github_token,omitempty"`
	Demo        bool   `json:"demo"`
}

// ReviewEvent 是写入单条 SSE 消息的通用事件信封。
type ReviewEvent struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

// StepPayload 描述一个用户可见的分析里程碑。
type StepPayload struct {
	Step    string `json:"step"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// PRInfo 是报告头部展示的标准化 PR 元数据。
type PRInfo struct {
	Title        string `json:"title"`
	Author       string `json:"author"`
	Repo         string `json:"repo"`
	Number       int    `json:"number"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	ChangedFiles int    `json:"changed_files"`
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	Commits      int    `json:"commits"`
}

// Risk 表示来自规则、AI 或合并证据的一条 review 风险。
type Risk struct {
	ID           string   `json:"id"`
	Source       string   `json:"source"`
	Severity     string   `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Category     string   `json:"category"`
	Title        string   `json:"title"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	RuleID       string   `json:"rule_id,omitempty"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
	Evidence     string   `json:"evidence,omitempty"`
	Reason       string   `json:"reason"`
	Suggestion   string   `json:"suggestion"`
}

// RulesPayload 将规则扫描风险打包成一个流式事件。
type RulesPayload struct {
	Risks []Risk `json:"risks"`
}

// Summary 是面向 reviewer 的高层报告摘要。
type Summary struct {
	RiskLevel   string   `json:"risk_level"`
	Overview    string   `json:"overview"`
	KeyChanges  []string `json:"key_changes"`
	ReviewFocus []string `json:"review_focus"`
}

// SuggestedComment 是可直接复制到 GitHub 的 review 评论草稿。
type SuggestedComment struct {
	ID           string   `json:"id"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	Body         string   `json:"body"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// ReportMeta records which analysis stages contributed to the final report.
type ReportMeta struct {
	AICompleted          bool   `json:"ai_completed"`
	RulesCompleted       bool   `json:"rules_completed"`
	ContextTruncated     bool   `json:"context_truncated"`
	DegradedReason       string `json:"degraded_reason,omitempty"`
	OmittedFilesCount    int    `json:"omitted_files_count"`
	OmittedSnippetsCount int    `json:"omitted_snippets_count"`
}

// Report 是前端展示的最终 review 产物。
type Report struct {
	PR       PRInfo             `json:"pr"`
	Summary  Summary            `json:"summary"`
	Risks    []Risk             `json:"risks"`
	Comments []SuggestedComment `json:"comments"`
	Meta     ReportMeta         `json:"meta"`
	Degraded bool               `json:"degraded,omitempty"`
}

// DonePayload 告诉客户端事件流是否成功结束。
type DonePayload struct {
	OK       bool `json:"ok"`
	Degraded bool `json:"degraded,omitempty"`
}

// ErrorPayload 给客户端提供结构化错误，避免解析纯文本。
type ErrorPayload struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Recoverable bool   `json:"recoverable"`
	Stage       string `json:"stage,omitempty"`
}
