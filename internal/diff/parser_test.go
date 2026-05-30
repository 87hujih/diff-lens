package diff

import "testing"

func TestClassifyFileIdentifiesTestFiles(t *testing.T) {
	kinds := ClassifyFile("src/app_test.go")

	assertHasKinds(t, kinds, FileKindTest)
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
