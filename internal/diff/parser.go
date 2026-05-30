package diff

// FileStats 是从 diff 解析阶段传给规则扫描阶段的精简统计。
type FileStats struct {
	ChangedFiles    int
	Additions       int
	Deletions       int
	TestFiles       int
	ConfigFiles     int
	DependencyFiles int
}

// Parser 预留给 patch 解析和文件分类逻辑。
type Parser struct{}

// NewParser 返回无状态解析器实例，供后续 diff 分析使用。
func NewParser() *Parser {
	return &Parser{}
}
