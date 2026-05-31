import type { RenderSeverity, Risk } from "../types/review";
import { normalizeSeverity } from "./reviewStatus";

export type RiskSeverityFilter = "all" | RenderSeverity;

const SEVERITY_ORDER: Record<RenderSeverity, number> = {
  high: 0,
  medium: 1,
  low: 2,
  unknown: 3
};

export function sortRisksBySeverity<T extends Risk>(risks: readonly T[]): T[] {
  return risks
    .map((risk, index) => ({ risk, index }))
    .sort((left, right) => {
      const severityDelta =
        SEVERITY_ORDER[normalizeSeverity(left.risk.severity)] -
        SEVERITY_ORDER[normalizeSeverity(right.risk.severity)];

      return severityDelta || left.index - right.index;
    })
    .map(({ risk }) => risk);
}

export function filterRisksBySeverity<T extends Risk>(
  risks: readonly T[],
  filter: RiskSeverityFilter
): T[] {
  if (filter === "all") {
    return [...risks];
  }

  return risks.filter((risk) => normalizeSeverity(risk.severity) === filter);
}

export function getVisibleRisks<T extends Risk>(
  risks: readonly T[],
  filter: RiskSeverityFilter
): T[] {
  return sortRisksBySeverity(filterRisksBySeverity(risks, filter));
}
