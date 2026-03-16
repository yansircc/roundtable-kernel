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

function currentRoundFindings(roundRecord) {
  return Array.isArray(roundRecord?.findings_against_proposal)
    ? roundRecord.findings_against_proposal
    : Array.isArray(roundRecord?.findings)
      ? roundRecord.findings
      : [];
}

function ensureOpenRound(session, round) {
  invariant(Number.isInteger(round) && round > 0, "open round index must be a positive integer");
  if (!session.open_round) {
    session.open_round = {
      index: round,
      proposal: null,
      evidence_added: [],
      findings_against_proposal: [],
      review_summary: { total: 0, high: 0, medium: 0, low: 0, material: 0, gaps: 0 },
      verdict: null,
      phase_history: [],
      created_at: nowIso(),
      updated_at: nowIso(),
      error: null,
    };
  }
  invariant(session.open_round.index === round, `session already has open round ${session.open_round.index}`);
  return session.open_round;
}

function refreshOpenRoundSummary(session) {
  const roundRecord = session.open_round;
  if (!roundRecord) {
    return;
  }
  const counts = findingCounts(currentRoundFindings(roundRecord));
  roundRecord.review_summary = counts;
  roundRecord.updated_at = nowIso();
  session.status = {
    ...session.status,
    round: roundRecord.index,
    unresolved_high: counts.high,
    unresolved_medium: counts.medium,
  };
}

function roundPhaseIndex(roundRecord, actor, phase, status = "running") {
  for (let index = roundRecord.phase_history.length - 1; index >= 0; index -= 1) {
    const item = roundRecord.phase_history[index];
    if (item.actor === actor && item.phase === phase && item.status === status) {
      return index;
    }
  }
  return -1;
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
    open_round: null,
    adjudicated_proposal: null,
    status: {
      round: 0,
      converged: false,
      unresolved_high: 0,
      unresolved_medium: 0,
      state: "initialized",
      active_actor: null,
      active_phase: null,
      error: null,
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

function startRound(session, round) {
  const roundRecord = ensureOpenRound(session, round);
  session.status = {
    ...session.status,
    round,
    converged: false,
    unresolved_high: roundRecord.review_summary.high || 0,
    unresolved_medium: roundRecord.review_summary.medium || 0,
    state: "running",
    active_actor: null,
    active_phase: null,
    error: null,
  };
  roundRecord.updated_at = nowIso();
  return roundRecord;
}

function recordPhaseStart(session, { round, actor, phase, input_summary }) {
  const roundRecord = startRound(session, round);
  roundRecord.phase_history.push({
    actor,
    phase,
    status: "running",
    input_summary: input_summary || null,
    output_summary: null,
    artifact: null,
    started_at: nowIso(),
    completed_at: null,
    duration_ms: null,
    error: null,
  });
  roundRecord.updated_at = nowIso();
  session.status = {
    ...session.status,
    active_actor: actor,
    active_phase: phase,
    error: null,
  };
  return roundRecord;
}

function completePhase(session, { round, actor, phase, output_summary, artifact = null, duration_ms, error = null }) {
  const roundRecord = ensureOpenRound(session, round);
  const index = roundPhaseIndex(roundRecord, actor, phase);
  invariant(index >= 0, `no running phase record for ${actor}/${phase} in round ${round}`);
  const nextStatus = error ? "failed" : "succeeded";
  const completedAt = nowIso();
  roundRecord.phase_history[index] = {
    ...roundRecord.phase_history[index],
    status: nextStatus,
    output_summary: output_summary || null,
    artifact,
    completed_at: completedAt,
    duration_ms: duration_ms ?? null,
    error: error || null,
  };
  roundRecord.updated_at = completedAt;
  session.status = {
    ...session.status,
    active_actor: error ? actor : null,
    active_phase: error ? phase : null,
    error: error
      ? {
          message: error.message || String(error),
          at: completedAt,
        }
      : null,
  };
}

function noteRoundEvidence(session, { round, evidence_added }) {
  const roundRecord = ensureOpenRound(session, round);
  invariant(Array.isArray(evidence_added), "evidence_added must be an array");
  const seen = new Set(roundRecord.evidence_added);
  for (const evidenceId of evidence_added) {
    invariant(typeof evidenceId === "string" && evidenceId.trim().length > 0, "evidence_added[] must be non-empty strings");
    if (!seen.has(evidenceId)) {
      roundRecord.evidence_added.push(evidenceId);
      seen.add(evidenceId);
    }
  }
  roundRecord.updated_at = nowIso();
}

function registerProposal(session, { round, proposal }) {
  validateProposal(proposal);
  const roundRecord = ensureOpenRound(session, round);
  roundRecord.proposal = proposal;
  roundRecord.updated_at = nowIso();
}

function appendRoundFindings(session, { round, findings }) {
  const roundRecord = ensureOpenRound(session, round);
  invariant(Array.isArray(findings), "findings must be an array");
  const evidence = evidenceIndex(session);
  const seenFindingIds = new Set(currentRoundFindings(roundRecord).map((item) => item.id));
  const nextFindings = findings.map((finding, index) => {
    const next = { ...finding };
    validateFinding(next, evidence, `open_round.findings_against_proposal[${index}]`);
    invariant(!seenFindingIds.has(next.id), `duplicate finding id ${next.id}`);
    seenFindingIds.add(next.id);
    return next;
  });
  roundRecord.findings_against_proposal.push(...nextFindings);
  refreshOpenRoundSummary(session);
}

function registerVerdict(session, { round, verdict }) {
  const roundRecord = ensureOpenRound(session, round);
  if (verdict === null) {
    roundRecord.verdict = null;
    roundRecord.updated_at = nowIso();
    return;
  }
  invariant(verdict && typeof verdict === "object", "verdict must be an object");
  invariant(Array.isArray(verdict.decisions), "verdict.decisions must be an array");
  if (verdict.revised_proposal !== undefined && verdict.revised_proposal !== null) {
    validateProposal(verdict.revised_proposal, "verdict.revised_proposal");
  }
  const findings = currentRoundFindings(roundRecord);
  const findingsById = findingIndex(findings);
  const evidence = evidenceIndex(session);
  invariant(verdict.decisions.length === findings.length, "verdict must contain exactly one decision for every finding");
  const seenDecisionIds = new Set();
  verdict.decisions.forEach((decision, index) => {
    validateDecision(decision, findingsById, evidence, `open_round.verdict.decisions[${index}]`);
    invariant(!seenDecisionIds.has(decision.finding_id), `duplicate decision for finding ${decision.finding_id}`);
    seenDecisionIds.add(decision.finding_id);
  });
  roundRecord.verdict = verdict;
  roundRecord.updated_at = nowIso();
}

function markSessionFailed(session, { round, actor, phase, error }) {
  const message = error?.message || String(error);
  session.status = {
    ...session.status,
    round: round || session.open_round?.index || session.status.round,
    state: "failed",
    active_actor: actor || session.status.active_actor,
    active_phase: phase || session.status.active_phase,
    error: {
      message,
      at: nowIso(),
    },
  };
  if (session.open_round) {
    session.open_round.error = {
      message,
      actor: actor || null,
      phase: phase || null,
      at: nowIso(),
    };
    session.open_round.updated_at = nowIso();
  }
}

function applyRound(session) {
  const roundRecord = session.open_round;
  invariant(roundRecord, "session has no open round to apply");
  validateProposal(roundRecord.proposal);
  invariant(Array.isArray(roundRecord.evidence_added), "open_round.evidence_added must be an array");

  const findings = currentRoundFindings(roundRecord).map((finding) => ({ ...finding }));
  const counts = findingCounts(findings);
  const material = findings.filter(isMaterialFinding);
  const verdict = roundRecord.verdict;

  if (verdict) {
    invariant(verdict && typeof verdict === "object", "verdict must be an object");
    invariant(Array.isArray(verdict.decisions), "verdict.decisions must be an array");
  } else {
    invariant(material.length === 0, "material findings require a verdict before the round can close");
  }

  const closedRound = {
    index: roundRecord.index,
    proposal: roundRecord.proposal,
    evidence_added: [...roundRecord.evidence_added],
    findings_against_proposal: findings,
    review_summary: counts,
    verdict,
    phase_history: [...roundRecord.phase_history],
    created_at: roundRecord.created_at,
    updated_at: nowIso(),
  };

  session.rounds.push(closedRound);
  session.open_round = null;
  session.adjudicated_proposal = verdict?.revised_proposal || roundRecord.proposal;
  session.status = {
    ...session.status,
    round: closedRound.index,
    converged: counts.material === 0,
    unresolved_high: counts.high,
    unresolved_medium: counts.medium,
    state: counts.material === 0 ? "converged" : "needs_revision",
    active_actor: null,
    active_phase: null,
    error: null,
  };

  return closedRound;
}

module.exports = {
  appendEvidence,
  applyRound,
  appendRoundFindings,
  completePhase,
  createSession,
  markSessionFailed,
  noteRoundEvidence,
  recordPhaseStart,
  registerProposal,
  registerVerdict,
};
