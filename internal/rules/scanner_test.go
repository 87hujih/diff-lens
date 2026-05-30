package rules

import (
	"os"
	"strings"
	"testing"

	"diff-lens/internal/diff"
)

func TestScanEmptyAnalysisReturnsNoFindings(t *testing.T) {
	findings := NewScanner().Scan(diff.Analysis{})

	if len(findings) != 0 {
		t.Fatalf("Scan returned %d findings, want 0", len(findings))
	}
}

func TestScanUsesAddedLinesForFindingsAndDeduplicatesExactDuplicates(t *testing.T) {
	scanner := newScannerWithRules(
		testRule("test.rule.one", "security"),
		testRule("test.rule.two", "security"),
		testRule("test.rule.one", "security"),
		testRule("test.rule.docs", "docs"),
	)

	findings := scanner.Scan(diff.Analysis{
		Files: []diff.FileDiff{{
			Filename: "app/config.go",
			Hunks: []diff.Hunk{{
				Lines: []diff.DiffLine{
					{Kind: diff.DiffLineRemoved, Content: "password := oldSecret", OldLine: 9},
					{Kind: diff.DiffLineContext, Content: "func configure() {", OldLine: 10, NewLine: 10},
					{Kind: diff.DiffLineAdded, Content: "password := newSecret", NewLine: 11},
				},
			}},
		}},
	})

	if len(findings) != 3 {
		t.Fatalf("Scan returned %d findings, want 3: %#v", len(findings), findings)
	}
	for _, finding := range findings {
		if finding.File != "app/config.go" || finding.Line != 11 {
			t.Fatalf("finding source = %s:%d, want app/config.go:11", finding.File, finding.Line)
		}
	}
	if findings[0].RuleID != "test.rule.one" || findings[1].RuleID != "test.rule.two" || findings[2].RuleID != "test.rule.docs" {
		t.Fatalf("rule IDs = %q, %q, %q; want both security rules and docs rule", findings[0].RuleID, findings[1].RuleID, findings[2].RuleID)
	}
}

func TestFindingIDIsStableAndNotEmpty(t *testing.T) {
	scanner := newScannerWithRules(testRule("test.rule", "security"))
	analysis := analysisWithAddedLine("app/config.go", 42, `token := "secret-value"`)

	first := scanner.Scan(analysis)
	second := scanner.Scan(analysis)

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("got finding counts %d and %d, want 1 and 1", len(first), len(second))
	}
	if first[0].ID == "" {
		t.Fatal("finding ID is empty")
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("finding ID changed between scans: %q != %q", first[0].ID, second[0].ID)
	}
}

func TestAddingUnrelatedFindingDoesNotChangeExistingFindingID(t *testing.T) {
	scanner := newScannerWithRules(testRule("test.rule", "security"))
	original := scanner.Scan(analysisWithAddedLine("app/config.go", 42, `token := "secret-value"`))

	expanded := scanner.Scan(diff.Analysis{
		Files: []diff.FileDiff{
			fileWithAddedLine("docs/readme.md", 1, "hello"),
			fileWithAddedLine("app/config.go", 42, `token := "secret-value"`),
		},
	})

	if len(original) != 1 || len(expanded) != 2 {
		t.Fatalf("got finding counts %d and %d, want 1 and 2", len(original), len(expanded))
	}
	var matched Finding
	for _, finding := range expanded {
		if finding.File == original[0].File && finding.Line == original[0].Line && finding.Category == original[0].Category {
			matched = finding
		}
	}
	if matched.ID == "" {
		t.Fatal("original finding was not present after adding unrelated finding")
	}
	if matched.ID != original[0].ID {
		t.Fatalf("existing finding ID changed: %q != %q", matched.ID, original[0].ID)
	}
}

func TestTruncateEvidenceLimitsLongEvidence(t *testing.T) {
	longEvidence := strings.Repeat("x", maxEvidenceLength+50)

	truncated := truncateEvidence(longEvidence)

	if len(truncated) > maxEvidenceLength {
		t.Fatalf("truncated evidence length = %d, want <= %d", len(truncated), maxEvidenceLength)
	}
	if !strings.HasSuffix(truncated, "...") {
		t.Fatalf("truncated evidence = %q, want ellipsis suffix", truncated)
	}
}

func TestMaskSensitiveEvidenceRemovesRawSecretValueAndKeepsContext(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		secret    string
		forbidden string
	}{
		{
			name:   "quoted secret",
			raw:    `AWS_SECRET_ACCESS_KEY = "abcdefghijklmnopqrstuvwxyz1234567890"`,
			secret: "abcdefghijklmnopqrstuvwxyz1234567890",
		},
		{
			name:   "quoted secret with shell punctuation",
			raw:    `token = "abc$restOfSecret!?:/+=-xyz"`,
			secret: "abc$restOfSecret!?:/+=-xyz",
		},
		{
			name:      "unquoted secret with punctuation",
			raw:       `password = abc$restOfSecret!?:/+=-xyz, next := true`,
			secret:    "abc$restOfSecret!?:/+=-xyz",
			forbidden: "restOfSecret",
		},
		{
			name:      "unquoted secret with comma",
			raw:       `api_key: abc,def`,
			secret:    "abc,def",
			forbidden: "def",
		},
		{
			name:      "unquoted secret with semicolon",
			raw:       `api_key: abc;def`,
			secret:    "abc;def",
			forbidden: "def",
		},
		{
			name:      "unquoted secret with paren",
			raw:       `api_key: abc)def`,
			secret:    "abc)def",
			forbidden: "def",
		},
		{
			name:      "unquoted secret with bracket",
			raw:       `api_key: abc]def`,
			secret:    "abc]def",
			forbidden: "def",
		},
		{
			name:      "unquoted secret with brace",
			raw:       `api_key: abc}def`,
			secret:    "abc}def",
			forbidden: "def",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := maskSensitiveEvidence(tt.raw)

			if strings.Contains(masked, tt.secret) {
				t.Fatalf("masked evidence still contains raw secret value: %q", masked)
			}
			if tt.forbidden != "" && strings.Contains(masked, tt.forbidden) {
				t.Fatalf("masked evidence still contains raw secret suffix %q: %q", tt.forbidden, masked)
			}
			if !strings.Contains(strings.ToLower(masked), strings.ToLower(secretKeyFromEvidence(tt.raw))) {
				t.Fatalf("masked evidence lost key context: %q", masked)
			}
			if !strings.Contains(masked, "<masked>") {
				t.Fatalf("masked evidence = %q, want <masked> marker", masked)
			}
		})
	}
}

func TestDefaultScannerDetectsSensitiveInformationAndMasksEvidence(t *testing.T) {
	analysis := diff.Analysis{Files: []diff.FileDiff{{
		Filename: "app/config.go",
		Hunks: []diff.Hunk{{
			Lines: []diff.DiffLine{
				{Kind: diff.DiffLineAdded, Content: `password = "correct-horse-battery-staple"`, NewLine: 10},
				{Kind: diff.DiffLineAdded, Content: `api_key = "sk_live_1234567890"`, NewLine: 11},
				{Kind: diff.DiffLineAdded, Content: `secret = "do-not-commit"`, NewLine: 12},
				{Kind: diff.DiffLineAdded, Content: `private key material follows`, NewLine: 13},
			},
		}},
	}}}

	findings := NewScanner().Scan(analysis)

	assertFindingCountByCategory(t, findings, "security", 4)
	forbidden := []string{
		"correct-horse-battery-staple",
		"sk_live_1234567890",
		"do-not-commit",
	}
	for _, finding := range findings {
		if finding.Category != "security" {
			continue
		}
		if finding.Severity != "medium" {
			t.Fatalf("sensitive finding severity = %q, want medium", finding.Severity)
		}
		if finding.Confidence <= 0 || finding.Confidence > 1 {
			t.Fatalf("sensitive finding confidence = %v, want within (0,1]", finding.Confidence)
		}
		if finding.Title == "" || finding.Reason == "" || finding.Suggestion == "" {
			t.Fatalf("sensitive finding is missing useful text: %#v", finding)
		}
		for _, raw := range forbidden {
			if strings.Contains(finding.MaskedEvidence, raw) {
				t.Fatalf("masked evidence contains raw secret %q: %q", raw, finding.MaskedEvidence)
			}
		}
	}
}

func TestDefaultScannerDetectsDangerousOperationsFromAddedLinesOnly(t *testing.T) {
	analysis := diff.Analysis{Files: []diff.FileDiff{{
		Filename: "scripts/deploy.sh",
		Hunks: []diff.Hunk{{
			Lines: []diff.DiffLine{
				{Kind: diff.DiffLineRemoved, Content: "rm -rf /tmp/old", OldLine: 1},
				{Kind: diff.DiffLineAdded, Content: "rm -rf /var/app/cache", NewLine: 2},
				{Kind: diff.DiffLineAdded, Content: "DROP TABLE users;", NewLine: 3},
				{Kind: diff.DiffLineAdded, Content: "TRUNCATE audit_log;", NewLine: 4},
				{Kind: diff.DiffLineAdded, Content: "DELETE FROM sessions;", NewLine: 5},
				{Kind: diff.DiffLineAdded, Content: "DELETE FROM sessions WHERE expires_at < now();", NewLine: 6},
				{Kind: diff.DiffLineAdded, Content: "chmod 777 ./uploads", NewLine: 7},
				{Kind: diff.DiffLineAdded, Content: "git push --force-with-lease origin main", NewLine: 8},
			},
		}},
	}}}

	findings := NewScanner().Scan(analysis)

	assertFindingCountByCategory(t, findings, "dangerous-operation", 6)
	for _, finding := range findings {
		if finding.Category != "dangerous-operation" {
			continue
		}
		if finding.Line == 1 {
			t.Fatalf("removed line produced finding: %#v", finding)
		}
		if finding.Line == 6 {
			t.Fatalf("DELETE FROM with WHERE produced finding: %#v", finding)
		}
		if finding.Title == "" || finding.Reason == "" || finding.Suggestion == "" {
			t.Fatalf("dangerous operation finding is missing useful text: %#v", finding)
		}
	}
}

func TestDefaultScannerReportsTestGapOnlyForSourceChangesWithoutTests(t *testing.T) {
	sourceWithoutTests := diff.Analysis{
		Files: []diff.FileDiff{{Filename: "internal/app/service.go", Kinds: []diff.FileKind{diff.FileKindSource}}},
		Stats: diff.FileStats{ChangedFiles: 1, SourceFiles: 1, HasSourceChanges: true},
	}

	findings := NewScanner().Scan(sourceWithoutTests)

	assertFindingCountByCategory(t, findings, "testing", 1)
	if findings[0].Severity != "medium" {
		t.Fatalf("test gap severity = %q, want medium", findings[0].Severity)
	}

	pureDocsAndTests := diff.Analysis{
		Files: []diff.FileDiff{
			{Filename: "README.md", Kinds: []diff.FileKind{diff.FileKindDocs}},
			{Filename: "internal/app/service_test.go", Kinds: []diff.FileKind{diff.FileKindSource, diff.FileKindTest}},
		},
		Stats: diff.FileStats{ChangedFiles: 2, TestFiles: 1, DocsFiles: 1, HasTestChanges: true},
	}

	findings = NewScanner().Scan(pureDocsAndTests)

	assertNoFindingCategory(t, findings, "testing")
}

func TestDefaultScannerReportsLargePRAtConservativeThresholds(t *testing.T) {
	atFileThreshold := diff.Analysis{Stats: diff.FileStats{ChangedFiles: 50}}
	overFileThreshold := diff.Analysis{Stats: diff.FileStats{ChangedFiles: 51}}
	atChangeThreshold := diff.Analysis{Stats: diff.FileStats{Additions: 700, Deletions: 300}}
	overChangeThreshold := diff.Analysis{Stats: diff.FileStats{Additions: 701, Deletions: 300}}

	assertNoFindingCategory(t, NewScanner().Scan(atFileThreshold), "change-size")
	assertFindingCountByCategory(t, NewScanner().Scan(overFileThreshold), "change-size", 1)
	assertNoFindingCategory(t, NewScanner().Scan(atChangeThreshold), "change-size")
	assertFindingCountByCategory(t, NewScanner().Scan(overChangeThreshold), "change-size", 1)
}

func TestDefaultScannerReportsConfigAndDependencyRisk(t *testing.T) {
	analysis := diff.Analysis{Files: []diff.FileDiff{
		classifiedFile(".github/workflows/ci.yml"),
		classifiedFile("Dockerfile"),
		classifiedFile(".env.example"),
		classifiedFile("package-lock.json"),
	}}

	findings := NewScanner().Scan(analysis)

	assertFindingCountByCategory(t, findings, "configuration", 4)
	for _, finding := range findings {
		if finding.Category != "configuration" {
			continue
		}
		if finding.File == "" || finding.Title == "" || finding.Reason == "" || finding.Suggestion == "" {
			t.Fatalf("configuration finding is missing useful context: %#v", finding)
		}
	}
}

func TestDefaultScannerDoesNotClaimDeferredRules(t *testing.T) {
	analysis := diff.Analysis{Files: []diff.FileDiff{{
		Filename: "internal/app/service.go",
		Hunks: []diff.Hunk{{
			Lines: []diff.DiffLine{
				{Kind: diff.DiffLineAdded, Content: `query := "SELECT * FROM users WHERE id = " + id`, NewLine: 10},
				{Kind: diff.DiffLineAdded, Content: `cmd := "rm " + userInput`, NewLine: 11},
				{Kind: diff.DiffLineAdded, Content: `result, _ := doWork()`, NewLine: 12},
			},
		}},
	}}}

	findings := NewScanner().Scan(analysis)

	for _, finding := range findings {
		if strings.Contains(strings.ToLower(finding.Title), "concat") ||
			strings.Contains(strings.ToLower(finding.Title), "sql injection") ||
			strings.Contains(strings.ToLower(finding.Title), "error handling") {
			t.Fatalf("scanner claimed deferred rule: %#v", finding)
		}
	}
}

func TestNewFindingWithSeverityAndConfidenceUsesSharedSafeguards(t *testing.T) {
	line := AddedLine{File: "app/config.go", Line: 9, Content: `token = "abc$restOfSecret!?:/+=-xyz"`}

	finding := newFindingWithSeverity("test.rule", "critical", 0.95, "security", "Secret", line, line.Content, "reason", "suggestion")

	if finding.ID == "" {
		t.Fatal("finding ID is empty")
	}
	if finding.Severity != "critical" {
		t.Fatalf("Severity = %q, want critical", finding.Severity)
	}
	if finding.Confidence != 0.95 {
		t.Fatalf("Confidence = %v, want 0.95", finding.Confidence)
	}
	if strings.Contains(finding.MaskedEvidence, "abc$restOfSecret!?:/+=-xyz") {
		t.Fatalf("MaskedEvidence still contains raw secret: %q", finding.MaskedEvidence)
	}
}

func TestRulesPackageDoesNotImportReview(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(content), `"diff-lens/internal/review"`) {
			t.Fatalf("%s imports internal/review", entry.Name())
		}
	}
}

func testRule(ruleID, category string) ruleFunc {
	return func(line AddedLine) []Finding {
		return []Finding{newFinding(ruleID, category, "Test finding", line, line.Content, "test reason", "test suggestion")}
	}
}

func secretKeyFromEvidence(evidence string) string {
	equalIndex := strings.Index(evidence, "=")
	colonIndex := strings.Index(evidence, ":")
	switch {
	case equalIndex == -1 && colonIndex == -1:
		return strings.TrimSpace(evidence)
	case equalIndex == -1:
		return strings.TrimSpace(evidence[:colonIndex])
	case colonIndex == -1:
		return strings.TrimSpace(evidence[:equalIndex])
	case equalIndex < colonIndex:
		return strings.TrimSpace(evidence[:equalIndex])
	default:
		return strings.TrimSpace(evidence[:colonIndex])
	}
}

func analysisWithAddedLine(filename string, line int, content string) diff.Analysis {
	return diff.Analysis{Files: []diff.FileDiff{fileWithAddedLine(filename, line, content)}}
}

func fileWithAddedLine(filename string, line int, content string) diff.FileDiff {
	return diff.FileDiff{
		Filename: filename,
		Hunks: []diff.Hunk{{
			Lines: []diff.DiffLine{{
				Kind:    diff.DiffLineAdded,
				Content: content,
				NewLine: line,
			}},
		}},
	}
}

func classifiedFile(filename string) diff.FileDiff {
	return diff.FileDiff{
		Filename: filename,
		Kinds:    diff.ClassifyFile(filename),
	}
}

func assertFindingCountByCategory(t *testing.T, findings []Finding, category string, want int) {
	t.Helper()

	got := 0
	for _, finding := range findings {
		if finding.Category == category {
			got++
		}
	}
	if got != want {
		t.Fatalf("finding count for category %q = %d, want %d; findings: %#v", category, got, want, findings)
	}
}

func assertNoFindingCategory(t *testing.T, findings []Finding, category string) {
	t.Helper()

	assertFindingCountByCategory(t, findings, category, 0)
}
