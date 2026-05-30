import { FormEvent, useReducer, useRef, useState } from "react";

import { analyzeReviewStream } from "./api/reviewStream";
import { initialReviewState, reviewReducer } from "./state/reviewReducer";
import type { Risk, SuggestedComment } from "./types/review";
import "./styles.css";

export default function App() {
  const [state, dispatch] = useReducer(reviewReducer, initialReviewState);
  const [prURL, setPrURL] = useState("");
  const [token, setToken] = useState("");
  const abortRef = useRef<AbortController | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    abortRef.current?.abort();

    // 只保留一个活跃分析流，避免不同请求的事件交错写入状态。
    const controller = new AbortController();
    abortRef.current = controller;

    await analyzeReviewStream(
      { pr_url: prURL, github_token: token || undefined, demo: false },
      dispatch,
      controller.signal
    );
  }

  async function runDemo() {
    abortRef.current?.abort();

    // Demo 模式无需 GitHub 凭据即可跑通完整 SSE 界面。
    const controller = new AbortController();
    abortRef.current = controller;

    await analyzeReviewStream({ pr_url: "", demo: true }, dispatch, controller.signal);
  }

  const report = state.result;
  // 规则风险到达后立即展示；最终报告生成后优先展示合并后的风险。
  const risks = report?.risks ?? state.ruleRisks;

  return (
    <main className="app-shell">
      <section className="input-panel">
        <div>
          <p className="eyebrow">diff-lens</p>
          <h1>AI PR review console</h1>
        </div>
        <form onSubmit={submit} className="review-form">
          <input
            value={prURL}
            onChange={(event) => setPrURL(event.target.value)}
            placeholder="https://github.com/owner/repo/pull/123"
          />
          <input
            value={token}
            onChange={(event) => setToken(event.target.value)}
            placeholder="GitHub token, optional"
            type="password"
          />
          <div className="form-actions">
            <button type="submit">Analyze PR</button>
            <button type="button" className="secondary" onClick={runDemo}>
              Demo PR
            </button>
          </div>
        </form>
      </section>

      <section className="workspace">
        <aside className="timeline">
          <h2>Steps</h2>
          {state.steps.length === 0 ? <p className="muted">No analysis running.</p> : null}
          {state.steps.map((step, index) => (
            <div className="step" key={`${step.step}-${index}`}>
              <span>{step.status}</span>
              <strong>{step.step}</strong>
              <p>{step.message}</p>
            </div>
          ))}
        </aside>

        <section className="report">
          {state.error ? (
            <div className="error-box">
              <strong>{state.error.code}</strong>
              <p>{state.error.message}</p>
            </div>
          ) : null}

          {report ? (
            <>
              <header className="report-header">
                <div>
                  <p className="eyebrow">{report.pr.repo} #{report.pr.number}</p>
                  <h2>{report.pr.title}</h2>
                </div>
                <span className={`risk-level ${report.summary.risk_level}`}>
                  {report.summary.risk_level}
                </span>
              </header>
              <p>{report.summary.overview}</p>
              <RiskList risks={risks} />
              <CommentList comments={report.comments} />
            </>
          ) : (
            <div className="empty-report">
              <h2>Waiting for review output</h2>
              <p>Start with a PR URL or use the demo stream.</p>
            </div>
          )}
        </section>
      </section>
    </main>
  );
}

// RiskList 使用同一卡片布局渲染扫描器风险和最终报告风险。
function RiskList({ risks }: { risks: Risk[] }) {
  if (risks.length === 0) {
    return null;
  }

  return (
    <section className="risk-list">
      <h3>Risk radar</h3>
      {risks.map((risk) => (
        <article className="risk-card" key={risk.id}>
          <div>
            <span className={`risk-dot ${risk.severity}`} />
            <strong>{risk.title}</strong>
          </div>
          <p>{risk.reason}</p>
          <small>
            {risk.source} · {Math.round(risk.confidence * 100)}%
          </small>
        </article>
      ))}
    </section>
  );
}

// CommentList 将复制到剪贴板的行为限定在建议评论区域。
function CommentList({ comments }: { comments: SuggestedComment[] }) {
  if (comments.length === 0) {
    return null;
  }

  return (
    <section className="comments">
      <h3>Suggested comments</h3>
      {comments.map((comment) => (
        <article className="comment-card" key={comment.id}>
          <p>{comment.body}</p>
          <button type="button" onClick={() => navigator.clipboard.writeText(comment.body)}>
            Copy
          </button>
        </article>
      ))}
    </section>
  );
}
