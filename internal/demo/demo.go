package demo

import (
	"context"

	"diff-lens/internal/review"
)

// Provider 为演示和前端开发输出稳定的示例 review 事件流。
type Provider struct{}

// NewProvider 构造确定性的演示数据提供者。
func NewProvider() *Provider {
	return &Provider{}
}

// Stream 发送一组有代表性的事件序列，不调用 GitHub 或 LLM。
func (p *Provider) Stream(ctx context.Context) (<-chan review.ReviewEvent, error) {
	events := make(chan review.ReviewEvent)

	go func() {
		defer close(events)

		pr := demoPR()
		ruleRisks := []review.Risk{
			{
				ID:         "rule-demo-test-gap",
				Source:     "rule",
				Severity:   "medium",
				Confidence: 0.78,
				Category:   "test_gap",
				Title:      "Source export path changed without matching tests",
				File:       "internal/exporter/export.go",
				Line:       48,
				Evidence:   "+ rows, err := db.QueryContext(ctx, query)",
				RuleID:     "source_without_tests",
				Reason:     "The PR changes billing export code, but no test file changed with it.",
				Suggestion: "Add tests for successful export, empty results, and database errors.",
			},
		}
		reportRisks := []review.Risk{
			ruleRisks[0],
			{
				ID:         "ai-demo-permission-boundary",
				Source:     "ai",
				Severity:   "high",
				Confidence: 0.84,
				Category:   "authz",
				Title:      "Export endpoint may bypass account-level permission checks",
				File:       "internal/http/export_handler.go",
				Line:       72,
				Evidence:   "+ return exporter.Export(ctx, accountID, format)",
				Reason:     "The new handler forwards the account ID to the exporter, but the visible change does not show an authorization check before returning billing data.",
				Suggestion: "Verify the caller can read the account before starting the export, and add a regression test for unauthorized users.",
			},
			{
				ID:         "merged-demo-query-scope",
				Source:     "merged",
				Severity:   "medium",
				Confidence: 0.88,
				Category:   "data_scope",
				Title:      "Billing export query needs tenant scoping evidence",
				File:       "internal/exporter/export.go",
				Line:       48,
				Evidence:   "+ rows, err := db.QueryContext(ctx, query)",
				RuleID:     "source_without_tests",
				Reason:     "The deterministic rule found an untested export path, and AI analysis points to the same query as a data isolation boundary.",
				Suggestion: "Keep the account filter close to the query and cover cross-account isolation in tests before merging.",
			},
		}

		report := review.Report{
			PR: pr,
			Summary: review.Summary{
				RiskLevel:   "medium",
				Overview:    "This demo PR adds a guarded billing export flow. The main review focus is tenant-safe data access plus tests around the new export path.",
				KeyChanges:  []string{"Adds a billing export HTTP endpoint", "Introduces exporter query and CSV field mapping", "Updates the API response to return export status"},
				ReviewFocus: []string{"Confirm account-level authorization before exporting billing data", "Verify the query is tenant-scoped", "Add tests for success, empty result, and database error cases"},
			},
			Risks: reportRisks,
			Evidence: []review.EvidenceItem{
				{
					ID:      "rule-demo-test-gap",
					File:    "internal/exporter/export.go",
					Line:    48,
					Snippet: "+ rows, err := db.QueryContext(ctx, query)",
					Source:  "rule",
				},
				{
					ID:      "ai-demo-permission-boundary",
					File:    "internal/http/export_handler.go",
					Line:    72,
					Snippet: "+ return exporter.Export(ctx, accountID, format)",
					Source:  "ai",
				},
			},
			Comments: []review.SuggestedComment{
				{
					ID:   "comment-demo-authz",
					File: "internal/http/export_handler.go",
					Line: 72,
					Body: "Please make the account-level authorization check explicit before starting the billing export, and add a regression test for a user without access to this account.",
				},
				{
					ID:   "comment-demo-export-tests",
					File: "internal/exporter/export.go",
					Line: 48,
					Body: "This export query would benefit from tests for the happy path, empty results, database errors, and cross-account isolation.",
				},
			},
			Meta: review.ReportMeta{
				AICompleted:    true,
				RulesCompleted: true,
			},
		}

		// 浏览器断开或请求取消时，立即停止继续发送事件。
		send := func(event review.ReviewEvent) bool {
			select {
			case <-ctx.Done():
				return false
			case events <- event:
				return true
			}
		}

		if !send(step("fetch_pr", "running", "Loading demo PR metadata")) {
			return
		}
		if !send(step("fetch_pr", "completed", "Loaded demo PR metadata")) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventPR, Data: pr}) {
			return
		}
		if !send(step("parse_diff", "running", "Parsing demo diff")) {
			return
		}
		if !send(step("parse_diff", "completed", "Parsed demo diff")) {
			return
		}
		if !send(step("scan_rules", "running", "Running deterministic demo rules")) {
			return
		}
		if !send(step("scan_rules", "completed", "Rule scan completed with 1 demo risk")) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventRules, Data: review.RulesPayload{Risks: ruleRisks}}) {
			return
		}
		if !send(step("build_context", "running", "Building bounded demo review context")) {
			return
		}
		if !send(step("build_context", "completed", "Built bounded demo review context")) {
			return
		}
		if !send(step("analyze_ai", "running", "Generating deterministic demo AI analysis")) {
			return
		}
		if !send(step("analyze_ai", "completed", "Generated deterministic demo AI analysis")) {
			return
		}
		if !send(step("result", "running", "Composing demo review report")) {
			return
		}
		if !send(step("result", "completed", "Generated final demo review report")) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventResult, Data: report}) {
			return
		}
		send(review.ReviewEvent{Type: review.EventDone, Data: review.DonePayload{OK: true}})
	}()

	return events, nil
}

func demoPR() review.PRInfo {
	return review.PRInfo{
		Title:        "feat: add guarded billing export",
		Author:       "demo-user",
		Repo:         "example/payments",
		Number:       42,
		SourceBranch: "feature/billing-export",
		TargetBranch: "main",
		ChangedFiles: 6,
		Additions:    128,
		Deletions:    31,
		Commits:      2,
	}
}

func step(name string, status string, message string) review.ReviewEvent {
	return review.ReviewEvent{
		Type: review.EventStep,
		Data: review.StepPayload{
			Step:    name,
			Status:  status,
			Message: message,
		},
	}
}
