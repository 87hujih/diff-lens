import type { StepPayload } from "../types/review";
import { formatStepStatus } from "../utils/displayText";
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

  let body = "本次运行尚未开始 AI 分析。";

  if (trimmedAIText) {
    body = trimmedAIText;
  } else if (aiStatus === "running") {
    body = "AI 分析正在运行，等待模型流式输出。";
  } else if (aiStatus === "completed" || aiStatus === "failed") {
    body = "AI 阶段已结束，但没有输出流式文本；结构化评审数据可能仍然可用。";
  }

  return (
    <section className="ai-trace-panel" aria-labelledby="ai-trace-title">
      <div className="section-heading">
        <div>
          <p className="eyebrow">AI 轨迹</p>
          <h3 id="ai-trace-title">模型输出流</h3>
        </div>
        <span className={`trace-status trace-status--${aiStatus}`}>
          {aiStatus === "not-started" ? "未开始" : formatStepStatus(aiStatus)}
        </span>
      </div>
      <pre className={trimmedAIText ? "" : "ai-trace-panel__empty"}>{body}</pre>
    </section>
  );
}
