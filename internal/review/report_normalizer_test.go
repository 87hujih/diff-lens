package review_test

import (
	"testing"

	"diff-lens/internal/review"
)

func TestReportNormalizerBuildsDegradedRulesOnlyReport(t *testing.T) {
	normalizer := review.NewReportNormalizer()
	pr := normalizerPR()
	ctx := normalizerContext()
	rules := []review.Risk{normalizerRuleRisk()}

	report := normalizer.Degraded(pr, rules, ctx, "llm_not_configured")

	if !report.Degraded {
		t.Fatalf("Degraded = false, want true")
	}
	if report.Meta.AICompleted {
		t.Fatalf("Meta.AICompleted = true, want false")
	}
	if !report.Meta.RulesCompleted {
		t.Fatalf("Meta.RulesCompleted = false, want true")
	}
	if report.Meta.DegradedReason != "llm_not_configured" {
		t.Fatalf("degraded reason = %q, want llm_not_configured", report.Meta.DegradedReason)
	}
	if len(report.Risks) != 1 || report.Risks[0].ID != "rule-auth-1" {
		t.Fatalf("risks = %#v, want retained rule risk", report.Risks)
	}
	if report.Comments == nil {
		t.Fatalf("comments = nil, want empty slice")
	}
}

func TestReportNormalizerMergesOnlyWhenEvidenceOrRuleIdentityMatches(t *testing.T) {
	normalizer := review.NewReportNormalizer()
	pr := normalizerPR()
	ctx := normalizerContext()
	rules := []review.Risk{normalizerRuleRisk()}

	analysis := review.ReviewAnalysis{
		Summary: "AI summary",
		Risks: []review.AIRisk{
			{
				ID:           "ai-same-evidence",
				Severity:     "high",
				Confidence:   0.91,
				Category:     "auth",
				Title:        "Authorization bypass remains possible",
				File:         "internal/auth/session.go",
				Line:         42,
				RuleID:       "auth/session-validation",
				EvidenceRefs: []string{"snippet-auth-1"},
				Reason:       "AI and rule point at the same snippet.",
				Suggestion:   "Require explicit authorization checks.",
			},
			{
				ID:           "ai-different-evidence",
				Severity:     "medium",
				Confidence:   0.82,
				Category:     "auth",
				Title:        "Different auth issue on same line",
				File:         "internal/auth/session.go",
				Line:         42,
				EvidenceRefs: []string{"snippet-auth-2"},
				Reason:       "Same location and category but different evidence.",
				Suggestion:   "Review the separate condition.",
			},
		},
	}

	report := normalizer.Normalize(pr, rules, analysis, ctx, review.ReportNormalizerOptions{AICompleted: true, RulesCompleted: true})

	if report.Degraded {
		t.Fatalf("Degraded = true, want false")
	}
	if report.Summary.Overview != "AI summary" {
		t.Fatalf("summary overview = %q, want AI summary", report.Summary.Overview)
	}
	if report.PR != pr {
		t.Fatalf("report PR = %#v, want %#v", report.PR, pr)
	}
	if len(report.Risks) != 2 {
		t.Fatalf("risk count = %d, want merged risk plus distinct AI risk: %#v", len(report.Risks), report.Risks)
	}
	if report.Risks[0].Source != "merged" {
		t.Fatalf("first source = %q, want merged; risks=%#v", report.Risks[0].Source, report.Risks)
	}
	if report.Risks[1].Source != "ai" || report.Risks[1].ID != "ai-different-evidence" {
		t.Fatalf("second risk = %#v, want distinct AI risk", report.Risks[1])
	}
}

func TestReportNormalizerRejectsAIHighMediumRisksWithoutValidEvidenceRefs(t *testing.T) {
	normalizer := review.NewReportNormalizer()
	pr := normalizerPR()
	ctx := normalizerContext()

	analysis := review.ReviewAnalysis{
		Summary: "AI summary",
		Risks: []review.AIRisk{
			{
				ID:           "ai-missing-evidence",
				Severity:     "high",
				Category:     "security",
				Title:        "No evidence",
				File:         "internal/auth/session.go",
				Line:         42,
				EvidenceRefs: nil,
			},
			{
				ID:           "ai-unknown-evidence",
				Severity:     "medium",
				Category:     "security",
				Title:        "Unknown evidence",
				File:         "internal/auth/session.go",
				Line:         42,
				EvidenceRefs: []string{"missing-snippet"},
			},
			{
				ID:           "ai-missing-location",
				Severity:     "high",
				Category:     "security",
				Title:        "Missing line",
				File:         "internal/auth/session.go",
				EvidenceRefs: []string{"snippet-auth-1"},
			},
			{
				ID:           "ai-low-valid",
				Severity:     "low",
				Category:     "maintainability",
				Title:        "Valid low risk",
				File:         "internal/auth/session.go",
				Line:         43,
				EvidenceRefs: []string{"snippet-auth-1"},
			},
		},
	}

	report := normalizer.Normalize(pr, nil, analysis, ctx, review.ReportNormalizerOptions{AICompleted: true})

	if len(report.Risks) != 1 {
		t.Fatalf("risks = %#v, want only valid low AI risk", report.Risks)
	}
	if report.Risks[0].ID != "ai-low-valid" {
		t.Fatalf("risk = %#v, want ai-low-valid", report.Risks[0])
	}
	if report.Summary.RiskLevel != "low" {
		t.Fatalf("risk level = %q, want low", report.Summary.RiskLevel)
	}
}

func TestReportNormalizerDerivesMetaRiskLevelAndComments(t *testing.T) {
	normalizer := review.NewReportNormalizer()
	pr := normalizerPR()
	ctx := normalizerContext()
	ctx.Stats.Truncated = true
	ctx.Stats.OmittedFilesCount = 3
	ctx.Stats.OmittedSnippetsCount = 5

	analysis := review.ReviewAnalysis{
		Summary: "AI found a serious issue",
		Risks: []review.AIRisk{{
			ID:           "ai-high-valid",
			Severity:     "high",
			Confidence:   0.9,
			Category:     "security",
			Title:        "Valid high risk",
			File:         "internal/auth/session.go",
			Line:         42,
			EvidenceRefs: []string{"snippet-auth-1"},
			Reason:       "Evidence is valid.",
			Suggestion:   "Fix it.",
		}},
		Comments: []review.AnalysisComment{{
			ID:           "comment-1",
			File:         "internal/auth/session.go",
			Line:         42,
			Body:         "Please revisit this authorization path.",
			EvidenceRefs: []string{"snippet-auth-1"},
		}},
	}

	report := normalizer.Normalize(pr, nil, analysis, ctx, review.ReportNormalizerOptions{AICompleted: true, RulesCompleted: true})

	if report.Summary.RiskLevel != "high" {
		t.Fatalf("risk level = %q, want high", report.Summary.RiskLevel)
	}
	if len(report.Comments) != 1 || report.Comments[0].ID != "comment-1" {
		t.Fatalf("comments = %#v, want normalized AI comment", report.Comments)
	}
	if !report.Meta.AICompleted || !report.Meta.RulesCompleted || !report.Meta.ContextTruncated {
		t.Fatalf("meta = %#v, want completed flags and context truncation", report.Meta)
	}
	if report.Meta.OmittedFilesCount != 3 || report.Meta.OmittedSnippetsCount != 5 {
		t.Fatalf("meta omitted counts = %#v, want context counts", report.Meta)
	}
}

func normalizerPR() review.PRInfo {
	return review.PRInfo{
		Title:        "Harden auth",
		Author:       "octocat",
		Repo:         "openai/example",
		Number:       123,
		SourceBranch: "feature/auth",
		TargetBranch: "main",
		ChangedFiles: 1,
		Additions:    10,
		Deletions:    2,
		Commits:      1,
	}
}

func normalizerRuleRisk() review.Risk {
	return review.Risk{
		ID:           "rule-auth-1",
		Source:       "rule",
		Severity:     "high",
		Confidence:   0.95,
		Category:     "auth",
		Title:        "Session validation changed",
		File:         "internal/auth/session.go",
		Line:         42,
		RuleID:       "auth/session-validation",
		EvidenceRefs: []string{"snippet-auth-1"},
		Reason:       "A session authorization guard changed.",
		Suggestion:   "Keep explicit authorization checks.",
	}
}

func normalizerContext() review.ReviewContext {
	return review.ReviewContext{
		SchemaVersion: "review-context/v1",
		ContextID:     "ctx-test",
		PR:            normalizerPR(),
		EvidenceRefs:  []string{"rule-auth-1", "snippet-auth-1", "snippet-auth-2"},
		Files: []review.ContextFile{{
			Filename: "internal/auth/session.go",
			Kind:     "source",
			Status:   "modified",
			Snippets: []review.ContextSnippet{
				{ID: "snippet-auth-1", File: "internal/auth/session.go", StartLine: 40, EndLine: 44, Patch: "@@ auth 1 @@", Reason: "rule"},
				{ID: "snippet-auth-2", File: "internal/auth/session.go", StartLine: 42, EndLine: 45, Patch: "@@ auth 2 @@", Reason: "context"},
			},
		}},
	}
}
