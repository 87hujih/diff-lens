package review

import (
	"fmt"
	"strings"
)

// ReportNormalizerOptions 控制报告阶段完成状态的元数据。
type ReportNormalizerOptions struct {
	AICompleted    bool
	RulesCompleted bool
	DegradedReason string
}

// ReportNormalizer 合并确定性规则和已验证的 AI 分析结果。
type ReportNormalizer struct{}

// NewReportNormalizer 创建报告归一化器。
func NewReportNormalizer() *ReportNormalizer {
	return &ReportNormalizer{}
}

// Normalize 根据规则风险、AI 分析和上下文构建最终报告。
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

	comments := normalizeComments(ai.Comments, validEvidence)
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
		Evidence: evidenceItemsFrom(risks, ctx),
		Comments: comments,
		Meta:     metaFromContext(ctx, options),
		Degraded: options.DegradedReason != "" || !options.AICompleted,
	}
}

// Degraded 在 AI 分析无法完成时构建仅包含规则结果的报告。
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
		Evidence: evidenceItemsFrom(risks, ctx),
		Comments: []SuggestedComment{},
		Meta: metaFromContext(ctx, ReportNormalizerOptions{
			AICompleted:    false,
			RulesCompleted: true,
			DegradedReason: reason,
		}),
		Degraded: true,
	}
}

// evidenceItemsFrom 汇总规则、AI 和上下文 snippet 的去重证据列表。
func evidenceItemsFrom(risks []Risk, ctx ReviewContext) []EvidenceItem {
	items := []EvidenceItem{}
	seen := map[string]bool{}

	add := func(item EvidenceItem) {
		item.ID = strings.TrimSpace(item.ID)
		item.Snippet = strings.TrimSpace(item.Snippet)
		if item.ID == "" || item.Snippet == "" || seen[item.ID] {
			return
		}
		seen[item.ID] = true
		items = append(items, item)
	}

	for _, risk := range risks {
		id := risk.ID
		if id == "" && risk.RuleID != "" {
			id = risk.RuleID
		}
		add(EvidenceItem{
			ID:      id,
			File:    risk.File,
			Line:    risk.Line,
			Snippet: risk.Evidence,
			Source:  risk.Source,
		})
	}

	for _, file := range ctx.Files {
		for _, snippet := range file.Snippets {
			line := snippet.StartLine
			if line == 0 {
				line = snippet.EndLine
			}
			add(EvidenceItem{
				ID:      snippet.ID,
				File:    snippet.File,
				Line:    line,
				Snippet: snippet.Patch,
				Source:  "context",
			})
		}
	}

	return items
}

// normalizeRuleRisks 复制规则风险并补齐缺省来源。
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

// normalizeAIRisk 只接纳引用有效证据且定位充分的 AI 风险。
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

// normalizeComments 将有证据支撑的 AI 评论转换为前端可复制的评论草稿。
func normalizeComments(comments []AnalysisComment, validEvidence map[string]bool) []SuggestedComment {
	out := make([]SuggestedComment, 0, len(comments))
	for _, comment := range comments {
		id := strings.TrimSpace(comment.ID)
		body := strings.TrimSpace(comment.Body)
		file := strings.TrimSpace(comment.File)
		if id == "" || body == "" {
			continue
		}
		if len(comment.EvidenceRefs) == 0 || !allEvidenceRefsExist(comment.EvidenceRefs, validEvidence) {
			continue
		}
		if comment.Line != 0 && file == "" {
			continue
		}
		out = append(out, SuggestedComment{
			ID:           id,
			File:         file,
			Line:         comment.Line,
			Body:         body,
			EvidenceRefs: cloneStrings(comment.EvidenceRefs),
		})
	}
	return out
}

// shouldMergeRisk 判断 AI 风险是否和规则风险描述同一问题。
func shouldMergeRisk(rule Risk, ai Risk) bool {
	if rule.Category != ai.Category || rule.File != ai.File || rule.Line != ai.Line {
		return false
	}
	if rule.RuleID != "" && ai.RuleID != "" && rule.RuleID == ai.RuleID {
		return true
	}
	return hasEvidenceOverlap(rule.EvidenceRefs, ai.EvidenceRefs)
}

// mergeRisks 保留规则稳定 ID，同时使用 AI 补强标题、原因和建议。
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

// evidenceSet 收集当前上下文中所有允许 AI 引用的证据 ID。
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

// allEvidenceRefsExist 确认 AI 输出没有引用不存在的证据。
func allEvidenceRefsExist(refs []string, valid map[string]bool) bool {
	for _, ref := range refs {
		if ref == "" || !valid[ref] {
			return false
		}
	}
	return true
}

// hasEvidenceOverlap 用共享证据判断两条风险是否可合并。
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

// deriveRiskLevel 从所有风险中推导报告总风险级别。
func deriveRiskLevel(risks []Risk) string {
	level := "low"
	for _, risk := range risks {
		level = higherSeverity(level, risk.Severity)
	}
	return level
}

// higherSeverity 返回两个严重级别中更高的标准化值。
func higherSeverity(left string, right string) string {
	if severityRank(right) > severityRank(left) {
		return normalizeSeverity(right)
	}
	return normalizeSeverity(left)
}

// normalizeSeverity 将未知严重级别降为 low，避免前端状态爆炸。
func normalizeSeverity(severity string) string {
	switch severity {
	case "high", "medium", "low":
		return severity
	default:
		return "low"
	}
}

// reviewFocusFromRisks 合并 AI 关注点和风险标题形成评审清单。
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

// metaFromContext 把上下文截断和阶段完成状态写入报告元数据。
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

// stableRiskSuffix 为缺少 ID 的 AI 风险生成稳定后缀。
func stableRiskSuffix(file string, line int, category string, title string) string {
	return stableSnippetID(file, line, category+"\n"+title)[len(contextSnippetEvidencePrefix):]
}
