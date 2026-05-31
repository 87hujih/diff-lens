import type { Report } from "../types/review";
import { normalizeRiskLevel } from "../utils/reviewStatus";

interface ReviewBriefProps {
  report: Report;
  degraded: boolean;
}

function formatCount(value: number): string {
  return new Intl.NumberFormat("en-US").format(value);
}

export function ReviewBrief({ report, degraded }: ReviewBriefProps) {
  const riskLevel = normalizeRiskLevel(report.summary.risk_level);
  const isDegraded = degraded || Boolean(report.degraded);
  const hasTruncation =
    Boolean(report.meta?.context_truncated) ||
    (report.meta?.omitted_files_count ?? 0) > 0 ||
    (report.meta?.omitted_snippets_count ?? 0) > 0;

  return (
    <section className="review-brief" aria-labelledby="review-brief-title">
      <header className="review-brief__header">
        <div className="review-brief__identity">
          <p className="eyebrow">
            {report.pr.repo} #{report.pr.number}
          </p>
          <h2 id="review-brief-title">{report.pr.title}</h2>
          <div className="review-brief__meta" aria-label="Pull request metadata">
            <span>by {report.pr.author}</span>
            <span>
              <code>{report.pr.source_branch}</code> to <code>{report.pr.target_branch}</code>
            </span>
          </div>
        </div>
        <span className={`risk-level ${riskLevel}`} title={`Normalized as ${riskLevel}`}>
          {report.summary.risk_level || "Unknown risk"}
        </span>
      </header>

      <dl className="review-brief__stats" aria-label="Pull request change statistics">
        <div>
          <dt>Files</dt>
          <dd>{formatCount(report.pr.changed_files)}</dd>
        </div>
        <div>
          <dt>Additions</dt>
          <dd className="stat-positive">+{formatCount(report.pr.additions)}</dd>
        </div>
        <div>
          <dt>Deletions</dt>
          <dd className="stat-negative">-{formatCount(report.pr.deletions)}</dd>
        </div>
        <div>
          <dt>Commits</dt>
          <dd>{formatCount(report.pr.commits)}</dd>
        </div>
      </dl>

      {isDegraded ? (
        <div className="review-alert review-alert--warning" role="note">
          <strong>Degraded report</strong>
          <p>{report.meta?.degraded_reason || "Some analysis stages did not complete fully."}</p>
        </div>
      ) : null}

      {hasTruncation ? (
        <div className="review-alert" role="note">
          <strong>Context limited</strong>
          <p>
            {report.meta?.context_truncated ? "Context was truncated. " : ""}
            {(report.meta?.omitted_files_count ?? 0) > 0
              ? `${formatCount(report.meta?.omitted_files_count ?? 0)} files omitted. `
              : ""}
            {(report.meta?.omitted_snippets_count ?? 0) > 0
              ? `${formatCount(report.meta?.omitted_snippets_count ?? 0)} snippets omitted.`
              : ""}
          </p>
        </div>
      ) : null}

      <div className="review-brief__summary">
        <section>
          <h3>Overview</h3>
          <p>{report.summary.overview || "No overview provided."}</p>
        </section>

        <section>
          <h3>Key changes</h3>
          {report.summary.key_changes.length > 0 ? (
            <ul>
              {report.summary.key_changes.map((change, index) => (
                <li key={`${change}-${index}`}>{change}</li>
              ))}
            </ul>
          ) : (
            <p className="muted">No key changes were reported.</p>
          )}
        </section>

        <section>
          <h3>Review focus</h3>
          {report.summary.review_focus.length > 0 ? (
            <ul>
              {report.summary.review_focus.map((focus, index) => (
                <li key={`${focus}-${index}`}>{focus}</li>
              ))}
            </ul>
          ) : (
            <p className="muted">No review focus areas were reported.</p>
          )}
        </section>
      </div>
    </section>
  );
}
