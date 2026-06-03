import type { ErrorPayload, Report } from "../types/review";

interface StatusBannerProps {
  error: ErrorPayload | null;
  degraded: boolean;
  result: Report | null;
}

function formatErrorMeta(error: ErrorPayload): string {
  const details = [];

  if (error.stage) {
    details.push(`阶段：${error.stage}`);
  }

  details.push(error.recoverable ? "可恢复" : "不可恢复");

  return details.join(" · ");
}

function formatDegradedReason(reason?: string): string {
  if (!reason) {
    return "部分分析阶段未完整完成。";
  }

  const reasons: Record<string, string> = {
    llm_not_configured: "LLM 未配置。",
    llm_output_invalid: "LLM 输出无法解析。",
    llm_request_failed: "LLM 请求失败。"
  };

  return reasons[reason] ?? reason;
}

export function StatusBanner({ error, degraded, result }: StatusBannerProps) {
  const isDegraded = degraded || Boolean(result?.degraded);

  if (!error && !isDegraded) {
    return null;
  }

  return (
    <div className="status-banner-stack" aria-label="评审状态">
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
            <strong>降级分析</strong>
            <p>
              {formatDegradedReason(result?.meta.degraded_reason)}
              基于规则的结果仍可用，即使 LLM 分析缺失或失败。
            </p>
          </div>
        </section>
      ) : null}
    </div>
  );
}
