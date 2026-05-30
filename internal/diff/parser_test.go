package diff

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestClassifyFileKinds(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     []FileKind
	}{
		{name: "go test", filename: "src/app_test.go", want: []FileKind{FileKindTest}},
		{name: "github workflow", filename: ".github/workflows/ci.yml", want: []FileKind{FileKindCI, FileKindConfig}},
		{name: "lockfile dependency", filename: "package-lock.json", want: []FileKind{FileKindDependency, FileKindLockfile}},
		{name: "docs", filename: "README.md", want: []FileKind{FileKindDocs}},
		{name: "go source", filename: "internal/app/main.go", want: []FileKind{FileKindSource}},
		{name: "typescript source", filename: "web/app.ts", want: []FileKind{FileKindSource}},
		{name: "tsx source", filename: "web/App.tsx", want: []FileKind{FileKindSource}},
		{name: "javascript source", filename: "web/app.js", want: []FileKind{FileKindSource}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kinds := ClassifyFile(tt.filename)
			for _, want := range tt.want {
				if !testHasKind(kinds, want) {
					t.Fatalf("ClassifyFile(%q) = %v, missing %q", tt.filename, kinds, want)
				}
			}
		})
	}
}

func TestParseFilesParsesUnifiedDiffHunksAndLineNumbers(t *testing.T) {
	patch := `@@ -10,4 +10,5 @@ func example() {
 context line
-old value
+new value
+another value
 unchanged
@@ -30,2 +31,2 @@ func second() {
-remove me
+add me
\ No newline at end of file`

	analysis, err := NewParser().ParseFiles([]FileInput{{
		Filename:  "internal/app/main.go",
		Status:    "modified",
		Additions: 3,
		Deletions: 2,
		Changes:   5,
		Patch:     patch,
	}})
	if err != nil {
		t.Fatalf("ParseFiles returned error: %v", err)
	}

	if len(analysis.Files) != 1 {
		t.Fatalf("got %d files, want 1", len(analysis.Files))
	}
	file := analysis.Files[0]
	if file.PatchStatus != PatchStatusPresent {
		t.Fatalf("PatchStatus = %q, want %q", file.PatchStatus, PatchStatusPresent)
	}
	if len(file.Hunks) != 2 {
		t.Fatalf("got %d hunks, want 2", len(file.Hunks))
	}

	first := file.Hunks[0]
	if first.OldStart != 10 || first.OldCount != 4 || first.NewStart != 10 || first.NewCount != 5 {
		t.Fatalf("unexpected first hunk header: %+v", first)
	}
	assertLine(t, first.Lines[0], DiffLineContext, 10, 10, "context line")
	assertLine(t, first.Lines[1], DiffLineRemoved, 11, 0, "old value")
	assertLine(t, first.Lines[2], DiffLineAdded, 0, 11, "new value")
	assertLine(t, first.Lines[3], DiffLineAdded, 0, 12, "another value")
	assertLine(t, first.Lines[4], DiffLineContext, 12, 13, "unchanged")

	second := file.Hunks[1]
	if len(second.Lines) != 2 {
		t.Fatalf("second hunk got %d normal diff lines, want 2", len(second.Lines))
	}
	assertLine(t, second.Lines[0], DiffLineRemoved, 30, 0, "remove me")
	assertLine(t, second.Lines[1], DiffLineAdded, 0, 31, "add me")
}

func TestParseFilesTracksPatchStatusAndMalformedWarnings(t *testing.T) {
	analysis, err := NewParser().ParseFiles([]FileInput{
		{Filename: "empty.go", Status: "modified", Patch: ""},
		{Filename: "missing.png", Status: "modified", Patch: "", PatchMissing: true},
		{Filename: "large.dat", Status: "modified", Patch: "", BinaryOrOmitted: true},
		{Filename: "bad.go", Status: "modified", Patch: "@@ not a valid hunk @@\n+still parsed as warning only"},
	})
	if err != nil {
		t.Fatalf("ParseFiles returned error: %v", err)
	}

	assertPatchStatus(t, analysis, "empty.go", PatchStatusEmpty)
	assertPatchStatus(t, analysis, "missing.png", PatchStatusMissing)
	assertPatchStatus(t, analysis, "large.dat", PatchStatusBinaryOrOmitted)
	assertPatchStatus(t, analysis, "bad.go", PatchStatusPresent)

	if analysis.Stats.MissingPatchFiles != 1 {
		t.Fatalf("MissingPatchFiles = %d, want 1", analysis.Stats.MissingPatchFiles)
	}
	if analysis.Stats.BinaryOrOmittedPatchFiles != 1 {
		t.Fatalf("BinaryOrOmittedPatchFiles = %d, want 1", analysis.Stats.BinaryOrOmittedPatchFiles)
	}
	if len(analysis.Warnings) == 0 {
		t.Fatal("expected malformed hunk warning")
	}
}

func TestParseFilesAggregatesStats(t *testing.T) {
	analysis, err := NewParser().ParseFiles([]FileInput{
		{Filename: "internal/app/main.go", Status: "modified", Additions: 10, Deletions: 2, Changes: 12, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "internal/app/main_test.go", Status: "modified", Additions: 3, Deletions: 1, Changes: 4, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: ".github/workflows/ci.yml", Status: "modified", Additions: 5, Deletions: 0, Changes: 5, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "package-lock.json", Status: "modified", Additions: 7, Deletions: 4, Changes: 11, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "README.md", Status: "modified", Additions: 2, Deletions: 0, Changes: 2, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "asset.bin", Status: "modified", Additions: 0, Deletions: 0, Changes: 0, BinaryOrOmitted: true},
	})
	if err != nil {
		t.Fatalf("ParseFiles returned error: %v", err)
	}

	stats := analysis.Stats
	if stats.ChangedFiles != 6 || stats.Additions != 27 || stats.Deletions != 7 {
		t.Fatalf("unexpected totals: %+v", stats)
	}
	if stats.SourceFiles != 1 || stats.TestFiles != 1 || stats.ConfigFiles != 1 || stats.DependencyFiles != 1 || stats.LockFiles != 1 || stats.CIFiles != 1 || stats.DocsFiles != 1 {
		t.Fatalf("unexpected kind counts: %+v", stats)
	}
	if !stats.HasSourceChanges || !stats.HasTestChanges {
		t.Fatalf("expected source and test changes: %+v", stats)
	}
	if stats.BinaryOrOmittedPatchFiles != 1 {
		t.Fatalf("BinaryOrOmittedPatchFiles = %d, want 1", stats.BinaryOrOmittedPatchFiles)
	}
}

func TestParseFilesSetsSourceWithoutTestsStats(t *testing.T) {
	analysis, err := NewParser().ParseFiles([]FileInput{
		{Filename: "internal/app/main.go", Status: "modified", Additions: 10, Deletions: 2, Changes: 12, Patch: "@@ -1 +1 @@\n-old\n+new"},
		{Filename: "README.md", Status: "modified", Additions: 2, Deletions: 0, Changes: 2, Patch: "@@ -1 +1 @@\n-old\n+new"},
	})
	if err != nil {
		t.Fatalf("ParseFiles returned error: %v", err)
	}
	if !analysis.Stats.HasSourceChanges {
		t.Fatalf("HasSourceChanges = false, want true")
	}
	if analysis.Stats.HasTestChanges {
		t.Fatalf("HasTestChanges = true, want false")
	}
}

func TestDiffPackageDoesNotImportGitHub(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if filepath.Base(file) == "parser_test.go" {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			if imp.Path.Value == `"diff-lens/internal/github"` {
				t.Fatalf("%s imports internal/github", file)
			}
		}
	}
}

func assertLine(t *testing.T, got DiffLine, wantType DiffLineType, wantOld, wantNew int, wantContent string) {
	t.Helper()
	if got.Type != wantType || got.OldLine != wantOld || got.NewLine != wantNew || got.Content != wantContent {
		t.Fatalf("line = %+v, want type=%q old=%d new=%d content=%q", got, wantType, wantOld, wantNew, wantContent)
	}
}

func assertPatchStatus(t *testing.T, analysis Analysis, filename string, want PatchStatus) {
	t.Helper()
	for _, file := range analysis.Files {
		if file.Filename == filename {
			if file.PatchStatus != want {
				t.Fatalf("%s PatchStatus = %q, want %q", filename, file.PatchStatus, want)
			}
			return
		}
	}
	t.Fatalf("file %q not found", filename)
}

func testHasKind(kinds []FileKind, want FileKind) bool {
	for _, kind := range kinds {
		if kind == want {
			return true
		}
	}
	return false
}
