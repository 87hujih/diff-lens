package review_test

import (
	"context"
	"testing"

	"diff-lens/internal/demo"
	"diff-lens/internal/review"
)

func TestAnalyzeDemoEmitsStableEventOrder(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})

	events, err := service.Analyze(context.Background(), review.AnalyzeRequest{Demo: true})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	var got []review.EventType
	for event := range events {
		got = append(got, event.Type)
	}

	want := []review.EventType{
		review.EventStep,
		review.EventPR,
		review.EventRules,
		review.EventResult,
		review.EventDone,
	}

	if len(got) != len(want) {
		t.Fatalf("event count = %d, want %d; events=%v", len(got), len(want), got)
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("event %d = %q, want %q; events=%v", i, got[i], want[i], got)
		}
	}
}
