# Release Checklist

Use this checklist for final release verification. Do not record real GitHub tokens, LLM API keys, or bearer headers in notes, screenshots, commits, logs, or issue comments.

## Required Commands

- `go test ./...`
- Confirm the `go test ./...` output no longer reports `diff-lens/internal/demo [no test files]`.
- `npm --prefix frontend run build`
- `node scripts/check-pr-quality.test.mjs`

## Manual API Checks

Start the backend:

```powershell
go run ./cmd/server
```

Demo SSE:

```powershell
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"demo\":true}"
```

Real public PR SSE, without writing tokens into the command:

```powershell
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

Invalid PR URL:

```powershell
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"not-a-pr-url\",\"demo\":false}"
```

No LLM key degraded path:

```powershell
Remove-Item Env:LLM_API_KEY -ErrorAction SilentlyContinue
go run ./cmd/server
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

Expected result: final report is degraded, deterministic rule findings are preserved, and no LLM key appears in output.

## Mock LLM Success Path

The mock service is only a release QA tool. It verifies the OpenAI-compatible transport, JSON parsing, report normalization, SSE result shape, and frontend rendering. It is not evidence of real model quality and must not be described as real AI analysis.

Start the mock:

```powershell
python scripts/mock-openai-compatible.py --host 127.0.0.1 --port 8787
```

In another shell, start the backend against the mock:

```powershell
$env:LLM_BASE_URL = "http://127.0.0.1:8787"
$env:LLM_API_KEY = "test-key"
$env:LLM_MODEL = "mock-model"
go run ./cmd/server
```

Run a real public PR through the backend:

```powershell
curl -N -X POST http://localhost:8080/api/reviews/analyze/stream -H "Content-Type: application/json" --data "{\"pr_url\":\"https://github.com/{owner}/{repo}/pull/{number}\",\"demo\":false}"
```

Expected result: the stream reaches `event: result` and `event: done`, `result.meta.ai_completed` is true, and the report includes a mock AI comment. A mock risk may become `ai` or `merged` when its `evidence_refs` match the bounded context.

The mock tries to cite stable evidence from the actual request context in this order: rule finding ID, rule evidence refs, snippet ID, then top-level context evidence refs. Because public PR context is not fixed, a mock risk can still be discarded by `ReportNormalizer` when refs are absent or no longer valid. If that happens, first adjust the mock or test PR to cite a real rule finding ID or context snippet ID from the current stream instead of weakening normalizer behavior.

Stop the mock and backend after the check. Do not leave background processes running.

## Security Scan

Run the targeted secret scan:

```powershell
rg -n "ghp_[A-Za-z0-9_]+|github_pat_|sk-[A-Za-z0-9_-]{20,}" cmd internal frontend scripts README.md competition_README_TEMPLATE.md competition_PR_TEMPLATE.md competition_COMMIT_CONVENTION.md --glob "!frontend/node_modules/**" --glob "!frontend/dist/**"
```

Run sensitive-field review:

```powershell
rg -n "LLM_API_KEY|GITHUB_TOKEN|github_token|Authorization|localStorage|sessionStorage" cmd internal frontend README.md competition_README_TEMPLATE.md competition_PR_TEMPLATE.md competition_COMMIT_CONVENTION.md --glob "!frontend/node_modules/**" --glob "!frontend/dist/**"
```

Run the broader repository scan, excluding generated or vendored paths:

```powershell
rg -n "ghp_|github_pat_|sk-|LLM_API_KEY|GITHUB_TOKEN|github_token|Authorization" . --glob "!frontend/node_modules/**" --glob "!frontend/dist/**" --glob "!ui-ux-pro-max-skill/**"
```

Allowlist only placeholders, schema examples, tests, and intentional documentation references. Remove any real token or API key immediately.

## Final Notes

- Record whether the real PR check used no token, a request-scoped GitHub token, a real LLM, no LLM key, or the mock LLM. Do not record the token or key value.
- Demo mode and mock LLM mode are validation aids, not real model capability claims.
- Keep release notes honest about degraded behavior, heuristic rule scanning, and the need for human review.
