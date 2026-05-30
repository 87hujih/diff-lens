package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"diff-lens/internal/demo"
	"diff-lens/internal/handler"
	"diff-lens/internal/review"
)

func TestAnalyzeStreamDemoReturnsSSEEvents(t *testing.T) {
	service := review.NewService(review.ServiceOptions{
		DemoProvider: demo.NewProvider(),
	})
	router := handler.NewRouter(service)

	req := httptest.NewRequest(http.MethodPost, "/api/reviews/analyze/stream", strings.NewReader(`{"demo":true}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	body := recorder.Body.String()
	for _, expected := range []string{"event: step", "event: pr", "event: rules", "event: result", "event: done"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("body missing %q:\n%s", expected, body)
		}
	}
}
