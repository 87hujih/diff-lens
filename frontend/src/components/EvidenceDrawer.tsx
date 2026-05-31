import { useEffect, useRef, useState } from "react";

import type { Risk } from "../types/review";
import { copyToClipboard } from "../utils/clipboard";
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
    return "No specific line";
  }

  return risk.line ? `${risk.file}:${risk.line}` : `${risk.file} · No specific line`;
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
            <p className="eyebrow">Evidence</p>
            <h3 id="evidence-title">Select a risk</h3>
          </div>
        </div>
        <p>Choose a finding to inspect evidence, references, and remediation guidance.</p>
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
          <p className="eyebrow">Evidence</p>
          <h3 id="evidence-title">{risk.title}</h3>
        </div>
        <button
          type="button"
          className="icon-button"
          aria-label="Close evidence drawer"
          title="Close"
          onClick={onClose}
        >
          <span aria-hidden="true">&times;</span>
        </button>
      </div>

      <dl className="evidence-drawer__meta">
        <div>
          <dt>Severity</dt>
          <dd>
            <span className={`severity-badge severity-badge--${severity}`}>
              {risk.severity || "unknown"}
            </span>
          </dd>
        </div>
        <div>
          <dt>Source</dt>
          <dd>{risk.source}</dd>
        </div>
        <div>
          <dt>Location</dt>
          <dd>{formatLocation(risk)}</dd>
        </div>
      </dl>

      <section className="evidence-block">
        <div className="evidence-block__heading">
          <h4>Evidence</h4>
          {hasEvidence ? (
            <button
              type="button"
              className="secondary compact-button"
              onClick={() => {
                void copyText("evidence", risk.evidence ?? "");
              }}
            >
              {feedback.evidence === "copied"
                ? "Copied"
                : feedback.evidence === "failed"
                  ? "Copy failed"
                  : "Copy evidence"}
            </button>
          ) : null}
        </div>
        {hasEvidence ? (
          <pre>{risk.evidence}</pre>
        ) : hasRefs ? (
          <p className="muted">The current report includes reference IDs but not full snippets.</p>
        ) : (
          <p className="muted">No evidence snippet was provided for this finding.</p>
        )}
      </section>

      <section className="evidence-block">
        <h4>Reference IDs</h4>
        {hasRefs ? (
          <ul className="reference-list">
            {risk.evidence_refs?.map((ref) => (
              <li key={ref}>
                <code>{ref}</code>
              </li>
            ))}
          </ul>
        ) : (
          <p className="muted">No evidence reference IDs were provided.</p>
        )}
      </section>

      <section className="evidence-block">
        <div className="evidence-block__heading">
          <h4>Suggestion</h4>
          {risk.suggestion ? (
            <button
              type="button"
              className="secondary compact-button"
              onClick={() => {
                void copyText("suggestion", risk.suggestion);
              }}
            >
              {feedback.suggestion === "copied"
                ? "Copied"
                : feedback.suggestion === "failed"
                  ? "Copy failed"
                  : "Copy suggestion"}
            </button>
          ) : null}
        </div>
        <p>{risk.suggestion || "No remediation suggestion was provided."}</p>
      </section>
    </aside>
  );
}
