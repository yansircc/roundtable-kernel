const {
  findingCounts,
  invariant,
  isMaterialFinding,
  validateDecision,
  validateEvidence,
  validateFinding,
  validateProposal,
} = require("./domain");

function nowIso() {
  return new Date().toISOString();
}

function makeEvidenceId(session) {
  return `E${session.evidence.length + 1}`;
}

function evidenceIndex(session) {
  return new Map(session.evidence.map((item) => [item.id, item]));
}

function findingIndex(findings) {
  return new Map(findings.map((item) => [item.id, item]));
}

function createSession({ id, topic, chair = "chair", critics = [], max_rounds = 3, adapter = "unknown" }) {
  invariant(typeof id === "string" && /^[A-Za-z0-9._-]+$/.test(id), "session id must match [A-Za-z0-9._-]+");
  invariant(typeof topic === "string" && topic.trim().length > 0, "topic must be a non-empty string");
  invariant(typeof chair === "string" && chair.trim().length > 0, "chair must be a non-empty string");
  invariant(Array.isArray(critics), "critics must be an array");
  invariant(Number.isInteger(max_rounds) && max_rounds > 0, "max_rounds must be a positive integer");
  invariant(typeof adapter === "string" && adapter.trim().length > 0, "adapter must be a non-empty string");

  return {
    version: 1,
    id,
    topic,
    created_at: nowIso(),
    chair,
    critics: [...critics],
    max_rounds,
    adapter,
    evidence: [],
    rounds: [],
    adjudicated_proposal: null,
    status: {
      round: 0,
      converged: false,
      unresolved_high: 0,
      unresolved_medium: 0,
      state: "initialized",
    },
  };
}

function appendEvidence(session, { items, collectedBy, phase, round }) {
  invariant(Array.isArray(items), "items must be an array");
  invariant(typeof collectedBy === "string" && collectedBy.trim().length > 0, "collectedBy must be a non-empty string");
  invariant(typeof phase === "string" && phase.trim().length > 0, "phase must be a non-empty string");
  invariant(Number.isInteger(round) && round >= 0, "round must be a non-negative integer");

  const added = [];
  for (const item of items) {
    invariant(item && typeof item === "object", "evidence input item must be an object");
    const evidence = {
      id: makeEvidenceId(session),
      source: item.source,
      kind: item.kind,
      phase,
      statement: item.statement,
      excerpt: item.excerpt,
      collected_by: collectedBy,
      round,
      created_at: nowIso(),
    };
    validateEvidence(evidence);
    session.evidence.push(evidence);
    added.push(evidence);
  }
  return added;
}

function applyRound(session, { proposal, findings, verdict = null, evidence_added = [] }) {
  validateProposal(proposal);
  invariant(Array.isArray(findings), "findings must be an array");
  invariant(Array.isArray(evidence_added), "evidence_added must be an array");

  const round = session.rounds.length + 1;
  const evidence = evidenceIndex(session);
  const seenFindingIds = new Set();
  const normalizedFindings = findings.map((finding, index) => {
    const next = { ...finding };
    validateFinding(next, evidence, `rounds[${round}].findings[${index}]`);
    invariant(!seenFindingIds.has(next.id), `duplicate finding id ${next.id}`);
    seenFindingIds.add(next.id);
    return next;
  });

  const counts = findingCounts(normalizedFindings);
  const material = normalizedFindings.filter(isMaterialFinding);

  if (verdict) {
    invariant(verdict && typeof verdict === "object", "verdict must be an object");
    invariant(Array.isArray(verdict.decisions), "verdict.decisions must be an array");
    if (verdict.revised_proposal !== undefined && verdict.revised_proposal !== null) {
      validateProposal(verdict.revised_proposal, "verdict.revised_proposal");
    }
    const findingsById = findingIndex(normalizedFindings);
    invariant(
      verdict.decisions.length === normalizedFindings.length,
      "verdict must contain exactly one decision for every finding in the round",
    );
    const seenDecisionIds = new Set();
    verdict.decisions.forEach((decision, index) => {
      validateDecision(decision, findingsById, evidence, `rounds[${round}].verdict.decisions[${index}]`);
      invariant(!seenDecisionIds.has(decision.finding_id), `duplicate decision for finding ${decision.finding_id}`);
      seenDecisionIds.add(decision.finding_id);
    });
  } else {
    invariant(material.length === 0, "material findings require a verdict before the round can close");
  }

  const roundRecord = {
    index: round,
    proposal,
    evidence_added,
    findings: normalizedFindings,
    review_summary: counts,
    verdict,
    created_at: nowIso(),
  };

  session.rounds.push(roundRecord);
  session.adjudicated_proposal = verdict?.revised_proposal || proposal;
  session.status = {
    round,
    converged: counts.material === 0,
    unresolved_high: counts.high,
    unresolved_medium: counts.medium,
    state: counts.material === 0 ? "converged" : "needs_revision",
  };

  return roundRecord;
}

module.exports = {
  appendEvidence,
  applyRound,
  createSession,
};
