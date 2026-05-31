import type { StepPayload } from "../types/review";
import { normalizeStepStatus } from "../utils/reviewStatus";

interface AITracePanelProps {
  aiText: string;
  steps: StepPayload[];
}

function getAIStatus(steps: StepPayload[]): "running" | "completed" | "failed" | "not-started" {
  const aiStep = [...steps].reverse().find((step) => step.step === "analyze_ai");

  if (!aiStep) {
    return "not-started";
  }

  const status = normalizeStepStatus(aiStep.status);

  if (status === "running" || status === "completed" || status === "failed") {
    return status;
  }

  return "not-started";
}

export function AITracePanel({ aiText, steps }: AITracePanelProps) {
  const trimmedAIText = aiText.trim();
  const aiStatus = getAIStatus(steps);

  let body = "AI analysis has not started for this run.";

  if (trimmedAIText) {
    body = trimmedAIText;
  } else if (aiStatus === "running") {
    body = "AI analysis is running. Waiting for streamed model trace.";
  } else if (aiStatus === "completed" || aiStatus === "failed") {
    body = "The AI stage finished without streaming trace text. Structured review data may still be available.";
  }

  const statusLabel = aiStatus === "not-started" ? "idle" : aiStatus;

  return (
    <section className="ai-trace-panel" aria-labelledby="ai-trace-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">AI trace</p>
          <h3 id="ai-trace-title">Model stream</h3>
        </div>
        <span className={`trace-status trace-status--${aiStatus}`}>{statusLabel}</span>
      </div>
      <pre className={trimmedAIText ? "" : "ai-trace-panel__empty"}>{body}</pre>
    </section>
  );
}
