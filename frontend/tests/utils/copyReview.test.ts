import test from "node:test";
import assert from "node:assert/strict";
import { formatFullReview, formatSingleComment } from "../../src/utils/copyReview";
import type { Report } from "../../src/types/review";

const report: Report = {
  pr: {
    title: "Add frontend unit tests",
    author: "mona",
    repo: "acme/diff-lens",
    number: 7,
    source_branch: "tests",
    target_branch: "main",
    changed_files: 3,
    additions: 80,
    deletions: 2,
    commits: 1
  },
  summary: {
    risk_level: "high",
    overview: "Adds deterministic frontend coverage.",
    key_changes: ["Adds node:test runner", "Covers formatting"],
    review_focus: ["No new dependencies"]
  },
  risks: [
    {
      id: "low-1",
      source: "rule",
      severity: "low",
      confidence: 0.7,
      category: "style",
      title: "Minor copy issue",
      file: "frontend/src/App.tsx",
      line: 12,
      reason: "Text is ambiguous.",
      suggestion: "Use a precise label."
    },
    {
      id: "high-1",
      source: "ai",
      severity: "high",
      confidence: 0.9,
      category: "runtime",
      title: "Stream errors can hide",
      file: "frontend/src/api/reviewStream.ts",
      line: 31,
      reason: "The error path is not visible.",
      suggestion: "Surface the failure."
    }
  ],
  evidence: [],
  comments: [
    {
      id: "comment-1",
      file: "frontend/src/api/reviewStream.ts",
      line: 31,
      body: "Please surface this stream failure to the caller."
    }
  ],
  meta: {
    ai_completed: true,
    rules_completed: true,
    context_truncated: false,
    omitted_files_count: 0,
    omitted_snippets_count: 0
  }
};

test("formatFullReview returns deterministic markdown sorted by risk severity", () => {
  assert.equal(
    formatFullReview(report),
    [
      "Review: acme/diff-lens #7",
      "Title: Add frontend unit tests",
      "",
      "Summary",
      "Adds deterministic frontend coverage.",
      "",
      "Risk level: high",
      "",
      "Key changes",
      "- Adds node:test runner",
      "- Covers formatting",
      "",
      "Review focus",
      "- No new dependencies",
      "",
      "Important risks",
      "- [high] Stream errors can hide",
      "  Location: frontend/src/api/reviewStream.ts:31",
      "  Reason: The error path is not visible.",
      "  Suggested fix: Surface the failure.",
      "",
      "- [low] Minor copy issue",
      "  Location: frontend/src/App.tsx:12",
      "  Reason: Text is ambiguous.",
      "  Suggested fix: Use a precise label.",
      "",
      "Suggested comments",
      "1. frontend/src/api/reviewStream.ts:31",
      "",
      "Please surface this stream failure to the caller."
    ].join("\n")
  );
});

test("formatSingleComment includes location and trims comment body", () => {
  assert.equal(
    formatSingleComment({
      id: "comment-2",
      file: "frontend/src/utils/copyReview.ts",
      line: 9,
      body: "  Keep this deterministic.  "
    }),
    "frontend/src/utils/copyReview.ts:9\n\nKeep this deterministic."
  );
});
