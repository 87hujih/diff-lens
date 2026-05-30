package review

import (
	"context"
	"errors"
)

// ErrRealAnalysisNotImplemented 标记非 demo PR 分析尚未实现的脚手架边界。
var ErrRealAnalysisNotImplemented = errors.New("real PR analysis is not implemented yet")

// DemoProvider 输出本地演示和冒烟测试使用的确定性事件流。
type DemoProvider interface {
	Stream(ctx context.Context) (<-chan ReviewEvent, error)
}

// ServiceOptions 聚合依赖，便于 service 保持可测试。
type ServiceOptions struct {
	DemoProvider DemoProvider
}

// Service 协调 review 分析模式，并向 handler 输出领域事件流。
type Service struct {
	demoProvider DemoProvider
}

// NewService 使用注入的 provider 构造应用服务。
func NewService(options ServiceOptions) *Service {
	return &Service{demoProvider: options.DemoProvider}
}

// Analyze 在真实 PR 分析完成前，将请求分派给 demo provider。
func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (<-chan ReviewEvent, error) {
	if req.Demo {
		if s.demoProvider == nil {
			return nil, errors.New("demo provider is not configured")
		}
		return s.demoProvider.Stream(ctx)
	}

	return nil, ErrRealAnalysisNotImplemented
}
