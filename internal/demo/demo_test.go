package demo_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"diff-lens/internal/demo"
	"diff-lens/internal/review"
)

func TestProviderStreamMatchesFullDemoContractWithRunningAndCompletedStages(t *testing.T) {
	events := collectDemoEvents(t)

	assertStepTransitions(t, events, []stepTransition{
		{step: "fetch_pr", status: "running"},
		{step: "fetch_pr", status: "completed"},
		{step: "parse_diff", status: "running"},
		{step: "parse_diff", status: "completed"},
		{step: "scan_rules", status: "running"},
		{step: "scan_rules", status: "completed"},
		{step: "build_context", status: "running"},
		{step: "build_context", status: "completed"},
		{step: "analyze_ai", status: "running"},
		{step: "analyze_ai", status: "completed"},
		{step: "result", status: "running"},
		{step: "result", status: "completed"},
	})
	assertEventTypesPresent(t, events, []review.EventType{
		review.EventPR,
		review.EventRules,
		review.EventResult,
		review.EventDone,
	})

	report := resultReport(t, events)
	if report.Degraded {
		t.Fatalf("result.degraded = true, want false or omitted false")
	}

	assertRiskSources(t, report.Risks, []string{"rule", "ai", "merged"})
	assertRisksAreReviewable(t, report.Risks)
	assertAtLeastOneRiskHasLocationAndEvidence(t, report.Risks)
	assertEvidenceIsCopyable(t, report.Evidence)
	assertCommentsAreCopyable(t, report.Comments)

	done := donePayload(t, events)
	if !done.OK {
		t.Fatalf("done.ok = false, want true")
	}
	if done.Degraded {
		t.Fatalf("done.degraded = true, want false or omitted false")
	}
}

func TestProviderStreamIgnoresExternalSecrets(t *testing.T) {
	baseline := collectDemoEvents(t)

	t.Setenv("GITHUB_TOKEN", "fake-demo-github-token-contract-should-not-be-read")
	t.Setenv("LLM_API_KEY", "fake-llm-key-demo-contract-should-not-be-read")

	withSecrets := collectDemoEvents(t)
	if !reflect.DeepEqual(withSecrets, baseline) {
		t.Fatalf("demo events changed after setting external secrets\nwith secrets: %#v\nbaseline: %#v", withSecrets, baseline)
	}
}

type stepTransition struct {
	step   string
	status string
}

func collectDemoEvents(t *testing.T) []review.ReviewEvent {
	t.Helper()

	events, err := demo.NewProvider().Stream(context.Background())
	if err != nil {
		t.Fatalf("Stream returned error: %v", err)
	}

	var got []review.ReviewEvent
	for event := range events {
		got = append(got, event)
	}
	if len(got) == 0 {
		t.Fatal("Stream emitted no events")
	}
	return got
}

func assertStepTransitions(t *testing.T, events []review.ReviewEvent, want []stepTransition) {
	t.Helper()

	var got []stepTransition
	for _, event := range events {
		if event.Type != review.EventStep {
			continue
		}
		step, ok := event.Data.(review.StepPayload)
		if !ok {
			t.Fatalf("step event data = %T, want review.StepPayload", event.Data)
		}
		got = append(got, stepTransition{step: step.Step, status: step.Status})
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("step transitions = %#v, want %#v", got, want)
	}
}

func assertEventTypesPresent(t *testing.T, events []review.ReviewEvent, want []review.EventType) {
	t.Helper()

	seen := map[review.EventType]bool{}
	for _, event := range events {
		seen[event.Type] = true
	}

	for _, eventType := range want {
		if !seen[eventType] {
			t.Fatalf("missing event type %q in %#v", eventType, events)
		}
	}
}

func resultReport(t *testing.T, events []review.ReviewEvent) review.Report {
	t.Helper()

	for _, event := range events {
		if event.Type != review.EventResult {
			continue
		}
		report, ok := event.Data.(review.Report)
		if !ok {
			t.Fatalf("result event data = %T, want review.Report", event.Data)
		}
		return report
	}
	t.Fatal("missing result event")
	return review.Report{}
}

func donePayload(t *testing.T, events []review.ReviewEvent) review.DonePayload {
	t.Helper()

	for _, event := range events {
		if event.Type != review.EventDone {
			continue
		}
		done, ok := event.Data.(review.DonePayload)
		if !ok {
			t.Fatalf("done event data = %T, want review.DonePayload", event.Data)
		}
		return done
	}
	t.Fatal("missing done event")
	return review.DonePayload{}
}

func assertRiskSources(t *testing.T, risks []review.Risk, want []string) {
	t.Helper()

	seen := map[string]bool{}
	for _, risk := range risks {
		seen[risk.Source] = true
	}

	for _, source := range want {
		if !seen[source] {
			t.Fatalf("missing risk source %q in %#v", source, risks)
		}
	}
}

func assertRisksAreReviewable(t *testing.T, risks []review.Risk) {
	t.Helper()

	if len(risks) == 0 {
		t.Fatal("result.risks is empty")
	}
	for _, risk := range risks {
		if strings.TrimSpace(risk.Title) == "" {
			t.Fatalf("risk %q has empty title: %#v", risk.ID, risk)
		}
		if strings.TrimSpace(risk.Severity) == "" {
			t.Fatalf("risk %q has empty severity: %#v", risk.ID, risk)
		}
		if risk.Confidence <= 0 {
			t.Fatalf("risk %q confidence = %v, want > 0", risk.ID, risk.Confidence)
		}
		if strings.TrimSpace(risk.Category) == "" {
			t.Fatalf("risk %q has empty category: %#v", risk.ID, risk)
		}
		if strings.TrimSpace(risk.Reason) == "" {
			t.Fatalf("risk %q has empty reason: %#v", risk.ID, risk)
		}
		if strings.TrimSpace(risk.Suggestion) == "" {
			t.Fatalf("risk %q has empty suggestion: %#v", risk.ID, risk)
		}
	}
}

func assertAtLeastOneRiskHasLocationAndEvidence(t *testing.T, risks []review.Risk) {
	t.Helper()

	for _, risk := range risks {
		if strings.TrimSpace(risk.File) != "" && risk.Line > 0 && strings.TrimSpace(risk.Evidence) != "" {
			return
		}
	}
	t.Fatalf("no risk has file, line, and evidence: %#v", risks)
}

func assertEvidenceIsCopyable(t *testing.T, evidence []review.EvidenceItem) {
	t.Helper()

	if len(evidence) == 0 {
		t.Fatal("result.evidence is empty")
	}
	for _, item := range evidence {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Snippet) == "" {
			t.Fatalf("evidence item is not copyable: %#v", item)
		}
	}
}

func assertCommentsAreCopyable(t *testing.T, comments []review.SuggestedComment) {
	t.Helper()

	if len(comments) == 0 {
		t.Fatal("result.comments is empty")
	}
	for _, comment := range comments {
		if strings.TrimSpace(comment.Body) == "" {
			t.Fatalf("comment %q has empty body: %#v", comment.ID, comment)
		}
	}
}
