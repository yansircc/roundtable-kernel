const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

function parseArgs(argv) {
  const args = { positionals: [] };
  for (let index = 0; index < argv.length; index += 1) {
    const arg = argv[index];
    if (!arg.startsWith("--")) {
      args.positionals.push(arg);
      continue;
    }
    const key = arg.slice(2);
    const next = argv[index + 1];
    if (next === undefined || next.startsWith("--")) {
      args[key] = true;
      continue;
    }
    args[key] = next;
    index += 1;
  }
  return args;
}

function readJsonStdin() {
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

function ensure(value, message) {
  if (!value) {
    throw new Error(message);
  }
}

function compactSession(session) {
  const roundFindings = (round) => round?.findings_against_proposal || round?.findings || [];
  return {
    id: session.id,
    topic: session.topic,
    chair: session.chair,
    critics: session.critics,
    max_rounds: session.max_rounds,
    status: session.status,
    adjudicated_proposal: session.adjudicated_proposal,
    evidence: (session.evidence || []).map((item) => ({
      id: item.id,
      phase: item.phase,
      round: item.round,
      source: item.source,
      statement: item.statement,
    })),
    rounds: (session.rounds || []).map((round) => ({
      index: round.index,
      proposal: round.proposal?.summary || "",
      findings_against_proposal: roundFindings(round).map((finding) => ({
        id: finding.id,
        critic: finding.critic,
        severity: finding.severity,
        basis: finding.basis,
        summary: finding.summary,
        evidence_ids: finding.evidence_ids,
      })),
      verdict: round.verdict?.summary || null,
    })),
    open_round: session.open_round
      ? {
          index: session.open_round.index,
          proposal: session.open_round.proposal?.summary || "",
          findings_against_proposal: roundFindings(session.open_round).map((finding) => ({
            id: finding.id,
            critic: finding.critic,
            severity: finding.severity,
            basis: finding.basis,
            summary: finding.summary,
            evidence_ids: finding.evidence_ids,
          })),
          verdict: session.open_round.verdict?.summary || null,
          phase_history: (session.open_round.phase_history || []).map((phase) => ({
            actor: phase.actor,
            phase: phase.phase,
            status: phase.status,
            output_summary: phase.output_summary,
          })),
          error: session.open_round.error || null,
        }
      : null,
  };
}

function outputSchemaForPhase(phase) {
  const evidenceItem = {
    type: "object",
    additionalProperties: false,
    properties: {
      source: { type: "string" },
      kind: { type: "string" },
      statement: { type: "string" },
      excerpt: { type: "string" },
    },
    required: ["source", "kind", "statement", "excerpt"],
  };

  const proposal = {
    type: "object",
    additionalProperties: false,
    properties: {
      summary: { type: "string" },
      claims: {
        type: "array",
        items: { type: "string" },
      },
      acceptance: {
        type: "array",
        items: { type: "string" },
      },
    },
    required: ["summary", "claims", "acceptance"],
  };

  const finding = {
    type: "object",
    additionalProperties: false,
    properties: {
      id: { type: "string" },
      critic: { type: "string" },
      severity: { type: "string", enum: ["high", "medium", "low"] },
      basis: { type: "string", enum: ["supported", "gap"] },
      summary: { type: "string" },
      rationale: { type: "string" },
      suggested_change: { type: "string" },
      evidence_ids: {
        type: "array",
        items: { type: "string" },
      },
    },
    required: ["id", "critic", "severity", "basis", "summary", "rationale", "suggested_change", "evidence_ids"],
  };

  const verdict = {
    type: "object",
    additionalProperties: false,
    properties: {
      summary: { type: "string" },
      revised_proposal: {
        anyOf: [{ type: "null" }, proposal],
      },
      decisions: {
        type: "array",
        items: {
          type: "object",
          additionalProperties: false,
          properties: {
            finding_id: { type: "string" },
            disposition: { type: "string", enum: ["accept", "reject"] },
            rationale: { type: "string" },
            evidence_ids: {
              type: "array",
              items: { type: "string" },
            },
          },
          required: ["finding_id", "disposition", "rationale", "evidence_ids"],
        },
      },
    },
    required: ["summary", "revised_proposal", "decisions"],
  };

  if (phase === "explore" || phase === "re-explore") {
    return {
      type: "object",
      additionalProperties: false,
      properties: {
        items: {
          type: "array",
          items: evidenceItem,
        },
      },
      required: ["items"],
    };
  }

  if (phase === "propose") {
    return {
      type: "object",
      additionalProperties: false,
      properties: {
        proposal,
      },
      required: ["proposal"],
    };
  }

  if (phase === "review") {
    return {
      type: "object",
      additionalProperties: false,
      properties: {
        findings: {
          type: "array",
          items: finding,
        },
      },
      required: ["findings"],
    };
  }

  if (phase === "adjudicate") {
    return {
      type: "object",
      additionalProperties: false,
      properties: {
        verdict: {
          anyOf: [{ type: "null" }, verdict],
        },
      },
      required: ["verdict"],
    };
  }

  throw new Error(`unsupported phase ${phase}`);
}

function promptForRequest(request) {
  const phase = request.phase;
  const actor = request.actor;
  const context = {
    protocol: request.protocol,
    actor,
    phase,
    round: request.round,
    session: compactSession(request.session),
    proposal: request.proposal || null,
    findings: request.findings || [],
  };

  const sharedRules = [
    "You are participating in a local roundtable.",
    "Return only JSON that matches the provided schema.",
    "Do not use markdown fences or explanatory prose.",
    "Prefer derivation over enumeration.",
    "Never invent evidence IDs.",
    "A supported finding or decision must cite evidence_ids from session.evidence.",
    "A gap finding must use basis='gap' and evidence_ids=[].",
    "Severity belongs on findings, not on verdict decisions.",
  ];

  let task;
  if (phase === "explore") {
    task = [
      "Inspect the workspace read-only and gather baseline evidence relevant to the topic.",
      "Return up to 5 evidence items.",
      "If nothing new matters, return {\"items\":[]}.",
    ];
  } else if (phase === "re-explore") {
    task = [
      "Challenge the current direction and gather only targeted evidence that may change the proposal.",
      "Return up to 5 evidence items.",
      "If the ledger is already sufficient, return {\"items\":[]}.",
    ];
  } else if (phase === "propose") {
    task = [
      "Produce the best current proposal.",
      "Keep it compressed: one summary, a few core claims, a few acceptance criteria.",
      "Do not mention evidence IDs in the proposal object.",
    ];
  } else if (phase === "review") {
    task = [
      `Review the current proposal as critic ${actor}.`,
      `Use finding ids of the form "${actor}:F1", "${actor}:F2", ...`,
      "Report only real findings against the proposal.",
      "If there are no findings, return {\"findings\":[]}.",
    ];
  } else if (phase === "adjudicate") {
    task = [
      "Adjudicate the current findings.",
      "Return exactly one decision per finding.",
      "If accepted findings change the plan, revise the proposal. Otherwise you may keep revised_proposal null.",
      "If there are no findings, return {\"verdict\":null}.",
    ];
  } else {
    throw new Error(`unsupported phase ${phase}`);
  }

  return [
    ...sharedRules,
    "",
    "Task:",
    ...task.map((line) => `- ${line}`),
    "",
    "Context JSON:",
    JSON.stringify(context, null, 2),
  ].join("\n");
}

function writeTempSchema(schema) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "roundtable-schema-"));
  const file = path.join(dir, "schema.json");
  fs.writeFileSync(file, JSON.stringify(schema, null, 2));
  return {
    file,
    cleanup() {
      try {
        fs.unlinkSync(file);
      } catch {}
      try {
        fs.rmdirSync(dir);
      } catch {}
    },
  };
}

function printJson(value) {
  process.stdout.write(`${JSON.stringify(value)}\n`);
}

module.exports = {
  ensure,
  outputSchemaForPhase,
  parseArgs,
  printJson,
  promptForRequest,
  readJsonStdin,
  writeTempSchema,
};
