package diff

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyFileIdentifiesTestFiles(t *testing.T) {
	kinds := ClassifyFile("src/app_test.go")

	assertHasKinds(t, kinds, FileKindTest)
}

func TestClassifyFileIdentifiesRootLevelTestDirectories(t *testing.T) {
	tests := []string{
		"tests/foo_test.ts",
		"test/helper.go",
		"__tests__/app.test.ts",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			kinds := ClassifyFile(filename)

			assertHasKinds(t, kinds, FileKindTest)
		})
	}
}

func TestClassifyFileIdentifiesWorkflowAsCIAndConfig(t *testing.T) {
	kinds := ClassifyFile(".github/workflows/ci.yml")

	assertHasKinds(t, kinds, FileKindCI, FileKindConfig)
}

func TestClassifyFileIdentifiesPackageLockAsDependencyAndLockfile(t *testing.T) {
	kinds := ClassifyFile("package-lock.json")

	assertHasKinds(t, kinds, FileKindDependency, FileKindLockfile)
}

func TestClassifyFileIdentifiesDocs(t *testing.T) {
	kinds := ClassifyFile("README.md")

	assertHasKinds(t, kinds, FileKindDocs)
}

func TestClassifyFileIdentifiesSourceFiles(t *testing.T) {
	tests := []string{
		"cmd/server/main.go",
		"src/app.ts",
		"src/App.tsx",
		"web/index.js",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			kinds := ClassifyFile(filename)

			assertHasKinds(t, kinds, FileKindSource)
		})
	}
}

func TestAnalyzeBuildsFileDiffsAndStats(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{
			Filename:  ".github/workflows/ci.yml",
			Status:    "modified",
			Additions: 4,
			Deletions: 1,
			Changes:   5,
			Patch:     "@@ -1 +1 @@\n-old\n+new",
		},
		{
			Filename: "README.md",
			Status:   "modified",
		},
		{
			Filename:             "dist/app.min.js",
			Status:               "modified",
			PatchBinaryOrOmitted: true,
		},
	})

	if len(analysis.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(analysis.Files))
	}

	workflow := analysis.Files[0]
	assertHasKinds(t, workflow.Kinds, FileKindCI, FileKindConfig)
	if !workflow.HasPatch {
		t.Fatalf("expected workflow file to have patch")
	}
	if workflow.PatchStatus != PatchStatusPresent {
		t.Fatalf("expected workflow patch status %q, got %q", PatchStatusPresent, workflow.PatchStatus)
	}

	readme := analysis.Files[1]
	assertHasKinds(t, readme.Kinds, FileKindDocs)
	if readme.HasPatch {
		t.Fatalf("expected readme file to not have patch")
	}
	if readme.PatchStatus != PatchStatusMissing {
		t.Fatalf("expected readme patch status %q, got %q", PatchStatusMissing, readme.PatchStatus)
	}

	binary := analysis.Files[2]
	assertHasKinds(t, binary.Kinds, FileKindSource)
	if binary.HasPatch {
		t.Fatalf("expected binary-or-omitted file to not have patch")
	}
	if binary.PatchStatus != PatchStatusBinaryOrOmitted {
		t.Fatalf("expected binary patch status %q, got %q", PatchStatusBinaryOrOmitted, binary.PatchStatus)
	}

	stats := analysis.Stats
	if stats.ChangedFiles != 3 {
		t.Fatalf("expected changed files 3, got %d", stats.ChangedFiles)
	}
	if stats.Additions != 4 {
		t.Fatalf("expected additions 4, got %d", stats.Additions)
	}
	if stats.Deletions != 1 {
		t.Fatalf("expected deletions 1, got %d", stats.Deletions)
	}
	if stats.SourceFiles != 1 {
		t.Fatalf("expected source files 1, got %d", stats.SourceFiles)
	}
	if stats.ConfigFiles != 1 {
		t.Fatalf("expected config files 1, got %d", stats.ConfigFiles)
	}
	if stats.CIFiles != 1 {
		t.Fatalf("expected CI files 1, got %d", stats.CIFiles)
	}
	if stats.DocsFiles != 1 {
		t.Fatalf("expected docs files 1, got %d", stats.DocsFiles)
	}
	if stats.MissingPatchFiles != 2 {
		t.Fatalf("expected missing patch files 2, got %d", stats.MissingPatchFiles)
	}
}

func TestAnalyzeParsesGitHubUnifiedPatchHunks(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{
			Filename: "internal/diff/parser.go",
			Status:   "modified",
			Patch: strings.Join([]string{
				"@@ -10,3 +10,4 @@ func parse() {",
				" context",
				"-old",
				"+new",
				"+added",
				" tail",
				"@@ -30,2 +31,2 @@ func other() {",
				"-gone",
				"+back",
				`\ No newline at end of file`,
			}, "\n") + "\n",
		},
	})

	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", analysis.Warnings)
	}
	if len(analysis.Files) != 1 {
		t.Fatalf("expected one file, got %d", len(analysis.Files))
	}

	hunks := analysis.Files[0].Hunks
	if len(hunks) != 2 {
		t.Fatalf("expected two hunks, got %#v", hunks)
	}
	if hunks[0].Header != "@@ -10,3 +10,4 @@ func parse() {" {
		t.Fatalf("unexpected first header: %q", hunks[0].Header)
	}
	if hunks[0].Context != "func parse() {" {
		t.Fatalf("unexpected first context: %q", hunks[0].Context)
	}

	assertDiffLine(t, hunks[0].Lines[0], DiffLineContext, "context", 10, 10)
	assertDiffLine(t, hunks[0].Lines[1], DiffLineRemoved, "old", 11, 0)
	assertDiffLine(t, hunks[0].Lines[2], DiffLineAdded, "new", 0, 11)
	assertDiffLine(t, hunks[0].Lines[3], DiffLineAdded, "added", 0, 12)
	assertDiffLine(t, hunks[0].Lines[4], DiffLineContext, "tail", 12, 13)

	assertDiffLine(t, hunks[1].Lines[0], DiffLineRemoved, "gone", 30, 0)
	assertDiffLine(t, hunks[1].Lines[1], DiffLineAdded, "back", 0, 31)
	if len(hunks[1].Lines) != 2 {
		t.Fatalf("expected newline marker to be skipped, got %#v", hunks[1].Lines)
	}
}

func TestAnalyzeParsesZeroCountNewFileHunkLineNumbers(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{
			Filename: "new-file.go",
			Status:   "added",
			Patch: strings.Join([]string{
				"@@ -0,0 +1,3 @@",
				"+package diff",
				"+",
				"+func added() {}",
			}, "\n"),
		},
	})

	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", analysis.Warnings)
	}
	hunks := analysis.Files[0].Hunks
	if len(hunks) != 1 {
		t.Fatalf("expected one hunk, got %#v", hunks)
	}
	if len(hunks[0].Lines) != 3 {
		t.Fatalf("expected three added lines, got %#v", hunks[0].Lines)
	}

	assertDiffLine(t, hunks[0].Lines[0], DiffLineAdded, "package diff", 0, 1)
	assertDiffLine(t, hunks[0].Lines[1], DiffLineAdded, "", 0, 2)
	assertDiffLine(t, hunks[0].Lines[2], DiffLineAdded, "func added() {}", 0, 3)
}

func TestAnalyzeParsesZeroCountDeletedFileHunkLineNumbers(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{
			Filename: "deleted-file.go",
			Status:   "removed",
			Patch: strings.Join([]string{
				"@@ -1,3 +0,0 @@",
				"-package diff",
				"-",
				"-func removed() {}",
			}, "\n"),
		},
	})

	if len(analysis.Warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", analysis.Warnings)
	}
	hunks := analysis.Files[0].Hunks
	if len(hunks) != 1 {
		t.Fatalf("expected one hunk, got %#v", hunks)
	}
	if len(hunks[0].Lines) != 3 {
		t.Fatalf("expected three removed lines, got %#v", hunks[0].Lines)
	}

	assertDiffLine(t, hunks[0].Lines[0], DiffLineRemoved, "package diff", 1, 0)
	assertDiffLine(t, hunks[0].Lines[1], DiffLineRemoved, "", 2, 0)
	assertDiffLine(t, hunks[0].Lines[2], DiffLineRemoved, "func removed() {}", 3, 0)
}

func TestAnalyzeMalformedHunkAddsWarningAndKeepsParseableHunks(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{
			Filename:  "broken.patch",
			Status:    "modified",
			Additions: 7,
			Deletions: 3,
			Patch: strings.Join([]string{
				"@@ -1,1 +1,1 @@ valid",
				"-old",
				"+new",
				"@@ definitely malformed @@",
				"+ignored",
				"@@ -10 +10 @@ another valid",
				" same",
			}, "\n"),
		},
	})

	if analysis.Stats.Additions != 7 || analysis.Stats.Deletions != 3 {
		t.Fatalf("expected file stats to be preserved, got additions=%d deletions=%d", analysis.Stats.Additions, analysis.Stats.Deletions)
	}
	if len(analysis.Warnings) == 0 {
		t.Fatalf("expected warning for malformed hunk")
	}
	if analysis.Warnings[0].Filename != "broken.patch" {
		t.Fatalf("expected warning filename broken.patch, got %q", analysis.Warnings[0].Filename)
	}

	hunks := analysis.Files[0].Hunks
	if len(hunks) != 2 {
		t.Fatalf("expected two parseable hunks, got %#v", hunks)
	}
	if hunks[0].Header != "@@ -1,1 +1,1 @@ valid" || hunks[1].Header != "@@ -10 +10 @@ another valid" {
		t.Fatalf("unexpected parseable hunks: %#v", hunks)
	}
}

func TestAnalyzeIncludesFilesWithoutPatchAndSetsPatchStatus(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{Filename: "empty.patch", Status: "modified", Patch: "   \n\t"},
		{Filename: "missing.patch", Status: "modified"},
		{Filename: "image.png", Status: "modified", PatchBinaryOrOmitted: true},
	})

	if len(analysis.Files) != 3 {
		t.Fatalf("expected all files to be included, got %d", len(analysis.Files))
	}
	if analysis.Files[0].PatchStatus != PatchStatusEmpty {
		t.Fatalf("expected empty patch status, got %q", analysis.Files[0].PatchStatus)
	}
	if analysis.Files[1].PatchStatus != PatchStatusMissing {
		t.Fatalf("expected missing patch status, got %q", analysis.Files[1].PatchStatus)
	}
	if analysis.Files[2].PatchStatus != PatchStatusBinaryOrOmitted {
		t.Fatalf("expected binary/omitted patch status, got %q", analysis.Files[2].PatchStatus)
	}
	if analysis.Stats.MissingPatchFiles != 2 {
		t.Fatalf("expected missing patch count to include missing and binary/omitted only, got %d", analysis.Stats.MissingPatchFiles)
	}
}

func TestAnalyzeSummaryStatsComeFromInputsAndKindsWithoutPatch(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{Filename: "cmd/server/main.go", Status: "modified", Additions: 10, Deletions: 2},
		{Filename: "cmd/server/main_test.go", Status: "modified", Additions: 5, Deletions: 1},
		{Filename: ".github/workflows/ci.yml", Status: "modified", Additions: 3, Deletions: 1},
		{Filename: "package-lock.json", Status: "modified", Additions: 100, Deletions: 90},
		{Filename: "docs/README.md", Status: "modified", Additions: 4, Deletions: 0, PatchBinaryOrOmitted: true},
	})

	stats := analysis.Stats
	if stats.ChangedFiles != 5 || stats.Additions != 122 || stats.Deletions != 94 {
		t.Fatalf("unexpected aggregate stats: %#v", stats)
	}
	if stats.SourceFiles != 2 {
		t.Fatalf("expected source files to include Go test file too, got %d", stats.SourceFiles)
	}
	if stats.TestFiles != 1 || stats.ConfigFiles != 2 || stats.DependencyFiles != 1 || stats.LockfileFiles != 1 || stats.CIFiles != 1 || stats.DocsFiles != 1 {
		t.Fatalf("unexpected kind stats: %#v", stats)
	}
	if !stats.HasSourceChanges || !stats.HasTestChanges {
		t.Fatalf("expected source and test changes, got %#v", stats)
	}
}

func TestAnalyzeMarksSourceChangesWithoutTestChanges(t *testing.T) {
	parser := NewParser()

	analysis := parser.Analyze([]FileInput{
		{Filename: "internal/diff/parser.go", Status: "modified", Additions: 1},
		{Filename: "README.md", Status: "modified", Additions: 1},
	})

	if !analysis.Stats.HasSourceChanges {
		t.Fatalf("expected source changes")
	}
	if analysis.Stats.HasTestChanges {
		t.Fatalf("expected no test changes")
	}
}

func TestDiffPackageDoesNotImportInternalGithub(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read diff package dir: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		filename := filepath.Join(".", entry.Name())
		file, err := parser.ParseFile(fset, filename, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports for %s: %v", filename, err)
		}
		for _, importSpec := range file.Imports {
			if strings.Contains(strings.Trim(importSpec.Path.Value, `"`), "/internal/github") {
				t.Fatalf("%s imports internal/github via %s", filename, importSpec.Path.Value)
			}
		}
	}
}

func assertHasKinds(t *testing.T, actual []FileKind, expected ...FileKind) {
	t.Helper()

	actualSet := make(map[FileKind]bool, len(actual))
	for _, kind := range actual {
		actualSet[kind] = true
	}

	for _, kind := range expected {
		if !actualSet[kind] {
			t.Fatalf("expected kinds %v to include %q", actual, kind)
		}
	}
}

func assertDiffLine(t *testing.T, actual DiffLine, expectedKind DiffLineKind, expectedContent string, expectedOldLine int, expectedNewLine int) {
	t.Helper()

	if actual.Kind != expectedKind {
		t.Fatalf("expected line kind %q, got %q in %#v", expectedKind, actual.Kind, actual)
	}
	if actual.Content != expectedContent {
		t.Fatalf("expected line content %q, got %q", expectedContent, actual.Content)
	}
	if actual.OldLine != expectedOldLine {
		t.Fatalf("expected old line %d, got %d in %#v", expectedOldLine, actual.OldLine, actual)
	}
	if actual.NewLine != expectedNewLine {
		t.Fatalf("expected new line %d, got %d in %#v", expectedNewLine, actual.NewLine, actual)
	}
}
