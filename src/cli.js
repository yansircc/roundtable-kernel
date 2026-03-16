#!/usr/bin/env node
const path = require("node:path");
const { listSessions, loadSession, projectRoot, sessionPath } = require("./store");
const { runSession } = require("./orchestrator");

function fail(message) {
  console.error(message);
  process.exit(1);
}

function usage() {
  console.log(`usage:
  node src/cli.js demo <session-id> [--force]
  node src/cli.js run fixture <session-id> <fixture-path> [--force]
  node src/cli.js run exec <session-id> <spec-path> [--force]
  node src/cli.js show <session-id> [--json]
  node src/cli.js list`);
}

function extractFlag(args, flag) {
  const next = [];
  let enabled = false;
  for (const arg of args) {
    if (arg === flag) {
      enabled = true;
      continue;
    }
    next.push(arg);
  }
  return { args: next, enabled };
}

function renderSession(session) {
  const roundFindings = (round) => round.findings_against_proposal || round.findings || [];
  const lines = [];
  lines.push(`session: ${session.id}`);
  lines.push(`topic:   ${session.topic}`);
  lines.push(`chair:   ${session.chair}`);
  lines.push(`critics: ${session.critics.join(", ") || "none"}`);
  lines.push(`adapter: ${session.adapter}`);
  lines.push(`status:  ${session.status.state}`);
  lines.push(`round:   ${session.status.round}/${session.max_rounds}`);
  lines.push(`evidence:${String(session.evidence.length).padStart(4, " ")}`);
  lines.push(`high:    ${session.status.unresolved_high}`);
  lines.push(`medium:  ${session.status.unresolved_medium}`);
  if (session.status.active_actor && session.status.active_phase) {
    lines.push(`active:  ${session.status.active_actor}/${session.status.active_phase}`);
  }
  if (session.status.error?.message) {
    lines.push(`error:   ${session.status.error.message}`);
  }
  lines.push("");
  lines.push("adjudicated proposal:");
  lines.push(`  ${session.adjudicated_proposal?.summary || "none"}`);
  lines.push("");

  session.rounds.forEach((round) => {
    lines.push(`round ${round.index}`);
    lines.push(`  proposal: ${round.proposal.summary}`);
    lines.push(`  evidence added: ${round.evidence_added.length}`);
    lines.push(
      `  findings: total=${round.review_summary.total} high=${round.review_summary.high} medium=${round.review_summary.medium} low=${round.review_summary.low} gaps=${round.review_summary.gaps}`,
    );
    if (round.verdict) {
      lines.push(`  verdict:  ${round.verdict.summary}`);
      lines.push(
        `  decisions:${round.verdict.decisions.filter((item) => item.disposition === "accept").length} accepted / ${round.verdict.decisions.filter((item) => item.disposition === "reject").length} rejected`,
      );
    } else {
      lines.push("  verdict:  skipped");
    }
    lines.push("");
  });

  if (session.open_round) {
    const round = session.open_round;
    const findings = roundFindings(round);
    lines.push(`open round ${round.index}`);
    lines.push(`  proposal: ${round.proposal?.summary || "none"}`);
    lines.push(`  evidence added: ${round.evidence_added.length}`);
    lines.push(
      `  findings: total=${round.review_summary?.total || findings.length} high=${round.review_summary?.high || 0} medium=${round.review_summary?.medium || 0} low=${round.review_summary?.low || 0} gaps=${round.review_summary?.gaps || 0}`,
    );
    lines.push(`  phases:   ${round.phase_history?.length || 0}`);
    if (round.error?.message) {
      lines.push(`  error:    ${round.error.message}`);
    }
    lines.push("");
  }

  return lines.join("\n");
}

async function main(argv) {
  const [command, ...rest] = argv;
  if (!command) {
    usage();
    return;
  }

  if (command === "list") {
    const sessions = listSessions();
    if (!sessions.length) {
      console.log("no sessions");
      return;
    }
    sessions.forEach((sessionId) => console.log(sessionId));
    return;
  }

  if (command === "show") {
    const { args, enabled: asJson } = extractFlag(rest, "--json");
    const sessionId = args[0];
    if (!sessionId) {
      fail("show requires a session id");
    }
    const session = loadSession(sessionId);
    if (asJson) {
      console.log(JSON.stringify(session, null, 2));
      return;
    }
    console.log(renderSession(session));
    return;
  }

  if (command === "demo") {
    const { args, enabled: force } = extractFlag(rest, "--force");
    const sessionId = args[0];
    if (!sessionId) {
      fail("demo requires a session id");
    }
    const fixturePath = path.join(projectRoot, "fixtures", "evidence-ledger.json");
    const session = await runSession({
      adapterKind: "fixture",
      adapterConfig: { fixturePath },
      sessionId,
      force,
    });
    console.log(`wrote ${sessionPath(session.id)}`);
    console.log(renderSession(session));
    return;
  }

  if (command === "run") {
    const { args, enabled: force } = extractFlag(rest, "--force");
    const adapterKind = args[0];
    const sessionId = args[1];
    const inputPath = args[2];
    if (!["fixture", "exec"].includes(adapterKind)) {
      fail("run requires adapter kind fixture|exec");
    }
    if (!sessionId) {
      fail(`run ${adapterKind || "<adapter>"} requires a session id`);
    }
    if (!inputPath) {
      fail(`run ${adapterKind} requires an input path`);
    }
    const adapterConfig =
      adapterKind === "fixture"
        ? { fixturePath: path.resolve(inputPath) }
        : { specPath: path.resolve(inputPath) };
    const session = await runSession({
      adapterKind,
      adapterConfig,
      sessionId,
      force,
    });
    console.log(`wrote ${sessionPath(session.id)}`);
    console.log(renderSession(session));
    return;
  }

  usage();
  process.exitCode = 1;
}

main(process.argv.slice(2)).catch((error) => {
  fail(error.stack || error.message);
});
