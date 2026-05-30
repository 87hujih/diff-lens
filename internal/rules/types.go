package rules

// Finding is a deterministic rules scanner result. The review layer maps this
// package-owned model to review.Risk after scanning.
type Finding struct {
	ID             string
	RuleID         string
	Severity       string
	Confidence     float64
	Category       string
	Title          string
	File           string
	Line           int
	MaskedEvidence string
	Reason         string
	Suggestion     string
}
