<script>
  import { onMount } from 'svelte';

  let projectRoot = '';
  let sessions = [];
  let selectedId = null;
  let selected = null;
  let now = Date.now();
  let error = '';

  async function getJson(url) {
    const response = await fetch(url);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(text || `${url} -> ${response.status}`);
    }
    return response.json();
  }

  function pickSelected(nextSessions) {
    if (selectedId && nextSessions.some((session) => session.id === selectedId)) {
      return selectedId;
    }
    return nextSessions[0]?.id || null;
  }

  async function refreshSessions() {
    const data = await getJson('/api/sessions');
    projectRoot = data.project_root || '';
    sessions = data.sessions || [];
    selectedId = pickSelected(sessions);
  }

  async function refreshSelected() {
    if (!selectedId) {
      selected = null;
      return;
    }
    selected = await getJson(`/api/session/${encodeURIComponent(selectedId)}`);
  }

  async function refresh() {
    try {
      await refreshSessions();
      await refreshSelected();
      error = '';
    } catch (nextError) {
      error = nextError.message;
    }
  }

  function selectSession(id) {
    selectedId = id;
    refreshSelected();
  }

  function tone(state) {
    switch (state) {
      case 'converged':
        return 'good';
      case 'needs_revision':
        return 'warn';
      case 'exhausted':
        return 'bad';
      default:
        return 'muted';
    }
  }

  function severityTone(severity) {
    return severity === 'high' ? 'sev-high' : severity === 'medium' ? 'sev-medium' : 'sev-low';
  }

  function phaseTone(phase) {
    return phase === 'seed' ? 'seed' : phase === 're-explore' ? 'reexplore' : 'explore';
  }

  function clock(iso) {
    if (!iso) return '--:--:--';
    return new Date(iso).toLocaleTimeString('en-GB', {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    });
  }

  function age(iso) {
    if (!iso) return 'now';
    const seconds = Math.max(0, Math.floor((now - new Date(iso).getTime()) / 1000));
    if (seconds < 60) return `${seconds}s`;
    const minutes = Math.floor(seconds / 60);
    if (minutes < 60) return `${minutes}m`;
    const hours = Math.floor(minutes / 60);
    return `${hours}h${minutes % 60}m`;
  }

  function evidenceMap(session) {
    return new Map((session?.evidence || []).map((item) => [item.id, item]));
  }

  $: selectedSummary = sessions.find((session) => session.id === selectedId) || null;
  $: evidenceById = evidenceMap(selected);

  onMount(() => {
    refresh();
    const sessionTimer = setInterval(refresh, 2000);
    const clockTimer = setInterval(() => {
      now = Date.now();
    }, 1000);

    return () => {
      clearInterval(sessionTimer);
      clearInterval(clockTimer);
    };
  });
</script>

<svelte:head>
  <title>Roundtable Kernel Dashboard</title>
</svelte:head>

<div class="shell">
  <header class="masthead">
    <div class="title-block">
      <div class="eyebrow">Kernel Truth</div>
      <h1>Roundtable Kernel</h1>
      <p class="subhead">{projectRoot || 'loading project root...'}</p>
    </div>
    <div class="law">
      <div class="law-label">Core Law</div>
      <div class="law-text">baseline facts -> challenge -> targeted re-explore -> verdict -> next baseline</div>
    </div>
  </header>

  <div class="layout">
    <aside class="sidebar panel">
      <div class="section-head">
        <h2>Sessions</h2>
        <span>{sessions.length}</span>
      </div>

      {#if sessions.length === 0}
        <div class="empty">No sessions yet.</div>
      {:else}
        <div class="session-list">
          {#each sessions as session}
            <button class:selected={selectedId === session.id} class="session-card" on:click={() => selectSession(session.id)}>
              <div class="card-top">
                <span class="session-id">{session.id}</span>
                <span class="pill {tone(session.state)}">{session.state}</span>
              </div>
              <div class="session-topic">{session.topic}</div>
              <div class="card-meta">
                <span>round {session.round}/{session.max_rounds}</span>
                <span>{session.evidence_count} evidence</span>
              </div>
              <div class="card-meta">
                <span>high {session.unresolved_high}</span>
                <span>medium {session.unresolved_medium}</span>
                <span>gap {session.gap_findings}</span>
              </div>
              <div class="critic-row">
                {#each session.critics as critic}
                  {@const counts = session.findings_by_critic?.[critic] || { total: 0, high: 0, medium: 0 }}
                  <span class="critic-chip">{critic} {counts.total}</span>
                {/each}
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </aside>

    <main class="canvas">
      {#if selected}
        <section class="hero panel">
          <div class="hero-copy">
            <div class="eyebrow">Selected Session</div>
            <h2>{selected.id}</h2>
            <p class="hero-topic">{selected.topic}</p>
            <p class="hero-summary">{selected.adjudicated_proposal?.summary || 'No adjudicated proposal yet.'}</p>
          </div>
          <div class="hero-metrics">
            <div class="metric">
              <div class="metric-label">State</div>
              <div class="metric-value">{selected.status.state}</div>
            </div>
            <div class="metric">
              <div class="metric-label">Round</div>
              <div class="metric-value">{selected.status.round}/{selected.max_rounds}</div>
            </div>
            <div class="metric">
              <div class="metric-label">Updated</div>
              <div class="metric-value">{age(selectedSummary?.updated_at)}</div>
            </div>
          </div>
        </section>

        <section class="summary-grid">
          <article class="panel summary-card">
            <div class="summary-label">Chair</div>
            <div class="summary-value">{selected.chair}</div>
            <p>Critics: {(selected.critics || []).join(', ') || 'none'}</p>
          </article>
          <article class="panel summary-card">
            <div class="summary-label">Unresolved</div>
            <div class="summary-value">{selected.status.unresolved_high} high / {selected.status.unresolved_medium} medium</div>
            <p>Convergence remains critic-driven.</p>
          </article>
          <article class="panel summary-card">
            <div class="summary-label">Evidence Ledger</div>
            <div class="summary-value">{selected.evidence.length}</div>
            <p>Seed, explore, and re-explore remain explicit phases.</p>
          </article>
        </section>

        <div class="main-grid">
          <section class="panel rounds">
            <div class="section-head">
              <h2>Rounds</h2>
              <span>{selected.rounds.length}</span>
            </div>

            <div class="round-list">
              {#each selected.rounds as round}
                <article class="round-card">
                  <div class="round-head">
                    <div>
                      <div class="eyebrow">Round {round.index}</div>
                      <h3>{round.proposal.summary}</h3>
                    </div>
                    <div class="round-badges">
                      <span class="pill muted">{round.review_summary.total} findings</span>
                      <span class="pill muted">{round.evidence_added.length} evidence</span>
                    </div>
                  </div>

                  {#if round.proposal.claims?.length}
                    <div class="claim-strip">
                      {#each round.proposal.claims as claim}
                        <span class="token">{claim}</span>
                      {/each}
                    </div>
                  {/if}

                  <div class="subgrid">
                    <div>
                      <div class="block-label">Findings Against Proposal</div>
                      {#if round.findings.length === 0}
                        <div class="empty thin">Clean critic pass. No material findings remain.</div>
                      {:else}
                        <div class="finding-list">
                          {#each round.findings as finding}
                            <article class="finding">
                              <div class="finding-head">
                                <div class="finding-tags">
                                  <span class="pill {severityTone(finding.severity)}">{finding.severity}</span>
                                  <span class="pill {finding.basis === 'gap' ? 'gap' : 'supported'}">{finding.basis}</span>
                                </div>
                                <span class="critic-name">{finding.critic}</span>
                              </div>
                              <div class="finding-summary">{finding.summary}</div>
                              <p>{finding.rationale}</p>
                              <div class="finding-change">Change: {finding.suggested_change}</div>
                              <div class="evidence-strip">
                                {#if finding.evidence_ids.length === 0}
                                  <span class="evidence-chip empty-chip">gap: no evidence yet</span>
                                {:else}
                                  {#each finding.evidence_ids as evidenceId}
                                    {@const evidence = evidenceById.get(evidenceId)}
                                    <span class="evidence-chip" title={evidence?.statement || ''}>
                                      {evidenceId} {evidence?.phase || ''}
                                    </span>
                                  {/each}
                                {/if}
                              </div>
                            </article>
                          {/each}
                        </div>
                      {/if}
                    </div>

                    <div>
                      <div class="block-label">Adjudication</div>
                      {#if round.verdict}
                        <article class="verdict">
                          <div class="verdict-summary">{round.verdict.summary}</div>
                          <div class="decision-list">
                            {#each round.verdict.decisions as decision}
                              <div class="decision-row">
                                <span class="decision-id">{decision.finding_id}</span>
                                <span class="pill {decision.disposition === 'accept' ? 'good' : 'bad'}">{decision.disposition}</span>
                              </div>
                            {/each}
                          </div>
                        </article>
                      {:else}
                        <div class="empty thin">Skipped because the critic pass already converged.</div>
                      {/if}
                    </div>
                  </div>
                </article>
              {/each}
            </div>
          </section>

          <section class="panel ledger">
            <div class="section-head">
              <h2>Evidence Ledger</h2>
              <span>{selected.evidence.length}</span>
            </div>

            <div class="evidence-list">
              {#each selected.evidence as evidence}
                <article class="evidence-card">
                  <div class="evidence-head">
                    <div class="evidence-tags">
                      <span class="evidence-id">{evidence.id}</span>
                      <span class="pill {phaseTone(evidence.phase)}">{evidence.phase}</span>
                    </div>
                    <span class="evidence-meta">r{evidence.round} · {evidence.collected_by}</span>
                  </div>
                  <div class="evidence-statement">{evidence.statement}</div>
                  <div class="evidence-source">{evidence.source}</div>
                  <details>
                    <summary>{clock(evidence.created_at)} · excerpt</summary>
                    <pre>{evidence.excerpt}</pre>
                  </details>
                </article>
              {/each}
            </div>
          </section>
        </div>
      {:else}
        <section class="panel empty big">No session selected.</section>
      {/if}
    </main>
  </div>

  {#if error}
    <div class="error-banner">{error}</div>
  {/if}
</div>

<style>
  :global(*) {
    box-sizing: border-box;
  }

  :global(body) {
    margin: 0;
    min-height: 100vh;
    background:
      radial-gradient(circle at top left, rgba(255, 188, 99, 0.22), transparent 24rem),
      radial-gradient(circle at top right, rgba(78, 191, 180, 0.18), transparent 28rem),
      linear-gradient(180deg, #f6f0e8 0%, #efe6db 100%);
    color: #1f1d1a;
    font-family: "Space Grotesk", "Avenir Next", "Helvetica Neue", sans-serif;
  }

  :global(button) {
    font: inherit;
  }

  :global(pre) {
    white-space: pre-wrap;
    margin: 0;
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
  }

  .shell {
    padding: 24px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }

  .masthead {
    display: flex;
    justify-content: space-between;
    gap: 24px;
    align-items: flex-end;
    padding: 26px 28px;
    border: 2px solid rgba(31, 29, 26, 0.9);
    background: rgba(255, 252, 247, 0.82);
    box-shadow: 0 14px 40px rgba(76, 62, 43, 0.12);
    backdrop-filter: blur(12px);
  }

  .title-block h1,
  .hero-copy h2,
  .section-head h2,
  .round-head h3 {
    margin: 0;
    font-weight: 700;
    letter-spacing: -0.04em;
  }

  .eyebrow,
  .law-label,
  .summary-label,
  .block-label,
  .session-id,
  .evidence-id,
  .evidence-source,
  .evidence-meta,
  .critic-chip,
  .decision-id {
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    font-size: 11px;
  }

  .subhead,
  .hero-topic,
  .hero-summary,
  .finding p,
  .summary-card p {
    margin: 0;
    line-height: 1.5;
    color: rgba(31, 29, 26, 0.76);
  }

  .law {
    max-width: 30rem;
    padding: 14px 16px;
    background: linear-gradient(135deg, #16110f, #2b2522);
    color: #fdf5ee;
    border-radius: 18px;
  }

  .law-text {
    margin-top: 6px;
    line-height: 1.35;
  }

  .layout {
    display: grid;
    grid-template-columns: 320px minmax(0, 1fr);
    gap: 18px;
    min-height: 0;
  }

  .panel {
    border: 2px solid rgba(31, 29, 26, 0.9);
    background: rgba(255, 252, 247, 0.84);
    box-shadow: 0 12px 32px rgba(76, 62, 43, 0.1);
    backdrop-filter: blur(10px);
  }

  .sidebar,
  .hero,
  .summary-card,
  .rounds,
  .ledger {
    border-radius: 24px;
  }

  .sidebar,
  .rounds,
  .ledger {
    padding: 18px;
  }

  .hero {
    padding: 24px;
    display: grid;
    grid-template-columns: minmax(0, 1fr) 320px;
    gap: 20px;
  }

  .hero-topic {
    margin-top: 8px;
    font-size: 1.08rem;
  }

  .hero-summary {
    margin-top: 14px;
    font-size: 1.02rem;
  }

  .hero-metrics,
  .summary-grid {
    display: grid;
    gap: 14px;
  }

  .hero-metrics {
    grid-template-columns: repeat(3, 1fr);
    align-self: stretch;
  }

  .summary-grid {
    margin-top: 18px;
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .summary-card,
  .metric {
    padding: 18px;
  }

  .metric {
    border-radius: 18px;
    background: linear-gradient(180deg, rgba(255, 247, 233, 0.95), rgba(242, 233, 219, 0.9));
    border: 1px solid rgba(31, 29, 26, 0.16);
  }

  .metric-label {
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: rgba(31, 29, 26, 0.62);
  }

  .metric-value,
  .summary-value {
    margin-top: 8px;
    font-size: 1.5rem;
    font-weight: 700;
    letter-spacing: -0.05em;
  }

  .main-grid {
    margin-top: 18px;
    display: grid;
    grid-template-columns: minmax(0, 1.2fr) minmax(320px, 0.8fr);
    gap: 18px;
  }

  .section-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    margin-bottom: 14px;
  }

  .session-list,
  .round-list,
  .evidence-list,
  .finding-list,
  .decision-list {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .session-card {
    width: 100%;
    padding: 16px;
    border-radius: 18px;
    border: 1px solid rgba(31, 29, 26, 0.14);
    background: linear-gradient(180deg, rgba(255, 247, 233, 0.92), rgba(246, 238, 227, 0.9));
    text-align: left;
    cursor: pointer;
    transition: transform 120ms ease, box-shadow 120ms ease, border-color 120ms ease;
  }

  .session-card:hover,
  .session-card.selected {
    transform: translateY(-2px);
    box-shadow: 0 10px 22px rgba(76, 62, 43, 0.12);
    border-color: rgba(31, 29, 26, 0.48);
  }

  .card-top,
  .card-meta,
  .critic-row,
  .round-head,
  .finding-head,
  .decision-row,
  .evidence-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 10px;
  }

  .session-topic,
  .finding-summary,
  .evidence-statement,
  .verdict-summary {
    font-size: 0.98rem;
    line-height: 1.45;
    margin-top: 8px;
  }

  .critic-row,
  .card-meta {
    margin-top: 10px;
    color: rgba(31, 29, 26, 0.68);
    font-size: 0.9rem;
  }

  .critic-row {
    flex-wrap: wrap;
    justify-content: flex-start;
  }

  .critic-chip,
  .token,
  .evidence-chip {
    padding: 6px 9px;
    border-radius: 999px;
    background: rgba(31, 29, 26, 0.06);
  }

  .pill {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    padding: 6px 10px;
    border-radius: 999px;
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
    font-size: 11px;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    border: 1px solid rgba(31, 29, 26, 0.12);
  }

  .good {
    background: #d6f5e7;
    color: #0f5d3c;
  }

  .warn {
    background: #fff2c7;
    color: #805600;
  }

  .bad {
    background: #ffd9d4;
    color: #8b2018;
  }

  .muted {
    background: rgba(31, 29, 26, 0.06);
    color: rgba(31, 29, 26, 0.7);
  }

  .sev-high {
    background: #ffd3cb;
    color: #8f1f12;
  }

  .sev-medium {
    background: #ffeab6;
    color: #875f00;
  }

  .sev-low {
    background: #def1ff;
    color: #0c5470;
  }

  .gap {
    background: #ece4ff;
    color: #5d2a96;
  }

  .supported {
    background: #d9f6f4;
    color: #0d5d57;
  }

  .seed {
    background: #e8e3ff;
    color: #4b3495;
  }

  .explore {
    background: #d9f1ff;
    color: #0a5674;
  }

  .reexplore {
    background: #dbf5e3;
    color: #0d6742;
  }

  .round-card,
  .evidence-card,
  .finding,
  .verdict {
    padding: 16px;
    border-radius: 18px;
    border: 1px solid rgba(31, 29, 26, 0.12);
    background: rgba(255, 250, 243, 0.92);
  }

  .claim-strip,
  .evidence-strip,
  .finding-tags,
  .evidence-tags,
  .round-badges {
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }

  .claim-strip {
    margin-top: 12px;
  }

  .subgrid {
    margin-top: 14px;
    display: grid;
    grid-template-columns: 1.2fr 0.8fr;
    gap: 14px;
  }

  .finding-change {
    margin-top: 10px;
    color: rgba(31, 29, 26, 0.86);
    font-weight: 500;
  }

  .evidence-strip {
    margin-top: 12px;
  }

  .empty {
    padding: 24px;
    border-radius: 18px;
    background: rgba(31, 29, 26, 0.04);
    color: rgba(31, 29, 26, 0.6);
    text-align: center;
  }

  .thin {
    padding: 14px;
    text-align: left;
  }

  .big {
    padding: 48px;
  }

  .evidence-source,
  details summary {
    margin-top: 10px;
    color: rgba(31, 29, 26, 0.62);
  }

  details {
    margin-top: 12px;
  }

  details summary {
    cursor: pointer;
  }

  .error-banner {
    padding: 12px 16px;
    border-radius: 16px;
    border: 1px solid rgba(139, 32, 24, 0.2);
    background: rgba(255, 217, 212, 0.85);
    color: #8b2018;
  }

  @media (max-width: 1180px) {
    .layout,
    .main-grid,
    .hero,
    .summary-grid,
    .subgrid {
      grid-template-columns: 1fr;
    }

    .hero-metrics {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }

  @media (max-width: 760px) {
    .shell {
      padding: 14px;
    }

    .masthead {
      padding: 18px;
      align-items: stretch;
      flex-direction: column;
    }

    .hero-metrics,
    .summary-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
