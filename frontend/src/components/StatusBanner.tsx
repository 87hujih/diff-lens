import type { ErrorPayload, Report } from "../types/review";

interface StatusBannerProps {
  error: ErrorPayload | null;
  degraded: boolean;
  result: Report | null;
}

function formatErrorMeta(error: ErrorPayload): string {
  const details = [];

  if (error.stage) {
    details.push(`Stage: ${error.stage}`);
  }

  details.push(error.recoverable ? "Recoverable" : "Not recoverable");

  return details.join(" · ");
}

export function StatusBanner({ error, degraded, result }: StatusBannerProps) {
  const isDegraded = degraded || Boolean(result?.degraded);

  if (!error && !isDegraded) {
    return null;
  }

  return (
    <div className="status-banner-stack" aria-label="Review status">
      {error ? (
        <section className="status-banner status-banner--error" role="alert" aria-live="assertive">
          <div>
            <strong>{error.code}</strong>
            <p>{error.message}</p>
          </div>
          <span>{formatErrorMeta(error)}</span>
        </section>
      ) : null}

      {isDegraded ? (
        <section className="status-banner status-banner--warning" role="status" aria-live="polite">
          <div>
            <strong>Degraded analysis</strong>
            <p>
              {result?.meta?.degraded_reason || "Some analysis stages did not complete fully."} Rules-based results
              remain available when LLM analysis is missing or fails.
            </p>
          </div>
        </section>
      ) : null}
    </div>
  );
}
