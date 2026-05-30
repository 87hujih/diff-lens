import assert from "node:assert/strict";

import { validatePullRequest } from "./check-pr-quality.mjs";

const validBody = `
## 功能描述

本 PR 新增 PR 规范自动检查，在创建或更新 Pull Request 时提示开发者补充必要信息。

用户创建 PR 后，GitHub Actions 会自动校验标题和描述是否符合比赛规范。

## 实现思路

通过 GitHub Actions 读取 PR 元信息，并使用本地脚本检查标题、描述章节和变更规模。

## 测试方式

\`\`\`bash
node --test scripts/check-pr-quality.test.mjs
\`\`\`

手动验证步骤：

1. 创建测试 PR。
2. 修改 PR 标题或描述。
3. 查看 PR Quality 检查结果。

## 合并前检查

- [x] 本 PR 只做一件事
`;

const tests = [];

function test(name, fn) {
  tests.push({ name, fn });
}

test("accepts a well-formed PR", () => {
  const errors = validatePullRequest({
    title: "ci: add pr quality checks",
    body: validBody,
    changedFiles: 3,
    additions: 120,
    deletions: 20,
    labels: [],
  });

  assert.deepEqual(errors, []);
});

test("rejects unclear PR title", () => {
  const errors = validatePullRequest({
    title: "update",
    body: validBody,
    changedFiles: 3,
    additions: 120,
    deletions: 20,
    labels: [],
  });

  assert.match(errors.join("\n"), /PR 标题需符合格式/);
});

test("rejects missing required sections", () => {
  const errors = validatePullRequest({
    title: "ci: add pr quality checks",
    body: "## 功能描述\n\n待补充",
    changedFiles: 3,
    additions: 120,
    deletions: 20,
    labels: [],
  });

  assert.match(errors.join("\n"), /功能描述/);
  assert.match(errors.join("\n"), /实现思路/);
  assert.match(errors.join("\n"), /测试方式/);
});

test("rejects PRs without the one-thing checkbox", () => {
  const errors = validatePullRequest({
    title: "ci: add pr quality checks",
    body: validBody.replace("- [x] 本 PR 只做一件事", "- [ ] 本 PR 只做一件事"),
    changedFiles: 3,
    additions: 120,
    deletions: 20,
    labels: [],
  });

  assert.match(errors.join("\n"), /本 PR 只做一件事/);
});

test("rejects large PRs unless allow-large-pr label is present", () => {
  const errors = validatePullRequest({
    title: "ci: add pr quality checks",
    body: validBody,
    changedFiles: 25,
    additions: 700,
    deletions: 200,
    labels: [],
  });

  assert.match(errors.join("\n"), /修改了 25 个文件/);
  assert.match(errors.join("\n"), /diff 约 900 行/);

  const allowed = validatePullRequest({
    title: "ci: add pr quality checks",
    body: validBody,
    changedFiles: 25,
    additions: 700,
    deletions: 200,
    labels: ["allow-large-pr"],
  });

  assert.deepEqual(allowed, []);
});

let failed = 0;
for (const { name, fn } of tests) {
  try {
    fn();
    console.log(`ok - ${name}`);
  } catch (error) {
    failed += 1;
    console.error(`not ok - ${name}`);
    console.error(error);
  }
}

if (failed > 0) {
  process.exitCode = 1;
}
