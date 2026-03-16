#!/usr/bin/env node
const fs = require("node:fs");
const path = require("node:path");

function readJson(file) {
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

function readStdin() {
  return new Promise((resolve, reject) => {
    let input = "";
    process.stdin.setEncoding("utf8");
    process.stdin.on("data", (chunk) => {
      input += chunk;
    });
    process.stdin.on("end", () => {
      try {
        resolve(JSON.parse(input));
      } catch (error) {
        reject(error);
      }
    });
    process.stdin.on("error", reject);
  });
}

function allEvidenceEntries(fixture) {
  const entries = [];
  if (fixture.seed_batch?.items) {
    for (const item of fixture.seed_batch.items) {
      entries.push({ ...item, phase: "seed" });
    }
  }
  for (const round of fixture.rounds || []) {
    for (const batch of round.evidence_batches || []) {
      for (const item of batch.items || []) {
        entries.push({ ...item, phase: batch.phase });
      }
    }
  }
  return entries;
}

function evidenceIdByKey(session, evidenceIndex, key) {
  const target = evidenceIndex.get(key);
  if (!target) {
    throw new Error(`unknown scenario evidence key ${key}`);
  }
  const match = (session.evidence || []).find(
    (item) =>
      item.source === target.source &&
      item.statement === target.statement &&
      item.phase === target.phase,
  );
  if (!match) {
    throw new Error(`session does not yet contain evidence for key ${key}`);
  }
  return match.id;
}

function mapFinding(finding, session, evidenceIndex) {
  return {
    id: finding.id,
    critic: finding.critic,
    severity: finding.severity,
    basis: finding.basis,
    summary: finding.summary,
    rationale: finding.rationale,
    suggested_change: finding.suggested_change,
    evidence_ids: (finding.evidence_keys || []).map((key) => evidenceIdByKey(session, evidenceIndex, key)),
  };
}

function mapVerdict(verdict, session, evidenceIndex) {
  if (!verdict) {
    return null;
  }
  return {
    summary: verdict.summary,
    revised_proposal: verdict.revised_proposal || null,
    decisions: (verdict.decisions || []).map((decision) => ({
      finding_id: decision.finding_id,
      disposition: decision.disposition,
      rationale: decision.rationale,
      evidence_ids: (decision.evidence_keys || []).map((key) => evidenceIdByKey(session, evidenceIndex, key)),
    })),
  };
}

async function main() {
  const scenarioPath = path.resolve(process.argv[2]);
  const fixture = readJson(scenarioPath);
  const request = await readStdin();
  const round = fixture.rounds[(request.round || 1) - 1];
  const evidenceIndex = new Map(allEvidenceEntries(fixture).map((item) => [item.key, item]));

  if (!round) {
    throw new Error(`scenario has no round ${request.round}`);
  }

  if (request.phase === "explore" || request.phase === "re-explore") {
    const items = (round.evidence_batches || [])
      .filter((batch) => batch.phase === request.phase && batch.actor === request.actor)
      .flatMap((batch) => batch.items || []);
    process.stdout.write(
      `${JSON.stringify({ items, collected_by: request.actor })}\n`,
    );
    return;
  }

  if (request.phase === "propose") {
    process.stdout.write(`${JSON.stringify({ proposal: round.proposal })}\n`);
    return;
  }

  if (request.phase === "review") {
    const findings = (round.findings || [])
      .filter((finding) => finding.critic === request.actor)
      .map((finding) => mapFinding(finding, request.session, evidenceIndex));
    process.stdout.write(`${JSON.stringify({ findings })}\n`);
    return;
  }

  if (request.phase === "adjudicate") {
    const verdict = mapVerdict(round.verdict, request.session, evidenceIndex);
    process.stdout.write(`${JSON.stringify({ verdict })}\n`);
    return;
  }

  throw new Error(`unsupported phase ${request.phase}`);
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error.message}\n`);
  process.exit(1);
});
