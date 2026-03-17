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
  let disclosureState = {};

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

  function unresolvedText(high, medium) {
    return `${high} high / ${medium} medium`;
  }

  function activityText(actor, phase) {
    return actor && phase ? `● ${actor} / ${phase}` : 'idle';
  }

  function sessionMeta(session) {
    return [`r${session.round}/${session.max_rounds}`, `${session.evidence_count} evidence`, unresolvedText(session.unresolved_high, session.unresolved_medium)].join(' · ');
  }

  function selectedMeta(session, summary) {
    return [
      `chair ${session.chair}`,
      `critics ${(session.critics || []).join(', ') || 'none'}`,
      `round ${session.status.round}/${session.max_rounds}`,
      `${session.evidence.length} evidence`,
      `${unresolvedText(session.status.unresolved_high, session.status.unresolved_medium)} unresolved`,
      `updated ${age(summary?.updated_at)}`,
    ].join(' · ');
  }

  function roundMeta(round, findings) {
    return [`${round.review_summary?.total || findings.length} findings`, `${round.evidence_added.length} evidence`].join(' · ');
  }

  function disclosureKey(...parts) {
    return parts
      .filter((part) => part !== undefined && part !== null && part !== '')
      .join(':');
  }

  function disclosureOpen(key, fallback = false) {
    return key in disclosureState ? disclosureState[key] : fallback;
  }

  function setDisclosureOpen(key, open) {
    disclosureState = { ...disclosureState, [key]: open };
  }

  function telemetryEventKey(event) {
    return disclosureKey('telemetry-item', event.ts, event.type, event.actor, event.phase, event.channel, event.source);
  }

  $: selectedSummary = sessions.find((session) => session.id === selectedId) || null;
  $: evidenceById = evidenceMap(selected);
  $: selectedOpenRound = selected?.open_round || null;
  $: selectedRounds = selected ? [...(selected.rounds || []), ...(selectedOpenRound ? [selectedOpenRound] : [])] : [];
  $: selectedArtifact = currentArtifact(selected);
  $: if (selected?.id) {
    const nextState = { ...disclosureState };
    let changed = false;

    for (const round of selectedRounds) {
      const key = disclosureKey('round', selected.id, round.index);
      if (!(key in nextState)) {
        nextState[key] = selectedOpenRound?.index === round.index;
        changed = true;
      }
    }

    if (changed) {
      disclosureState = nextState;
    }
  }

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

<div class="pf-page">
  <header class="pf-header">
    <div class="pf-header__title">
      <h1>Roundtable Kernel</h1>
      <p class="meta-copy">{projectRoot || 'loading project root...'}</p>
    </div>

    <p class="header-law">baseline facts -> challenge -> targeted re-explore -> verdict -> next baseline</p>
  </header>

  {#if error}
    <div class="note-surface note-surface--bad">
      <p>{error}</p>
    </div>
  {/if}

  <div class="pf-layout">
    <aside class="section pf-sidebar">
      <div class="section-head">
        <h2>Sessions</h2>
        <span class="section-count">{sessions.length}</span>
      </div>

      {#if sessions.length === 0}
        <div class="empty-state">No sessions available.</div>
      {:else}
        <div class="stack">
          {#each sessions as session}
            <button
              type="button"
              class:is-active={selectedId === session.id}
              class="session-item"
              on:click={() => selectSession(session.id)}
            >
              <div class="item-head">
                <span class="session-id">{session.id}</span>
                <span class="tag {tone(session.state)}">{session.state}</span>
              </div>

              <p class="body-copy session-topic">{session.topic}</p>
              <p class="meta-copy compact-line">{sessionMeta(session)}</p>

              {#if session.active_actor && session.active_phase}
                <p class="meta-copy compact-line">live {activityText(session.active_actor, session.active_phase)}</p>
              {:else if session.error_message}
                <p class="meta-copy compact-line meta-row--bad">{session.error_message}</p>
              {/if}
            </button>
          {/each}
        </div>
      {/if}
    </aside>

    <main class="pf-document">
      {#if selected}
        <section class="section">
          <div class="section-head">
            <h2>{selected.id}</h2>
            <span class="tag {tone(selected.status.state)}">{selected.status.state}</span>
          </div>

          <div class="stack">
            <p class="lead-copy">{selected.topic}</p>
            <p class="meta-copy facts-line">{selectedMeta(selected, selectedSummary)}</p>

            <div class="note-surface">
              <p>{selectedArtifact?.summary || 'No proposal material yet.'}</p>
            </div>

            {#if selected.open_round}
              <p class="meta-copy">open round {selected.open_round.index} remains provisional</p>
            {/if}

            {#if selected.status.active_actor && selected.status.active_phase}
              <p class="meta-copy">live {activityText(selected.status.active_actor, selected.status.active_phase)}</p>
            {/if}

            {#if selected.status?.error?.message}
              <div class="note-surface note-surface--bad">
                <p>{selected.status.error.message}</p>
              </div>
            {/if}
          </div>
        </section>

        <section class="section">
          <div class="section-head">
            <h2>Rounds</h2>
            <span class="section-count">{selectedRounds.length}</span>
          </div>

          <div class="stack">
            {#each selectedRounds as round}
              {@const findings = roundFindings(round)}
              {@const phases = roundPhaseHistory(round)}
              {@const isOpenRound = selectedOpenRound?.index === round.index}
              {@const roundDisclosureKey = disclosureKey('round', selected.id, round.index)}

              <details
                class="round-item"
                open={disclosureOpen(roundDisclosureKey, isOpenRound)}
                on:toggle={(event) => setDisclosureOpen(roundDisclosureKey, event.currentTarget.open)}
              >
                <summary>
                  <div class="item-head">
                    <div class="stack stack--compact">
                      <div class="tag-row">
                        <span class="session-id">round {round.index}</span>
                        <span class="tag {isOpenRound ? tone(selected.status.state) : 'muted'}">{isOpenRound ? selected.status.state : 'closed'}</span>
                      </div>
                      <p class="body-copy">{round.proposal?.summary || 'No proposal yet.'}</p>
                    </div>

                    <p class="meta-copy">{roundMeta(round, findings)}</p>
                  </div>
                </summary>

                <div class="round-body">
                  {#if round.proposal?.claims?.length}
                    <div class="token-row">
                      {#each round.proposal.claims as claim}
                        <span class="token">{claim}</span>
                      {/each}
                    </div>
                  {/if}

                  {#if findings.length === 0}
                    <div class="note-surface note-surface--good">
                      <p>Clean critic pass. No material findings remain.</p>
                    </div>
                  {:else}
                    <div class="stack stack--tight">
                      {#each findings as finding}
                        <article class="list-item">
                          <div class="item-head">
                            <div class="tag-row">
                              <span class="tag {severityTone(finding.severity)}">{finding.severity}</span>
                              <span class="tag {finding.basis === 'gap' ? 'gap' : 'supported'}">{finding.basis}</span>
                            </div>
                            <span class="meta-copy">{finding.critic}</span>
                          </div>

                          <p class="body-copy finding-summary">{finding.summary}</p>
                          <p class="body-copy">{finding.rationale}</p>
                          <p class="body-copy finding-change">change: {finding.suggested_change}</p>

                          <div class="token-row">
                            {#if finding.evidence_ids.length === 0}
                              <span class="token">gap: no evidence yet</span>
                            {:else}
                              {#each finding.evidence_ids as evidenceId}
                                {@const evidence = evidenceById.get(evidenceId)}
                                <span class="token" title={evidence?.statement || ''}>
                                  {evidenceId} {evidence?.phase || ''}
                                </span>
                              {/each}
                            {/if}
                          </div>
                        </article>
                      {/each}
                    </div>
                  {/if}

                  {#if round.verdict}
                    <article class="list-item">
                      <p class="meta-copy">verdict</p>
                      <p class="body-copy finding-summary">{round.verdict.summary}</p>

                      <div class="stack stack--compact">
                        {#each round.verdict.decisions as decision}
                          <div class="item-head item-head--compact">
                            <span class="decision-id">{decision.finding_id}</span>
                            <span class="tag {decision.disposition === 'accept' ? 'good' : 'bad'}">{decision.disposition}</span>
                          </div>
                        {/each}
                      </div>
                    </article>
                  {:else if isOpenRound && selected.status.state === 'failed'}
                    <div class="note-surface note-surface--bad">
                      <p>Interrupted before adjudication.</p>
                    </div>
                  {:else if isOpenRound}
                    <div class="note-surface">
                      <p>Round is still in flight.</p>
                    </div>
                  {:else}
                    <div class="note-surface">
                      <p>Skipped because the critic pass already converged.</p>
                    </div>
                  {/if}

                  {#if phases.length > 0}
                    {@const phaseHistoryDisclosureKey = disclosureKey('phase-history', selected.id, round.index)}
                    <details
                      class="inline-details"
                      open={disclosureOpen(phaseHistoryDisclosureKey)}
                      on:toggle={(event) => setDisclosureOpen(phaseHistoryDisclosureKey, event.currentTarget.open)}
                    >
                      <summary>phase history {phases.length}</summary>

                      <div class="stack stack--tight nested-stack">
                        {#each phases as entry}
                          {@const phaseDisclosureKey = disclosureKey('phase', selected.id, round.index, entry.actor, entry.phase, entry.started_at)}
                          <details
                            class="stream-item"
                            open={disclosureOpen(phaseDisclosureKey)}
                            on:toggle={(event) => setDisclosureOpen(phaseDisclosureKey, event.currentTarget.open)}
                          >
                            <summary>
                              <div class="item-head">
                                <div class="tag-row">
                                  <span class="tag {phaseStatusTone(entry.status)}">{entry.status}</span>
                                  <span class="stream-subject">{entry.actor} / {entry.phase}</span>
                                </div>

                                <div class="meta-row">
                                  <span>{clock(entry.started_at)}</span>
                                  {#if entry.duration_ms !== null}
                                    <span>{entry.duration_ms}ms</span>
                                  {/if}
                                </div>
                              </div>
                            </summary>

                            {#if entry.input_summary}
                              <div class="detail-block">
                                <div class="meta-copy">input</div>
                                <pre>{JSON.stringify(entry.input_summary, null, 2)}</pre>
                              </div>
                            {/if}

                            {#if entry.output_summary}
                              <div class="detail-block">
                                <div class="meta-copy">output</div>
                                <pre>{JSON.stringify(entry.output_summary, null, 2)}</pre>
                              </div>
                            {/if}

                            {#if entry.artifact}
                              <div class="detail-block">
                                <div class="meta-copy">artifact</div>
                                <pre>{JSON.stringify(entry.artifact, null, 2)}</pre>
                              </div>
                            {/if}

                            {#if entry.error}
                              <div class="detail-block">
                                <div class="meta-copy">error</div>
                                <pre>{JSON.stringify(entry.error, null, 2)}</pre>
                              </div>
                            {/if}
                          </details>
                        {/each}
                      </div>
                    </details>
                  {/if}
                </div>
              </details>
            {/each}
          </div>
        </section>

        {@const evidenceSectionDisclosureKey = disclosureKey('section', selected.id, 'evidence')}
        <details
          class="section fold-section"
          open={disclosureOpen(evidenceSectionDisclosureKey)}
          on:toggle={(event) => setDisclosureOpen(evidenceSectionDisclosureKey, event.currentTarget.open)}
        >
          <summary>
            <div class="section-head">
              <h2>Evidence</h2>
              <span class="section-count">{selected.evidence.length}</span>
            </div>
          </summary>

          <div class="section-body">
            {#if selected.evidence.length === 0}
              <div class="empty-state">No evidence yet.</div>
            {:else}
              <div class="stack">
                {#each selected.evidence as evidence}
                  {@const evidenceDisclosureKey = disclosureKey('evidence', selected.id, evidence.id)}
                  <article class="stream-item">
                    <div class="item-head">
                      <div class="tag-row">
                        <span class="session-id">{evidence.id}</span>
                        <span class="tag muted">{evidence.phase}</span>
                      </div>
                      <span class="meta-copy">r{evidence.round} · {evidence.collected_by}</span>
                    </div>

                    <p class="body-copy">{evidence.statement}</p>
                    <div class="meta-copy">{evidence.source}</div>

                    <details
                      class="inline-details"
                      open={disclosureOpen(evidenceDisclosureKey)}
                      on:toggle={(event) => setDisclosureOpen(evidenceDisclosureKey, event.currentTarget.open)}
                    >
                      <summary>{clock(evidence.created_at)} · excerpt</summary>
                      <pre>{evidence.excerpt}</pre>
                    </details>
                  </article>
                {/each}
              </div>
            {/if}
          </div>
        </details>

        {@const telemetrySectionDisclosureKey = disclosureKey('section', selected.id, 'telemetry')}
        <details
          class="section fold-section"
          open={disclosureOpen(telemetrySectionDisclosureKey)}
          on:toggle={(event) => setDisclosureOpen(telemetrySectionDisclosureKey, event.currentTarget.open)}
        >
          <summary>
            <div class="section-head">
              <h2>Telemetry</h2>
              <span class="section-count">{telemetry.length}</span>
            </div>
          </summary>

          <div class="section-body">
            <p class="meta-copy">Transport side-channel only. Durable semantic truth lives in the round records above.</p>

            {#if telemetry.length === 0}
              <div class="empty-state">No telemetry yet.</div>
            {:else}
              <div class="stack">
                {#each [...telemetry].reverse().slice(0, 80) as event}
                  {@const eventDisclosureKey = telemetryEventKey(event)}
                  <details
                    class="stream-item"
                    open={disclosureOpen(eventDisclosureKey)}
                    on:toggle={(event) => setDisclosureOpen(eventDisclosureKey, event.currentTarget.open)}
                  >
                    <summary>
                      <div class="item-head">
                        <div class="tag-row">
                          <span class="tag {eventTone(event.type)}">{event.type}</span>
                          <span class="stream-subject">{eventSubject(event)}</span>
                        </div>

                        <div class="meta-row">
                          <span>{clock(event.ts)}</span>
                          {#if event.duration_ms !== undefined}
                            <span>{event.duration_ms}ms</span>
                          {/if}
                          {#if event.source}
                            <span>{event.source}</span>
                          {/if}
                        </div>
                      </div>
                    </summary>

                    <pre>{JSON.stringify(event, null, 2)}</pre>
                  </details>
                {/each}
              </div>
            {/if}
          </div>
        </details>
      {:else}
        <section class="section">
          <div class="empty-state">No session selected.<br/><br/>Choose a session from the sidebar to view its details.</div>
        </section>
      {/if}
    </main>
  </div>
</div>
