const MATERIAL_SEVERITIES = new Set(["high", "medium"]);
const ALL_SEVERITIES = new Set(["high", "medium", "low"]);
const FINDING_BASIS = new Set(["supported", "gap"]);
const DECISIONS = new Set(["accept", "reject"]);

function invariant(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

function isPlainObject(value) {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function validateString(value, path) {
  invariant(typeof value === "string" && value.trim().length > 0, `${path} must be a non-empty string`);
}

function validateArray(value, path) {
  invariant(Array.isArray(value), `${path} must be an array`);
}

function validateProposal(proposal, path = "proposal") {
  invariant(isPlainObject(proposal), `${path} must be an object`);
  validateString(proposal.summary, `${path}.summary`);
  invariant(
    proposal.claims === undefined || Array.isArray(proposal.claims),
    `${path}.claims must be an array when present`,
  );
  invariant(
    proposal.acceptance === undefined || Array.isArray(proposal.acceptance),
    `${path}.acceptance must be an array when present`,
  );
  return proposal;
}

function validateEvidence(evidence, path = "evidence") {
  invariant(isPlainObject(evidence), `${path} must be an object`);
  validateString(evidence.id, `${path}.id`);
  validateString(evidence.source, `${path}.source`);
  validateString(evidence.kind, `${path}.kind`);
  validateString(evidence.phase, `${path}.phase`);
  validateString(evidence.statement, `${path}.statement`);
  validateString(evidence.excerpt, `${path}.excerpt`);
  validateString(evidence.collected_by, `${path}.collected_by`);
  invariant(Number.isInteger(evidence.round) && evidence.round >= 0, `${path}.round must be a non-negative integer`);
  validateString(evidence.created_at, `${path}.created_at`);
  return evidence;
}

function validateFinding(finding, evidenceIndex, path = "finding") {
  invariant(isPlainObject(finding), `${path} must be an object`);
  validateString(finding.id, `${path}.id`);
  validateString(finding.critic, `${path}.critic`);
  validateString(finding.summary, `${path}.summary`);
  validateString(finding.rationale, `${path}.rationale`);
  validateString(finding.suggested_change, `${path}.suggested_change`);
  invariant(ALL_SEVERITIES.has(finding.severity), `${path}.severity must be one of high|medium|low`);
  invariant(FINDING_BASIS.has(finding.basis), `${path}.basis must be one of supported|gap`);
  validateArray(finding.evidence_ids, `${path}.evidence_ids`);

  if (finding.basis === "supported") {
    invariant(finding.evidence_ids.length > 0, `${path}.evidence_ids must be non-empty for supported findings`);
  } else {
    invariant(finding.evidence_ids.length === 0, `${path}.evidence_ids must be empty for gap findings`);
  }

  for (const [index, evidenceId] of finding.evidence_ids.entries()) {
    validateString(evidenceId, `${path}.evidence_ids[${index}]`);
    invariant(evidenceIndex.has(evidenceId), `${path}.evidence_ids[${index}] references unknown evidence ${evidenceId}`);
  }

  return finding;
}

function validateDecision(decision, findingIndex, evidenceIndex, path = "decision") {
  invariant(isPlainObject(decision), `${path} must be an object`);
  validateString(decision.finding_id, `${path}.finding_id`);
  invariant(DECISIONS.has(decision.disposition), `${path}.disposition must be one of accept|reject`);
  validateString(decision.rationale, `${path}.rationale`);
  validateArray(decision.evidence_ids, `${path}.evidence_ids`);

  const finding = findingIndex.get(decision.finding_id);
  invariant(finding, `${path}.finding_id references unknown finding ${decision.finding_id}`);

  if (finding.basis === "supported") {
    invariant(decision.evidence_ids.length > 0, `${path}.evidence_ids must be non-empty for supported findings`);
  }

  for (const [index, evidenceId] of decision.evidence_ids.entries()) {
    validateString(evidenceId, `${path}.evidence_ids[${index}]`);
    invariant(evidenceIndex.has(evidenceId), `${path}.evidence_ids[${index}] references unknown evidence ${evidenceId}`);
  }

  return decision;
}

function findingCounts(findings) {
  return findings.reduce(
    (acc, finding) => {
      acc.total += 1;
      acc[finding.severity] += 1;
      if (MATERIAL_SEVERITIES.has(finding.severity)) {
        acc.material += 1;
      }
      if (finding.basis === "gap") {
        acc.gaps += 1;
      }
      return acc;
    },
    { total: 0, high: 0, medium: 0, low: 0, material: 0, gaps: 0 },
  );
}

function isMaterialFinding(finding) {
  return MATERIAL_SEVERITIES.has(finding.severity);
}

module.exports = {
  findingCounts,
  invariant,
  isMaterialFinding,
  validateDecision,
  validateEvidence,
  validateFinding,
  validateProposal,
};
