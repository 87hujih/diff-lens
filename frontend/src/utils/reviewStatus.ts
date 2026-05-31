import type { RenderRiskLevel, RenderSeverity, RenderStepStatus } from "../types/review";

const RISK_LEVELS = new Set<RenderRiskLevel>(["low", "medium", "high", "unknown"]);
const SEVERITIES = new Set<RenderSeverity>(["low", "medium", "high", "unknown"]);
const STEP_STATUSES = new Set<RenderStepStatus>([
  "running",
  "completed",
  "failed",
  "unknown"
]);

function normalizeToken(value: unknown): string {
  return typeof value === "string" ? value.trim().toLowerCase() : "";
}

export function normalizeRiskLevel(value: unknown): RenderRiskLevel {
  const token = normalizeToken(value) as RenderRiskLevel;
  return RISK_LEVELS.has(token) ? token : "unknown";
}

export function normalizeSeverity(value: unknown): RenderSeverity {
  const token = normalizeToken(value) as RenderSeverity;
  return SEVERITIES.has(token) ? token : "unknown";
}

export function normalizeStepStatus(value: unknown): RenderStepStatus {
  const token = normalizeToken(value) as RenderStepStatus;
  return STEP_STATUSES.has(token) ? token : "unknown";
}
