# Roundtable Kernel

`roundtable-kernel` is a clean-room prototype for one idea:

```text
provider is the change axis
deliberation kernel is the stable axis
```

The stable axis is not:

- CLI output
- streaming events
- UI state
- provider auth
- retries

The stable axis is:

- `evidence`
- `finding`
- `verdict`
- `convergence`

## Core law

```text
baseline facts -> challenge -> targeted re-explore -> evidence-backed findings -> verdict -> next baseline
```

Convergence is decided from critic findings, not from the chair's self-report.

## Invariants

- Events are optional observability only. They are not source of truth.
- Evidence IDs are runtime-assigned.
- A finding can be either:
  - `supported`: it must cite one or more evidence IDs
  - `gap`: it explicitly says the current ledger is insufficient, so it cites zero evidence IDs
- A verdict never duplicates severity. Severity stays authoritative on the finding.
- A clean critic pass converges even if no adjudication phase runs.

## Layout

- [`src/domain.js`](/Users/yansir/code/52/roundtable-kernel/src/domain.js): value-level invariants
- [`src/kernel.js`](/Users/yansir/code/52/roundtable-kernel/src/kernel.js): session state machine
- [`src/orchestrator.js`](/Users/yansir/code/52/roundtable-kernel/src/orchestrator.js): bounded round runner
- [`src/adapters/fixture.js`](/Users/yansir/code/52/roundtable-kernel/src/adapters/fixture.js): fixture adapter
- [`src/adapters/exec.js`](/Users/yansir/code/52/roundtable-kernel/src/adapters/exec.js): shell-command adapter for real runtimes
- [`src/store.js`](/Users/yansir/code/52/roundtable-kernel/src/store.js): JSON persistence
- [`src/view.js`](/Users/yansir/code/52/roundtable-kernel/src/view.js): derived read models for UI/API
- [`src/server.js`](/Users/yansir/code/52/roundtable-kernel/src/server.js): HTTP API + static UI server
- [`src/cli.js`](/Users/yansir/code/52/roundtable-kernel/src/cli.js): minimal CLI
- [`fixtures/evidence-ledger.json`](/Users/yansir/code/52/roundtable-kernel/fixtures/evidence-ledger.json): 2-round semantic redesign sample
- [`fixtures/minigit-plan.json`](/Users/yansir/code/52/roundtable-kernel/fixtures/minigit-plan.json): 3-round planning sample
- [`fixtures/minigit-exec.json`](/Users/yansir/code/52/roundtable-kernel/fixtures/minigit-exec.json): exec-adapter sample config
- [`examples/claude-agent.js`](/Users/yansir/code/52/roundtable-kernel/examples/claude-agent.js): Claude CLI semantic wrapper
- [`examples/codex-agent.js`](/Users/yansir/code/52/roundtable-kernel/examples/codex-agent.js): Codex CLI semantic wrapper
- [`examples/runtime-spec.template.json`](/Users/yansir/code/52/roundtable-kernel/examples/runtime-spec.template.json): real-runtime spec template
- [`ui/src/App.svelte`](/Users/yansir/code/52/roundtable-kernel/ui/src/App.svelte): semantic dashboard

## Commands

```bash
cd /Users/yansir/code/52/roundtable-kernel

node src/cli.js demo evidence-ledger --force
node src/cli.js run fixture minigit-plan fixtures/minigit-plan.json --force
node src/cli.js run exec minigit-exec fixtures/minigit-exec.json --force
node src/cli.js show evidence-ledger
node src/cli.js list
npm --prefix ui install
npm run ui:build
npm run serve
```

`demo` is just a shortcut for the bundled `evidence-ledger` fixture. `run fixture` is the generic adapter entrypoint.
`run exec` shells out to a configured command and admits only semantic JSON back into the kernel.

The dashboard defaults to [http://127.0.0.1:3133](http://127.0.0.1:3133) and serves:

- `GET /api/sessions`: derived session summaries from durable JSON
- `GET /api/session/:id`: full session truth for one discussion
- `GET /api/telemetry/:id`: durable runtime telemetry sidecar for one discussion
- `GET /api/telemetry/:id?since=N`: incremental telemetry tail from durable history

Durable state is split on purpose:

- `sessions/<id>.json`: semantic truth only, including `open_round` when a round is still running or has failed before adjudication
- `telemetry/<id>.jsonl`: runtime sidecar only

The UI reads both, but it does not treat stream logs or provider chatter as source of truth.
`open_round.phase_history` is the durable semantic record for in-flight or failed discussion state.

## Exec Adapter

The `exec` adapter is the runtime bridge for real providers. It treats LLM CLIs as replaceable command executors.

The config names one command template plus optional per-actor overrides:

```json
{
  "topic": "...",
  "chair": "opus",
  "critics": ["gpt-5.4", "sonnet"],
  "max_rounds": 3,
  "agent": {
    "cmd": ["node", "examples/mock-agent.js", "fixtures/minigit-plan.json"],
    "cwd": "..",
    "timeout_ms": 10000
  }
}
```

For every phase, the adapter sends one JSON document on stdin:

```json
{
  "protocol": "roundtable-kernel.exec.v1",
  "actor": "sonnet",
  "phase": "review",
  "round": 2,
  "session": { "...": "durable semantic truth so far" },
  "proposal": { "...": "present for review/adjudicate" },
  "findings": [{ "...": "present for adjudicate" }]
}
```

The command must print one JSON document to stdout:

- `explore` / `re-explore`: `{ "items": [...] }`
- `propose`: `{ "proposal": { ... } }`
- `review`: `{ "findings": [{ ... evidence_ids ... }] }`
- `adjudicate`: `{ "verdict": { ... } }`

This keeps the kernel ignorant of providers. Real CLIs can sit behind a thin wrapper that only translates prompts and JSON.

## CLI Wrappers

Two thin wrappers are included:

- [`examples/claude-agent.js`](/Users/yansir/code/52/roundtable-kernel/examples/claude-agent.js): calls `${CLAUDE_BIN:-claude} -p --output-format json --json-schema ...` with isolated settings/MCP defaults and extracts `result.structured_output`
- [`examples/codex-agent.js`](/Users/yansir/code/52/roundtable-kernel/examples/codex-agent.js): calls `codex exec --output-schema ... --output-last-message ...`

Both wrappers:

- read one roundtable request JSON document from stdin
- build a phase-specific prompt plus JSON Schema
- return one semantic JSON document on stdout
- keep provider/runtime details out of the kernel

The Claude wrapper defaults to a minimal runtime surface:

- `--setting-sources ""`
- `--strict-mcp-config`
- `--mcp-config '{"mcpServers":{}}'`
- `--disable-slash-commands`

This removes ambient user/project MCP and plugin drift from roundtable runs while still allowing an explicit `--settings` file.

Claude authentication can be injected entirely through environment variables. The exec adapter already merges the parent shell environment with optional `agent.env` / `actors.<name>.env` overrides, so you do not need to hardcode credentials in the repository. Prefer launching runs like this:

```bash
ANTHROPIC_BASE_URL="https://your-relay.example" \
ANTHROPIC_AUTH_TOKEN="..." \
node src/cli.js run exec my-session /absolute/path/to/spec.json --force
```

If you need a different binary, set `CLAUDE_BIN=ccc` or pass `--bin ccc` to the wrapper. Telemetry records only environment key names, never credential values.

Runtime execution remains observable through telemetry events such as:

- `session_started` / `session_finished`
- `phase_started` / `phase_succeeded` / `phase_failed`
- `command_started` / `command_finished` / `command_failed`

Each command event records the sanitized argv, cwd, env key names, duration, exit code, and clipped stdout/stderr excerpts.

The fastest way to wire a real discussion is to copy [`examples/runtime-spec.template.json`](/Users/yansir/code/52/roundtable-kernel/examples/runtime-spec.template.json), replace `__TARGET_REPO__` and `__CLAUDE_SETTINGS__`, then run:

```bash
node src/cli.js run exec my-session /absolute/path/to/spec.json --force
```

## Fixture Shape

The fixture adapter is explicit about who gathered evidence and in which phase:

```json
{
  "seed_batch": {
    "actor": "chair",
    "items": [{ "key": "..." }]
  },
  "rounds": [
    {
      "evidence_batches": [
        {
          "phase": "re-explore",
          "actor": "sonnet",
          "items": [{ "key": "..." }]
        }
      ],
      "proposal": { "summary": "..." },
      "findings_against_proposal": [],
      "verdict": null
    }
  ]
}
```

This is intentional. The adapter should not guess which actor produced a piece of evidence from incidental fields like `collected_by`.

## What This Prototype Deliberately Omits

- no provider integrations
- no transport retries
- no stream-driven UI truth
- no stream parsing
- no workflow runner

Those belong outside the kernel. The next layer should adapt providers into this state machine, not leak their runtime behavior into it.
