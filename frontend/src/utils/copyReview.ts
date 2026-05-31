import type { Report, Risk, SuggestedComment } from "../types/review";

function formatLocation(file?: string, line?: number): string {
  if (!file) {
    return "";
  }

  return line ? `${file}:${line}` : file;
}

function formatList(title: string, values: string[], emptyText: string): string {
  if (values.length === 0) {
    return `${title}\n- ${emptyText}`;
  }

  return `${title}\n${values.map((value) => `- ${value}`).join("\n")}`;
}

function riskWeight(risk: Risk): number {
  switch (risk.severity.toLowerCase()) {
    case "high":
      return 0;
    case "medium":
      return 1;
    case "low":
      return 2;
    default:
      return 3;
  }
}

function formatRisk(risk: Risk): string {
  const location = formatLocation(risk.file, risk.line);
  const parts = [`- [${risk.severity || "unknown"}] ${risk.title}`];

  if (location) {
    parts.push(`  Location: ${location}`);
  }

  if (risk.reason) {
    parts.push(`  Reason: ${risk.reason}`);
  }

  if (risk.suggestion) {
    parts.push(`  Suggested fix: ${risk.suggestion}`);
  }

  return parts.join("\n");
}

export function formatSingleComment(comment: SuggestedComment): string {
  const location = formatLocation(comment.file, comment.line);
  const body = comment.body.trim() || "No suggested comment body was provided.";

  return location ? `${location}\n\n${body}` : body;
}

export function formatFullReview(report: Report): string {
  const importantRisks = [...report.risks].sort((left, right) => riskWeight(left) - riskWeight(right));
  const formattedRisks =
    importantRisks.length > 0
      ? importantRisks.map(formatRisk).join("\n\n")
      : "- No risks were reported.";
  const formattedComments =
    report.comments.length > 0
      ? report.comments
          .map((comment, index) => `${index + 1}. ${formatSingleComment(comment)}`)
          .join("\n\n")
      : "No suggested comments were generated.";

  return [
    `Review: ${report.pr.repo} #${report.pr.number}`,
    `Title: ${report.pr.title}`,
    "",
    "Summary",
    report.summary.overview || "No overview was provided.",
    "",
    `Risk level: ${report.summary.risk_level || "unknown"}`,
    "",
    formatList("Key changes", report.summary.key_changes, "No key changes were reported."),
    "",
    formatList("Review focus", report.summary.review_focus, "No review focus areas were reported."),
    "",
    "Important risks",
    formattedRisks,
    "",
    "Suggested comments",
    formattedComments
  ].join("\n");
}
