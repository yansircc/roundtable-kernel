const fs = require("node:fs");
const path = require("node:path");

function readFixture(file) {
  return JSON.parse(fs.readFileSync(file, "utf8"));
}

function uniqueCritics(rounds) {
  return [...new Set((rounds || []).flatMap((round) => (round.findings || []).map((finding) => finding.critic)))];
}

function replaceEvidenceKeys(values, evidenceKeyMap) {
  return (values || []).map((key) => {
    const mapped = evidenceKeyMap.get(key);
    if (!mapped) {
      throw new Error(`unknown evidence key ${key}`);
    }
    return mapped;
  });
}

function createFixtureAdapter({ fixturePath }) {
  const fixture = readFixture(path.resolve(fixturePath));
  const critics = fixture.critics || uniqueCritics(fixture.rounds);
  const chair = fixture.chair || "chair";

  function toBatch(batch, { actor, phase }) {
    if (!batch || !Array.isArray(batch.items) || batch.items.length === 0) {
      return null;
    }
    return {
      items: batch.items,
      collected_by: batch.collected_by || actor,
      phase,
    };
  }

  function roundAt(round) {
    const value = fixture.rounds[round - 1];
    if (!value) {
      throw new Error(`fixture has no round ${round}`);
    }
    return value;
  }

  return {
    kind: "fixture",
    metadata() {
      return {
        topic: fixture.topic,
        chair,
        critics,
        max_rounds: fixture.max_rounds || fixture.rounds.length,
      };
    },
    seedEvidence() {
      const batch = toBatch(fixture.seed_batch, { actor: fixture.seed_batch?.actor || chair, phase: "seed" });
      return batch ? [batch] : [];
    },
    collectEvidence({ round, actor, phase }) {
      const spec = roundAt(round);
      return (spec.evidence_batches || [])
        .filter((batch) => batch.phase === phase && batch.actor === actor)
        .map((batch) => toBatch(batch, { actor, phase }))
        .filter(Boolean);
    },
    propose({ round }) {
      return roundAt(round).proposal;
    },
    review({ round, critic, evidenceKeyMap }) {
      const spec = roundAt(round);
      return (spec.findings || [])
        .filter((finding) => finding.critic === critic)
        .map((finding) => ({
          id: finding.id,
          critic: finding.critic,
          severity: finding.severity,
          summary: finding.summary,
          rationale: finding.rationale,
          suggested_change: finding.suggested_change,
          basis: finding.basis,
          evidence_ids: replaceEvidenceKeys(finding.evidence_keys, evidenceKeyMap),
        }));
    },
    adjudicate({ round, evidenceKeyMap }) {
      const spec = roundAt(round);
      if (!spec.verdict) {
        return null;
      }
      return {
        summary: spec.verdict.summary,
        revised_proposal: spec.verdict.revised_proposal || null,
        decisions: (spec.verdict.decisions || []).map((decision) => ({
          finding_id: decision.finding_id,
          disposition: decision.disposition,
          rationale: decision.rationale,
          evidence_ids: replaceEvidenceKeys(decision.evidence_keys, evidenceKeyMap),
        })),
      };
    },
  };
}

module.exports = {
  createFixtureAdapter,
};
