import type { FormEvent } from "react";

interface PrInputPanelProps {
  prURL: string;
  token: string;
  isRunning: boolean;
  onPrURLChange: (value: string) => void;
  onTokenChange: (value: string) => void;
  onAnalyze: (event: FormEvent<HTMLFormElement>) => void;
  onRunDemo: () => void;
}

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
      </div>

      <form onSubmit={onAnalyze} className="review-form">
        <div className="field-group">
          <label htmlFor="pr-url">Pull request URL</label>
          <input
            id="pr-url"
            value={prURL}
            onChange={(event) => onPrURLChange(event.target.value)}
            placeholder="https://github.com/owner/repo/pull/123"
            autoComplete="url"
            disabled={isRunning}
            required
          />
        </div>

        <details className="token-details">
          <summary>GitHub token</summary>
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
          </div>
        </details>

        <div className="form-actions">
          <button type="submit" disabled={isRunning}>
            {isRunning ? "Analyzing..." : "Analyze PR"}
          </button>
          <button type="button" className="secondary" onClick={onRunDemo} disabled={isRunning}>
            Demo PR
          </button>
        </div>
      </form>
    </section>
  );
}
