package rules

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"diff-lens/internal/diff"
)

func TestScanEmptyAnalysisReturnsNoFindings(t *testing.T) {
	findings := NewScanner().Scan(diff.Analysis{})
	if len(findings) != 0 {
		t.Fatalf("got %d findings, want 0", len(findings))
	}
}

func TestScanMasksSecretEvidence(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		secret string
	}{
		{name: "password", line: `password = "super-secret-password"`, secret: "super-secret-password"},
		{name: "api key", line: `api_key: "sk-live-1234567890abcdef"`, secret: "sk-live-1234567890abcdef"},
		{name: "secret", line: `client_secret = "client-secret-value"`, secret: "client-secret-value"},
		{name: "private key", line: `PRIVATE KEY-----BEGIN PRIVATE KEY-----abc`, secret: "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := NewScanner().Scan(analysisWithAddedLines("config.yml", tt.line))
			finding := requireRule(t, findings, RuleSecret)
			if finding.File != "config.yml" || finding.Line != 1 {
				t.Fatalf("unexpected location: %+v", finding)
			}
			if strings.Contains(finding.MaskedEvidence, tt.secret) {
				t.Fatalf("masked evidence leaked secret %q in %q", tt.secret, finding.MaskedEvidence)
			}
			if finding.MaskedEvidence == "" || !strings.Contains(strings.ToLower(finding.MaskedEvidence), strings.Fields(strings.ToLower(tt.line))[0]) {
				t.Fatalf("masked evidence lost useful key context: %q", finding.MaskedEvidence)
			}
		})
	}
}

func TestScanFlagsDangerousOperationsOnlyOnAddedLines(t *testing.T) {
	analysis := diff.Analysis{
		Files: []diff.FileDiff{{
			Filename: "scripts/deploy.sh",
			Kinds:    []diff.FileKind{diff.FileKindSource},
			Hunks: []diff.DiffHunk{{
				Lines: []diff.DiffLine{
					{Type: diff.DiffLineAdded, NewLine: 10, Content: "rm -rf /var/app"},
					{Type: diff.DiffLineAdded, NewLine: 11, Content: "DROP TABLE users;"},
					{Type: diff.DiffLineAdded, NewLine: 12, Content: "TRUNCATE audit_log;"},
					{Type: diff.DiffLineAdded, NewLine: 13, Content: "DELETE FROM sessions;"},
					{Type: diff.DiffLineRemoved, OldLine: 14, Content: "rm -rf /old/path"},
				},
			}},
		}},
	}

	findings := NewScanner().Scan(analysis)
	if got := countRule(findings, RuleDangerousOperation); got != 4 {
		t.Fatalf("dangerous operation findings = %d, want 4: %+v", got, findings)
	}
	for _, finding := range findings {
		if finding.RuleID == RuleDangerousOperation && finding.Line == 14 {
			t.Fatalf("removed line should not be scanned: %+v", finding)
		}
	}
}

func TestScanFlagsTestGapOnlyForSourceWithoutTests(t *testing.T) {
	sourceOnly := diff.Analysis{
		Files: []diff.FileDiff{{Filename: "internal/app/main.go", Kinds: []diff.FileKind{diff.FileKindSource}}},
		Stats: diff.FileStats{HasSourceChanges: true, SourceFiles: 1},
	}
	findings := NewScanner().Scan(sourceOnly)
	gap := requireRule(t, findings, RuleTestGap)
	if gap.Severity != SeverityMedium {
		t.Fatalf("test gap severity = %q, want %q", gap.Severity, SeverityMedium)
	}

	docsOnly := diff.Analysis{
		Files: []diff.FileDiff{{Filename: "README.md", Kinds: []diff.FileKind{diff.FileKindDocs}}},
		Stats: diff.FileStats{DocsFiles: 1},
	}
	if got := countRule(NewScanner().Scan(docsOnly), RuleTestGap); got != 0 {
		t.Fatalf("docs-only change produced %d test gap findings", got)
	}

	withTests := diff.Analysis{
		Files: []diff.FileDiff{
			{Filename: "internal/app/main.go", Kinds: []diff.FileKind{diff.FileKindSource}},
			{Filename: "internal/app/main_test.go", Kinds: []diff.FileKind{diff.FileKindTest}},
		},
		Stats: diff.FileStats{HasSourceChanges: true, HasTestChanges: true, SourceFiles: 1, TestFiles: 1},
	}
	if got := countRule(NewScanner().Scan(withTests), RuleTestGap); got != 0 {
		t.Fatalf("source with tests produced %d test gap findings", got)
	}
}

func TestScanFlagsLargePRByFileCountAndLineCount(t *testing.T) {
	largeFiles := diff.Analysis{Stats: diff.FileStats{ChangedFiles: 26, Additions: 10, Deletions: 5}}
	if got := countRule(NewScanner().Scan(largeFiles), RuleLargePR); got != 1 {
		t.Fatalf("large file count findings = %d, want 1", got)
	}

	largeLines := diff.Analysis{Stats: diff.FileStats{ChangedFiles: 3, Additions: 900, Deletions: 250}}
	if got := countRule(NewScanner().Scan(largeLines), RuleLargePR); got != 1 {
		t.Fatalf("large line count findings = %d, want 1", got)
	}
}

func TestScanFlagsConfigDependencyAndCIRisks(t *testing.T) {
	analysis := diff.Analysis{
		Files: []diff.FileDiff{
			{Filename: ".github/workflows/ci.yml", Kinds: []diff.FileKind{diff.FileKindCI, diff.FileKindConfig}},
			{Filename: "Dockerfile", Kinds: []diff.FileKind{diff.FileKindConfig}},
			{Filename: ".env.example", Kinds: []diff.FileKind{diff.FileKindConfig}},
			{Filename: "package-lock.json", Kinds: []diff.FileKind{diff.FileKindDependency, diff.FileKindLockfile}},
		},
		Stats: diff.FileStats{ChangedFiles: 4, CIFiles: 1, ConfigFiles: 3, DependencyFiles: 1, LockFiles: 1},
	}

	findings := NewScanner().Scan(analysis)
	if got := countRule(findings, RuleConfigChange); got != 4 {
		t.Fatalf("config/dependency findings = %d, want 4: %+v", got, findings)
	}
}

func TestFindingIDIsStableAndIndependentOfScanOrder(t *testing.T) {
	base := NewScanner().Scan(analysisWithAddedLines("config.yml", `password = "super-secret-password"`))
	secret := requireRule(t, base, RuleSecret)
	if secret.ID == "" {
		t.Fatal("finding ID is empty")
	}

	withUnrelated := NewScanner().Scan(analysisWithAddedLines("config.yml",
		`password = "super-secret-password"`,
		`rm -rf /tmp/app`,
	))
	matching := findingForLocation(withUnrelated, RuleSecret, "config.yml", 1)
	if matching == nil {
		t.Fatalf("secret finding not found: %+v", withUnrelated)
	}
	if matching.ID != secret.ID {
		t.Fatalf("stable ID changed after unrelated finding: got %q want %q", matching.ID, secret.ID)
	}
}

func TestRulesPackageDoesNotImportReview(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) == "scanner_test.go" {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			if imp.Path.Value == `"diff-lens/internal/review"` {
				t.Fatalf("%s imports internal/review", file)
			}
		}
	}
}

func analysisWithAddedLines(filename string, lines ...string) diff.Analysis {
	diffLines := make([]diff.DiffLine, 0, len(lines))
	for i, line := range lines {
		diffLines = append(diffLines, diff.DiffLine{
			Type:    diff.DiffLineAdded,
			NewLine: i + 1,
			Content: line,
		})
	}
	return diff.Analysis{
		Files: []diff.FileDiff{{
			Filename: filename,
			Kinds:    diff.ClassifyFile(filename),
			Hunks:    []diff.DiffHunk{{Lines: diffLines}},
		}},
		Stats: diff.FileStats{ChangedFiles: 1},
	}
}

func requireRule(t *testing.T, findings []Finding, ruleID string) Finding {
	t.Helper()
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			return finding
		}
	}
	t.Fatalf("missing rule %q in %+v", ruleID, findings)
	return Finding{}
}

func countRule(findings []Finding, ruleID string) int {
	count := 0
	for _, finding := range findings {
		if finding.RuleID == ruleID {
			count++
		}
	}
	return count
}

func findingForLocation(findings []Finding, ruleID, file string, line int) *Finding {
	for i := range findings {
		if findings[i].RuleID == ruleID && findings[i].File == file && findings[i].Line == line {
			return &findings[i]
		}
	}
	return nil
}
