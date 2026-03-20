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
  const integerFormatter = new Intl.NumberFormat('en-US');

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
    const events = telemetryEntries(data.events || [], since);
    telemetry = reset || since === 0 ? events : [...telemetry, ...events];
    telemetryOffset = Number.isInteger(data.next_offset) ? data.next_offset : since + events.length;
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
      case 'stopped':
        return 'muted';
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
    return status === 'failed' ? 'bad' : status === 'succeeded' ? 'good' : status === 'running' ? 'warn' : status === 'stopped' ? 'muted' : 'muted';
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

  function formatInteger(value) {
    return integerFormatter.format(value || 0);
  }

  function formatCost(value) {
    if (!Number.isFinite(value)) return '';
    if (value >= 10) return `$${value.toFixed(2)}`;
    if (value >= 1) return `$${value.toFixed(3)}`;
    if (value >= 0.1) return `$${value.toFixed(4)}`;
    return `$${value.toFixed(5)}`;
  }

  function aggregateUsage(values) {
    const totals = {
      input_tokens: 0,
      output_tokens: 0,
      cache_read_input_tokens: 0,
      cache_creation_input_tokens: 0,
      known_cost_usd: 0,
      cost_count: 0,
      estimated_cost_count: 0,
      missing_cost_count: 0,
      usage_count: 0,
    };

    for (const usage of values || []) {
      if (!usage) continue;
      totals.usage_count += 1;
      totals.input_tokens += Number(usage.input_tokens || 0);
      totals.output_tokens += Number(usage.output_tokens || 0);
      totals.cache_read_input_tokens += Number(usage.cache_read_input_tokens || 0);
      totals.cache_creation_input_tokens += Number(usage.cache_creation_input_tokens || 0);

      if (typeof usage.cost_usd === 'number' && Number.isFinite(usage.cost_usd)) {
        totals.known_cost_usd += usage.cost_usd;
        totals.cost_count += 1;
        if (usage.cost_source && usage.cost_source !== 'provider') {
          totals.estimated_cost_count += 1;
        }
      } else {
        totals.missing_cost_count += 1;
      }
    }

    return totals;
  }

  function hasUsageTotals(totals) {
    return (totals?.usage_count || 0) > 0;
  }

  function usageSummaryText(totals) {
    if (!hasUsageTotals(totals)) return '';

    const parts = [];
    const approximate = totals.estimated_cost_count > 0;
    const costLabel = (value) => `${approximate ? '~' : ''}${formatCost(value)}`;
    if (totals.cost_count > 0 && totals.missing_cost_count === 0) {
      parts.push(costLabel(totals.known_cost_usd));
    } else if (totals.cost_count > 0) {
      parts.push(`known ${costLabel(totals.known_cost_usd)} + ${totals.missing_cost_count} unknown`);
    } else if (totals.missing_cost_count > 0) {
      parts.push(`cost ${totals.missing_cost_count} unknown`);
    }

    const tokenParts = [];
    if (totals.input_tokens > 0) tokenParts.push(`in ${formatInteger(totals.input_tokens)}`);
    if (totals.output_tokens > 0) tokenParts.push(`out ${formatInteger(totals.output_tokens)}`);
    if (totals.cache_read_input_tokens > 0) tokenParts.push(`cache-read ${formatInteger(totals.cache_read_input_tokens)}`);
    if (totals.cache_creation_input_tokens > 0) tokenParts.push(`cache-write ${formatInteger(totals.cache_creation_input_tokens)}`);
    if (tokenParts.length > 0) parts.push(tokenParts.join(' · '));

    return parts.join(' · ');
  }

  function phaseUsageText(usage) {
    if (!usage) return '';
    const summary = usageSummaryText(aggregateUsage([usage]));
    if (usage.model && summary) return `${usage.model} · ${summary}`;
    return usage.model || summary;
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
    if (type?.endsWith('stopped')) return 'muted';
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

  function roundLimitText(limit) {
    return limit == null ? 'unbounded' : `${limit}`;
  }

  function roundProgressText(round, limit, prefix = '') {
    return `${prefix}${round}/${roundLimitText(limit)}`;
  }

  function sessionMeta(session) {
    return [roundProgressText(session.round, session.max_rounds, 'r'), `${session.evidence_count} evidence`, unresolvedText(session.unresolved_high, session.unresolved_medium)].join(' · ');
  }

  function selectedMeta(session, summary) {
    return [
      `chair ${session.chair}`,
      `critics ${(session.critics || []).join(', ') || 'none'}`,
      `round ${roundProgressText(session.status.round, session.max_rounds)}`,
      `${session.evidence.length} evidence`,
      `${unresolvedText(session.status.unresolved_high, session.status.unresolved_medium)} unresolved`,
      `updated ${age(summary?.updated_at)}`,
    ].join(' · ');
  }

  function roundMeta(round, findings, usageTotals) {
    const parts = [`${round.review_summary?.total || findings.length} findings`, `${round.evidence_added.length} evidence`];
    const usage = usageSummaryText(usageTotals);
    if (usage) parts.push(usage);
    return parts.join(' · ');
  }

  function telemetryEntries(events, offset) {
    return events.map((event, index) => ({
      key: disclosureKey('telemetry-item', offset + index),
      event,
    }));
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

  function disclosure(node, options) {
    let current = disclosureOptions(options);

    syncDisclosure(node, current);

    const onToggle = () => {
      setDisclosureOpen(current.key, node.open);
    };

    node.addEventListener('toggle', onToggle);

    return {
      update(nextOptions) {
        const next = disclosureOptions(nextOptions);
        const shouldSync =
          next.key !== current.key ||
          (!(next.key in disclosureState) && next.fallback !== current.fallback);

        if (shouldSync) {
          current = next;
          syncDisclosure(node, current);
          return;
        }

        current = next;
      },
      destroy() {
        node.removeEventListener('toggle', onToggle);
      },
    };
  }

  function disclosureOptions(options) {
    return {
      key: options?.key || '',
      fallback: options?.fallback === true,
    };
  }

  function syncDisclosure(node, options) {
    if (!options.key) {
      return;
    }

    const nextOpen = disclosureOpen(options.key, options.fallback);
    if (node.open !== nextOpen) {
      node.open = nextOpen;
    }
  }

  $: selectedSummary = sessions.find((session) => session.id === selectedId) || null;
  $: evidenceById = evidenceMap(selected);
  $: selectedOpenRound = selected?.open_round || null;
  $: selectedRounds = selected ? [...(selected.rounds || []), ...(selectedOpenRound ? [selectedOpenRound] : [])] : [];
  $: selectedArtifact = currentArtifact(selected);
  $: selectedUsageTotals = aggregateUsage(selectedRounds.flatMap((round) => roundPhaseHistory(round).map((entry) => entry.usage)));

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
          {#each sessions as session (session.id)}
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
            {#if hasUsageTotals(selectedUsageTotals)}
              <p class="meta-copy">usage {usageSummaryText(selectedUsageTotals)}</p>
            {/if}

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
            {#each selectedRounds as round (disclosureKey('round', selected.id, round.index))}
              {@const findings = roundFindings(round)}
              {@const phases = roundPhaseHistory(round)}
              {@const phaseUsageTotals = aggregateUsage(phases.map((entry) => entry.usage))}
              {@const isOpenRound = selectedOpenRound?.index === round.index}
              {@const roundDisclosureKey = disclosureKey('round', selected.id, round.index)}

              <details
                class="round-item"
                use:disclosure={{ key: roundDisclosureKey, fallback: isOpenRound }}
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

                    <p class="meta-copy">{roundMeta(round, findings, phaseUsageTotals)}</p>
                  </div>
                </summary>

                <div class="round-body">
                  {#if round.proposal?.claims?.length}
                    <div class="token-row">
                      {#each round.proposal.claims as claim (claim)}
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
                  {:else if isOpenRound && selected.status.state === 'stopped'}
                    <div class="note-surface">
                      <p>Session was stopped before the round closed.</p>
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
                      use:disclosure={{ key: phaseHistoryDisclosureKey }}
                    >
                      <summary>phase history {phases.length}</summary>

                      <div class="stack stack--tight nested-stack">
                        {#each phases as entry (disclosureKey('phase', selected.id, round.index, entry.actor, entry.phase, entry.started_at))}
                          {@const phaseDisclosureKey = disclosureKey('phase', selected.id, round.index, entry.actor, entry.phase, entry.started_at)}
                          {@const phaseUsage = phaseUsageText(entry.usage)}
                          <details
                            class="stream-item"
                            use:disclosure={{ key: phaseDisclosureKey }}
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
                                  {#if phaseUsage}
                                    <span>{phaseUsage}</span>
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

                            {#if entry.usage}
                              <div class="detail-block">
                                <div class="meta-copy">usage</div>
                                <pre>{JSON.stringify(entry.usage, null, 2)}</pre>
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
          use:disclosure={{ key: evidenceSectionDisclosureKey }}
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
                {#each selected.evidence as evidence (evidence.id)}
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
                      use:disclosure={{ key: evidenceDisclosureKey }}
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
          use:disclosure={{ key: telemetrySectionDisclosureKey }}
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
                {#each [...telemetry].reverse().slice(0, 80) as entry (entry.key)}
                  {@const eventDisclosureKey = entry.key}
                  {@const event = entry.event}
                  <details
                    class="stream-item"
                    use:disclosure={{ key: eventDisclosureKey }}
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
