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

- [`src/domain.js`](/private/tmp/roundtable-kernel/src/domain.js): value-level invariants
- [`src/kernel.js`](/private/tmp/roundtable-kernel/src/kernel.js): session state machine
- [`src/orchestrator.js`](/private/tmp/roundtable-kernel/src/orchestrator.js): bounded round runner
- [`src/adapters/fixture.js`](/private/tmp/roundtable-kernel/src/adapters/fixture.js): fixture adapter
- [`src/adapters/exec.js`](/private/tmp/roundtable-kernel/src/adapters/exec.js): shell-command adapter for real runtimes
- [`src/store.js`](/private/tmp/roundtable-kernel/src/store.js): JSON persistence
- [`src/view.js`](/private/tmp/roundtable-kernel/src/view.js): derived read models for UI/API
- [`src/server.js`](/private/tmp/roundtable-kernel/src/server.js): HTTP API + static UI server
- [`src/cli.js`](/private/tmp/roundtable-kernel/src/cli.js): minimal CLI
- [`fixtures/evidence-ledger.json`](/private/tmp/roundtable-kernel/fixtures/evidence-ledger.json): 2-round semantic redesign sample
- [`fixtures/minigit-plan.json`](/private/tmp/roundtable-kernel/fixtures/minigit-plan.json): 3-round planning sample
- [`fixtures/minigit-exec.json`](/private/tmp/roundtable-kernel/fixtures/minigit-exec.json): exec-adapter sample config
- [`ui/src/App.svelte`](/private/tmp/roundtable-kernel/ui/src/App.svelte): semantic dashboard

## Commands

```bash
cd /private/tmp/roundtable-kernel

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

The UI intentionally does not read stream logs or provider telemetry.

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
      "findings": [],
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
