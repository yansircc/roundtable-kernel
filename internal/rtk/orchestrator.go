package rtk

import (
	"context"
	"fmt"
	"time"
)

type RunSessionOptions struct {
	Paths         Paths
	AdapterKind   string
	AdapterConfig AdapterConfig
	SessionID     string
	Force         bool
}

func collectEvidenceBatches(session *Session, evidenceKeyMap map[string]string, batches []EvidenceBatch, round int) ([]string, error) {
	addedIDs := []string{}
	for _, batch := range batches {
		if len(batch.Items) == 0 {
			continue
		}
		added, err := AppendEvidence(session, batch.Items, batch.CollectedBy, batch.Phase, round)
		if err != nil {
			return nil, err
		}
		for index, item := range batch.Items {
			if item.Source == "" && item.Statement == "" {
				continue
			}
			key := item.Key
			if key != "" {
				if _, ok := evidenceKeyMap[key]; ok {
					return nil, fmt.Errorf("duplicate evidence key %s", key)
				}
				evidenceKeyMap[key] = added[index].ID
			}
		}
		for _, evidence := range added {
			addedIDs = append(addedIDs, evidence.ID)
		}
	}
	return addedIDs, nil
}

func summarizeEvidenceBatches(value any) map[string]any {
	batches, _ := value.([]EvidenceBatch)
	itemCount := 0
	evidenceKeys := []string{}
	for _, batch := range batches {
		itemCount += len(batch.Items)
	}
	return map[string]any{
		"batch_count":   len(batches),
		"item_count":    itemCount,
		"evidence_keys": evidenceKeys,
	}
}

func summarizeProposal(value any) map[string]any {
	proposal, _ := value.(*Proposal)
	if proposal == nil {
		return map[string]any{"summary": "", "claim_count": 0, "acceptance_count": 0}
	}
	return map[string]any{
		"summary":          proposal.Summary,
		"claim_count":      len(proposal.Claims),
		"acceptance_count": len(proposal.Acceptance),
	}
}

func summarizeFindings(value any) map[string]any {
	findings, _ := value.([]Finding)
	counts := FindingCounts(findings)
	return map[string]any{
		"finding_count": len(findings),
		"high":          counts.High,
		"medium":        counts.Medium,
		"low":           counts.Low,
		"gaps":          counts.Gaps,
	}
}

func summarizeVerdict(value any) map[string]any {
	verdict, _ := value.(*Verdict)
	if verdict == nil {
		return map[string]any{"skipped": true}
	}
	accepted := 0
	rejected := 0
	for _, decision := range verdict.Decisions {
		if decision.Disposition == "accept" {
			accepted++
		}
		if decision.Disposition == "reject" {
			rejected++
		}
	}
	return map[string]any{
		"summary":        verdict.Summary,
		"decision_count": len(verdict.Decisions),
		"accepted":       accepted,
		"rejected":       rejected,
	}
}

type phaseRun struct {
	TelemetryFile string
	Session       *Session
	Round         int
	Actor         string
	Phase         string
	Adapter       string
	InputSummary  map[string]any
	Summarize     func(any) map[string]any
	OnSuccess     func(any) (map[string]any, error)
	Fn            func() (any, error)
	Paths         Paths
}

func runPhase(ctx context.Context, spec phaseRun) (any, error) {
	startedAt := nowISO()
	_ = startedAt
	if spec.Round > 0 {
		if err := RecordPhaseStart(spec.Session, spec.Round, spec.Actor, spec.Phase, spec.InputSummary); err != nil {
			return nil, err
		}
		if err := SaveSession(spec.Paths, spec.Session); err != nil {
			return nil, err
		}
	}
	_ = AppendTelemetryEvent(spec.TelemetryFile, map[string]any{
		"type":          "phase_started",
		"session_id":    spec.Session.ID,
		"round":         spec.Round,
		"actor":         spec.Actor,
		"phase":         spec.Phase,
		"adapter":       spec.Adapter,
		"input_summary": spec.InputSummary,
	})
	start := time.Now()
	result, err := spec.Fn()
	if err != nil {
		if spec.Round > 0 {
			_ = CompletePhase(spec.Session, spec.Round, spec.Actor, spec.Phase, nil, nil, time.Since(start).Milliseconds(), err)
			MarkSessionFailed(spec.Session, spec.Round, spec.Actor, spec.Phase, err)
			_ = SaveSession(spec.Paths, spec.Session)
		}
		_ = AppendTelemetryEvent(spec.TelemetryFile, map[string]any{
			"type":        "phase_failed",
			"session_id":  spec.Session.ID,
			"round":       spec.Round,
			"actor":       spec.Actor,
			"phase":       spec.Phase,
			"adapter":     spec.Adapter,
			"duration_ms": time.Since(start).Milliseconds(),
			"error":       SanitizeError(err),
		})
		return nil, err
	}
	var outputSummary map[string]any
	if spec.Summarize != nil {
		outputSummary = spec.Summarize(result)
	}
	var artifact map[string]any
	if spec.OnSuccess != nil {
		artifact, err = spec.OnSuccess(result)
		if err != nil {
			return nil, err
		}
	}
	if spec.Round > 0 {
		if err := CompletePhase(spec.Session, spec.Round, spec.Actor, spec.Phase, outputSummary, artifact, time.Since(start).Milliseconds(), nil); err != nil {
			return nil, err
		}
		if err := SaveSession(spec.Paths, spec.Session); err != nil {
			return nil, err
		}
	} else if spec.OnSuccess != nil {
		if err := SaveSession(spec.Paths, spec.Session); err != nil {
			return nil, err
		}
	}
	_ = AppendTelemetryEvent(spec.TelemetryFile, map[string]any{
		"type":           "phase_succeeded",
		"session_id":     spec.Session.ID,
		"round":          spec.Round,
		"actor":          spec.Actor,
		"phase":          spec.Phase,
		"adapter":        spec.Adapter,
		"duration_ms":    time.Since(start).Milliseconds(),
		"output_summary": outputSummary,
	})
	return result, nil
}

func RunSession(ctx context.Context, options RunSessionOptions) (*Session, error) {
	bootstrap, err := CreateAdapter(options.AdapterKind, options.AdapterConfig)
	if err != nil {
		return nil, err
	}
	metadata := bootstrap.Metadata()
	session, err := NewSession(options.SessionID, metadata.Topic, metadata.Chair, metadata.Critics, metadata.MaxRounds, bootstrap.Kind())
	if err != nil {
		return nil, err
	}
	if err := CreateSessionFile(options.Paths, session, options.Force); err != nil {
		return nil, err
	}
	telemetryFile, err := ResetTelemetryFile(options.Paths, session.ID)
	if err != nil {
		return nil, err
	}
	adapter, err := CreateAdapter(options.AdapterKind, AdapterConfig{
		FixturePath:   options.AdapterConfig.FixturePath,
		SpecPath:      options.AdapterConfig.SpecPath,
		TelemetryFile: telemetryFile,
	})
	if err != nil {
		return nil, err
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":       "session_started",
		"session_id": session.ID,
		"adapter":    session.Adapter,
		"topic":      session.Topic,
		"chair":      session.Chair,
		"critics":    session.Critics,
		"max_rounds": session.MaxRounds,
	})

	evidenceKeyMap := map[string]string{}
	fail := func(runErr error) (*Session, error) {
		if session.Status.State != "failed" {
			round := session.Status.Round
			if session.OpenRound != nil {
				round = session.OpenRound.Index
			}
			actor := ""
			if session.Status.ActiveActor != nil {
				actor = *session.Status.ActiveActor
			}
			phase := ""
			if session.Status.ActivePhase != nil {
				phase = *session.Status.ActivePhase
			}
			MarkSessionFailed(session, round, actor, phase, runErr)
			_ = SaveSession(options.Paths, session)
		}
		_ = AppendTelemetryEvent(telemetryFile, map[string]any{
			"type":       "session_failed",
			"session_id": session.ID,
			"adapter":    session.Adapter,
			"round":      session.Status.Round,
			"error":      SanitizeError(runErr),
		})
		return nil, runErr
	}

	_, err = runPhase(ctx, phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         0,
		Actor:         session.Chair,
		Phase:         "seed",
		Adapter:       session.Adapter,
		Summarize:     summarizeEvidenceBatches,
		Paths:         options.Paths,
		Fn: func() (any, error) {
			return adapter.SeedEvidence(ctx)
		},
		OnSuccess: func(value any) (map[string]any, error) {
			batches, _ := value.([]EvidenceBatch)
			_, err := collectEvidenceBatches(session, evidenceKeyMap, batches, 0)
			return nil, err
		},
	})
	if err != nil {
		return fail(err)
	}

	for round := 1; round <= session.MaxRounds; round++ {
		_, err = runPhase(ctx, phaseRun{
			TelemetryFile: telemetryFile,
			Session:       session,
			Round:         round,
			Actor:         session.Chair,
			Phase:         "explore",
			Adapter:       session.Adapter,
			Summarize:     summarizeEvidenceBatches,
			Paths:         options.Paths,
			Fn: func() (any, error) {
				return adapter.CollectEvidence(ctx, CollectEvidenceArgs{Session: session, Round: round, Actor: session.Chair, Phase: "explore"})
			},
			OnSuccess: func(value any) (map[string]any, error) {
				batches, _ := value.([]EvidenceBatch)
				added, err := collectEvidenceBatches(session, evidenceKeyMap, batches, round)
				if err != nil {
					return nil, err
				}
				if err := NoteRoundEvidence(session, round, added); err != nil {
					return nil, err
				}
				return map[string]any{"evidence_added": added}, nil
			},
		})
		if err != nil {
			return fail(err)
		}

		_, err = runPhase(ctx, phaseRun{
			TelemetryFile: telemetryFile,
			Session:       session,
			Round:         round,
			Actor:         session.Chair,
			Phase:         "propose",
			Adapter:       session.Adapter,
			Summarize:     summarizeProposal,
			Paths:         options.Paths,
			Fn: func() (any, error) {
				return adapter.Propose(ctx, ProposeArgs{Session: session, Round: round})
			},
			OnSuccess: func(value any) (map[string]any, error) {
				proposal, _ := value.(*Proposal)
				if err := RegisterProposal(session, round, proposal); err != nil {
					return nil, err
				}
				return map[string]any{"proposal": proposal}, nil
			},
		})
		if err != nil {
			return fail(err)
		}

		proposal := session.OpenRound.Proposal
		findings := []Finding{}

		for _, critic := range session.Critics {
			criticName := critic
			_, err = runPhase(ctx, phaseRun{
				TelemetryFile: telemetryFile,
				Session:       session,
				Round:         round,
				Actor:         criticName,
				Phase:         "re-explore",
				Adapter:       session.Adapter,
				InputSummary:  map[string]any{"proposal_summary": proposal.Summary},
				Summarize:     summarizeEvidenceBatches,
				Paths:         options.Paths,
				Fn: func() (any, error) {
					return adapter.CollectEvidence(ctx, CollectEvidenceArgs{Session: session, Round: round, Actor: criticName, Phase: "re-explore"})
				},
				OnSuccess: func(value any) (map[string]any, error) {
					batches, _ := value.([]EvidenceBatch)
					added, err := collectEvidenceBatches(session, evidenceKeyMap, batches, round)
					if err != nil {
						return nil, err
					}
					if err := NoteRoundEvidence(session, round, added); err != nil {
						return nil, err
					}
					return map[string]any{"evidence_added": added}, nil
				},
			})
			if err != nil {
				return fail(err)
			}

			_, err = runPhase(ctx, phaseRun{
				TelemetryFile: telemetryFile,
				Session:       session,
				Round:         round,
				Actor:         criticName,
				Phase:         "review",
				Adapter:       session.Adapter,
				InputSummary:  map[string]any{"proposal_summary": proposal.Summary},
				Summarize:     summarizeFindings,
				Paths:         options.Paths,
				Fn: func() (any, error) {
					return adapter.Review(ctx, ReviewArgs{
						Session:        session,
						Round:          round,
						Critic:         criticName,
						Proposal:       proposal,
						EvidenceKeyMap: evidenceKeyMap,
					})
				},
				OnSuccess: func(value any) (map[string]any, error) {
					criticFindings, _ := value.([]Finding)
					if err := AppendRoundFindings(session, round, criticFindings); err != nil {
						return nil, err
					}
					findings = append(findings, criticFindings...)
					return map[string]any{"findings_against_proposal": criticFindings}, nil
				},
			})
			if err != nil {
				return fail(err)
			}
		}

		_, err = runPhase(ctx, phaseRun{
			TelemetryFile: telemetryFile,
			Session:       session,
			Round:         round,
			Actor:         session.Chair,
			Phase:         "adjudicate",
			Adapter:       session.Adapter,
			InputSummary: map[string]any{
				"proposal_summary": proposal.Summary,
				"finding_count":    len(findings),
			},
			Summarize: summarizeVerdict,
			Paths:     options.Paths,
			Fn: func() (any, error) {
				return adapter.Adjudicate(ctx, AdjudicateArgs{
					Session:        session,
					Round:          round,
					Proposal:       proposal,
					Findings:       findings,
					EvidenceKeyMap: evidenceKeyMap,
				})
			},
			OnSuccess: func(value any) (map[string]any, error) {
				verdict, _ := value.(*Verdict)
				if err := RegisterVerdict(session, round, verdict); err != nil {
					return nil, err
				}
				return map[string]any{"verdict": verdict}, nil
			},
		})
		if err != nil {
			return fail(err)
		}

		if _, err := ApplyRound(session); err != nil {
			return fail(err)
		}
		if err := SaveSession(options.Paths, session); err != nil {
			return fail(err)
		}
		if session.Status.Converged {
			break
		}
	}

	if !session.Status.Converged && session.Status.Round == session.MaxRounds {
		session.Status.State = "exhausted"
	}
	if err := SaveSession(options.Paths, session); err != nil {
		return fail(err)
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":              "session_finished",
		"session_id":        session.ID,
		"adapter":           session.Adapter,
		"state":             session.Status.State,
		"round":             session.Status.Round,
		"converged":         session.Status.Converged,
		"unresolved_high":   session.Status.UnresolvedHigh,
		"unresolved_medium": session.Status.UnresolvedMedium,
	})
	return session, nil
}
