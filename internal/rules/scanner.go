package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"diff-lens/internal/diff"
)

const (
	defaultSeverity   = "medium"
	defaultConfidence = 0.8
	maxEvidenceLength = 240
)

var secretAssignmentPattern = regexp.MustCompile(`(?i)\b([a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*)(\s*[:=]\s*["']?)([a-z0-9_./+=-]{8,})(["']?)`)

type ruleFunc func(AddedLine) []Finding

// AddedLine is the normalized added line context passed to deterministic rules.
type AddedLine struct {
	File    string
	Line    int
	Content string
}

// Scanner runs deterministic rules over parsed diff analysis.
type Scanner struct {
	rules []ruleFunc
}

// NewScanner returns a stateless scanner. Concrete rules are added in later tasks.
func NewScanner() *Scanner {
	return &Scanner{}
}

func newScannerWithRules(rules ...ruleFunc) *Scanner {
	return &Scanner{rules: rules}
}

// Scan returns rules findings derived from added lines in the diff analysis.
func (s *Scanner) Scan(analysis diff.Analysis) []Finding {
	if len(s.rules) == 0 {
		return nil
	}

	var findings []Finding
	seen := make(map[string]struct{})

	for _, file := range analysis.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Kind != diff.DiffLineAdded {
					continue
				}

				added := AddedLine{
					File:    file.Filename,
					Line:    line.NewLine,
					Content: line.Content,
				}
				for _, rule := range s.rules {
					for _, finding := range rule(added) {
						key := dedupeKey(finding)
						if _, ok := seen[key]; ok {
							continue
						}
						seen[key] = struct{}{}
						findings = append(findings, finding)
					}
				}
			}
		}
	}

	return findings
}

func newFinding(ruleID, category, title string, line AddedLine, evidence, reason, suggestion string) Finding {
	return Finding{
		ID:             findingID(ruleID, line.File, line.Line, evidence),
		RuleID:         ruleID,
		Severity:       defaultSeverity,
		Confidence:     defaultConfidence,
		Category:       category,
		Title:          title,
		File:           line.File,
		Line:           line.Line,
		MaskedEvidence: truncateEvidence(maskSensitiveEvidence(evidence)),
		Reason:         reason,
		Suggestion:     suggestion,
	}
}

func findingID(ruleID, file string, line int, evidence string) string {
	normalized := normalizeEvidence(evidence)
	sum := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%s:%s:%d:%s", ruleID, file, line, hex.EncodeToString(sum[:])[:12])
}

func dedupeKey(finding Finding) string {
	return fmt.Sprintf("%s:%d:%s", finding.File, finding.Line, finding.Category)
}

func normalizeEvidence(evidence string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(evidence)), " ")
}

func truncateEvidence(evidence string) string {
	if len(evidence) <= maxEvidenceLength {
		return evidence
	}
	if maxEvidenceLength <= 3 {
		return evidence[:maxEvidenceLength]
	}
	return evidence[:maxEvidenceLength-3] + "..."
}

func maskSensitiveEvidence(evidence string) string {
	return secretAssignmentPattern.ReplaceAllString(evidence, "$1$2<masked>$4")
}
