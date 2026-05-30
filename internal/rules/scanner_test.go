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
		name   string
		raw    string
		secret string
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
			name:   "unquoted secret with punctuation",
			raw:    `password = abc$restOfSecret!?:/+=-xyz, next := true`,
			secret: "abc$restOfSecret!?:/+=-xyz",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masked := maskSensitiveEvidence(tt.raw)

			if strings.Contains(masked, tt.secret) {
				t.Fatalf("masked evidence still contains raw secret value: %q", masked)
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
	beforeAssignment, _, _ := strings.Cut(evidence, "=")
	return strings.TrimSpace(beforeAssignment)
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
