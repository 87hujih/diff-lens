package diff

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// FileStats 是从 diff 解析阶段传给规则扫描阶段的精简统计。
type FileStats struct {
	ChangedFiles              int
	Additions                 int
	Deletions                 int
	TestFiles                 int
	ConfigFiles               int
	DependencyFiles           int
	LockFiles                 int
	CIFiles                   int
	DocsFiles                 int
	SourceFiles               int
	MissingPatchFiles         int
	BinaryOrOmittedPatchFiles int
	HasSourceChanges          bool
	HasTestChanges            bool
}

// Parser 预留给 patch 解析和文件分类逻辑。
type Parser struct{}

// NewParser 返回无状态解析器实例，供后续 diff 分析使用。
func NewParser() *Parser {
	return &Parser{}
}

var hunkHeaderPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@(.*)$`)

// ParseFiles turns provider-neutral changed file inputs into structured diff
// analysis. Recoverable per-file parse problems are returned as warnings.
func (p *Parser) ParseFiles(files []FileInput) (Analysis, error) {
	analysis := Analysis{
		Files: make([]FileDiff, 0, len(files)),
	}

	for _, input := range files {
		kinds := ClassifyFile(input.Filename)
		status := patchStatus(input)
		file := FileDiff{
			Filename:    input.Filename,
			Status:      input.Status,
			Kinds:       kinds,
			Additions:   input.Additions,
			Deletions:   input.Deletions,
			Changes:     input.Changes,
			Patch:       input.Patch,
			HasPatch:    status == PatchStatusPresent,
			PatchStatus: status,
		}

		if status == PatchStatusPresent {
			hunks, warnings := parsePatch(input.Filename, input.Patch)
			file.Hunks = hunks
			analysis.Warnings = append(analysis.Warnings, warnings...)
		}

		analysis.Files = append(analysis.Files, file)
		addStats(&analysis.Stats, file)
	}

	return analysis, nil
}

// ClassifyFile returns all broad categories that apply to filename.
func ClassifyFile(filename string) []FileKind {
	normalized := strings.ReplaceAll(filename, `\`, `/`)
	lower := strings.ToLower(normalized)
	base := strings.ToLower(filepath.Base(lower))
	ext := strings.ToLower(filepath.Ext(base))

	var kinds []FileKind
	if isTestFile(lower, base) {
		kinds = append(kinds, FileKindTest)
	}
	if isCIFile(lower, base) {
		kinds = append(kinds, FileKindCI)
	}
	if isDependencyFile(lower, base) {
		kinds = append(kinds, FileKindDependency)
	}
	if isLockFile(lower, base) {
		kinds = append(kinds, FileKindLockfile)
	}
	if isDocsFile(lower, ext) {
		kinds = append(kinds, FileKindDocs)
	}
	if isConfigFile(lower, base, ext) {
		kinds = append(kinds, FileKindConfig)
	}
	if isSourceFile(ext) && !hasKind(kinds, FileKindTest) {
		kinds = append(kinds, FileKindSource)
	}

	return kinds
}

func patchStatus(input FileInput) PatchStatus {
	switch {
	case input.BinaryOrOmitted:
		return PatchStatusBinaryOrOmitted
	case input.PatchMissing:
		return PatchStatusMissing
	case input.Patch == "":
		return PatchStatusEmpty
	default:
		return PatchStatusPresent
	}
}

func parsePatch(filename, patch string) ([]DiffHunk, []Warning) {
	lines := strings.Split(patch, "\n")
	hunks := make([]DiffHunk, 0)
	warnings := make([]Warning, 0)
	var current *DiffHunk
	oldLine := 0
	newLine := 0

	flush := func() {
		if current == nil {
			return
		}
		hunks = append(hunks, *current)
		current = nil
	}

	for idx, rawLine := range lines {
		lineNumber := idx + 1
		line := strings.TrimSuffix(rawLine, "\r")
		if line == `\ No newline at end of file` {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			match := hunkHeaderPattern.FindStringSubmatch(line)
			if match == nil {
				warnings = append(warnings, Warning{
					File:    filename,
					Line:    lineNumber,
					Message: "malformed hunk header",
				})
				flush()
				continue
			}

			flush()
			oldStart, oldCount := parseRange(match[1], match[2])
			newStart, newCount := parseRange(match[3], match[4])
			current = &DiffHunk{
				Header:   line,
				Context:  strings.TrimSpace(match[5]),
				OldStart: oldStart,
				OldCount: oldCount,
				NewStart: newStart,
				NewCount: newCount,
			}
			oldLine = oldStart
			newLine = newStart
			continue
		}

		if current == nil {
			continue
		}

		if line == "" {
			current.Lines = append(current.Lines, DiffLine{
				Type:    DiffLineContext,
				OldLine: oldLine,
				NewLine: newLine,
				Content: "",
			})
			oldLine++
			newLine++
			continue
		}

		switch line[0] {
		case '+':
			current.Lines = append(current.Lines, DiffLine{
				Type:    DiffLineAdded,
				NewLine: newLine,
				Content: line[1:],
			})
			newLine++
		case '-':
			current.Lines = append(current.Lines, DiffLine{
				Type:    DiffLineRemoved,
				OldLine: oldLine,
				Content: line[1:],
			})
			oldLine++
		case ' ':
			current.Lines = append(current.Lines, DiffLine{
				Type:    DiffLineContext,
				OldLine: oldLine,
				NewLine: newLine,
				Content: line[1:],
			})
			oldLine++
			newLine++
		default:
			warnings = append(warnings, Warning{
				File:    filename,
				Line:    lineNumber,
				Message: fmt.Sprintf("unexpected diff line %q", line),
			})
		}
	}
	flush()

	return hunks, warnings
}

func parseRange(start, count string) (int, int) {
	parsedStart, _ := strconv.Atoi(start)
	if count == "" {
		return parsedStart, 1
	}
	parsedCount, _ := strconv.Atoi(count)
	return parsedStart, parsedCount
}

func addStats(stats *FileStats, file FileDiff) {
	stats.ChangedFiles++
	stats.Additions += file.Additions
	stats.Deletions += file.Deletions

	if file.PatchStatus == PatchStatusMissing {
		stats.MissingPatchFiles++
	}
	if file.PatchStatus == PatchStatusBinaryOrOmitted {
		stats.BinaryOrOmittedPatchFiles++
	}
	for _, kind := range file.Kinds {
		switch kind {
		case FileKindSource:
			stats.SourceFiles++
			stats.HasSourceChanges = true
		case FileKindTest:
			stats.TestFiles++
			stats.HasTestChanges = true
		case FileKindConfig:
			stats.ConfigFiles++
		case FileKindDependency:
			stats.DependencyFiles++
		case FileKindLockfile:
			stats.LockFiles++
		case FileKindCI:
			stats.CIFiles++
		case FileKindDocs:
			stats.DocsFiles++
		}
	}
}

func isTestFile(path, base string) bool {
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.Contains(path, "/test/") ||
		strings.Contains(path, "/tests/")
}

func isCIFile(path, base string) bool {
	return strings.HasPrefix(path, ".github/workflows/") ||
		base == ".gitlab-ci.yml" ||
		base == ".travis.yml" ||
		base == "azure-pipelines.yml" ||
		strings.Contains(path, ".circleci/")
}

func isDependencyFile(path, base string) bool {
	switch base {
	case "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.mod", "go.sum", "requirements.txt", "poetry.lock", "pipfile",
		"pipfile.lock", "cargo.toml", "cargo.lock", "gemfile", "gemfile.lock",
		"composer.json", "composer.lock":
		return true
	}
	return strings.HasSuffix(path, "/requirements.txt")
}

func isLockFile(_ string, base string) bool {
	return strings.HasSuffix(base, ".lock") ||
		base == "package-lock.json" ||
		base == "yarn.lock" ||
		base == "pnpm-lock.yaml" ||
		base == "go.sum" ||
		base == "cargo.lock" ||
		base == "gemfile.lock" ||
		base == "composer.lock"
}

func isDocsFile(path, ext string) bool {
	return ext == ".md" ||
		ext == ".mdx" ||
		ext == ".rst" ||
		ext == ".txt" ||
		strings.HasPrefix(path, "docs/")
}

func isConfigFile(path, base, ext string) bool {
	if isDependencyFile(path, base) {
		return false
	}
	if isCIFile(path, base) {
		return true
	}
	if base == "dockerfile" || strings.HasPrefix(base, "dockerfile.") {
		return true
	}
	if strings.HasPrefix(base, ".env") {
		return true
	}
	switch ext {
	case ".yml", ".yaml", ".toml", ".ini", ".cfg", ".conf", ".xml":
		return true
	}
	return strings.HasPrefix(base, ".")
}

func isSourceFile(ext string) bool {
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rb", ".rs", ".java",
		".kt", ".kts", ".c", ".cc", ".cpp", ".h", ".hpp", ".cs", ".php",
		".swift", ".m", ".mm":
		return true
	default:
		return false
	}
}

func hasKind(kinds []FileKind, want FileKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
