import type { Risk } from "../types/review";
import type { RiskSeverityFilter } from "../utils/riskFilters";
import { formatCategory, formatSeverity, formatSource } from "../utils/displayText";
import { getVisibleRisks } from "../utils/riskFilters";
import { normalizeSeverity } from "../utils/reviewStatus";

// FILTERS 定义风险列表可切换的严重级别筛选项。
type VisibleRiskFilter = "all" | "high" | "medium" | "low";

const FILTERS: Array<{ value: VisibleRiskFilter; label: string }> = [
  { value: "all", label: "全部" },
  { value: "high", label: "高" },
  { value: "medium", label: "中" },
  { value: "low", label: "低" }
];

// RiskRadarProps 控制风险列表筛选、选中态和点击行为。
interface RiskRadarProps {
  risks: Risk[];
  activeRiskId: string | null;
  filter: RiskSeverityFilter;
  onFilterChange: (filter: RiskSeverityFilter) => void;
  onSelectRisk: (risk: Risk) => void;
}

// formatConfidence 将 0-1 置信度转换成百分比标签。
function formatConfidence(value: number): string {
  if (!Number.isFinite(value)) {
    return "无";
  }

  return `${Math.round(value * 100)}%`;
}

// formatLocation 将风险文件和行号压缩成列表中的短标签。
function formatLocation(risk: Risk): string {
  if (!risk.file) {
    return "无具体行号";
  }

  return risk.line ? `${risk.file}:${risk.line}` : `${risk.file} · 无具体行号`;
}

// RiskRadar 按严重级别展示可筛选、可选中的风险列表。
export function RiskRadar({
  risks,
  activeRiskId,
  filter,
  onFilterChange,
  onSelectRisk
}: RiskRadarProps) {
  const visibleRisks = getVisibleRisks(risks, filter);
  const filterCounts: Record<VisibleRiskFilter, number> = {
    all: risks.length,
    high: getVisibleRisks(risks, "high").length,
    medium: getVisibleRisks(risks, "medium").length,
    low: getVisibleRisks(risks, "low").length
  };

  return (
    <section className="risk-radar" aria-labelledby="risk-radar-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">风险雷达</p>
          <h3 id="risk-radar-title">风险发现</h3>
        </div>
        <span className="count-pill">{visibleRisks.length}</span>
      </div>

      <div className="filter-tabs" role="group" aria-label="按严重级别筛选风险">
        {FILTERS.map((item) => (
          <button
            key={item.value}
            type="button"
            className={filter === item.value ? "filter-tab filter-tab--active" : "filter-tab"}
            aria-pressed={filter === item.value}
            onClick={() => onFilterChange(item.value)}
          >
            <span>{item.label}</span>
            <span className="filter-tab__count">{filterCounts[item.value]}</span>
          </button>
        ))}
      </div>

      {visibleRisks.length === 0 ? (
        <div className="risk-empty">
          <strong>暂无可展示风险</strong>
          <p>{risks.length === 0 ? "尚未报告风险。" : "没有风险匹配当前筛选条件。"}</p>
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
                data-risk-id={risk.id}
                aria-pressed={isActive}
                onClick={() => onSelectRisk(risk)}
              >
                <span className="risk-card__topline">
                  <span className={`severity-badge severity-badge--${severity}`}>
                    {formatSeverity(risk.severity)}
                  </span>
                  <span className="risk-card__source">{formatSource(risk.source)}</span>
                  <span className="risk-card__confidence">{formatConfidence(risk.confidence)}</span>
                </span>

                <span className="risk-card__title">{risk.title}</span>
                <span className="risk-card__meta">
                  {formatCategory(risk.category)} · {formatLocation(risk)}
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
