import type { FormEvent } from "react";

// PrInputPanelProps 定义 PR 输入表单的受控字段和提交回调。
interface PrInputPanelProps {
  prURL: string;
  token: string;
  isRunning: boolean;
  onPrURLChange: (value: string) => void;
  onTokenChange: (value: string) => void;
  onAnalyze: (event: FormEvent<HTMLFormElement>) => void;
  onRunDemo: () => void;
}

// PrInputPanel 收集 PR URL、可选 token，并提供演示流入口。
export function PrInputPanel({
  prURL,
  token,
  isRunning,
  onPrURLChange,
  onTokenChange,
  onAnalyze,
  onRunDemo
}: PrInputPanelProps) {
  return (
    <section className="input-panel" aria-labelledby="review-console-title">
      <div className="input-panel__heading">
        <p className="eyebrow">diff-lens</p>
        <h1 id="review-console-title">PR review console</h1>
        <p className="input-panel__subtitle">
          Stream local checks, AI context, evidence, and copy-ready comments from one review console.
        </p>
      </div>

      <form onSubmit={onAnalyze} className="review-form" aria-busy={isRunning}>
        <div className="field-group">
          <label htmlFor="pr-url">Pull request URL</label>
          <input
            id="pr-url"
            type="url"
            value={prURL}
            onChange={(event) => onPrURLChange(event.target.value)}
            placeholder="https://github.com/owner/repo/pull/123"
            autoComplete="url"
            inputMode="url"
            spellCheck={false}
            disabled={isRunning}
            required
          />
        </div>

        <details className="token-details">
          <summary>
            <span>GitHub token</span>
            <span className="token-details__hint">Optional</span>
          </summary>
          <div className="field-group">
            <label htmlFor="github-token">Token (optional)</label>
            <input
              id="github-token"
              value={token}
              onChange={(event) => onTokenChange(event.target.value)}
              type="password"
              autoComplete="off"
              disabled={isRunning}
            />
            <p className="field-help">Use a token for private repositories or higher GitHub API limits.</p>
          </div>
        </details>

        <div className="form-actions">
          <button type="submit" disabled={isRunning}>
            {isRunning ? "Analyzing PR..." : "Analyze PR"}
          </button>
          <button type="button" className="secondary" onClick={onRunDemo} disabled={isRunning}>
            Run demo
          </button>
        </div>
        {isRunning ? (
          <p className="form-status" role="status" aria-live="polite">
            Streaming pipeline events from the backend.
          </p>
        ) : null}
      </form>
    </section>
  );
}
