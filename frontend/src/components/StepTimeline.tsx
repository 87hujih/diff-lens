import type { StepPayload } from "../types/review";
import { normalizeStepStatus } from "../utils/reviewStatus";

const STEP_LABELS: Record<string, string> = {
  fetch_pr: "Fetch PR",
  parse_diff: "Parse diff",
  scan_rules: "Scan rules",
  build_context: "Build context",
  analyze_ai: "Analyze AI"
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
        <p className="eyebrow">Pipeline</p>
        <h2 id="timeline-title">Steps</h2>
      </div>

      {steps.length === 0 ? (
        <div className="timeline-empty">
          <strong>No stream events yet</strong>
          <p>Run a PR analysis to see backend progress.</p>
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
                    <span title={`Original status: ${step.status}`}>{status}</span>
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
