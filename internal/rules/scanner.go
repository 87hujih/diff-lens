package rules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"diff-lens/internal/diff"
)

const (
	largePRFileThreshold = 25
	largePRLineThreshold = 1000
	maxEvidenceLength    = 180
)

var (
	secretAssignmentPattern = regexp.MustCompile(`(?i)\b([a-z0-9_-]*(?:password|api[_-]?key|secret|token|private[_-]?key)[a-z0-9_-]*)\b(\s*[:=]\s*)(['"]?)([^'"\s]+)`)
	privateKeyPattern       = regexp.MustCompile(`(?i)private key|begin [a-z ]*private key`)
	rmRFPattern             = regexp.MustCompile(`(?i)\brm\s+-[a-z]*r[a-z]*f|rm\s+-[a-z]*f[a-z]*r`)
	dropTablePattern        = regexp.MustCompile(`(?i)\bdrop\s+table\b`)
	truncatePattern         = regexp.MustCompile(`(?i)\btruncate\b`)
	deleteFromPattern       = regexp.MustCompile(`(?i)\bdelete\s+from\b`)
	wherePattern            = regexp.MustCompile(`(?i)\bwhere\b`)
	chmod777Pattern         = regexp.MustCompile(`(?i)\bchmod\s+777\b`)
	forcePushPattern        = regexp.MustCompile(`(?i)\bgit\s+push\b.*(?:--force(?:-with-lease)?\b|\s-f\b)`)
)

// Scanner runs deterministic checks before any LLM-based analysis.
type Scanner struct{}

// NewScanner returns a stateless scanner.
func NewScanner() *Scanner {
	return &Scanner{}
}

// Scan returns deterministic findings for the supplied diff analysis.
func (s *Scanner) Scan(analysis diff.Analysis) []Finding {
	var findings []Finding
	seen := make(map[string]struct{})

	for _, file := range analysis.Files {
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				if line.Type != diff.DiffLineAdded {
					continue
				}
				if finding, ok := secretFinding(file.Filename, line); ok {
					addFinding(&findings, seen, finding)
				}
				if finding, ok := dangerousOperationFinding(file.Filename, line); ok {
					addFinding(&findings, seen, finding)
				}
			}
		}

		if finding, ok := configFinding(file); ok {
			addFinding(&findings, seen, finding)
		}
	}

	if finding, ok := testGapFinding(analysis); ok {
		addFinding(&findings, seen, finding)
	}
	if finding, ok := largePRFinding(analysis); ok {
		addFinding(&findings, seen, finding)
	}

	sort.Slice(findings, func(i, j int) bool {
		return findings[i].ID < findings[j].ID
	})
	return findings
}

func secretFinding(filename string, line diff.DiffLine) (Finding, bool) {
	content := strings.TrimSpace(line.Content)
	masked, ok := maskSecret(content)
	if !ok {
		return Finding{}, false
	}
	finding := Finding{
		RuleID:         RuleSecret,
		Severity:       SeverityHigh,
		Confidence:     0.82,
		Category:       "secret",
		Title:          "Possible secret added",
		File:           filename,
		Line:           line.NewLine,
		MaskedEvidence: masked,
		Reason:         "The added line looks like it contains a credential or private key material.",
		Suggestion:     "Remove the secret from the patch, rotate it if it was real, and load it from a secret manager or environment variable.",
	}
	finding.ID = stableID(finding, content)
	return finding, true
}

func dangerousOperationFinding(filename string, line diff.DiffLine) (Finding, bool) {
	content := strings.TrimSpace(line.Content)
	if !isDangerousOperation(content) {
		return Finding{}, false
	}
	finding := Finding{
		RuleID:         RuleDangerousOperation,
		Severity:       SeverityHigh,
		Confidence:     0.78,
		Category:       "dangerous_operation",
		Title:          "Dangerous operation added",
		File:           filename,
		Line:           line.NewLine,
		MaskedEvidence: truncateEvidence(content),
		Reason:         "The added line contains a destructive shell, SQL, permission, or force-push operation.",
		Suggestion:     "Add explicit safeguards, narrow the target, require confirmation, or replace the operation with a reversible migration path.",
	}
	finding.ID = stableID(finding, content)
	return finding, true
}

func testGapFinding(analysis diff.Analysis) (Finding, bool) {
	if !analysis.Stats.HasSourceChanges || analysis.Stats.HasTestChanges {
		return Finding{}, false
	}
	finding := Finding{
		RuleID:     RuleTestGap,
		Severity:   SeverityMedium,
		Confidence: 0.72,
		Category:   "test_coverage",
		Title:      "Source changes without test changes",
		Reason:     "The diff changes source files but does not include test file changes.",
		Suggestion: "Add or update focused tests for the changed behavior, or explain why existing coverage is sufficient.",
	}
	finding.ID = stableID(finding, fmt.Sprintf("source:%d:test:%d", analysis.Stats.SourceFiles, analysis.Stats.TestFiles))
	return finding, true
}

func largePRFinding(analysis diff.Analysis) (Finding, bool) {
	totalLines := analysis.Stats.Additions + analysis.Stats.Deletions
	if analysis.Stats.ChangedFiles <= largePRFileThreshold && totalLines <= largePRLineThreshold {
		return Finding{}, false
	}

	evidence := fmt.Sprintf("%d files, +%d/-%d lines", analysis.Stats.ChangedFiles, analysis.Stats.Additions, analysis.Stats.Deletions)
	finding := Finding{
		RuleID:         RuleLargePR,
		Severity:       SeverityMedium,
		Confidence:     0.88,
		Category:       "change_size",
		Title:          "Large pull request",
		MaskedEvidence: evidence,
		Reason:         "The diff is large enough to raise review and regression risk.",
		Suggestion:     "Consider splitting unrelated changes, or provide a concise review guide and rollout plan.",
	}
	finding.ID = stableID(finding, evidence)
	return finding, true
}

func configFinding(file diff.FileDiff) (Finding, bool) {
	if !isConfigRiskFile(file) {
		return Finding{}, false
	}
	evidence := file.Filename
	finding := Finding{
		RuleID:         RuleConfigChange,
		Severity:       SeverityMedium,
		Confidence:     0.7,
		Category:       "configuration",
		Title:          "Configuration or dependency file changed",
		File:           file.Filename,
		MaskedEvidence: evidence,
		Reason:         "The changed file can affect build, deployment, runtime configuration, dependencies, or CI behavior.",
		Suggestion:     "Review the operational impact, lockfile consistency, permissions, and rollback path for this change.",
	}
	finding.ID = stableID(finding, evidence)
	return finding, true
}

func maskSecret(content string) (string, bool) {
	if secretAssignmentPattern.MatchString(content) {
		masked := secretAssignmentPattern.ReplaceAllString(content, `$1$2$3[REDACTED]`)
		return truncateEvidence(masked), true
	}
	if privateKeyPattern.MatchString(content) {
		return truncateEvidence("PRIVATE KEY [REDACTED]"), true
	}
	return "", false
}

func isDangerousOperation(content string) bool {
	return rmRFPattern.MatchString(content) ||
		dropTablePattern.MatchString(content) ||
		truncatePattern.MatchString(content) ||
		(deleteFromPattern.MatchString(content) && !wherePattern.MatchString(content)) ||
		chmod777Pattern.MatchString(content) ||
		forcePushPattern.MatchString(content)
}

func isConfigRiskFile(file diff.FileDiff) bool {
	lower := strings.ToLower(file.Filename)
	if strings.HasPrefix(strings.TrimPrefix(lower, "./"), ".env") ||
		strings.HasSuffix(lower, "/.env") ||
		strings.Contains(lower, "/.env.") ||
		strings.HasSuffix(lower, "dockerfile") ||
		strings.Contains(lower, "/dockerfile.") {
		return true
	}
	for _, kind := range file.Kinds {
		switch kind {
		case diff.FileKindCI, diff.FileKindDependency, diff.FileKindLockfile:
			return true
		case diff.FileKindConfig:
			return true
		}
	}
	return false
}

func addFinding(findings *[]Finding, seen map[string]struct{}, finding Finding) {
	key := fmt.Sprintf("%s:%s:%d:%s", finding.File, finding.Category, finding.Line, finding.RuleID)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*findings = append(*findings, finding)
}

func stableID(finding Finding, evidence string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(evidence)), " ")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d|%s", finding.RuleID, finding.File, finding.Line, normalized)))
	return finding.RuleID + "-" + hex.EncodeToString(sum[:])[:12]
}

func truncateEvidence(evidence string) string {
	if len(evidence) <= maxEvidenceLength {
		return evidence
	}
	return evidence[:maxEvidenceLength] + "..."
}
