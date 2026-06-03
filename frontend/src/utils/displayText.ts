import type { RenderRiskLevel, RenderSeverity, RenderStepStatus } from "../types/review";
import { normalizeRiskLevel, normalizeSeverity, normalizeStepStatus } from "./reviewStatus";

const RISK_LEVEL_LABELS: Record<RenderRiskLevel, string> = {
  high: "高",
  medium: "中",
  low: "低",
  unknown: "未知"
};

const SEVERITY_LABELS: Record<RenderSeverity, string> = {
  high: "高",
  medium: "中",
  low: "低",
  unknown: "未知"
};

const STEP_STATUS_LABELS: Record<RenderStepStatus, string> = {
  running: "运行中",
  completed: "已完成",
  failed: "失败",
  unknown: "未知"
};

const SOURCE_LABELS: Record<string, string> = {
  rule: "规则",
  ai: "AI",
  merged: "合并"
};

const CATEGORY_LABELS: Record<string, string> = {
  auth: "权限",
  authz: "授权",
  data_scope: "数据范围",
  runtime: "运行时",
  security: "安全",
  style: "样式",
  test: "测试",
  test_gap: "测试缺口"
};

export function formatRiskLevel(value: unknown): string {
  return RISK_LEVEL_LABELS[normalizeRiskLevel(value)];
}

export function formatSeverity(value: unknown): string {
  return SEVERITY_LABELS[normalizeSeverity(value)];
}

export function formatStepStatus(value: unknown): string {
  return STEP_STATUS_LABELS[normalizeStepStatus(value)];
}

export function formatSource(value: string): string {
  return SOURCE_LABELS[value.toLowerCase()] ?? value;
}

export function formatCategory(value?: string): string {
  if (!value) {
    return "未分类";
  }

  return CATEGORY_LABELS[value.toLowerCase()] ?? value;
}
