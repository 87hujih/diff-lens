package review

import "fmt"

// ReportNormalizerOptions controls report stage completion metadata.
type ReportNormalizerOptions struct {
	AICompleted    bool
	RulesCompleted bool
	DegradedReason string
}

// ReportNormalizer merges deterministic rules and validated AI analysis.
type ReportNormalizer struct{}

// NewReportNormalizer creates a report normalizer.
func NewReportNormalizer() *ReportNormalizer {
	return &ReportNormalizer{}
}

// Normalize builds a final report from rule risks, AI analysis, and context.
func (n *ReportNormalizer) Normalize(pr PRInfo, ruleRisks []Risk, ai ReviewAnalysis, ctx ReviewContext, options ReportNormalizerOptions) Report {
	risks := normalizeRuleRisks(ruleRisks)
	validEvidence := evidenceSet(ctx)

	for _, aiRisk := range ai.Risks {
		risk, ok := normalizeAIRisk(aiRisk, validEvidence)
		if !ok {
			continue
		}

		merged := false
		for i := range risks {
			if shouldMergeRisk(risks[i], risk) {
				risks[i] = mergeRisks(risks[i], risk)
				merged = true
				break
			}
		}
		if !merged {
			risks = append(risks, risk)
		}
	}

	comments := normalizeComments(ai.Comments)
	overview := ai.Summary
	if overview == "" {
		overview = "规则扫描已完成，AI 未返回摘要。"
	}

	return Report{
		PR: pr,
		Summary: Summary{
			RiskLevel:   deriveRiskLevel(risks),
			Overview:    overview,
			KeyChanges:  []string{},
			ReviewFocus: reviewFocusFromRisks(risks, ai.AttentionItems),
		},
		Risks:    risks,
		Comments: comments,
		Meta:     metaFromContext(ctx, options),
		Degraded: options.DegradedReason != "" || !options.AICompleted,
	}
}

// Degraded builds a rules-only report when AI analysis cannot complete.
func (n *ReportNormalizer) Degraded(pr PRInfo, ruleRisks []Risk, ctx ReviewContext, reason string) Report {
	risks := normalizeRuleRisks(ruleRisks)
	return Report{
		PR: pr,
		Summary: Summary{
			RiskLevel: deriveRiskLevel(risks),
			Overview:  "AI 分析未完成，当前报告仅包含确定性规则扫描结果。",
			KeyChanges: []string{
				fmt.Sprintf("变更 %d 个文件，新增 %d 行、删除 %d 行。", pr.ChangedFiles, pr.Additions, pr.Deletions),
			},
			ReviewFocus: reviewFocusFromRisks(risks, nil),
		},
		Risks:    risks,
		Comments: []SuggestedComment{},
		Meta: metaFromContext(ctx, ReportNormalizerOptions{
			AICompleted:    false,
			RulesCompleted: true,
			DegradedReason: reason,
		}),
		Degraded: true,
	}
}

func normalizeRuleRisks(ruleRisks []Risk) []Risk {
	risks := cloneRisks(ruleRisks)
	if risks == nil {
		return []Risk{}
	}
	for i := range risks {
		if risks[i].Source == "" {
			risks[i].Source = "rule"
		}
	}
	return risks
}

func normalizeAIRisk(ai AIRisk, validEvidence map[string]bool) (Risk, bool) {
	if len(ai.EvidenceRefs) == 0 || !allEvidenceRefsExist(ai.EvidenceRefs, validEvidence) {
		return Risk{}, false
	}
	if (ai.Severity == "high" || ai.Severity == "medium") && (ai.File == "" || ai.Line == 0) {
		return Risk{}, false
	}
	sourceID := ai.ID
	if sourceID == "" {
		sourceID = "ai-" + stableRiskSuffix(ai.File, ai.Line, ai.Category, ai.Title)
	}
	return Risk{
		ID:           sourceID,
		Source:       "ai",
		Severity:     normalizeSeverity(ai.Severity),
		Confidence:   ai.Confidence,
		Category:     ai.Category,
		Title:        ai.Title,
		File:         ai.File,
		Line:         ai.Line,
		RuleID:       ai.RuleID,
		EvidenceRefs: cloneStrings(ai.EvidenceRefs),
		Reason:       ai.Reason,
		Suggestion:   ai.Suggestion,
	}, true
}

func normalizeComments(comments []AnalysisComment) []SuggestedComment {
	out := make([]SuggestedComment, 0, len(comments))
	for _, comment := range comments {
		out = append(out, SuggestedComment{
			ID:           comment.ID,
			File:         comment.File,
			Line:         comment.Line,
			Body:         comment.Body,
			EvidenceRefs: cloneStrings(comment.EvidenceRefs),
		})
	}
	return out
}

func shouldMergeRisk(rule Risk, ai Risk) bool {
	if rule.Category != ai.Category || rule.File != ai.File || rule.Line != ai.Line {
		return false
	}
	if rule.RuleID != "" && ai.RuleID != "" && rule.RuleID == ai.RuleID {
		return true
	}
	return hasEvidenceOverlap(rule.EvidenceRefs, ai.EvidenceRefs)
}

func mergeRisks(rule Risk, ai Risk) Risk {
	merged := rule
	merged.ID = rule.ID
	if merged.ID == "" {
		merged.ID = ai.ID
	}
	merged.Source = "merged"
	merged.Severity = higherSeverity(rule.Severity, ai.Severity)
	if ai.Confidence > merged.Confidence {
		merged.Confidence = ai.Confidence
	}
	if ai.Title != "" {
		merged.Title = ai.Title
	}
	if ai.Reason != "" {
		merged.Reason = ai.Reason
	}
	if ai.Suggestion != "" {
		merged.Suggestion = ai.Suggestion
	}
	for _, ref := range ai.EvidenceRefs {
		merged.EvidenceRefs = appendUnique(merged.EvidenceRefs, ref)
	}
	return merged
}

func evidenceSet(ctx ReviewContext) map[string]bool {
	out := map[string]bool{}
	for _, ref := range ctx.EvidenceRefs {
		out[ref] = true
	}
	for _, risk := range ctx.RuleRisks {
		if risk.ID != "" {
			out[risk.ID] = true
		}
		for _, ref := range risk.EvidenceRefs {
			out[ref] = true
		}
	}
	for _, file := range ctx.Files {
		for _, snippet := range file.Snippets {
			out[snippet.ID] = true
		}
	}
	return out
}

func allEvidenceRefsExist(refs []string, valid map[string]bool) bool {
	for _, ref := range refs {
		if ref == "" || !valid[ref] {
			return false
		}
	}
	return true
}

func hasEvidenceOverlap(left []string, right []string) bool {
	for _, l := range left {
		for _, r := range right {
			if l != "" && l == r {
				return true
			}
		}
	}
	return false
}

func deriveRiskLevel(risks []Risk) string {
	level := "low"
	for _, risk := range risks {
		level = higherSeverity(level, risk.Severity)
	}
	return level
}

func higherSeverity(left string, right string) string {
	if severityRank(right) > severityRank(left) {
		return normalizeSeverity(right)
	}
	return normalizeSeverity(left)
}

func normalizeSeverity(severity string) string {
	switch severity {
	case "high", "medium", "low":
		return severity
	default:
		return "low"
	}
}

func reviewFocusFromRisks(risks []Risk, attention []string) []string {
	focus := make([]string, 0, len(risks)+len(attention))
	focus = append(focus, attention...)
	for _, risk := range risks {
		if risk.Title != "" {
			focus = append(focus, risk.Title)
		}
	}
	if focus == nil {
		return []string{}
	}
	return focus
}

func metaFromContext(ctx ReviewContext, options ReportNormalizerOptions) ReportMeta {
	return ReportMeta{
		AICompleted:          options.AICompleted,
		RulesCompleted:       options.RulesCompleted,
		ContextTruncated:     ctx.Stats.Truncated,
		DegradedReason:       options.DegradedReason,
		OmittedFilesCount:    ctx.Stats.OmittedFilesCount,
		OmittedSnippetsCount: ctx.Stats.OmittedSnippetsCount,
	}
}

func stableRiskSuffix(file string, line int, category string, title string) string {
	return stableSnippetID(file, line, category+"\n"+title)[len(contextSnippetEvidencePrefix):]
}
