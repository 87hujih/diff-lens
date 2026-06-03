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
      "评审：acme/diff-lens #7",
      "标题：Add frontend unit tests",
      "",
      "摘要",
      "Adds deterministic frontend coverage.",
      "",
      "风险级别：高",
      "",
      "关键变更",
      "- Adds node:test runner",
      "- Covers formatting",
      "",
      "评审重点",
      "- No new dependencies",
      "",
      "重要风险",
      "- [高] Stream errors can hide",
      "  位置：frontend/src/api/reviewStream.ts:31",
      "  原因：The error path is not visible.",
      "  建议修复：Surface the failure.",
      "",
      "- [低] Minor copy issue",
      "  位置：frontend/src/App.tsx:12",
      "  原因：Text is ambiguous.",
      "  建议修复：Use a precise label.",
      "",
      "建议评论",
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
