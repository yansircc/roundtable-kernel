package rtk

type AgentRequest struct {
	Protocol string    `json:"protocol"`
	Actor    string    `json:"actor"`
	Phase    string    `json:"phase"`
	Round    int       `json:"round"`
	Session  *Session  `json:"session"`
	Proposal *Proposal `json:"proposal"`
	Findings []Finding `json:"findings"`
}

func compactFindings(round *RoundRecord) []map[string]any {
	findings := []map[string]any{}
	for _, finding := range roundFindings(round) {
		findings = append(findings, map[string]any{
			"id":           finding.ID,
			"critic":       finding.Critic,
			"severity":     finding.Severity,
			"basis":        finding.Basis,
			"summary":      finding.Summary,
			"evidence_ids": finding.EvidenceIDs,
		})
	}
	return findings
}

func CompactSession(session *Session) map[string]any {
	if session == nil {
		return nil
	}
	rounds := []map[string]any{}
	for index := range session.Rounds {
		round := &session.Rounds[index]
		proposal := ""
		if round.Proposal != nil {
			proposal = round.Proposal.Summary
		}
		verdict := any(nil)
		if round.Verdict != nil {
			verdict = round.Verdict.Summary
		}
		rounds = append(rounds, map[string]any{
			"index":                     round.Index,
			"proposal":                  proposal,
			"findings_against_proposal": compactFindings(round),
			"verdict":                   verdict,
		})
	}
	var openRound any
	if session.OpenRound != nil {
		proposal := ""
		if session.OpenRound.Proposal != nil {
			proposal = session.OpenRound.Proposal.Summary
		}
		verdict := any(nil)
		if session.OpenRound.Verdict != nil {
			verdict = session.OpenRound.Verdict.Summary
		}
		phaseHistory := []map[string]any{}
		for _, phase := range session.OpenRound.PhaseHistory {
			phaseHistory = append(phaseHistory, map[string]any{
				"actor":          phase.Actor,
				"phase":          phase.Phase,
				"status":         phase.Status,
				"output_summary": phase.OutputSummary,
			})
		}
		openRound = map[string]any{
			"index":                     session.OpenRound.Index,
			"proposal":                  proposal,
			"findings_against_proposal": compactFindings(session.OpenRound),
			"verdict":                   verdict,
			"phase_history":             phaseHistory,
			"error":                     session.OpenRound.Error,
		}
	}
	var adjudicated any
	if session.AdjudicatedProposal != nil {
		adjudicated = session.AdjudicatedProposal
	}
	evidence := []map[string]any{}
	for _, item := range session.Evidence {
		evidence = append(evidence, map[string]any{
			"id":        item.ID,
			"phase":     item.Phase,
			"round":     item.Round,
			"source":    item.Source,
			"statement": item.Statement,
		})
	}
	return map[string]any{
		"id":                   session.ID,
		"topic":                session.Topic,
		"chair":                session.Chair,
		"critics":              session.Critics,
		"max_rounds":           session.MaxRounds,
		"status":               session.Status,
		"adjudicated_proposal": adjudicated,
		"evidence":             evidence,
		"rounds":               rounds,
		"open_round":           openRound,
	}
}
