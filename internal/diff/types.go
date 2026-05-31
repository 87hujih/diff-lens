package diff

// FileKind describes broad, non-exclusive categories for a changed file.
type FileKind string

const (
	FileKindSource     FileKind = "source"
	FileKindTest       FileKind = "test"
	FileKindConfig     FileKind = "config"
	FileKindDependency FileKind = "dependency"
	FileKindLockfile   FileKind = "lockfile"
	FileKindCI         FileKind = "ci"
	FileKindDocs       FileKind = "docs"
)

// PatchStatus records whether patch text was available for a file.
type PatchStatus string

const (
	PatchStatusPresent         PatchStatus = "present"
	PatchStatusEmpty           PatchStatus = "empty"
	PatchStatusMissing         PatchStatus = "missing"
	PatchStatusBinaryOrOmitted PatchStatus = "binary_or_omitted"
)

// DiffLineType identifies how a line participates in a hunk.
type DiffLineType string

const (
	DiffLineContext DiffLineType = "context"
	DiffLineAdded   DiffLineType = "added"
	DiffLineRemoved DiffLineType = "removed"
)

// FileInput is the parser's provider-neutral input for one changed file.
type FileInput struct {
	Filename        string
	Status          string
	Additions       int
	Deletions       int
	Changes         int
	Patch           string
	PatchMissing    bool
	BinaryOrOmitted bool
}

// Analysis is the parser output consumed by later review stages.
type Analysis struct {
	Files    []FileDiff
	Stats    FileStats
	Warnings []Warning
}

// FileDiff contains parsed patch data and metadata for one file.
type FileDiff struct {
	Filename    string
	Status      string
	Kinds       []FileKind
	Additions   int
	Deletions   int
	Changes     int
	Patch       string
	Hunks       []DiffHunk
	HasPatch    bool
	PatchStatus PatchStatus
}

// DiffHunk is one unified diff hunk.
type DiffHunk struct {
	Header   string
	Context  string
	OldStart int
	OldCount int
	NewStart int
	NewCount int
	Lines    []DiffLine
}

// DiffLine is a parsed line inside a hunk. Only one of OldLine/NewLine is set
// for removed or added lines; context lines have both.
type DiffLine struct {
	Type    DiffLineType
	OldLine int
	NewLine int
	Content string
}

// Warning records recoverable parse issues.
type Warning struct {
	File    string
	Message string
	Line    int
}
