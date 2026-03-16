const { appendEvidence, applyRound, createSession } = require("./kernel");
const { createAdapter } = require("./adapters");
const { createSessionFile, saveSession } = require("./store");

function collectEvidenceBatches(session, evidenceKeyMap, batches, round) {
  if (!Array.isArray(batches) || batches.length === 0) {
    return [];
  }

  const addedIds = [];
  for (const batch of batches) {
    if (!batch || !Array.isArray(batch.items) || batch.items.length === 0) {
      continue;
    }
    const added = appendEvidence(session, {
      items: batch.items,
      collectedBy: batch.collected_by,
      phase: batch.phase,
      round,
    });
    for (let index = 0; index < batch.items.length; index += 1) {
      const key = batch.items[index].key;
      if (!key) {
        continue;
      }
      if (evidenceKeyMap.has(key)) {
        throw new Error(`duplicate evidence key ${key}`);
      }
      evidenceKeyMap.set(key, added[index].id);
    }
    addedIds.push(...added.map((evidence) => evidence.id));
  }
  return addedIds;
}

async function runSession({ adapterKind, adapterConfig, sessionId, force = false }) {
  const adapter = createAdapter(adapterKind, adapterConfig);
  const metadata = adapter.metadata();

  const session = createSession({
    id: sessionId,
    topic: metadata.topic,
    chair: metadata.chair,
    critics: metadata.critics,
    max_rounds: metadata.max_rounds,
    adapter: adapter.kind,
  });
  createSessionFile(session, { force });

  const evidenceKeyMap = new Map();
  collectEvidenceBatches(session, evidenceKeyMap, await adapter.seedEvidence(), 0);
  saveSession(session);

  for (let round = 1; round <= session.max_rounds; round += 1) {
    const evidenceAdded = [];

    const explore = await adapter.collectEvidence({
      session,
      round,
      actor: session.chair,
      phase: "explore",
    });
    evidenceAdded.push(...collectEvidenceBatches(session, evidenceKeyMap, explore, round));

    const proposal = await adapter.propose({ session, round });

    const findings = [];
    for (const critic of session.critics) {
      const reExplore = await adapter.collectEvidence({
        session,
        round,
        actor: critic,
        phase: "re-explore",
      });
      evidenceAdded.push(...collectEvidenceBatches(session, evidenceKeyMap, reExplore, round));
      findings.push(...(await adapter.review({ session, round, critic, proposal, evidenceKeyMap })));
    }

    const verdict = await adapter.adjudicate({
      session,
      round,
      proposal,
      findings,
      evidenceKeyMap,
    });

    applyRound(session, {
      proposal,
      findings,
      verdict,
      evidence_added: evidenceAdded,
    });
    saveSession(session);

    if (session.status.converged) {
      break;
    }
  }

  if (!session.status.converged && session.status.round === session.max_rounds) {
    session.status = {
      ...session.status,
      state: "exhausted",
    };
  }
  saveSession(session);
  return session;
}

module.exports = {
  runSession,
};
