import test from "node:test";
import assert from "node:assert/strict";
import { initialReviewState, reviewReducer } from "../../src/state/reviewReducer";
import type { Report } from "../../src/types/review";

const report: Report = {
  pr: {
    title: "Tighten review stream",
    author: "mona",
    repo: "acme/diff-lens",
    number: 42,
    source_branch: "feature",
    target_branch: "main",
    changed_files: 2,
    additions: 10,
    deletions: 4,
    commits: 1
  },
  summary: {
    risk_level: "medium",
    overview: "Parser and reducer changes.",
    key_changes: ["SSE parsing"],
    review_focus: ["stream handling"]
  },
  risks: [],
  evidence: [],
  comments: [],
  meta: {
    ai_completed: false,
    rules_completed: true,
    context_truncated: false,
    omitted_files_count: 0,
    omitted_snippets_count: 0
  },
  degraded: true
};

test("reviewReducer stores result as final report and marks degraded when done completes", () => {
  const withResult = reviewReducer(initialReviewState, {
    type: "result",
    data: report
  });

  assert.equal(withResult.result, report);
  assert.equal(withResult.degraded, true);

  const completed = reviewReducer(withResult, {
    type: "done",
    data: { ok: true }
  });

  assert.equal(completed.status, "completed");
  assert.equal(completed.result, report);
  assert.equal(completed.degraded, true);
});

test("reviewReducer marks failed and degraded when done reports a degraded failure", () => {
  const failed = reviewReducer(initialReviewState, {
    type: "done",
    data: { ok: false, degraded: true }
  });

  assert.equal(failed.status, "failed");
  assert.equal(failed.degraded, true);
});
