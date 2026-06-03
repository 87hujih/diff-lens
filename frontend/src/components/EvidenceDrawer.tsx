import { useEffect, useRef, useState } from "react";

import type { Risk } from "../types/review";
import { copyToClipboard } from "../utils/clipboard";
import { formatSeverity, formatSource } from "../utils/displayText";
import { normalizeSeverity } from "../utils/reviewStatus";

type CopyFeedback = "idle" | "copied" | "failed";

// EvidenceDrawerProps 传入当前风险和关闭详情抽屉的回调。
interface EvidenceDrawerProps {
  risk: Risk | null;
  onClose: () => void;
}

// formatLocation 将风险定位格式化成人类可读路径。
function formatLocation(risk: Risk): string {
  if (!risk.file) {
    return "无具体行号";
  }

  return risk.line ? `${risk.file}:${risk.line}` : `${risk.file} · 无具体行号`;
}

// EvidenceDrawer 展示单条风险的证据、引用和修复建议。
export function EvidenceDrawer({ risk, onClose }: EvidenceDrawerProps) {
  const [feedback, setFeedback] = useState<Record<string, CopyFeedback>>({});
  const timersRef = useRef<number[]>([]);

  useEffect(() => {
    setFeedback({});
  }, [risk?.id]);

  useEffect(() => {
    return () => {
      timersRef.current.forEach((timer) => window.clearTimeout(timer));
    };
  }, []);

  // 抽屉打开时监听 Escape，关闭时移除监听。
  useEffect(() => {
    if (!risk) {
      return undefined;
    }

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
    };

    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [onClose, risk]);

  async function copyText(key: string, value: string) {
    const result = await copyToClipboard(value);
    setFeedback((current) => ({ ...current, [key]: result.ok ? "copied" : "failed" }));

    const timer = window.setTimeout(() => {
      setFeedback((current) => ({ ...current, [key]: "idle" }));
    }, 1800);
    timersRef.current.push(timer);
  }

  if (!risk) {
    return (
      <aside className="evidence-drawer evidence-drawer--empty" aria-labelledby="evidence-title">
        <div className="section-heading">
          <div>
            <p className="eyebrow">证据</p>
            <h3 id="evidence-title">选择一条风险</h3>
          </div>
        </div>
        <p>选择一条风险后，可查看证据、引用和修复建议。</p>
      </aside>
    );
  }

  const severity = normalizeSeverity(risk.severity);
  const hasEvidence = Boolean(risk.evidence?.trim());
  const hasRefs = Boolean(risk.evidence_refs?.length);

  return (
    <aside className="evidence-drawer" aria-labelledby="evidence-title">
      <div className="evidence-drawer__header">
        <div>
          <p className="eyebrow">证据</p>
          <h3 id="evidence-title">{risk.title}</h3>
        </div>
        <button type="button" className="icon-button" aria-label="关闭证据抽屉" onClick={onClose}>
          X
        </button>
      </div>

      <dl className="evidence-drawer__meta">
        <div>
          <dt>严重级别</dt>
          <dd>
            <span className={`severity-badge severity-badge--${severity}`}>
              {formatSeverity(risk.severity)}
            </span>
          </dd>
        </div>
        <div>
          <dt>来源</dt>
          <dd>{formatSource(risk.source)}</dd>
        </div>
        <div>
          <dt>位置</dt>
          <dd>{formatLocation(risk)}</dd>
        </div>
      </dl>

      <section className="evidence-block">
        <div className="evidence-block__heading">
          <h4>证据</h4>
          {hasEvidence ? (
            <button
              type="button"
              className="secondary compact-button"
              onClick={() => {
                void copyText("evidence", risk.evidence ?? "");
              }}
            >
              {feedback.evidence === "copied" ? "已复制" : feedback.evidence === "failed" ? "复制失败" : "复制"}
            </button>
          ) : null}
        </div>
        {hasEvidence ? (
          <pre>{risk.evidence}</pre>
        ) : hasRefs ? (
          <p className="muted">当前报告包含引用 ID，但未包含完整片段。</p>
        ) : (
          <p className="muted">这条风险未提供证据片段。</p>
        )}
      </section>

      <section className="evidence-block">
        <h4>引用 ID</h4>
        {hasRefs ? (
          <ul className="reference-list">
            {risk.evidence_refs?.map((ref) => (
              <li key={ref}>
                <code>{ref}</code>
              </li>
            ))}
          </ul>
        ) : (
          <p className="muted">未提供证据引用 ID。</p>
        )}
      </section>

      <section className="evidence-block">
        <div className="evidence-block__heading">
          <h4>建议</h4>
          {risk.suggestion ? (
            <button
              type="button"
              className="secondary compact-button"
              onClick={() => {
                void copyText("suggestion", risk.suggestion);
              }}
            >
              {feedback.suggestion === "copied"
                ? "已复制"
                : feedback.suggestion === "failed"
                  ? "复制失败"
                  : "复制"}
            </button>
          ) : null}
        </div>
        <p>{risk.suggestion || "未提供修复建议。"}</p>
      </section>
    </aside>
  );
}
