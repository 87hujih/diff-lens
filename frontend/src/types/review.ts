// ReviewEventType 对齐 Go 端通过 SSE 写出的 EventType 常量。
export type ReviewEventType =
  | "step"
  | "pr"
  | "rules"
  | "ai_delta"
  | "result"
  | "error"
  | "done";

// AnalyzeRequest 是前端发送流式分析请求时使用的请求体类型。
export interface AnalyzeRequest {
  pr_url: string;
  github_token?: string;
  demo: boolean;
}

// ReviewEvent 包装一条来自后端的类型化 SSE 载荷。
export interface ReviewEvent<T = unknown> {
  type: ReviewEventType;
  data: T;
}

// StepPayload 描述时间线中的一条进度项。
export interface StepPayload {
  step: string;
  status: string;
  message: string;
}

// PRInfo 是用于展示的标准化 PR 元数据。
export interface PRInfo {
  title: string;
  author: string;
  repo: string;
  number: number;
  source_branch: string;
  target_branch: string;
  changed_files: number;
  additions: number;
  deletions: number;
  commits: number;
}

// Risk 表示来自规则、AI 或合并证据的一条 review 风险。
export interface Risk {
  id: string;
  source: "rule" | "ai" | "merged";
  severity: "low" | "medium" | "high";
  confidence: number;
  category: string;
  title: string;
  file?: string;
  line?: number;
  evidence?: string;
  reason: string;
  suggestion: string;
}

// RulesPayload 聚合确定性扫描器发现的问题。
export interface RulesPayload {
  risks: Risk[];
}

// Summary 是展示在详细风险上方的高层说明。
export interface Summary {
  risk_level: "low" | "medium" | "high";
  overview: string;
  key_changes: string[];
  review_focus: string[];
}

// SuggestedComment 是可直接复制到 GitHub 的 review 评论草稿。
export interface SuggestedComment {
  id: string;
  file?: string;
  line?: number;
  body: string;
}

// Report 是最终的结构化 review 结果。
export interface Report {
  pr: PRInfo;
  summary: Summary;
  risks: Risk[];
  comments: SuggestedComment[];
  degraded?: boolean;
}

// ErrorPayload 让 UI 无需解析人工文本即可渲染失败状态。
export interface ErrorPayload {
  code: string;
  message: string;
  recoverable: boolean;
  stage?: string;
}

// DonePayload 关闭事件流，并记录结果是否降级。
export interface DonePayload {
  ok: boolean;
  degraded?: boolean;
}
