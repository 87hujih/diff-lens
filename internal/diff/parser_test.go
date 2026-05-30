package diff

import "testing"

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
