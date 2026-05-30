package diff

import (
	"path/filepath"
	"strings"
)

// FileStats 是从 diff 解析阶段传给规则扫描阶段的精简统计。
type FileStats struct {
	ChangedFiles      int
	Additions         int
	Deletions         int
	SourceFiles       int
	TestFiles         int
	ConfigFiles       int
	DependencyFiles   int
	LockfileFiles     int
	CIFiles           int
	DocsFiles         int
	MissingPatchFiles int
}

// Parser 预留给 patch 解析和文件分类逻辑。
type Parser struct{}

// NewParser 返回无状态解析器实例，供后续 diff 分析使用。
func NewParser() *Parser {
	return &Parser{}
}

// Analyze normalizes file inputs and classifies each file. Patch hunk parsing
// is intentionally deferred to a later task.
func (p *Parser) Analyze(files []FileInput) Analysis {
	analysis := Analysis{
		Files: make([]FileDiff, 0, len(files)),
		Stats: FileStats{
			ChangedFiles: len(files),
		},
	}

	for _, file := range files {
		patchStatus := classifyPatchStatus(file.Patch)
		kinds := ClassifyFile(file.Filename)

		fileDiff := FileDiff{
			Filename:    file.Filename,
			Status:      file.Status,
			Kinds:       kinds,
			Additions:   file.Additions,
			Deletions:   file.Deletions,
			Changes:     file.Changes,
			Patch:       file.Patch,
			Hunks:       nil,
			HasPatch:    patchStatus == PatchStatusPresent,
			PatchStatus: patchStatus,
		}

		analysis.Files = append(analysis.Files, fileDiff)
		analysis.Stats.Additions += file.Additions
		analysis.Stats.Deletions += file.Deletions
		addKindStats(&analysis.Stats, kinds)
		if patchStatus == PatchStatusMissing || patchStatus == PatchStatusBinaryOrOmitted {
			analysis.Stats.MissingPatchFiles++
		}
	}

	return analysis
}

// ClassifyFile returns all known roles for filename in stable order.
func ClassifyFile(filename string) []FileKind {
	normalized := normalizeFilename(filename)
	base := pathBase(normalized)
	ext := strings.ToLower(filepath.Ext(base))

	var kinds []FileKind
	if isSourceFile(ext) {
		kinds = append(kinds, FileKindSource)
	}
	if isTestFile(normalized, base) {
		kinds = append(kinds, FileKindTest)
	}
	if isCIFile(normalized, base) {
		kinds = append(kinds, FileKindCI)
	}
	if isConfigFile(normalized, base, ext) {
		kinds = append(kinds, FileKindConfig)
	}
	if isDependencyFile(normalized, base) {
		kinds = append(kinds, FileKindDependency)
	}
	if isLockfile(base) {
		kinds = append(kinds, FileKindLockfile)
	}
	if isDocsFile(normalized, base, ext) {
		kinds = append(kinds, FileKindDocs)
	}

	return kinds
}

func normalizeFilename(filename string) string {
	return strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))
}

func pathBase(filename string) string {
	idx := strings.LastIndex(filename, "/")
	if idx == -1 {
		return filename
	}
	return filename[idx+1:]
}

func isSourceFile(ext string) bool {
	switch ext {
	case ".go", ".ts", ".tsx", ".js":
		return true
	default:
		return false
	}
}

func isTestFile(filename, base string) bool {
	if strings.Contains(filename, "/__tests__/") || strings.Contains(filename, "/test/") || strings.Contains(filename, "/tests/") {
		return true
	}

	testSuffixes := []string{
		"_test.go",
		".test.ts",
		".test.tsx",
		".test.js",
		".spec.ts",
		".spec.tsx",
		".spec.js",
	}
	for _, suffix := range testSuffixes {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func isCIFile(filename, base string) bool {
	if strings.HasPrefix(filename, ".github/workflows/") {
		return true
	}

	switch base {
	case ".gitlab-ci.yml", ".gitlab-ci.yaml", "azure-pipelines.yml", "azure-pipelines.yaml", "jenkinsfile":
		return true
	default:
		return strings.HasPrefix(filename, ".circleci/")
	}
}

func isConfigFile(filename, base, ext string) bool {
	if strings.HasPrefix(filename, ".github/workflows/") || strings.HasPrefix(filename, ".circleci/") {
		return true
	}

	switch base {
	case "dockerfile", "makefile", ".editorconfig", ".gitignore", ".golangci.yml", ".golangci.yaml":
		return true
	}

	switch ext {
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".env":
		return true
	default:
		return false
	}
}

func isDependencyFile(filename, base string) bool {
	switch base {
	case "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.mod", "go.sum", "requirements.txt", "pyproject.toml", "poetry.lock",
		"cargo.toml", "cargo.lock", "gemfile", "gemfile.lock":
		return true
	default:
		return strings.HasPrefix(filename, "vendor/") || strings.Contains(filename, "/vendor/")
	}
}

func isLockfile(base string) bool {
	switch base {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml", "go.sum", "poetry.lock", "cargo.lock", "gemfile.lock":
		return true
	default:
		return false
	}
}

func isDocsFile(filename, base, ext string) bool {
	if base == "readme.md" || strings.HasPrefix(base, "readme.") {
		return true
	}
	if strings.HasPrefix(filename, "docs/") || strings.Contains(filename, "/docs/") {
		return true
	}

	switch ext {
	case ".md", ".rst", ".adoc":
		return true
	default:
		return false
	}
}

func classifyPatchStatus(patch string) PatchStatus {
	if patch == "" {
		return PatchStatusMissing
	}
	if strings.TrimSpace(patch) == "" {
		return PatchStatusEmpty
	}
	return PatchStatusPresent
}

func addKindStats(stats *FileStats, kinds []FileKind) {
	for _, kind := range kinds {
		switch kind {
		case FileKindSource:
			stats.SourceFiles++
		case FileKindTest:
			stats.TestFiles++
		case FileKindConfig:
			stats.ConfigFiles++
		case FileKindDependency:
			stats.DependencyFiles++
		case FileKindLockfile:
			stats.LockfileFiles++
		case FileKindCI:
			stats.CIFiles++
		case FileKindDocs:
			stats.DocsFiles++
		}
	}
}
