# Architecture Scaffold Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first backend/frontend architecture scaffold for diff-lens without implementing real GitHub or LLM analysis.

**Architecture:** The backend is a Go/Gin single service with `handler -> review.Service -> adapters` boundaries. The scaffold defines `ReviewEvent`, demo streaming, SSE encoding, config loading, and adapter placeholders. The frontend is a React/Vite shell with typed SSE state managed by a reducer.

**Tech Stack:** Go, Gin, React, Vite, TypeScript, Node test runner for existing scripts.

---

### Task 1: Backend Core Types And Demo Stream

**Files:**
- Create: `go.mod`
- Create: `internal/review/types.go`
- Create: `internal/review/service.go`
- Create: `internal/review/service_test.go`
- Create: `internal/demo/demo.go`

- [ ] Write a failing Go test that demo mode emits `step`, `pr`, `rules`, `result`, and `done` events in order.
- [ ] Run `go test ./internal/review` and verify it fails because the package does not exist.
- [ ] Implement minimal review types, service wiring, and demo provider to pass the test.
- [ ] Run `go test ./internal/review` and verify it passes.

### Task 2: Config And SSE Handler Boundary

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/handler/sse.go`
- Create: `internal/handler/sse_test.go`
- Create: `internal/handler/review_handler.go`
- Create: `internal/handler/router.go`
- Create: `cmd/server/main.go`

- [ ] Write failing tests for config defaults and SSE event formatting.
- [ ] Run targeted tests and verify they fail before implementation.
- [ ] Implement minimal config loader, SSE encoder, Gin router, and server entrypoint.
- [ ] Run targeted tests and verify they pass.

### Task 3: Adapter Package Placeholders

**Files:**
- Create: `internal/github/client.go`
- Create: `internal/diff/parser.go`
- Create: `internal/rules/scanner.go`
- Create: `internal/llm/analyzer.go`

- [ ] Add compile-time placeholders matching the architecture boundaries.
- [ ] Run `go test ./...` and verify all backend packages compile.

### Task 4: Frontend Shell

**Files:**
- Create: `frontend/package.json`
- Create: `frontend/index.html`
- Create: `frontend/tsconfig.json`
- Create: `frontend/vite.config.ts`
- Create: `frontend/src/types/review.ts`
- Create: `frontend/src/api/reviewStream.ts`
- Create: `frontend/src/state/reviewReducer.ts`
- Create: `frontend/src/App.tsx`
- Create: `frontend/src/main.tsx`
- Create: `frontend/src/styles.css`

- [ ] Create a minimal typed React/Vite shell that mirrors the backend event contract.
- [ ] Keep business judgment out of the frontend; reducer only updates state from events.
- [ ] Run install/build only if dependencies are already available or installation is possible.

### Task 5: Verification

- [ ] Run `go test ./...`.
- [ ] Run existing PR quality script test: `node scripts/check-pr-quality.test.mjs`.
- [ ] Report any frontend build limitation caused by missing dependencies or restricted network.
