---
name: rtk
description: Use when you need to run, inspect, or participate in roundtable-kernel sessions through the `rtk` CLI. Covers autonomous runs, live chair/critic handoff, durable session truth, and common commands such as `help`, `show`, `wait`, `next`, and `apply`.
---

# RTK

Use this skill when the task is about operating Roundtable Kernel rather than editing its internals.

Start with `rtk -h` or `rtk help <command>` if the exact subcommand is unclear.

## Mental Model

- `sessions/<id>.json` is source of truth.
- `telemetry/<id>.jsonl` is observability only.
- `run` is autonomous mode.
- `init` / `next` / `apply` / `wait` is live mode.
- `next` is the only command that claims a step.
- `apply` should echo the `started_at` token returned by `next`.

Do not infer semantic state from stream logs when `show`, `wait`, or the session file can answer the question directly.

## Common Workflows

Autonomous:

```bash
rtk run my-session /absolute/path/to/spec.json --force
rtk show my-session
```

Live chair / critic:

```bash
rtk init my-session /absolute/path/to/spec.json --force
rtk next my-session --actor chair
rtk apply my-session result.json
rtk wait my-session --until turn --actor critic
```

Useful waits:

```bash
rtk wait my-session --until turn --actor critic
rtk wait my-session --until turn --actor chair
rtk wait my-session --until terminal
rtk wait my-session --until change --since 2026-03-17T04:07:07.708Z
```

## Operating Rules

- Use `rtk show <session>` when you need the durable session state.
- Use `rtk wait` instead of ad hoc polling loops.
- Use `rtk list` to discover existing sessions.
- Use `rtk serve --port 3133` when a human needs the web UI.
- If `rtk` is not on `PATH`, build it with `make build` or install it with `make install`.

## When Not To Use

- Do not use this skill just to modify roundtable-kernel source code.
- Do not read telemetry as if it were authoritative state.
