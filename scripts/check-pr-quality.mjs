const TITLE_PATTERN = /^(feat|fix|docs|style|test|refactor|chore|perf|ci):\s.{6,80}$/;
const REQUIRED_SECTIONS = ["功能描述", "实现思路", "测试方式"];
const MAX_CHANGED_FILES = 20;
const MAX_CHANGED_LINES = 800;

// validatePullRequest 返回适合 CI 输出的人类可读规则失败信息。
export function validatePullRequest(input) {
  const title = String(input.title ?? "");
  const body = String(input.body ?? "");
  const changedFiles = toNumber(input.changedFiles);
  const additions = toNumber(input.additions);
  const deletions = toNumber(input.deletions);
  const labels = normalizeLabels(input.labels);
  const errors = [];

  if (!TITLE_PATTERN.test(title)) {
    errors.push("PR 标题需符合格式：`feat: add xxx`、`fix: handle xxx`，并清楚说明变更。");
  }

  for (const section of REQUIRED_SECTIONS) {
    if (sectionContent(body, section).length < 10) {
      errors.push(`PR 描述缺少有效内容：\`## ${section}\`。`);
    }
  }

  if (!/- \[[xX]\]\s*本 PR 只做一件事/.test(body)) {
    errors.push("请在合并前检查中勾选：`本 PR 只做一件事`。");
  }

  const allowLargePr = labels.includes("allow-large-pr");
  const changedLines = additions + deletions;

  if (!allowLargePr && changedFiles > MAX_CHANGED_FILES) {
    errors.push(
      `当前 PR 修改了 ${changedFiles} 个文件，可能不符合“每个 PR 只做一件事”。如确实必要，请添加 \`allow-large-pr\` 标签并在描述中解释。`,
    );
  }

  if (!allowLargePr && changedLines > MAX_CHANGED_LINES) {
    errors.push(
      `当前 PR diff 约 ${changedLines} 行，建议拆分 PR。如确实必要，请添加 \`allow-large-pr\` 标签并在描述中解释。`,
    );
  }

  return errors;
}

// readPullRequestFromEnv 是验证逻辑面向 GitHub Actions 的适配层。
export function readPullRequestFromEnv(env = process.env) {
  return {
    title: env.PR_TITLE,
    body: env.PR_BODY,
    changedFiles: env.PR_CHANGED_FILES,
    additions: env.PR_ADDITIONS,
    deletions: env.PR_DELETIONS,
    labels: parseLabels(env.PR_LABELS),
  };
}

// formatErrors 让 CI 日志和注解中的失败信息更易读。
export function formatErrors(errors) {
  return ["PR 提交规范检查失败：", "", ...errors.map((error) => `- ${error}`)].join("\n");
}

// sectionContent 会移除模板提示，避免占位文本被算作有效内容。
function sectionContent(markdown, heading) {
  const escaped = escapeRegExp(heading);
  const regex = new RegExp(`(?:^|\\n)##\\s+${escaped}\\s*\\n([\\s\\S]*?)(?=\\n##\\s+|$)`, "i");
  const match = markdown.match(regex);
  if (!match) {
    return "";
  }

  return match[1]
    .replace(/```[\s\S]*?```/g, "")
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/请.*$/gm, "")
    .replace(/示例.*$/gm, "")
    .replace(/待补充/g, "")
    .replace(/-\s*$/gm, "")
    .trim();
}

// normalizeLabels 同时支持 GitHub label 对象和纯标签名。
function normalizeLabels(labels) {
  if (!Array.isArray(labels)) {
    return [];
  }

  return labels
    .map((label) => {
      if (typeof label === "string") {
        return label;
      }
      return label?.name;
    })
    .filter(Boolean);
}

// parseLabels 支持 GitHub Actions 的 JSON 输入和逗号分隔的兜底输入。
function parseLabels(value) {
  if (!value) {
    return [];
  }

  try {
    return normalizeLabels(JSON.parse(value));
  } catch {
    return String(value)
      .split(",")
      .map((label) => label.trim())
      .filter(Boolean);
  }
}

function toNumber(value) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function escapeRegExp(value) {
  return String(value).replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// 直接运行脚本时，校验 workflow 步骤导出的 PR 数据。
if (import.meta.url === `file:///${process.argv[1]?.replace(/\\/g, "/")}`) {
  const errors = validatePullRequest(readPullRequestFromEnv());
  if (errors.length > 0) {
    console.error(formatErrors(errors));
    process.exitCode = 1;
  } else {
    console.log("PR 提交规范检查通过。");
  }
}
