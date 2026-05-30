package review

import "context"

const ReviewContextSchemaVersion = "review-context/v1"

// ReviewContext is the bounded, structured payload passed to AI analyzers.
type ReviewContext struct {
	SchemaVersion string          `json:"schema_version"`
	ContextID     string          `json:"context_id"`
	PR            PRInfo          `json:"pr"`
	Commits       []ContextCommit `json:"commits,omitempty"`
	Stats         ContextStats    `json:"stats"`
	RuleRisks     []Risk          `json:"rule_risks"`
	Files         []ContextFile   `json:"files"`
	EvidenceRefs  []string        `json:"evidence_refs"`
}

// ContextCommit is the model-safe commit summary kept in review context.
type ContextCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  string `json:"author,omitempty"`
}

// ContextFile summarizes one changed file and its bounded evidence snippets.
type ContextFile struct {
	Filename     string           `json:"filename"`
	Kind         string           `json:"kind"`
	Status       string           `json:"status"`
	Additions    int              `json:"additions"`
	Deletions    int              `json:"deletions"`
	RiskIDs      []string         `json:"risk_ids,omitempty"`
	Snippets     []ContextSnippet `json:"snippets"`
	PatchOmitted bool             `json:"patch_omitted,omitempty"`
}

// ContextSnippet is a stable, redacted diff fragment that AI risks may cite.
type ContextSnippet struct {
	ID        string `json:"id"`
	File      string `json:"file"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Patch     string `json:"patch"`
	Reason    string `json:"reason"`
}

// ContextStats records context composition and truncation decisions.
type ContextStats struct {
	ChangedFiles          int            `json:"changed_files"`
	Additions             int            `json:"additions"`
	Deletions             int            `json:"deletions"`
	TestFiles             int            `json:"test_files,omitempty"`
	ConfigFiles           int            `json:"config_files,omitempty"`
	DependencyFiles       int            `json:"dependency_files,omitempty"`
	CIFiles               int            `json:"ci_files,omitempty"`
	SourceFiles           int            `json:"source_files,omitempty"`
	FilesByKind           map[string]int `json:"files_by_kind,omitempty"`
	Truncated             bool           `json:"truncated"`
	OmittedFilesCount     int            `json:"omitted_files_count,omitempty"`
	OmittedSnippetsCount  int            `json:"omitted_snippets_count,omitempty"`
	MetadataTruncated     bool           `json:"metadata_truncated,omitempty"`
	RulesTruncated        bool           `json:"rules_truncated,omitempty"`
	FileSummaryTruncated  bool           `json:"file_summary_truncated,omitempty"`
	SnippetsTruncated     bool           `json:"snippets_truncated,omitempty"`
	PromptWrapperReserved int            `json:"prompt_wrapper_reserved,omitempty"`
}

// AIAnalyzer is implemented by adapters outside review, such as internal/llm.
type AIAnalyzer interface {
	Analyze(ctx context.Context, input ReviewContext) (ReviewAnalysis, error)
}

// ReviewAnalysis is the parsed, structured AI analysis output.
type ReviewAnalysis struct {
	Summary        string            `json:"summary"`
	Risks          []AIRisk          `json:"risks"`
	Comments       []AnalysisComment `json:"comments"`
	AttentionItems []string          `json:"attention_items,omitempty"`
	Meta           AnalysisMeta      `json:"meta,omitempty"`
}

// AIRisk is an AI-produced risk. EvidenceRefs must cite ReviewContext refs.
type AIRisk struct {
	ID           string   `json:"id"`
	Severity     string   `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Category     string   `json:"category"`
	Title        string   `json:"title"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	RuleID       string   `json:"rule_id,omitempty"`
	EvidenceRefs []string `json:"evidence_refs"`
	Reason       string   `json:"reason"`
	Suggestion   string   `json:"suggestion"`
}

// AnalysisComment is an AI-suggested review comment.
type AnalysisComment struct {
	ID           string   `json:"id"`
	File         string   `json:"file,omitempty"`
	Line         int      `json:"line,omitempty"`
	Body         string   `json:"body"`
	EvidenceRefs []string `json:"evidence_refs,omitempty"`
}

// AnalysisMeta is analyzer-specific metadata that can feed ReportMeta.
type AnalysisMeta struct {
	Completed      bool   `json:"completed"`
	DegradedReason string `json:"degraded_reason,omitempty"`
}
