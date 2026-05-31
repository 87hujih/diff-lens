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

		pr := review.PRInfo{
			Title:        "feat: add guarded billing export",
			Author:       "demo-user",
			Repo:         "example/payments",
			Number:       42,
			SourceBranch: "feature/export",
			TargetBranch: "main",
			ChangedFiles: 6,
			Additions:    128,
			Deletions:    31,
			Commits:      2,
		}

		risks := []review.Risk{
			{
				ID:         "risk-demo-1",
				Source:     "merged",
				Severity:   "medium",
				Confidence: 0.86,
				Category:   "test_gap",
				Title:      "核心导出逻辑变更但未看到测试覆盖",
				File:       "internal/exporter/export.go",
				Line:       48,
				Evidence:   "+ rows, err := db.QueryContext(ctx, query)",
				Reason:     "导出路径影响用户数据，但测试文件没有同步变化。",
				Suggestion: "建议补充成功导出、空结果和数据库错误场景的测试。",
			},
		}

		report := review.Report{
			PR: pr,
			Summary: review.Summary{
				RiskLevel:   "medium",
				Overview:    "该 PR 增加了账单导出能力，主要风险集中在数据查询路径和测试覆盖。",
				KeyChanges:  []string{"新增账单导出入口", "调整导出字段映射", "更新 API 响应结构"},
				ReviewFocus: []string{"确认导出权限边界", "补齐数据库错误和空结果测试"},
			},
			Risks: risks,
			Comments: []review.SuggestedComment{
				{
					ID:   "comment-demo-1",
					File: "internal/exporter/export.go",
					Line: 48,
					Body: "这里建议补充导出查询失败和空结果的测试，避免账单导出路径在异常场景下静默失败。",
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

		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "fetch_pr", Status: "completed", Message: "已加载示例 PR"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventPR, Data: pr}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "parse_diff", Status: "completed", Message: "已解析示例 diff"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "scan_rules", Status: "completed", Message: "规则扫描完成，发现 1 条风险"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventRules, Data: review.RulesPayload{Risks: risks}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "build_context", Status: "completed", Message: "已构建示例 ReviewContext"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "analyze_ai", Status: "completed", Message: "已生成示例 AI 分析"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventStep, Data: review.StepPayload{Step: "result", Status: "completed", Message: "已生成最终 Review 报告"}}) {
			return
		}
		if !send(review.ReviewEvent{Type: review.EventResult, Data: report}) {
			return
		}
		send(review.ReviewEvent{Type: review.EventDone, Data: review.DonePayload{OK: true}})
	}()

	return events, nil
}
