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

function deriveSessionSummary(session) {
  const rounds = Array.isArray(session.rounds) ? session.rounds : [];
  const findings = rounds.flatMap((round) => round.findings || []);
  const lastRound = rounds[rounds.length - 1] || null;

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
    evidence_count: Array.isArray(session.evidence) ? session.evidence.length : 0,
    total_findings: findings.length,
    gap_findings: findings.filter((finding) => finding.basis === "gap").length,
    adjudicated_summary: session.adjudicated_proposal?.summary || "",
    latest_proposal_summary: lastRound?.proposal?.summary || "",
    findings_by_critic: countByCritic(findings),
    updated_at: lastRound?.created_at || session.created_at,
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
