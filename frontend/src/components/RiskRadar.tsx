import type { Risk } from "../types/review";
import type { RiskSeverityFilter } from "../utils/riskFilters";
import { getVisibleRisks } from "../utils/riskFilters";
import { normalizeSeverity } from "../utils/reviewStatus";

const FILTERS: Array<{ value: RiskSeverityFilter; label: string }> = [
  { value: "all", label: "All" },
  { value: "high", label: "High" },
  { value: "medium", label: "Medium" },
  { value: "low", label: "Low" }
];

interface RiskRadarProps {
  risks: Risk[];
  activeRiskId: string | null;
  filter: RiskSeverityFilter;
  onFilterChange: (filter: RiskSeverityFilter) => void;
  onSelectRisk: (risk: Risk) => void;
}

function formatConfidence(value: number): string {
  if (!Number.isFinite(value)) {
    return "n/a";
  }

  return `${Math.round(value * 100)}%`;
}

function formatLocation(risk: Risk): string {
  if (!risk.file) {
    return "No specific line";
  }

  return risk.line ? `${risk.file}:${risk.line}` : `${risk.file} · No specific line`;
}

export function RiskRadar({
  risks,
  activeRiskId,
  filter,
  onFilterChange,
  onSelectRisk
}: RiskRadarProps) {
  const visibleRisks = getVisibleRisks(risks, filter);

  return (
    <section className="risk-radar" aria-labelledby="risk-radar-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">Risk radar</p>
          <h3 id="risk-radar-title">Findings</h3>
        </div>
        <span className="count-pill">{visibleRisks.length}</span>
      </div>

      <div className="filter-tabs" role="group" aria-label="Filter risks by severity">
        {FILTERS.map((item) => (
          <button
            key={item.value}
            type="button"
            className={filter === item.value ? "filter-tab filter-tab--active" : "filter-tab"}
            aria-pressed={filter === item.value}
            onClick={() => onFilterChange(item.value)}
          >
            {item.label}
          </button>
        ))}
      </div>

      {visibleRisks.length === 0 ? (
        <div className="risk-empty">
          <strong>No risks to show</strong>
          <p>{risks.length === 0 ? "No risks have been reported." : "No risks match this filter."}</p>
        </div>
      ) : (
        <div className="risk-list">
          {visibleRisks.map((risk) => {
            const severity = normalizeSeverity(risk.severity);
            const isActive = activeRiskId === risk.id;

            return (
              <button
                type="button"
                className={isActive ? "risk-card risk-card--active" : "risk-card"}
                key={risk.id}
                aria-pressed={isActive}
                onClick={() => onSelectRisk(risk)}
              >
                <span className="risk-card__topline">
                  <span className={`severity-badge severity-badge--${severity}`}>
                    {risk.severity || "unknown"}
                  </span>
                  <span className="risk-card__source">{risk.source}</span>
                  <span className="risk-card__confidence">{formatConfidence(risk.confidence)}</span>
                </span>

                <span className="risk-card__title">{risk.title}</span>
                <span className="risk-card__meta">
                  {risk.category || "Uncategorized"} · {formatLocation(risk)}
                </span>
                <span className="risk-card__reason">{risk.reason}</span>
                <span className="risk-card__suggestion">{risk.suggestion}</span>
              </button>
            );
          })}
        </div>
      )}
    </section>
  );
}
