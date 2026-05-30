package handler_test

import (
	"bytes"
	"testing"

	"diff-lens/internal/handler"
	"diff-lens/internal/review"
)

func TestWriteSSEEncodesEventAndJSONData(t *testing.T) {
	var buffer bytes.Buffer

	err := handler.WriteSSE(&buffer, review.ReviewEvent{
		Type: review.EventStep,
		Data: map[string]string{"step": "fetch_pr"},
	})
	if err != nil {
		t.Fatalf("WriteSSE returned error: %v", err)
	}

	want := "event: step\n" + `data: {"step":"fetch_pr"}` + "\n\n"
	if buffer.String() != want {
		t.Fatalf("SSE output = %q, want %q", buffer.String(), want)
	}
}
