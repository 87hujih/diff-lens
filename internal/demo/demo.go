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
				Title:      "导出路径变更缺少配套测试",
				File:       "internal/exporter/export.go",
				Line:       48,
				Evidence:   "+ rows, err := db.QueryContext(ctx, query)",
				RuleID:     "source_without_tests",
				Reason:     "该 PR 修改了账单导出代码，但没有同步修改测试文件。",
				Suggestion: "补充成功导出、空结果和数据库错误场景的测试。",
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
				Title:      "导出接口可能绕过账户级权限检查",
				File:       "internal/http/export_handler.go",
				Line:       72,
				Evidence:   "+ return exporter.Export(ctx, accountID, format)",
				Reason:     "新 handler 将账户 ID 转发给 exporter，但可见变更中没有在返回账单数据前执行授权检查。",
				Suggestion: "开始导出前先验证调用方是否可读取该账户，并补充未授权用户的回归测试。",
			},
			{
				ID:         "merged-demo-query-scope",
				Source:     "merged",
				Severity:   "medium",
				Confidence: 0.88,
				Category:   "data_scope",
				Title:      "账单导出查询需要租户隔离证据",
				File:       "internal/exporter/export.go",
				Line:       48,
				Evidence:   "+ rows, err := db.QueryContext(ctx, query)",
				RuleID:     "source_without_tests",
				Reason:     "确定性规则发现了未测试的导出路径，AI 分析也将同一查询识别为数据隔离边界。",
				Suggestion: "将账户过滤条件靠近查询逻辑，并在合并前用测试覆盖跨账户隔离。",
			},
		}

		report := review.Report{
			PR: pr,
			Summary: review.Summary{
				RiskLevel:   "medium",
				Overview:    "这个演示 PR 增加了受保护的账单导出流程。主要评审重点是租户安全的数据访问，以及新导出路径周围的测试覆盖。",
				KeyChanges:  []string{"新增账单导出 HTTP 接口", "引入导出查询和 CSV 字段映射", "更新 API 响应以返回导出状态"},
				ReviewFocus: []string{"确认导出账单数据前完成账户级授权", "验证查询已按租户范围收敛", "补充成功、空结果和数据库错误场景测试"},
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
					Body: "请在开始账单导出前显式执行账户级授权检查，并为无权访问该账户的用户补充回归测试。",
				},
				{
					ID:   "comment-demo-export-tests",
					File: "internal/exporter/export.go",
					Line: 48,
					Body: "这段导出查询建议补充正常路径、空结果、数据库错误和跨账户隔离测试。",
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

		if !send(step("fetch_pr", "running", "正在加载演示 PR 元数据")) {
			return
		}
		if !send(step("fetch_pr", "completed", "已加载演示 PR 元数据")) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventPR, Data: pr}) {
			return
		}
		if !send(step("parse_diff", "running", "正在解析演示 diff")) {
			return
		}
		if !send(step("parse_diff", "completed", "已解析演示 diff")) {
			return
		}
		if !send(step("scan_rules", "running", "正在运行确定性演示规则")) {
			return
		}
		if !send(step("scan_rules", "completed", "规则扫描完成，发现 1 条演示风险")) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventRules, Data: review.RulesPayload{Risks: ruleRisks}}) {
			return
		}
		if !send(step("build_context", "running", "正在构建受控演示评审上下文")) {
			return
		}
		if !send(step("build_context", "completed", "已构建受控演示评审上下文")) {
			return
		}
		if !send(step("analyze_ai", "running", "正在生成确定性演示 AI 分析")) {
			return
		}
		if !send(step("analyze_ai", "completed", "已生成确定性演示 AI 分析")) {
			return
		}
		if !send(step("result", "running", "正在组合演示评审报告")) {
			return
		}
		if !send(step("result", "completed", "已生成最终演示评审报告")) {
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
		Title:        "feat: 增加受保护的账单导出",
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
