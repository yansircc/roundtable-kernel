const {
  appendEvidence,
  appendRoundFindings,
  applyRound,
  completePhase,
  createSession,
  markSessionFailed,
  noteRoundEvidence,
  recordPhaseStart,
  registerProposal,
  registerVerdict,
} = require("./kernel");
const { createAdapter } = require("./adapters");
const { createSessionFile, saveSession } = require("./store");
const { appendTelemetryEvent, resetTelemetryFile, sanitizeError } = require("./telemetry");

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

function summarizeEvidenceBatches(batches) {
  const list = Array.isArray(batches) ? batches : [];
  const items = list.flatMap((batch) => (Array.isArray(batch?.items) ? batch.items : []));
  return {
    batch_count: list.length,
    item_count: items.length,
    evidence_keys: items.map((item) => item.key).filter(Boolean),
  };
}

function summarizeProposal(proposal) {
  return {
    summary: proposal?.summary || "",
    claim_count: Array.isArray(proposal?.claims) ? proposal.claims.length : 0,
    acceptance_count: Array.isArray(proposal?.acceptance) ? proposal.acceptance.length : 0,
  };
}

function summarizeFindings(findings) {
  const list = Array.isArray(findings) ? findings : [];
  return {
    finding_count: list.length,
    high: list.filter((item) => item.severity === "high").length,
    medium: list.filter((item) => item.severity === "medium").length,
    low: list.filter((item) => item.severity === "low").length,
    gaps: list.filter((item) => item.basis === "gap").length,
  };
}

function summarizeVerdict(verdict) {
  if (!verdict) {
    return { skipped: true };
  }
  const decisions = Array.isArray(verdict.decisions) ? verdict.decisions : [];
  return {
    summary: verdict.summary || "",
    decision_count: decisions.length,
    accepted: decisions.filter((item) => item.disposition === "accept").length,
    rejected: decisions.filter((item) => item.disposition === "reject").length,
  };
}

async function runPhase({ telemetryFile, session, round, actor, phase, adapter, input_summary, summarize, onSuccess, fn }) {
  const startedAt = Date.now();
  if (round > 0) {
    recordPhaseStart(session, { round, actor, phase, input_summary });
    saveSession(session);
  }
  appendTelemetryEvent(telemetryFile, {
    type: "phase_started",
    session_id: session.id,
    round,
    actor,
    phase,
    adapter,
    input_summary,
  });

  try {
    const result = await fn();
    const outputSummary = summarize ? summarize(result) : undefined;
    const artifact = onSuccess ? onSuccess(result) : null;
    if (round > 0) {
      completePhase(session, {
        round,
        actor,
        phase,
        output_summary: outputSummary,
        artifact,
        duration_ms: Date.now() - startedAt,
      });
      saveSession(session);
    } else if (onSuccess) {
      saveSession(session);
    }
    appendTelemetryEvent(telemetryFile, {
      type: "phase_succeeded",
      session_id: session.id,
      round,
      actor,
      phase,
      adapter,
      duration_ms: Date.now() - startedAt,
      output_summary: outputSummary,
    });
    return result;
  } catch (error) {
    if (round > 0) {
      completePhase(session, {
        round,
        actor,
        phase,
        duration_ms: Date.now() - startedAt,
        error: sanitizeError(error),
      });
      markSessionFailed(session, { round, actor, phase, error });
      saveSession(session);
    }
    appendTelemetryEvent(telemetryFile, {
      type: "phase_failed",
      session_id: session.id,
      round,
      actor,
      phase,
      adapter,
      duration_ms: Date.now() - startedAt,
      error: sanitizeError(error),
    });
    throw error;
  }
}

async function runSession({ adapterKind, adapterConfig, sessionId, force = false }) {
  const bootstrapAdapter = createAdapter(adapterKind, adapterConfig);
  const metadata = bootstrapAdapter.metadata();

  const session = createSession({
    id: sessionId,
    topic: metadata.topic,
    chair: metadata.chair,
    critics: metadata.critics,
    max_rounds: metadata.max_rounds,
    adapter: bootstrapAdapter.kind,
  });
  createSessionFile(session, { force });
  const telemetryFile = resetTelemetryFile(session.id);
  const adapter = createAdapter(adapterKind, {
    ...adapterConfig,
    telemetryFile,
  });

  appendTelemetryEvent(telemetryFile, {
    type: "session_started",
    session_id: session.id,
    adapter: session.adapter,
    topic: session.topic,
    chair: session.chair,
    critics: session.critics,
    max_rounds: session.max_rounds,
  });

  try {
    const evidenceKeyMap = new Map();
    await runPhase({
      telemetryFile,
      session,
      round: 0,
      actor: session.chair,
      phase: "seed",
      adapter: session.adapter,
      summarize: summarizeEvidenceBatches,
      fn: () => adapter.seedEvidence(),
      onSuccess: (batches) => {
        collectEvidenceBatches(session, evidenceKeyMap, batches, 0);
        return null;
      },
    });

    for (let round = 1; round <= session.max_rounds; round += 1) {
      await runPhase({
        telemetryFile,
        session,
        round,
        actor: session.chair,
        phase: "explore",
        adapter: session.adapter,
        summarize: summarizeEvidenceBatches,
        fn: () =>
          adapter.collectEvidence({
            session,
            round,
            actor: session.chair,
            phase: "explore",
          }),
        onSuccess: (batches) => {
          const added = collectEvidenceBatches(session, evidenceKeyMap, batches, round);
          noteRoundEvidence(session, { round, evidence_added: added });
          return { evidence_added: added };
        },
      });

      await runPhase({
        telemetryFile,
        session,
        round,
        actor: session.chair,
        phase: "propose",
        adapter: session.adapter,
        summarize: summarizeProposal,
        fn: () => adapter.propose({ session, round }),
        onSuccess: (nextProposal) => {
          registerProposal(session, { round, proposal: nextProposal });
          return { proposal: nextProposal };
        },
      });
      const proposal = session.open_round?.proposal;

      const findings = [];
      for (const critic of session.critics) {
        await runPhase({
          telemetryFile,
          session,
          round,
          actor: critic,
          phase: "re-explore",
          adapter: session.adapter,
          input_summary: { proposal_summary: proposal.summary },
          summarize: summarizeEvidenceBatches,
          fn: () =>
            adapter.collectEvidence({
              session,
              round,
              actor: critic,
              phase: "re-explore",
            }),
          onSuccess: (batches) => {
            const added = collectEvidenceBatches(session, evidenceKeyMap, batches, round);
            noteRoundEvidence(session, { round, evidence_added: added });
            return { evidence_added: added };
          },
        });
        await runPhase({
          telemetryFile,
          session,
          round,
          actor: critic,
          phase: "review",
          adapter: session.adapter,
          input_summary: { proposal_summary: proposal.summary },
          summarize: summarizeFindings,
          fn: () => adapter.review({ session, round, critic, proposal, evidenceKeyMap }),
          onSuccess: (criticFindings) => {
            appendRoundFindings(session, { round, findings: criticFindings });
            findings.push(...criticFindings);
            return { findings_against_proposal: criticFindings };
          },
        });
      }

      await runPhase({
        telemetryFile,
        session,
        round,
        actor: session.chair,
        phase: "adjudicate",
        adapter: session.adapter,
        input_summary: {
          proposal_summary: proposal.summary,
          finding_count: findings.length,
        },
        summarize: summarizeVerdict,
        fn: () =>
          adapter.adjudicate({
            session,
            round,
            proposal,
            findings,
            evidenceKeyMap,
          }),
        onSuccess: (nextVerdict) => {
          registerVerdict(session, { round, verdict: nextVerdict });
          return { verdict: nextVerdict };
        },
      });

      applyRound(session);
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
    appendTelemetryEvent(telemetryFile, {
      type: "session_finished",
      session_id: session.id,
      adapter: session.adapter,
      state: session.status.state,
      round: session.status.round,
      converged: session.status.converged,
      unresolved_high: session.status.unresolved_high,
      unresolved_medium: session.status.unresolved_medium,
    });
    return session;
  } catch (error) {
    if (session.status.state !== "failed") {
      markSessionFailed(session, {
        round: session.open_round?.index || session.status.round,
        actor: session.status.active_actor,
        phase: session.status.active_phase,
        error,
      });
      saveSession(session);
    }
    appendTelemetryEvent(telemetryFile, {
      type: "session_failed",
      session_id: session.id,
      adapter: session.adapter,
      round: session.status.round,
      error: sanitizeError(error),
    });
    throw error;
  }
}

module.exports = {
  runSession,
};
