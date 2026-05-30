package rules

import "diff-lens/internal/review"

// Scanner 后续承载 LLM 分析前运行的确定性检查。
type Scanner struct{}

// NewScanner 返回用于规则风险检测的无状态 scanner。
func NewScanner() *Scanner {
	return &Scanner{}
}

// Scan 返回规则派生风险；当前空实现是脚手架扩展点。
func (s *Scanner) Scan() []review.Risk {
	return nil
}
