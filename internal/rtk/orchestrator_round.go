package rtk

import "context"

func appendCollectedEvidence(session *Session, round int, evidenceKeyMap map[string]string, batches []EvidenceBatch) ([]string, error) {
	added, err := collectEvidenceBatches(session, evidenceKeyMap, batches, round)
	if err != nil {
		return nil, err
	}
	if round > 0 {
		if err := NoteRoundEvidence(session, round, added); err != nil {
			return nil, err
		}
	}
	return added, nil
}

func runSeedPhase(ctx context.Context, adapter *execAdapter, session *Session, evidenceKeyMap map[string]string, telemetryFile string, paths Paths) error {
	_, err := runPhase(phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         0,
		Actor:         session.Chair,
		Phase:         "seed",
		Adapter:       session.Adapter,
		Summarize:     summarizeEvidenceBatches,
		Paths:         paths,
		Fn: func() (AgentPhaseResult, error) {
			batches, err := adapter.SeedEvidence(ctx)
			return AgentPhaseResult{Value: batches}, err
		},
		OnSuccess: func(value any) (map[string]any, error) {
			batches, _ := value.([]EvidenceBatch)
			_, err := appendCollectedEvidence(session, 0, evidenceKeyMap, batches)
			return nil, err
		},
	})
	return err
}

func runCriticRound(ctx context.Context, adapter *execAdapter, session *Session, round int, critic string, proposal *Proposal, findings *[]Finding, evidenceKeyMap map[string]string, telemetryFile string, paths Paths) error {
	_, err := runPhase(phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         round,
		Actor:         critic,
		Phase:         "re-explore",
		Adapter:       session.Adapter,
		InputSummary:  map[string]any{"proposal_summary": proposal.Summary},
		Summarize:     summarizeEvidenceBatches,
		Paths:         paths,
		Fn: func() (AgentPhaseResult, error) {
			return adapter.CollectEvidence(ctx, CollectEvidenceArgs{Session: session, Round: round, Actor: critic, Phase: "re-explore"})
		},
		OnSuccess: func(value any) (map[string]any, error) {
			batches, _ := value.([]EvidenceBatch)
			added, err := appendCollectedEvidence(session, round, evidenceKeyMap, batches)
			if err != nil {
				return nil, err
			}
			return map[string]any{"evidence_added": added}, nil
		},
	})
	if err != nil {
		return err
	}

	_, err = runPhase(phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         round,
		Actor:         critic,
		Phase:         "review",
		Adapter:       session.Adapter,
		InputSummary:  map[string]any{"proposal_summary": proposal.Summary},
		Summarize:     summarizeFindings,
		Paths:         paths,
		Fn: func() (AgentPhaseResult, error) {
			return adapter.Review(ctx, ReviewArgs{
				Session:        session,
				Round:          round,
				Critic:         critic,
				Proposal:       proposal,
				EvidenceKeyMap: evidenceKeyMap,
			})
		},
		OnSuccess: func(value any) (map[string]any, error) {
			criticFindings, _ := value.([]Finding)
			if err := AppendRoundFindings(session, round, criticFindings); err != nil {
				return nil, err
			}
			*findings = append(*findings, criticFindings...)
			return map[string]any{"findings_against_proposal": criticFindings}, nil
		},
	})
	return err
}

func runRound(ctx context.Context, adapter *execAdapter, session *Session, round int, evidenceKeyMap map[string]string, telemetryFile string, paths Paths) error {
	_, err := runPhase(phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         round,
		Actor:         session.Chair,
		Phase:         "explore",
		Adapter:       session.Adapter,
		Summarize:     summarizeEvidenceBatches,
		Paths:         paths,
		Fn: func() (AgentPhaseResult, error) {
			return adapter.CollectEvidence(ctx, CollectEvidenceArgs{Session: session, Round: round, Actor: session.Chair, Phase: "explore"})
		},
		OnSuccess: func(value any) (map[string]any, error) {
			batches, _ := value.([]EvidenceBatch)
			added, err := appendCollectedEvidence(session, round, evidenceKeyMap, batches)
			if err != nil {
				return nil, err
			}
			return map[string]any{"evidence_added": added}, nil
		},
	})
	if err != nil {
		return err
	}

	_, err = runPhase(phaseRun{
		TelemetryFile: telemetryFile,
		Session:       session,
		Round:         round,
		Actor:         session.Chair,
		Phase:         "propose",
		Adapter:       session.Adapter,
		Summarize:     summarizeProposal,
		Paths:         paths,
		Fn: func() (AgentPhaseResult, error) {
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
		return err
	}

	proposal := session.OpenRound.Proposal
	findings := []Finding{}
	for _, critic := range session.Critics {
		if err := runCriticRound(ctx, adapter, session, round, critic, proposal, &findings, evidenceKeyMap, telemetryFile, paths); err != nil {
			return err
		}
	}

	_, err = runPhase(phaseRun{
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
		Paths:     paths,
		Fn: func() (AgentPhaseResult, error) {
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
		return err
	}

	if _, err := ApplyRound(session); err != nil {
		return err
	}
	return SaveSession(paths, session)
}
