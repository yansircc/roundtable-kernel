# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
make build              # Build ./rtk binary (injects git version/commit/date via ldflags)
make install            # Install rtk to GOBIN or GOPATH/bin
make test               # All Go tests (unit + integration)
make test-unit          # Unit tests only
make test-integration   # Integration tests only (requires -tags=integration)
make build-ui           # Build Svelte UI (npm ci + npm run build in ui/)
make ci                 # Full CI: test-unit, test-integration, build, build-ui
```

Run a single test:
```bash
go test ./internal/rtk/ -run TestValidateFindingSupportedRequiresEvidenceIDs
```

Run integration tests:
```bash
go test ./internal/rtk/ -tags=integration -run TestLiveFlowConvergesWithoutCritics
```

Go 1.22+ required. Node.js 20+ for UI builds.

## Architecture

Roundtable Kernel (`rtk`) is a multi-LLM deliberation loop. It orchestrates rounds of chair/critic debate until convergence or bounded exhaustion.

### Two Operating Modes

- **Autonomous** (`rtk run`): kernel drives the full session loop internally
- **Live handoff** (`rtk init/next/apply/wait`): external windows (agents, TUIs) claim and complete phases by session ID

### Phase Flow Per Round

```
Seed Evidence → Chair Explore → Chair Proposal → Critic Re-Explore →
Critic Review → {Material Findings?}
  ├─ Yes → Chair Adjudicate → Next Round
  └─ No → Converged
```

### Key Invariant: Durable Truth vs. Observability

- `sessions/<id>.json` — **source of truth**. Semantic state: evidence, proposals, findings, verdicts, round history.
- `telemetry/<id>.jsonl` — **observability only**. Command execution events, timing, streaming chunks. Never read for decisions.

### Adapter Pattern

The kernel is provider-agnostic. Agent invocation is abstracted behind an adapter interface (`internal/rtk/adapters.go`). The only current implementation is `execAdapter` which runs external commands via stdin/stdout using the `roundtable-kernel.exec.v1` protocol.

Provider-specific logic (Claude, GPT) lives exclusively in wrapper binaries under `cmd/claude-agent/` and `cmd/codex-agent/`.

### Live Protocol Token Handoff

`next` claims a phase (marks it "running") and returns a `started_at` timestamp. `apply` must echo that `started_at` to complete the phase — this guards against stale writes from old/retried windows. `wait` uses fsnotify to block until session state changes.

### Domain Rules

- Evidence IDs are runtime-assigned (E1, E2, ...) and immutable once added
- `supported` findings **must** cite evidence IDs; `gap` findings cite **none**
- Material findings (severity high/medium) require a verdict; low findings don't
- Clean critic pass (no material findings) converges without adjudication
- Session statuses: `running`, `converged`, `failed`, `exhausted`

## Code Layout

- `cmd/rtk/` — CLI entry point, command routing, output formatting
- `internal/rtk/` — All kernel logic: orchestration, domain validation, adapters, store, live protocol, telemetry, HTTP server
- `ui/` — Svelte web UI for session inspection (served by `rtk serve`)

### Orchestration Split

- `orchestrator.go` — `RunSession()` top-level autonomous loop
- `orchestrator_round.go` — Per-round logic
- `orchestrator_phase.go` — Per-phase execution
- `live.go` — Live mode commands (init/next/apply) and internal state advancement
- `wait.go` — Blocking wait with fsnotify file watching

## External Dependencies

Intentionally minimal: only `github.com/fsnotify/fsnotify` (file watching) and `golang.org/x/sys`.
