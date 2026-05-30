# Commit Message 规范

## 基本原则

1. 每个 commit 只表达一个清晰变更点。
2. 不要把大量无关改动合在一个 commit 里。
3. commit 时间必须落在比赛批次开始与截止时间内。
4. 不要最后一天一次性导入全部代码。
5. commit 信息要能让评委理解持续开发过程。

## 推荐格式

```text
<type>: <short summary>
```

示例：

```text
feat: add project dashboard
fix: handle empty input validation
docs: add local setup guide
style: polish home page layout
test: add parser unit tests
refactor: split task storage helper
chore: configure lint script
```

## Type 列表

| Type | 用途 |
| --- | --- |
| feat | 新增功能 |
| fix | 修复问题 |
| docs | 文档变更 |
| style | 样式、排版、格式调整，不改变逻辑 |
| refactor | 重构代码，不新增功能也不修 bug |
| test | 新增或修改测试 |
| chore | 构建、配置、依赖、脚手架等杂项 |
| perf | 性能优化 |
| ci | CI/CD 配置 |

## 推荐粒度

好的 commit：

```text
feat: add task creation form
feat: persist tasks in local storage
fix: prevent submit when title is empty
docs: document third-party dependencies
```

不推荐的 commit：

```text
update
fix bug
final version
add all files
complete project
```

## 比赛期间建议提交节奏

### 第 1 天

```text
chore: initialize project scaffold
docs: add initial README structure
feat: add base app layout
```

### 第 2 天

```text
feat: add core feature one
feat: add core feature two
fix: handle invalid user input
style: improve responsive layout
```

### 第 3 天

```text
test: add core workflow checks
docs: add dependency and originality notes
docs: add demo video link
fix: polish final demo flow
```

## PR 与 commit 对应建议

一个 PR 可以包含多个 commit，但这些 commit 应围绕同一个功能或变更目标。

示例：

```text
PR: feat: add task management flow

commits:
- feat: add task data model
- feat: add task creation form
- fix: validate empty task title
- test: add task creation tests
```

## 提交前检查

- [ ] 当前变更是否属于正在做的这个 PR？
- [ ] commit 是否只包含一个清晰目的？
- [ ] commit message 是否能说明实际变更？
- [ ] 是否误提交了 `.env`、密钥、临时文件或构建产物？
- [ ] 如复用历史代码，是否已在 PR 描述中说明来源？
- [ ] 如新增依赖，是否已在 README 中说明用途？
