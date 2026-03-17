package rtk

import "sort"

func countByCritic(findings []Finding) map[string]CountsByCritic {
	counts := map[string]CountsByCritic{}
	for _, finding := range findings {
		critic := finding.Critic
		if critic == "" {
			critic = "unknown"
		}
		entry := counts[critic]
		entry.Total++
		switch finding.Severity {
		case "high":
			entry.High++
		case "medium":
			entry.Medium++
		case "low":
			entry.Low++
		}
		if finding.Basis == "gap" {
			entry.Gap++
		}
		counts[critic] = entry
	}
	return counts
}

func roundFindings(round *RoundRecord) []Finding {
	if round == nil {
		return nil
	}
	return round.CurrentFindings()
}

func DeriveSessionSummary(session *Session) SessionSummary {
	rounds := session.Rounds
	openRound := session.OpenRound
	findings := []Finding{}
	for index := range rounds {
		findings = append(findings, roundFindings(&rounds[index])...)
	}
	findings = append(findings, roundFindings(openRound)...)
	var lastRound *RoundRecord
	if len(rounds) > 0 {
		lastRound = &rounds[len(rounds)-1]
	}
	latestRound := openRound
	if latestRound == nil {
		latestRound = lastRound
	}
	errorMessage := ""
	if session.Status.Error != nil {
		errorMessage = session.Status.Error.Message
	} else if openRound != nil && openRound.Error != nil {
		errorMessage = openRound.Error.Message
	}
	adjudicatedSummary := ""
	if session.AdjudicatedProposal != nil {
		adjudicatedSummary = session.AdjudicatedProposal.Summary
	}
	latestProposal := ""
	if latestRound != nil && latestRound.Proposal != nil {
		latestProposal = latestRound.Proposal.Summary
	}
	updatedAt := session.CreatedAt
	if latestRound != nil {
		if latestRound.UpdatedAt != "" {
			updatedAt = latestRound.UpdatedAt
		} else if latestRound.CreatedAt != "" {
			updatedAt = latestRound.CreatedAt
		}
	}
	gaps := 0
	for _, finding := range findings {
		if finding.Basis == "gap" {
			gaps++
		}
	}
	return SessionSummary{
		ID:                    session.ID,
		Topic:                 session.Topic,
		Chair:                 session.Chair,
		Critics:               session.Critics,
		MaxRounds:             cloneIntPtr(session.MaxRounds),
		Round:                 session.Status.Round,
		State:                 session.Status.State,
		Converged:             session.Status.Converged,
		UnresolvedHigh:        session.Status.UnresolvedHigh,
		UnresolvedMedium:      session.Status.UnresolvedMedium,
		ActiveActor:           session.Status.ActiveActor,
		ActivePhase:           session.Status.ActivePhase,
		ErrorMessage:          errorMessage,
		EvidenceCount:         len(session.Evidence),
		TotalFindings:         len(findings),
		GapFindings:           gaps,
		AdjudicatedSummary:    adjudicatedSummary,
		LatestProposalSummary: latestProposal,
		FindingsByCritic:      countByCritic(findings),
		HasOpenRound:          openRound != nil,
		UpdatedAt:             updatedAt,
		CreatedAt:             session.CreatedAt,
	}
}

func SortSessionSummaries(summaries []SessionSummary) {
	sort.Slice(summaries, func(i, j int) bool {
		a := summaries[i].UpdatedAt
		if a == "" {
			a = summaries[i].CreatedAt
		}
		b := summaries[j].UpdatedAt
		if b == "" {
			b = summaries[j].CreatedAt
		}
		if a != b {
			return a > b
		}
		return summaries[i].ID < summaries[j].ID
	})
}
