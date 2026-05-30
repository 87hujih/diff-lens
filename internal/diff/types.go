package diff

// FileKind describes one language-neutral role a changed file can play.
// Classification is multi-label, so a file can have multiple kinds.
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

// PatchStatus describes whether patch text is available for analysis.
type PatchStatus string

const (
	PatchStatusPresent         PatchStatus = "present"
	PatchStatusEmpty           PatchStatus = "empty"
	PatchStatusMissing         PatchStatus = "missing"
	PatchStatusBinaryOrOmitted PatchStatus = "binary_or_omitted"
)

// FileInput is the service-neutral shape accepted by the diff parser.
type FileInput struct {
	Filename             string
	Status               string
	Additions            int
	Deletions            int
	Changes              int
	Patch                string
	PatchBinaryOrOmitted bool
}

// Analysis is the language-neutral output passed to downstream scanners.
type Analysis struct {
	Files    []FileDiff
	Stats    FileStats
	Warnings []Warning
}

// FileDiff is the normalized representation of one changed file.
type FileDiff struct {
	Filename    string
	Status      string
	Kinds       []FileKind
	Additions   int
	Deletions   int
	Changes     int
	Patch       string
	Hunks       []Hunk
	HasPatch    bool
	PatchStatus PatchStatus
}

// Hunk is reserved for parsed patch hunk data.
type Hunk struct {
	Header string
}

// Warning describes non-fatal issues encountered during diff analysis.
type Warning struct {
	Filename string
	Message  string
}
