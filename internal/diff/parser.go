package diff

import (
	"path/filepath"
	"regexp"
	"strconv"
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
	HasSourceChanges  bool
	HasTestChanges    bool
}

// Parser 预留给 patch 解析和文件分类逻辑。
type Parser struct{}

// NewParser 返回无状态解析器实例，供后续 diff 分析使用。
func NewParser() *Parser {
	return &Parser{}
}

// Analyze normalizes file inputs, classifies each file, and parses available
// patch hunks without aborting analysis on malformed hunks.
func (p *Parser) Analyze(files []FileInput) Analysis {
	analysis := Analysis{
		Files: make([]FileDiff, 0, len(files)),
		Stats: FileStats{
			ChangedFiles: len(files),
		},
	}

	for _, file := range files {
		patchStatus := classifyPatchStatus(file)
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
		if patchStatus == PatchStatusPresent {
			hunks, warnings := parsePatch(file.Filename, file.Patch)
			fileDiff.Hunks = hunks
			analysis.Warnings = append(analysis.Warnings, warnings...)
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
	if strings.HasPrefix(filename, "__tests__/") || strings.HasPrefix(filename, "test/") || strings.HasPrefix(filename, "tests/") {
		return true
	}
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

func classifyPatchStatus(file FileInput) PatchStatus {
	if file.PatchBinaryOrOmitted {
		return PatchStatusBinaryOrOmitted
	}
	if file.Patch == "" {
		return PatchStatusMissing
	}
	if strings.TrimSpace(file.Patch) == "" {
		return PatchStatusEmpty
	}
	return PatchStatusPresent
}

func addKindStats(stats *FileStats, kinds []FileKind) {
	for _, kind := range kinds {
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
			stats.LockfileFiles++
		case FileKindCI:
			stats.CIFiles++
		case FileKindDocs:
			stats.DocsFiles++
		}
	}
}

var hunkHeaderPattern = regexp.MustCompile(`^@@ -([0-9]+)(?:,([0-9]+))? \+([0-9]+)(?:,([0-9]+))? @@ ?(.*)$`)

func parsePatch(filename, patch string) ([]Hunk, []Warning) {
	lines := strings.Split(patch, "\n")
	hunks := make([]Hunk, 0)
	var warnings []Warning
	var current *Hunk
	var oldLine int
	var newLine int

	for idx, line := range lines {
		if strings.HasPrefix(line, `\ No newline at end of file`) {
			continue
		}

		if strings.HasPrefix(line, "@@") {
			match := hunkHeaderPattern.FindStringSubmatch(line)
			if match == nil {
				warnings = append(warnings, Warning{
					Filename: filename,
					Message:  "malformed patch hunk header: " + line,
				})
				current = nil
				continue
			}

			parsedOldLine, err := strconv.Atoi(match[1])
			if err != nil {
				warnings = append(warnings, Warning{
					Filename: filename,
					Message:  "malformed patch hunk old start: " + line,
				})
				current = nil
				continue
			}
			parsedNewLine, err := strconv.Atoi(match[3])
			if err != nil {
				warnings = append(warnings, Warning{
					Filename: filename,
					Message:  "malformed patch hunk new start: " + line,
				})
				current = nil
				continue
			}

			hunks = append(hunks, Hunk{
				Header:  line,
				Context: strings.TrimSpace(match[5]),
				Lines:   []DiffLine{},
			})
			current = &hunks[len(hunks)-1]
			oldLine = parsedOldLine
			newLine = parsedNewLine
			continue
		}

		if current == nil {
			continue
		}

		if line == "" {
			if idx == len(lines)-1 {
				continue
			}
			warnings = append(warnings, Warning{
				Filename: filename,
				Message:  "malformed patch line outside unified diff marker",
			})
			continue
		}

		switch line[0] {
		case '+':
			current.Lines = append(current.Lines, DiffLine{
				Kind:    DiffLineAdded,
				Content: line[1:],
				NewLine: newLine,
			})
			newLine++
		case '-':
			current.Lines = append(current.Lines, DiffLine{
				Kind:    DiffLineRemoved,
				Content: line[1:],
				OldLine: oldLine,
			})
			oldLine++
		case ' ':
			current.Lines = append(current.Lines, DiffLine{
				Kind:    DiffLineContext,
				Content: line[1:],
				OldLine: oldLine,
				NewLine: newLine,
			})
			oldLine++
			newLine++
		default:
			warnings = append(warnings, Warning{
				Filename: filename,
				Message:  "malformed patch line: " + line,
			})
		}
	}

	return hunks, warnings
}
