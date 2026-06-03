import type { Report } from "../types/review";
import { formatRiskLevel } from "../utils/displayText";
import { normalizeRiskLevel } from "../utils/reviewStatus";

// ReviewBriefProps 包含最终归一化报告。
interface ReviewBriefProps {
  report: Report;
}

// formatCount 用统一数字格式展示文件数和行数。
function formatCount(value: number): string {
  return new Intl.NumberFormat("zh-CN").format(value);
}

// ReviewBrief 渲染 PR 摘要、统计、降级提示和评审重点。
export function ReviewBrief({ report }: ReviewBriefProps) {
  const riskLevel = normalizeRiskLevel(report.summary.risk_level);
  const hasTruncation =
    Boolean(report.meta.context_truncated) ||
    report.meta.omitted_files_count > 0 ||
    report.meta.omitted_snippets_count > 0;

  return (
    <section className="review-brief" aria-labelledby="review-brief-title">
      <header className="review-brief__header">
        <div className="review-brief__identity">
          <p className="eyebrow">
            {report.pr.repo} #{report.pr.number}
          </p>
          <h2 id="review-brief-title">{report.pr.title}</h2>
          <div className="review-brief__meta" aria-label="Pull Request 元数据">
            <span>作者：{report.pr.author}</span>
            <span>
              <code>{report.pr.source_branch}</code> 到 <code>{report.pr.target_branch}</code>
            </span>
          </div>
        </div>
        <span className={`risk-level ${riskLevel}`} title={`标准化为 ${riskLevel}`}>
          {formatRiskLevel(report.summary.risk_level)}
        </span>
      </header>

      <dl className="review-brief__stats" aria-label="Pull Request 变更统计">
        <div>
          <dt>文件</dt>
          <dd>{formatCount(report.pr.changed_files)}</dd>
        </div>
        <div>
          <dt>新增</dt>
          <dd className="stat-positive">+{formatCount(report.pr.additions)}</dd>
        </div>
        <div>
          <dt>删除</dt>
          <dd className="stat-negative">-{formatCount(report.pr.deletions)}</dd>
        </div>
        <div>
          <dt>提交</dt>
          <dd>{formatCount(report.pr.commits)}</dd>
        </div>
      </dl>

      {hasTruncation ? (
        <div className="review-alert" role="note">
          <strong>上下文受限</strong>
          <p>
            {report.meta.context_truncated ? "上下文已被截断。 " : ""}
            {report.meta.omitted_files_count > 0
              ? `已省略 ${formatCount(report.meta.omitted_files_count)} 个文件。 `
              : ""}
            {report.meta.omitted_snippets_count > 0
              ? `已省略 ${formatCount(report.meta.omitted_snippets_count)} 个片段。`
              : ""}
          </p>
        </div>
      ) : null}

      <div className="review-brief__summary">
        <section>
          <h3>概览</h3>
          <p>{report.summary.overview || "未提供概览。"}</p>
        </section>

        <section>
          <h3>关键变更</h3>
          {report.summary.key_changes.length > 0 ? (
            <ul>
              {report.summary.key_changes.map((change, index) => (
                <li key={`${change}-${index}`}>{change}</li>
              ))}
            </ul>
          ) : (
            <p className="muted">未报告关键变更。</p>
          )}
        </section>

        <section>
          <h3>评审重点</h3>
          {report.summary.review_focus.length > 0 ? (
            <ul>
              {report.summary.review_focus.map((focus, index) => (
                <li key={`${focus}-${index}`}>{focus}</li>
              ))}
            </ul>
          ) : (
            <p className="muted">未报告评审重点。</p>
          )}
        </section>
      </div>
    </section>
  );
}
