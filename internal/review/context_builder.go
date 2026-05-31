package review

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"diff-lens/internal/github"
)

const (
	defaultContextMaxChars        = 24000
	defaultMetadataMaxChars       = 3000
	defaultRuleMaxChars           = 6000
	defaultFileSummaryMaxChars    = 6000
	defaultMaxFiles               = 20
	defaultMaxSnippetsPerFile     = 3
	defaultMaxSnippetChars        = 1200
	defaultPromptWrapperMaxChars  = 1500
	contextSnippetEvidencePrefix  = "snippet-"
	contextRedactionMarker        = "[REDACTED]"
	maxCommitMessageCharsFallback = 180
)

// ContextBuilderOptions controls review context budget partitions.
type ContextBuilderOptions struct {
	MaxChars              int
	MaxMetadataChars      int
	MaxRuleChars          int
	MaxFileSummaryChars   int
	MaxFiles              int
	MaxSnippetsPerFile    int
	MaxSnippetChars       int
	PromptWrapperMaxChars int
}

// ContextBuilder turns PR data and rule risks into a bounded AI context.
type ContextBuilder struct {
	options ContextBuilderOptions
}

// NewContextBuilder creates a builder with conservative default budgets.
func NewContextBuilder(options ContextBuilderOptions) *ContextBuilder {
	return &ContextBuilder{options: normalizeContextBuilderOptions(options)}
}

// Build creates a deterministic, redacted ReviewContext.
func (b *ContextBuilder) Build(pr github.PullRequestData, ruleRisks []Risk) ReviewContext {
	options := b.options
	ctx := ReviewContext{
		SchemaVersion: ReviewContextSchemaVersion,
		PR:            prInfoFromPullRequest(pr),
		Stats: ContextStats{
			ChangedFiles:          pr.ChangedFiles,
			Additions:             pr.Additions,
			Deletions:             pr.Deletions,
			PromptWrapperReserved: options.PromptWrapperMaxChars,
		},
		RuleRisks: cloneRisks(ruleRisks),
		Files:     []ContextFile{},
	}
	if ctx.Stats.ChangedFiles == 0 {
		ctx.Stats.ChangedFiles = len(pr.Files)
	}

	ctx.Commits = boundedCommits(pr.Commits, options.MaxMetadataChars, &ctx.Stats)
	ctx.RuleRisks = boundedRuleRisks(ctx.RuleRisks, options.MaxRuleChars, &ctx.Stats)

	riskIDsByFile := map[string][]string{}
	severitiesByFile := map[string]string{}
	for _, risk := range ctx.RuleRisks {
		if risk.ID != "" {
			ctx.EvidenceRefs = appendUnique(ctx.EvidenceRefs, risk.ID)
		}
		if risk.File == "" {
			continue
		}
		riskIDsByFile[risk.File] = appendUnique(riskIDsByFile[risk.File], risk.ID)
		severitiesByFile[risk.File] = maxSeverity(severitiesByFile[risk.File], risk.Severity)
	}

	candidates := make([]contextFileCandidate, 0, len(pr.Files))
	for index, file := range pr.Files {
		kind := classifyContextFile(file.Filename)
		recordKind(kind, &ctx.Stats)
		candidate := contextFileCandidate{
			file:     file,
			kind:     kind,
			index:    index,
			priority: filePriority(file, kind, severitiesByFile[file.Filename]),
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].priority == candidates[j].priority {
			return candidates[i].index < candidates[j].index
		}
		return candidates[i].priority > candidates[j].priority
	})

	if len(candidates) > options.MaxFiles {
		ctx.Stats.Truncated = true
		ctx.Stats.FileSummaryTruncated = true
		ctx.Stats.OmittedFilesCount += len(candidates) - options.MaxFiles
		candidates = candidates[:options.MaxFiles]
	}

	for _, candidate := range candidates {
		contextFile := ContextFile{
			Filename:  candidate.file.Filename,
			Kind:      candidate.kind,
			Status:    candidate.file.Status,
			Additions: candidate.file.Additions,
			Deletions: candidate.file.Deletions,
			RiskIDs:   cloneStrings(riskIDsByFile[candidate.file.Filename]),
			Snippets:  []ContextSnippet{},
		}
		if candidate.file.Patch == "" {
			contextFile.PatchOmitted = true
			ctx.Files = append(ctx.Files, contextFile)
			continue
		}

		snippets, omitted := buildSnippets(candidate.file, options)
		contextFile.Snippets = snippets
		for _, snippet := range snippets {
			ctx.EvidenceRefs = appendUnique(ctx.EvidenceRefs, snippet.ID)
		}
		if omitted > 0 {
			ctx.Stats.Truncated = true
			ctx.Stats.SnippetsTruncated = true
			ctx.Stats.OmittedSnippetsCount += omitted
		}
		ctx.Files = append(ctx.Files, contextFile)
	}

	ctx.ContextID = stableContextID(ctx)
	enforceTotalBudget(&ctx, options.MaxChars)
	return ctx
}

type contextFileCandidate struct {
	file     github.PullRequestFile
	kind     string
	index    int
	priority int
}

func normalizeContextBuilderOptions(options ContextBuilderOptions) ContextBuilderOptions {
	if options.MaxChars <= 0 {
		options.MaxChars = defaultContextMaxChars
	}
	if options.MaxMetadataChars <= 0 {
		options.MaxMetadataChars = defaultMetadataMaxChars
	}
	if options.MaxRuleChars <= 0 {
		options.MaxRuleChars = defaultRuleMaxChars
	}
	if options.MaxFileSummaryChars <= 0 {
		options.MaxFileSummaryChars = defaultFileSummaryMaxChars
	}
	if options.MaxFiles <= 0 {
		options.MaxFiles = defaultMaxFiles
	}
	if options.MaxSnippetsPerFile <= 0 {
		options.MaxSnippetsPerFile = defaultMaxSnippetsPerFile
	}
	if options.MaxSnippetChars <= 0 {
		options.MaxSnippetChars = defaultMaxSnippetChars
	}
	if options.PromptWrapperMaxChars <= 0 {
		options.PromptWrapperMaxChars = defaultPromptWrapperMaxChars
	}
	return options
}

func boundedCommits(commits []github.PullRequestCommit, maxChars int, stats *ContextStats) []ContextCommit {
	var out []ContextCommit
	used := 0
	for _, commit := range commits {
		message := commit.Message
		if maxChars > 0 && len(message) > maxCommitMessageCharsFallback {
			message = truncateString(message, maxCommitMessageCharsFallback)
			stats.MetadataTruncated = true
			stats.Truncated = true
		}
		item := ContextCommit{
			SHA:     shortSHA(commit.SHA),
			Message: message,
			Author:  commit.AuthorLogin,
		}
		size := roughJSONSize(item)
		if used > 0 && used+size > maxChars {
			stats.MetadataTruncated = true
			stats.Truncated = true
			break
		}
		used += size
		out = append(out, item)
	}
	return out
}

func boundedRuleRisks(risks []Risk, maxChars int, stats *ContextStats) []Risk {
	out := make([]Risk, 0, len(risks))
	used := 0
	for _, risk := range risks {
		copied := risk
		if copied.Evidence != "" && len(copied.Evidence) > 400 {
			copied.Evidence = truncateString(copied.Evidence, 400)
			stats.RulesTruncated = true
			stats.Truncated = true
		}
		size := roughJSONSize(copied)
		if used > 0 && used+size > maxChars {
			stats.RulesTruncated = true
			stats.Truncated = true
			break
		}
		used += size
		out = append(out, copied)
	}
	return out
}

func classifyContextFile(filename string) string {
	lower := strings.ToLower(strings.ReplaceAll(filename, "\\", "/"))
	base := lower
	if slash := strings.LastIndex(base, "/"); slash >= 0 {
		base = base[slash+1:]
	}
	switch {
	case strings.HasPrefix(lower, ".github/workflows/") || strings.Contains(lower, "/.github/workflows/") || strings.Contains(lower, "jenkinsfile") || strings.Contains(lower, "gitlab-ci"):
		return "ci"
	case base == "go.mod" || base == "go.sum" || base == "package.json" || base == "package-lock.json" || base == "pnpm-lock.yaml" || base == "yarn.lock" || base == "requirements.txt":
		return "dependency"
	case strings.Contains(lower, "config") || strings.HasSuffix(base, ".yaml") || strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".toml") || strings.HasSuffix(base, ".json") || strings.HasPrefix(base, ".env"):
		return "config"
	case strings.HasSuffix(base, "_test.go") || strings.Contains(lower, "/test/") || strings.Contains(lower, "/tests/") || strings.Contains(lower, ".test."):
		return "test"
	default:
		return "source"
	}
}

func recordKind(kind string, stats *ContextStats) {
	if stats.FilesByKind != nil {
		stats.FilesByKind[kind]++
	}
	switch kind {
	case "test":
		stats.TestFiles++
	case "config":
		stats.ConfigFiles++
	case "dependency":
		stats.DependencyFiles++
	case "ci":
		stats.CIFiles++
	default:
		stats.SourceFiles++
	}
}

func filePriority(file github.PullRequestFile, kind string, riskSeverity string) int {
	priority := 0
	switch riskSeverity {
	case "high":
		priority += 1000
	case "medium":
		priority += 800
	case "low":
		priority += 500
	}
	switch kind {
	case "config":
		priority += 450
	case "dependency", "ci":
		priority += 400
	case "test":
		priority += 120
	}
	if file.Patch == "" {
		priority += 380
	}
	lowerPatch := strings.ToLower(file.Patch)
	if strings.Contains(lowerPatch, "ignore previous instructions") || strings.Contains(lowerPatch, "system prompt") {
		priority += 430
	}
	lowerName := strings.ToLower(file.Filename)
	for _, keyword := range []string{"auth", "permission", "database", "migration", "delete", "secret"} {
		if strings.Contains(lowerName, keyword) {
			priority += 160
			break
		}
	}
	return priority
}

func buildSnippets(file github.PullRequestFile, options ContextBuilderOptions) ([]ContextSnippet, int) {
	hunks := splitPatchHunks(file.Patch)
	if len(hunks) == 0 {
		hunks = []string{file.Patch}
	}
	limit := options.MaxSnippetsPerFile
	omitted := 0
	if len(hunks) > limit {
		omitted = len(hunks) - limit
		hunks = hunks[:limit]
	}

	snippets := make([]ContextSnippet, 0, len(hunks))
	for index, hunk := range hunks {
		start, end := hunkLineRange(hunk)
		redacted := redactSecrets(hunk)
		if len(redacted) > options.MaxSnippetChars {
			redacted = truncateString(redacted, options.MaxSnippetChars)
			omitted++
		}
		id := stableSnippetID(file.Filename, index, hunk)
		snippets = append(snippets, ContextSnippet{
			ID:        id,
			File:      file.Filename,
			StartLine: start,
			EndLine:   end,
			Patch:     redacted,
			Reason:    "changed_hunk",
		})
	}
	return snippets, omitted
}

func splitPatchHunks(patch string) []string {
	lines := strings.Split(patch, "\n")
	var hunks []string
	var current []string
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") && len(current) > 0 {
			hunks = append(hunks, strings.Join(current, "\n"))
			current = nil
		}
		if line != "" || len(current) > 0 {
			current = append(current, line)
		}
	}
	if len(current) > 0 {
		hunks = append(hunks, strings.Join(current, "\n"))
	}
	return hunks
}

var hunkHeaderPattern = regexp.MustCompile(`@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@`)

func hunkLineRange(hunk string) (int, int) {
	matches := hunkHeaderPattern.FindStringSubmatch(hunk)
	if len(matches) == 0 {
		return 0, 0
	}
	start, _ := strconv.Atoi(matches[1])
	count := 1
	if len(matches) > 2 && matches[2] != "" {
		count, _ = strconv.Atoi(matches[2])
	}
	if count <= 0 {
		count = 1
	}
	return start, start + count - 1
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|api[_-]?key|secret|token)\s*[:=]\s*["']?[^"'\s]+["']?`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{10,}`),
	regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
}

func redactSecrets(input string) string {
	redacted := input
	for _, pattern := range secretPatterns {
		redacted = pattern.ReplaceAllStringFunc(redacted, func(match string) string {
			if strings.Contains(match, "PRIVATE KEY") {
				return contextRedactionMarker
			}
			if idx := strings.IndexAny(match, "=:"); idx >= 0 {
				return strings.TrimSpace(match[:idx]) + "=" + contextRedactionMarker
			}
			return contextRedactionMarker
		})
	}
	return redacted
}

func stableSnippetID(filename string, index int, hunk string) string {
	hash := sha1.Sum([]byte(fmt.Sprintf("%s\n%d\n%s", filename, index, hunk)))
	return contextSnippetEvidencePrefix + hex.EncodeToString(hash[:])[:12]
}

func stableContextID(ctx ReviewContext) string {
	hash := sha1.New()
	_, _ = hash.Write([]byte(ctx.SchemaVersion))
	_, _ = hash.Write([]byte(ctx.PR.Repo))
	_, _ = hash.Write([]byte(strconv.Itoa(ctx.PR.Number)))
	for _, file := range ctx.Files {
		_, _ = hash.Write([]byte(file.Filename))
		for _, snippet := range file.Snippets {
			_, _ = hash.Write([]byte(snippet.ID))
		}
	}
	return "ctx-" + hex.EncodeToString(hash.Sum(nil))[:12]
}

func enforceTotalBudget(ctx *ReviewContext, maxChars int) {
	if maxChars <= 0 {
		return
	}
	for roughJSONSize(ctx) > maxChars {
		if dropLastSnippet(ctx, false) {
			ctx.Stats.Truncated = true
			ctx.Stats.SnippetsTruncated = true
			ctx.Stats.OmittedSnippetsCount++
			rebuildEvidenceRefs(ctx)
			continue
		}
		if dropLastFile(ctx, false) {
			ctx.Stats.Truncated = true
			ctx.Stats.FileSummaryTruncated = true
			ctx.Stats.OmittedFilesCount++
			rebuildEvidenceRefs(ctx)
			continue
		}
		if len(ctx.Commits) > 0 {
			ctx.Commits = ctx.Commits[:len(ctx.Commits)-1]
			ctx.Stats.Truncated = true
			ctx.Stats.MetadataTruncated = true
			continue
		}
		if compactRiskText(ctx) {
			ctx.Stats.Truncated = true
			ctx.Stats.RulesTruncated = true
			continue
		}
		if dropLastSnippet(ctx, true) {
			ctx.Stats.Truncated = true
			ctx.Stats.SnippetsTruncated = true
			ctx.Stats.OmittedSnippetsCount++
			rebuildEvidenceRefs(ctx)
			continue
		}
		if dropLastFile(ctx, true) {
			ctx.Stats.Truncated = true
			ctx.Stats.FileSummaryTruncated = true
			ctx.Stats.OmittedFilesCount++
			rebuildEvidenceRefs(ctx)
			continue
		}
		break
	}
}

func dropLastSnippet(ctx *ReviewContext, allowCritical bool) bool {
	for fileIndex := len(ctx.Files) - 1; fileIndex >= 0; fileIndex-- {
		snippets := ctx.Files[fileIndex].Snippets
		for snippetIndex := len(snippets) - 1; snippetIndex >= 0; snippetIndex-- {
			if !allowCritical && preservesCriticalSnippet(snippets[snippetIndex]) {
				continue
			}
			ctx.Files[fileIndex].Snippets = append(snippets[:snippetIndex], snippets[snippetIndex+1:]...)
			return true
		}
	}
	for fileIndex := len(ctx.Files) - 1; fileIndex >= 0; fileIndex-- {
		snippets := ctx.Files[fileIndex].Snippets
		if len(snippets) == 0 {
			continue
		}
		ctx.Files[fileIndex].Snippets = snippets[:len(snippets)-1]
		return true
	}
	return false
}

func preservesCriticalSnippet(snippet ContextSnippet) bool {
	patch := strings.ToLower(snippet.Patch)
	return strings.Contains(snippet.Patch, contextRedactionMarker) || strings.Contains(patch, "ignore previous instructions")
}

func dropLastFile(ctx *ReviewContext, allowCritical bool) bool {
	if len(ctx.Files) == 0 {
		return false
	}
	for fileIndex := len(ctx.Files) - 1; fileIndex >= 0; fileIndex-- {
		if !allowCritical && fileHasCriticalSnippet(ctx.Files[fileIndex]) {
			continue
		}
		ctx.Files = append(ctx.Files[:fileIndex], ctx.Files[fileIndex+1:]...)
		return true
	}
	return false
}

func fileHasCriticalSnippet(file ContextFile) bool {
	for _, snippet := range file.Snippets {
		if preservesCriticalSnippet(snippet) {
			return true
		}
	}
	return false
}

func compactRiskText(ctx *ReviewContext) bool {
	for i := range ctx.RuleRisks {
		if len(ctx.RuleRisks[i].Reason) > 80 {
			ctx.RuleRisks[i].Reason = truncateString(ctx.RuleRisks[i].Reason, 80)
			return true
		}
		if len(ctx.RuleRisks[i].Suggestion) > 80 {
			ctx.RuleRisks[i].Suggestion = truncateString(ctx.RuleRisks[i].Suggestion, 80)
			return true
		}
	}
	return false
}

func rebuildEvidenceRefs(ctx *ReviewContext) {
	refs := []string{}
	for _, risk := range ctx.RuleRisks {
		if risk.ID != "" {
			refs = appendUnique(refs, risk.ID)
		}
	}
	for _, file := range ctx.Files {
		for _, snippet := range file.Snippets {
			refs = appendUnique(refs, snippet.ID)
		}
	}
	ctx.EvidenceRefs = refs
}

func shortSHA(sha string) string {
	if len(sha) <= 12 {
		return sha
	}
	return sha[:12]
}

func truncateString(value string, maxChars int) string {
	if maxChars <= 0 || len(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return value[:maxChars]
	}
	return value[:maxChars-3] + "..."
}

func roughJSONSize(value any) int {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return len(encoded)
}

func cloneRisks(risks []Risk) []Risk {
	out := make([]Risk, len(risks))
	for i, risk := range risks {
		out[i] = risk
		out[i].EvidenceRefs = cloneStrings(risk.EvidenceRefs)
	}
	return out
}

func cloneStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func appendUnique(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func maxSeverity(left string, right string) string {
	if severityRank(right) > severityRank(left) {
		return right
	}
	return left
}

func severityRank(severity string) int {
	switch strings.ToLower(severity) {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}
