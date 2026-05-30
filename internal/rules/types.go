package rules

const (
	RuleSecret             = "secret"
	RuleDangerousOperation = "dangerous_operation"
	RuleTestGap            = "test_gap"
	RuleLargePR            = "large_pr"
	RuleConfigChange       = "config_dependency_change"

	SeverityLow    = "low"
	SeverityMedium = "medium"
	SeverityHigh   = "high"
)

// Finding is a deterministic rule scanner result. It is intentionally
// independent from review.Risk so this package stays reusable.
type Finding struct {
	ID             string  `json:"id"`
	RuleID         string  `json:"rule_id"`
	Severity       string  `json:"severity"`
	Confidence     float64 `json:"confidence"`
	Category       string  `json:"category"`
	Title          string  `json:"title"`
	File           string  `json:"file,omitempty"`
	Line           int     `json:"line,omitempty"`
	MaskedEvidence string  `json:"masked_evidence,omitempty"`
	Reason         string  `json:"reason"`
	Suggestion     string  `json:"suggestion"`
}
