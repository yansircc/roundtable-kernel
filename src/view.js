function countByCritic(findings = []) {
  const counts = {};
  for (const finding of findings) {
    const critic = finding.critic || "unknown";
    if (!counts[critic]) {
      counts[critic] = { total: 0, high: 0, medium: 0, low: 0, gap: 0 };
    }
    counts[critic].total += 1;
    counts[critic][finding.severity] += 1;
    if (finding.basis === "gap") {
      counts[critic].gap += 1;
    }
  }
  return counts;
}

function roundFindings(round) {
  return Array.isArray(round?.findings_against_proposal)
    ? round.findings_against_proposal
    : Array.isArray(round?.findings)
      ? round.findings
      : [];
}

function deriveSessionSummary(session) {
  const rounds = Array.isArray(session.rounds) ? session.rounds : [];
  const openRound = session.open_round || null;
  const findings = rounds.flatMap((round) => roundFindings(round)).concat(roundFindings(openRound));
  const lastRound = rounds[rounds.length - 1] || null;
  const latestRound = openRound || lastRound;

  return {
    id: session.id,
    topic: session.topic,
    chair: session.chair,
    critics: session.critics || [],
    max_rounds: session.max_rounds,
    round: session.status?.round || 0,
    state: session.status?.state || "initialized",
    converged: Boolean(session.status?.converged),
    unresolved_high: session.status?.unresolved_high || 0,
    unresolved_medium: session.status?.unresolved_medium || 0,
    active_actor: session.status?.active_actor || null,
    active_phase: session.status?.active_phase || null,
    error_message: session.status?.error?.message || openRound?.error?.message || "",
    evidence_count: Array.isArray(session.evidence) ? session.evidence.length : 0,
    total_findings: findings.length,
    gap_findings: findings.filter((finding) => finding.basis === "gap").length,
    adjudicated_summary: session.adjudicated_proposal?.summary || "",
    latest_proposal_summary: latestRound?.proposal?.summary || "",
    findings_by_critic: countByCritic(findings),
    has_open_round: Boolean(openRound),
    updated_at: latestRound?.updated_at || latestRound?.created_at || session.created_at,
    created_at: session.created_at,
  };
}

function sortSessionSummaries(summaries) {
  return [...summaries].sort((left, right) => {
    const a = Date.parse(left.updated_at || left.created_at || 0);
    const b = Date.parse(right.updated_at || right.created_at || 0);
    if (a !== b) {
      return b - a;
    }
    return left.id.localeCompare(right.id);
  });
}

module.exports = {
  deriveSessionSummary,
  sortSessionSummaries,
};
