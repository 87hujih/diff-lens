import type { Report, Risk, SuggestedComment } from "../types/review";
import { formatSeverity } from "./displayText";

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
  const parts = [`- [${formatSeverity(risk.severity)}] ${risk.title}`];

  if (location) {
    parts.push(`  位置：${location}`);
  }

  if (risk.reason) {
    parts.push(`  原因：${risk.reason}`);
  }

  if (risk.suggestion) {
    parts.push(`  建议修复：${risk.suggestion}`);
  }

  return parts.join("\n");
}

export function formatSingleComment(comment: SuggestedComment): string {
  const location = formatLocation(comment.file, comment.line);
  const body = comment.body.trim() || "未提供建议评论正文。";

  return location ? `${location}\n\n${body}` : body;
}

export function formatFullReview(report: Report): string {
  const importantRisks = [...report.risks].sort((left, right) => riskWeight(left) - riskWeight(right));
  const formattedRisks =
    importantRisks.length > 0
      ? importantRisks.map(formatRisk).join("\n\n")
      : "- 未报告风险。";
  const formattedComments =
    report.comments.length > 0
      ? report.comments
          .map((comment, index) => `${index + 1}. ${formatSingleComment(comment)}`)
          .join("\n\n")
      : "未生成建议评论。";

  return [
    `评审：${report.pr.repo} #${report.pr.number}`,
    `标题：${report.pr.title}`,
    "",
    "摘要",
    report.summary.overview || "未提供概览。",
    "",
    `风险级别：${formatSeverity(report.summary.risk_level)}`,
    "",
    formatList("关键变更", report.summary.key_changes, "未报告关键变更。"),
    "",
    formatList("评审重点", report.summary.review_focus, "未报告评审重点。"),
    "",
    "重要风险",
    formattedRisks,
    "",
    "建议评论",
    formattedComments
  ].join("\n");
}
