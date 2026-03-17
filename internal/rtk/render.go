package rtk

import (
	"fmt"
	"strings"
)

func RenderSession(session *Session) string {
	lines := []string{
		fmt.Sprintf("session: %s", session.ID),
		fmt.Sprintf("topic:   %s", session.Topic),
		fmt.Sprintf("chair:   %s", session.Chair),
		fmt.Sprintf("critics: %s", strings.Join(session.Critics, ", ")),
		fmt.Sprintf("adapter: %s", session.Adapter),
		fmt.Sprintf("status:  %s", session.Status.State),
		fmt.Sprintf("round:   %s", roundProgressText(session.Status.Round, session.MaxRounds)),
		fmt.Sprintf("evidence:%4d", len(session.Evidence)),
		fmt.Sprintf("high:    %d", session.Status.UnresolvedHigh),
		fmt.Sprintf("medium:  %d", session.Status.UnresolvedMedium),
	}
	if session.Status.ActiveActor != nil && session.Status.ActivePhase != nil {
		lines = append(lines, fmt.Sprintf("active:  %s/%s", *session.Status.ActiveActor, *session.Status.ActivePhase))
	}
	if session.Status.Error != nil {
		lines = append(lines, fmt.Sprintf("error:   %s", session.Status.Error.Message))
	}
	lines = append(lines, "", "adjudicated proposal:")
	if session.AdjudicatedProposal != nil {
		lines = append(lines, "  "+session.AdjudicatedProposal.Summary)
	} else {
		lines = append(lines, "  none")
	}
	lines = append(lines, "")

	for _, round := range session.Rounds {
		lines = append(lines, fmt.Sprintf("round %d", round.Index))
		if round.Proposal != nil {
			lines = append(lines, "  proposal: "+round.Proposal.Summary)
		} else {
			lines = append(lines, "  proposal: none")
		}
		lines = append(lines, fmt.Sprintf("  evidence added: %d", len(round.EvidenceAdded)))
		lines = append(lines, fmt.Sprintf("  findings: total=%d high=%d medium=%d low=%d gaps=%d", round.ReviewSummary.Total, round.ReviewSummary.High, round.ReviewSummary.Medium, round.ReviewSummary.Low, round.ReviewSummary.Gaps))
		if round.Verdict != nil {
			accepted := 0
			rejected := 0
			for _, decision := range round.Verdict.Decisions {
				if decision.Disposition == "accept" {
					accepted++
				}
				if decision.Disposition == "reject" {
					rejected++
				}
			}
			lines = append(lines, "  verdict:  "+round.Verdict.Summary)
			lines = append(lines, fmt.Sprintf("  decisions:%d accepted / %d rejected", accepted, rejected))
		} else {
			lines = append(lines, "  verdict:  skipped")
		}
		lines = append(lines, "")
	}

	if session.OpenRound != nil {
		open := session.OpenRound
		lines = append(lines, fmt.Sprintf("open round %d", open.Index))
		if open.Proposal != nil {
			lines = append(lines, "  proposal: "+open.Proposal.Summary)
		} else {
			lines = append(lines, "  proposal: none")
		}
		lines = append(lines, fmt.Sprintf("  evidence added: %d", len(open.EvidenceAdded)))
		lines = append(lines, fmt.Sprintf("  findings: total=%d high=%d medium=%d low=%d gaps=%d", open.ReviewSummary.Total, open.ReviewSummary.High, open.ReviewSummary.Medium, open.ReviewSummary.Low, open.ReviewSummary.Gaps))
		lines = append(lines, fmt.Sprintf("  phases:   %d", len(open.PhaseHistory)))
		if open.Error != nil {
			lines = append(lines, "  error:    "+open.Error.Message)
		}
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}
