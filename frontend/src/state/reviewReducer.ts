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

// reviewReducer 将每个 SSE 事件折叠为可渲染的应用状态。
export function reviewReducer(state: ReviewState, event: ReviewEvent): ReviewState {
  switch (event.type) {
    case "step":
      return {
        ...state,
        status: "running",
        steps: [...state.steps, event.data as StepPayload]
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
        aiText: state.aiText + String(event.data)
      };
    case "result":
      return {
        ...state,
        result: event.data as Report,
        degraded: Boolean((event.data as Report).degraded)
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
