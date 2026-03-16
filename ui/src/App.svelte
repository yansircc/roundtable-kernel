<script>
  import { onMount } from 'svelte';

  let projectRoot = '';
  let sessions = [];
  let selectedId = null;
  let selected = null;
  let telemetry = [];
  let telemetryOffset = 0;
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

  async function refreshTelemetry({ reset = false } = {}) {
    if (!selectedId) {
      telemetry = [];
      telemetryOffset = 0;
      return;
    }
    const since = reset ? 0 : telemetryOffset;
    const data = await getJson(`/api/telemetry/${encodeURIComponent(selectedId)}?since=${since}`);
    const events = data.events || [];
    telemetry = reset || since === 0 ? events : [...telemetry, ...events];
    telemetryOffset = data.next_offset || telemetry.length;
  }

  async function refresh({ resetTelemetry = false } = {}) {
    try {
      const previousSelectedId = selectedId;
      await refreshSessions();
      const selectionChanged = resetTelemetry || previousSelectedId !== selectedId;
      await Promise.all([refreshSelected(), refreshTelemetry({ reset: selectionChanged })]);
      error = '';
    } catch (nextError) {
      error = nextError.message;
    }
  }

  function selectSession(id) {
    selectedId = id;
    Promise.all([refreshSelected(), refreshTelemetry({ reset: true })]);
  }

  function tone(state) {
    switch (state) {
      case 'running':
        return 'warn';
      case 'converged':
        return 'good';
      case 'needs_revision':
        return 'warn';
      case 'failed':
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
    return phase === 'seed'
      ? 'seed'
      : phase === 're-explore'
        ? 'reexplore'
        : phase === 'propose'
          ? 'propose'
          : phase === 'review'
            ? 'review'
            : phase === 'adjudicate'
              ? 'adjudicate'
              : 'explore';
  }

  function phaseStatusTone(status) {
    return status === 'failed' ? 'bad' : status === 'succeeded' ? 'good' : status === 'running' ? 'warn' : 'muted';
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

  function roundFindings(round) {
    return round?.findings_against_proposal || round?.findings || [];
  }

  function roundPhaseHistory(round) {
    return round?.phase_history || [];
  }

  function eventTone(type) {
    if (type === 'wrapper_stream') return 'warn';
    if (type?.endsWith('failed')) return 'bad';
    if (type?.endsWith('succeeded') || type?.endsWith('finished')) return 'good';
    if (type?.endsWith('started')) return 'muted';
    return 'warn';
  }

  function eventSubject(event) {
    const parts = [];
    if (event.actor) parts.push(event.actor);
    if (event.phase) parts.push(event.phase);
    if (event.type === 'wrapper_stream' && event.channel) parts.push(event.channel);
    if (!parts.length && event.source) parts.push(event.source);
    return parts.join(' / ') || 'session';
  }

  function currentArtifact(session) {
    return session?.open_round?.proposal || session?.adjudicated_proposal || null;
  }

  $: selectedSummary = sessions.find((session) => session.id === selectedId) || null;
  $: evidenceById = evidenceMap(selected);
  $: selectedOpenRound = selected?.open_round || null;
  $: selectedRounds = selected ? [...(selected.rounds || []), ...(selectedOpenRound ? [selectedOpenRound] : [])] : [];
  $: selectedArtifact = currentArtifact(selected);

  onMount(() => {
    refresh({ resetTelemetry: true });
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
              {#if session.active_actor && session.active_phase}
                <div class="card-meta">
                  <span>live {session.active_actor}</span>
                  <span>{session.active_phase}</span>
                </div>
              {:else if session.error_message}
                <div class="card-meta">
                  <span>{session.error_message}</span>
                </div>
              {/if}
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
            <p class="hero-summary">{selectedArtifact?.summary || 'No proposal material yet.'}</p>
            {#if selected.open_round}
              <p class="hero-status">Open round {selected.open_round.index} is still provisional.</p>
            {/if}
            {#if selected.status?.error?.message}
              <p class="hero-status failure">{selected.status.error.message}</p>
            {/if}
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
            <p>
              {#if selected.status.active_actor && selected.status.active_phase}
                live: {selected.status.active_actor} / {selected.status.active_phase}
              {:else}
                convergence remains critic-driven.
              {/if}
            </p>
          </article>
          <article class="panel summary-card">
            <div class="summary-label">Evidence Ledger</div>
            <div class="summary-value">{selected.evidence.length}</div>
            <p>{selected.open_round ? `open round ${selected.open_round.index} remains durable.` : 'Seed, explore, and re-explore remain explicit phases.'}</p>
          </article>
        </section>

        <div class="main-grid">
          <section class="panel rounds">
            <div class="section-head">
              <h2>Rounds</h2>
              <span>{selectedRounds.length}</span>
            </div>

            <div class="round-list">
              {#each selectedRounds as round}
                {@const findings = roundFindings(round)}
                {@const phases = roundPhaseHistory(round)}
                {@const isOpenRound = selectedOpenRound?.index === round.index}
                <article class="round-card">
                  <div class="round-head">
                    <div>
                      <div class="eyebrow">{isOpenRound ? 'Open Round' : 'Round'} {round.index}</div>
                      <h3>{round.proposal?.summary || 'No proposal yet.'}</h3>
                    </div>
                    <div class="round-badges">
                      <span class="pill {tone(isOpenRound ? selected.status.state : 'converged')}">{isOpenRound ? selected.status.state : 'closed'}</span>
                      <span class="pill muted">{round.review_summary?.total || findings.length} findings</span>
                      <span class="pill muted">{round.evidence_added.length} evidence</span>
                    </div>
                  </div>

                  {#if round.proposal?.claims?.length}
                    <div class="claim-strip">
                      {#each round.proposal.claims as claim}
                        <span class="token">{claim}</span>
                      {/each}
                    </div>
                  {/if}

                  <div class="subgrid">
                    <div>
                      <div class="block-label">Findings Against Proposal</div>
                      {#if findings.length === 0}
                        <div class="empty thin">Clean critic pass. No material findings remain.</div>
                      {:else}
                        <div class="finding-list">
                          {#each findings as finding}
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
                      {:else if isOpenRound && selected.status.state === 'failed'}
                        <div class="empty thin">Interrupted before adjudication.</div>
                      {:else if isOpenRound}
                        <div class="empty thin">Round is still in flight.</div>
                      {:else}
                        <div class="empty thin">Skipped because the critic pass already converged.</div>
                      {/if}
                    </div>
                  </div>

                  <div class="phase-panel">
                    <div class="block-label">Durable Phase History</div>
                    {#if phases.length === 0}
                      <div class="empty thin">No durable phase records yet.</div>
                    {:else}
                      <div class="phase-list">
                        {#each phases as entry}
                          <details class="phase-card">
                            <summary>
                              <div class="telemetry-top">
                                <span class="pill {phaseStatusTone(entry.status)}">{entry.status}</span>
                                <span class="telemetry-subject">{entry.actor} / {entry.phase}</span>
                              </div>
                              <div class="telemetry-meta">
                                <span>{clock(entry.started_at)}</span>
                                {#if entry.duration_ms !== null}
                                  <span>{entry.duration_ms}ms</span>
                                {/if}
                              </div>
                            </summary>
                            {#if entry.input_summary}
                              <div class="phase-block">
                                <div class="block-label">Input</div>
                                <pre>{JSON.stringify(entry.input_summary, null, 2)}</pre>
                              </div>
                            {/if}
                            {#if entry.output_summary}
                              <div class="phase-block">
                                <div class="block-label">Output</div>
                                <pre>{JSON.stringify(entry.output_summary, null, 2)}</pre>
                              </div>
                            {/if}
                            {#if entry.artifact}
                              <div class="phase-block">
                                <div class="block-label">Artifact</div>
                                <pre>{JSON.stringify(entry.artifact, null, 2)}</pre>
                              </div>
                            {/if}
                            {#if entry.error}
                              <div class="phase-block">
                                <div class="block-label">Error</div>
                                <pre>{JSON.stringify(entry.error, null, 2)}</pre>
                              </div>
                            {/if}
                          </details>
                        {/each}
                      </div>
                    {/if}
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

        <section class="panel telemetry-panel">
          <div class="section-head">
            <h2>Telemetry</h2>
            <span>{telemetry.length}</span>
          </div>
          <div class="telemetry-note">Transport side-channel only. Durable semantic truth lives in the round records above.</div>

          {#if telemetry.length === 0}
            <div class="empty">No telemetry yet.</div>
          {:else}
            <div class="telemetry-list">
              {#each [...telemetry].reverse().slice(0, 80) as event}
                <details class="telemetry-card">
                  <summary>
                    <div class="telemetry-top">
                      <span class="pill {eventTone(event.type)}">{event.type}</span>
                      <span class="telemetry-subject">{eventSubject(event)}</span>
                    </div>
                    <div class="telemetry-meta">
                      <span>{clock(event.ts)}</span>
                      {#if event.duration_ms !== undefined}
                        <span>{event.duration_ms}ms</span>
                      {/if}
                      {#if event.source}
                        <span>{event.source}</span>
                      {/if}
                    </div>
                  </summary>
                  <pre>{JSON.stringify(event, null, 2)}</pre>
                </details>
              {/each}
            </div>
          {/if}
        </section>
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

  .hero-status {
    margin-top: 10px;
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
    font-size: 12px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: rgba(31, 29, 26, 0.64);
  }

  .hero-status.failure {
    color: #8b2018;
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
  .decision-list,
  .telemetry-list {
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

  .propose {
    background: #ffe6cc;
    color: #8a4c00;
  }

  .review {
    background: #f0e0ff;
    color: #6a2ea0;
  }

  .adjudicate {
    background: #dff5ee;
    color: #126452;
  }

  .round-card,
  .evidence-card,
  .finding,
  .verdict,
  .telemetry-card {
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

  .phase-panel {
    margin-top: 14px;
  }

  .phase-list {
    display: flex;
    flex-direction: column;
    gap: 10px;
    margin-top: 10px;
  }

  .phase-card {
    padding: 14px;
    border-radius: 16px;
    border: 1px solid rgba(31, 29, 26, 0.12);
    background: rgba(255, 248, 238, 0.92);
  }

  .phase-card summary {
    list-style: none;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 12px;
    margin: 0;
  }

  .phase-block {
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

  .telemetry-panel {
    margin-top: 18px;
    padding: 18px;
    border-radius: 24px;
  }

  .telemetry-note {
    margin-bottom: 14px;
    color: rgba(31, 29, 26, 0.62);
    line-height: 1.5;
  }

  .telemetry-card summary {
    list-style: none;
    cursor: pointer;
    display: flex;
    justify-content: space-between;
    gap: 12px;
    align-items: center;
    margin: 0;
  }

  .telemetry-top,
  .telemetry-meta {
    display: flex;
    align-items: center;
    gap: 10px;
    flex-wrap: wrap;
  }

  .telemetry-subject,
  .telemetry-meta {
    font-family: "IBM Plex Mono", "SFMono-Regular", monospace;
    font-size: 11px;
    letter-spacing: 0.06em;
    text-transform: uppercase;
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
