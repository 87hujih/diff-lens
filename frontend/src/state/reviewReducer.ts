import type {
  DonePayload,
  ErrorPayload,
  PRInfo,
  Report,
  ReviewEvent,
  Risk,
  RulesPayload,
  StepPayload
} from "../types/review";

// ReviewStatus 记录事件流生命周期，用于粗粒度 UI 状态。
export type ReviewStatus = "idle" | "running" | "completed" | "failed";

// ReviewState 是后端 SSE 事件流在前端的状态投影。
export interface ReviewState {
  status: ReviewStatus;
  steps: StepPayload[];
  pr: PRInfo | null;
  ruleRisks: Risk[];
  aiText: string;
  result: Report | null;
  error: ErrorPayload | null;
  degraded: boolean;
}

// initialReviewState 在 reducer 启动新会话时复用。
export const initialReviewState: ReviewState = {
  status: "idle",
  steps: [],
  pr: null,
  ruleRisks: [],
  aiText: "",
  result: null,
  error: null,
  degraded: false
};

export type ReviewAction =
  | { type: "reset" }
  | { type: "stream_event"; event: ReviewEvent }
  | ReviewEvent;

function asStreamEvent(action: ReviewAction): ReviewEvent | null {
  if ("event" in action && action.type === "stream_event") {
    return action.event;
  }

  if ("data" in action) {
    return action;
  }

  return null;
}

function toDisplayText(value: unknown): string {
  if (value == null) {
    return "";
  }

  if (typeof value === "string") {
    return value;
  }

  try {
    const json = JSON.stringify(value);
    return json ?? "";
  } catch {
    return String(value);
  }
}

function mergeStep(steps: StepPayload[], nextStep: StepPayload): StepPayload[] {
  const existingIndex = steps.findIndex((step) => step.step === nextStep.step);

  if (existingIndex < 0) {
    return [...steps, nextStep];
  }

  const nextSteps = steps.slice();
  nextSteps[existingIndex] = nextStep;
  return nextSteps;
}

// reviewReducer 将每个 SSE 事件折叠为可渲染的应用状态。
export function reviewReducer(state: ReviewState, action: ReviewAction): ReviewState {
  const event = asStreamEvent(action);

  if (!event) {
    return initialReviewState;
  }

  switch (event.type) {
    case "step":
      return {
        ...state,
        status: "running",
        steps: mergeStep(state.steps, event.data as StepPayload)
      };
    case "pr":
      return {
        ...state,
        pr: event.data as PRInfo
      };
    case "rules":
      return {
        ...state,
        ruleRisks: (event.data as RulesPayload).risks
      };
    case "ai_delta":
      return {
        ...state,
        aiText: state.aiText + toDisplayText(event.data)
      };
    case "result":
      return {
        ...state,
        result: event.data as Report,
        degraded: Boolean((event.data as Report).degraded || state.degraded)
      };
    case "error":
      return {
        ...state,
        status: "failed",
        error: event.data as ErrorPayload
      };
    case "done": {
      const done = event.data as DonePayload;
      return {
        ...state,
        status: done.ok ? "completed" : "failed",
        degraded: Boolean(done.degraded || state.degraded)
      };
    }
    default:
      return state;
  }
}
