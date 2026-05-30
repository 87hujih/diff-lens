package github_test

import (
	"testing"

	"diff-lens/internal/github"
)

func TestParsePRURLAcceptsGitHubPullRequestURL(t *testing.T) {
	ref, err := github.ParsePRURL("https://github.com/openai/example/pull/123?tab=files#discussion")
	if err != nil {
		t.Fatalf("ParsePRURL returned error: %v", err)
	}

	if ref.Owner != "openai" {
		t.Fatalf("Owner = %q, want %q", ref.Owner, "openai")
	}
	if ref.Repo != "example" {
		t.Fatalf("Repo = %q, want %q", ref.Repo, "example")
	}
	if ref.Number != 123 {
		t.Fatalf("Number = %d, want %d", ref.Number, 123)
	}
}

func TestParsePRURLRejectsNonPullRequestURL(t *testing.T) {
	_, err := github.ParsePRURL("https://github.com/openai/example/issues/123")
	if err == nil {
		t.Fatal("ParsePRURL returned nil error for issue URL")
	}
}
