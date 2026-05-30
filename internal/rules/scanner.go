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

	largePRFileCountThreshold = 50
	largePRChangeThreshold    = 1000
)

var (
	sensitiveKeyPatternText = `[a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*|private\s+key`

	doubleQuotedConfigSecretPattern = regexp.MustCompile(`(?i)("(?:` + sensitiveKeyPatternText + `)")(\s*(?::=|[:=])\s*")([^"\r\n]*)(")`)
	singleQuotedConfigSecretPattern = regexp.MustCompile(`(?i)('(?:` + sensitiveKeyPatternText + `)')(\s*(?::=|[:=])\s*')([^'\r\n]*)(')`)
	doubleQuotedSecretPattern       = regexp.MustCompile(`(?i)\b([a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*)(\s*(?::=|[:=])\s*")([^"\r\n]*)(")`)
	singleQuotedSecretPattern       = regexp.MustCompile(`(?i)\b([a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*)(\s*(?::=|[:=])\s*')([^'\r\n]*)(')`)
	unquotedSecretPattern           = regexp.MustCompile(`(?i)\b([a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*)(\s*(?::=|[:=])\s*)([^\r\n]*)`)
	doubleQuotedPrivateKeyPattern   = regexp.MustCompile(`(?i)\b(private\s+key)(\s*(?::=|[:=])\s*")([^"\r\n]*)(")`)
	singleQuotedPrivateKeyPattern   = regexp.MustCompile(`(?i)\b(private\s+key)(\s*(?::=|[:=])\s*')([^'\r\n]*)(')`)
	unquotedPrivateKeyPattern       = regexp.MustCompile(`(?i)\b(private\s+key)(\s*(?::=|[:=])\s*)([^\r\n]*)`)
	sensitiveLinePattern            = regexp.MustCompile(`(?i)\b(?:[a-z0-9_.-]*(?:secret|token|password|passwd|pwd|api[_-]?key|access[_-]?key)[a-z0-9_.-]*|private\s+key)\b`)

	rmRFPattern       = regexp.MustCompile(`(?i)(?:^|[;&|(\s])rm\s+(?:-[^\s\r\n]*r[^\s\r\n]*f[^\s\r\n]*|-[^\s\r\n]*f[^\s\r\n]*r[^\s\r\n]*|-[^\s\r\n]*r[^\s\r\n]*(?:\s+\S+)*\s+-[^\s\r\n]*f[^\s\r\n]*|-[^\s\r\n]*f[^\s\r\n]*(?:\s+\S+)*\s+-[^\s\r\n]*r[^\s\r\n]*)\b`)
	dropTablePattern  = regexp.MustCompile(`(?i)\bdrop\s+table\b`)
	truncatePattern   = regexp.MustCompile(`(?i)\btruncate(?:\s+table)?\b`)
	deleteFromPattern = regexp.MustCompile(`(?i)\bdelete\s+from\b`)
	wherePattern      = regexp.MustCompile(`(?i)\bwhere\b`)
	chmod777Pattern   = regexp.MustCompile(`(?i)\bchmod\s+777\b`)
	forcePushPattern  = regexp.MustCompile(`(?i)\bgit\s+push\b.*(?:--force(?:-with-lease)?|-f)\b`)
)

type ruleFunc func(AddedLine) []Finding
type analysisRuleFunc func(diff.Analysis) []Finding

// AddedLine is the normalized added line context passed to deterministic rules.
type AddedLine struct {
	File    string
	Line    int
	Content string
}

// Scanner runs deterministic rules over parsed diff analysis.
type Scanner struct {
	rules         []ruleFunc
	analysisRules []analysisRuleFunc
}

// NewScanner returns a stateless scanner with the built-in deterministic rules.
func NewScanner() *Scanner {
	return &Scanner{
		rules: []ruleFunc{
			sensitiveInformationRule,
			dangerousOperationRule,
		},
		analysisRules: []analysisRuleFunc{
			testGapRule,
			largePRRule,
			configDependencyRule,
		},
	}
}

func newScannerWithRules(rules ...ruleFunc) *Scanner {
	return &Scanner{rules: rules}
}

// Scan returns rules findings derived from added lines in the diff analysis.
func (s *Scanner) Scan(analysis diff.Analysis) []Finding {
	if len(s.rules) == 0 && len(s.analysisRules) == 0 {
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
						findings = appendFinding(findings, seen, finding)
					}
				}
			}
		}
	}

	for _, rule := range s.analysisRules {
		for _, finding := range rule(analysis) {
			findings = appendFinding(findings, seen, finding)
		}
	}

	return findings
}

func sensitiveInformationRule(line AddedLine) []Finding {
	if !sensitiveLinePattern.MatchString(line.Content) {
		return nil
	}

	return []Finding{newFindingWithSeverity(
		"security.sensitive-information",
		"medium",
		0.75,
		"security",
		"Sensitive information added",
		line,
		line.Content,
		"Added code appears to include a credential or private key reference.",
		"Move secrets to a managed secret store or environment-specific configuration and rotate any exposed value.",
	)}
}

func dangerousOperationRule(line AddedLine) []Finding {
	if !isDangerousOperation(line.Content) {
		return nil
	}

	return []Finding{newFindingWithSeverity(
		"operations.dangerous-command",
		"medium",
		0.8,
		"dangerous-operation",
		"Dangerous operation added",
		line,
		line.Content,
		"Added code contains an operation that can delete data, weaken permissions, or rewrite shared history.",
		"Add explicit safeguards, scope limits, backups, dry-run behavior, or review gates before running this operation.",
	)}
}

func isDangerousOperation(content string) bool {
	if rmRFPattern.MatchString(content) ||
		dropTablePattern.MatchString(content) ||
		truncatePattern.MatchString(content) ||
		chmod777Pattern.MatchString(content) ||
		forcePushPattern.MatchString(content) {
		return true
	}
	return deleteFromPattern.MatchString(content) && !wherePattern.MatchString(content)
}

func testGapRule(analysis diff.Analysis) []Finding {
	if !analysis.Stats.HasSourceChanges || analysis.Stats.HasTestChanges {
		return nil
	}

	line := AddedLine{File: firstFileWithKind(analysis.Files, diff.FileKindSource), Content: "source changes without test changes"}
	return []Finding{newFindingWithSeverity(
		"quality.test-gap",
		"medium",
		0.7,
		"testing",
		"Source changes without test updates",
		line,
		line.Content,
		"Source files changed, but no test files changed in this diff.",
		"Add or update focused tests, or explain why existing coverage is sufficient.",
	)}
}

func largePRRule(analysis diff.Analysis) []Finding {
	totalChanges := analysis.Stats.Additions + analysis.Stats.Deletions
	if analysis.Stats.ChangedFiles <= largePRFileCountThreshold && totalChanges <= largePRChangeThreshold {
		return nil
	}

	evidence := fmt.Sprintf("changed files: %d, total line changes: %d", analysis.Stats.ChangedFiles, totalChanges)
	return []Finding{newFindingWithSeverity(
		"maintainability.large-pr",
		"medium",
		0.7,
		"change-size",
		"Large change set",
		AddedLine{Content: evidence},
		evidence,
		"Large diffs are harder to review thoroughly and carry higher regression risk.",
		"Split independent changes where practical or provide a focused review guide with testing notes.",
	)}
}

func configDependencyRule(analysis diff.Analysis) []Finding {
	var findings []Finding
	for _, file := range analysis.Files {
		if !isConfigDependencyRiskFile(file) {
			continue
		}

		line := AddedLine{File: file.Filename, Content: file.Filename}
		findings = append(findings, newFindingWithSeverity(
			"maintainability.config-dependency-change",
			"medium",
			0.75,
			"configuration",
			"Configuration or dependency file changed",
			line,
			file.Filename,
			"Changes to CI, container, environment, dependency, or lock files can affect builds and deployment behavior.",
			"Verify the build, dependency resolution, and deployment path that consume this file.",
		))
	}
	return findings
}

func isConfigDependencyRiskFile(file diff.FileDiff) bool {
	filename := normalizePath(file.Filename)
	base := pathBase(filename)

	if base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") {
		return true
	}
	if strings.HasPrefix(base, ".env") || strings.Contains(base, ".env.") {
		return true
	}
	return hasAnyKind(file.Kinds, diff.FileKindCI, diff.FileKindDependency, diff.FileKindLockfile)
}

func appendFinding(findings []Finding, seen map[string]struct{}, finding Finding) []Finding {
	key := dedupeKey(finding)
	if _, ok := seen[key]; ok {
		return findings
	}
	seen[key] = struct{}{}
	return append(findings, finding)
}

func firstFileWithKind(files []diff.FileDiff, kind diff.FileKind) string {
	for _, file := range files {
		if hasAnyKind(file.Kinds, kind) {
			return file.Filename
		}
	}
	return ""
}

func hasAnyKind(kinds []diff.FileKind, wanted ...diff.FileKind) bool {
	for _, kind := range kinds {
		for _, want := range wanted {
			if kind == want {
				return true
			}
		}
	}
	return false
}

func normalizePath(filename string) string {
	return strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))
}

func pathBase(filename string) string {
	idx := strings.LastIndex(filename, "/")
	if idx == -1 {
		return filename
	}
	return filename[idx+1:]
}

func newFinding(ruleID, category, title string, line AddedLine, evidence, reason, suggestion string) Finding {
	return newFindingWithSeverity(ruleID, defaultSeverity, defaultConfidence, category, title, line, evidence, reason, suggestion)
}

func newFindingWithSeverity(ruleID, severity string, confidence float64, category, title string, line AddedLine, evidence, reason, suggestion string) Finding {
	return Finding{
		ID:             findingID(ruleID, line.File, line.Line, evidence),
		RuleID:         ruleID,
		Severity:       severity,
		Confidence:     confidence,
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
	return fmt.Sprintf(
		"%s:%d:%s:%s:%s:%s",
		finding.File,
		finding.Line,
		finding.Category,
		finding.RuleID,
		normalizeEvidence(finding.Title),
		finding.ID,
	)
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
	masked := doubleQuotedConfigSecretPattern.ReplaceAllString(evidence, "$1$2<masked>$4")
	masked = singleQuotedConfigSecretPattern.ReplaceAllString(masked, "$1$2<masked>$4")
	masked = doubleQuotedPrivateKeyPattern.ReplaceAllString(masked, "$1$2<masked>$4")
	masked = singleQuotedPrivateKeyPattern.ReplaceAllString(masked, "$1$2<masked>$4")
	masked = unquotedPrivateKeyPattern.ReplaceAllString(masked, "$1$2<masked>")
	masked = doubleQuotedSecretPattern.ReplaceAllString(masked, "$1$2<masked>$4")
	masked = singleQuotedSecretPattern.ReplaceAllString(masked, "$1$2<masked>$4")
	return unquotedSecretPattern.ReplaceAllString(masked, "$1$2<masked>")
}
