import type { StepPayload } from "../types/review";
import { formatStepStatus } from "../utils/displayText";
import { normalizeStepStatus } from "../utils/reviewStatus";

const STEP_LABELS: Record<string, string> = {
  fetch_pr: "获取 PR",
  parse_diff: "解析 diff",
  scan_rules: "规则扫描",
  build_context: "构建上下文",
  analyze_ai: "AI 分析",
  result: "生成报告"
};

function getStepLabel(stepId: string): string {
  return STEP_LABELS[stepId] ?? stepId;
}

interface StepTimelineProps {
  steps: StepPayload[];
}

export function StepTimeline({ steps }: StepTimelineProps) {
  return (
    <aside className="timeline" aria-labelledby="timeline-title">
      <div className="timeline__header">
        <p className="eyebrow">流水线</p>
        <h2 id="timeline-title">步骤</h2>
      </div>

      {steps.length === 0 ? (
        <div className="timeline-empty">
          <strong>暂无流式事件</strong>
          <p>运行 PR 分析后可查看后端进度。</p>
        </div>
      ) : (
        <ol className="step-list">
          {steps.map((step, index) => {
            const status = normalizeStepStatus(step.status);
            const label = getStepLabel(step.step);

            return (
              <li className={`step step--${status}`} key={`${step.step}-${index}`}>
                <div className="step__marker" aria-hidden="true" />
                <div className="step__body">
                  <div className="step__topline">
                    <strong>{label}</strong>
                    <span title={`原始状态：${step.status}`}>{formatStepStatus(status)}</span>
                  </div>
                  <code>{step.step}</code>
                  {step.message ? <p>{step.message}</p> : null}
                </div>
              </li>
            );
          })}
        </ol>
      )}
    </aside>
  );
}
