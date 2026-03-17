package rtk

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Step struct {
	SessionID    string         `json:"session_id"`
	Round        int            `json:"round"`
	Actor        string         `json:"actor"`
	Phase        string         `json:"phase"`
	StartedAt    string         `json:"started_at,omitempty"`
	InputSummary map[string]any `json:"input_summary,omitempty"`
	Request      AgentRequest   `json:"request"`
	Schema       map[string]any `json:"schema"`
}

type NextResult struct {
	Ready    bool           `json:"ready"`
	Terminal bool           `json:"terminal"`
	Reason   string         `json:"reason,omitempty"`
	Summary  SessionSummary `json:"summary"`
	Step     *Step          `json:"step,omitempty"`
}

type ApplyInput struct {
	StartedAt string         `json:"started_at"`
	Round     int            `json:"round,omitempty"`
	Actor     string         `json:"actor,omitempty"`
	Phase     string         `json:"phase,omitempty"`
	Result    map[string]any `json:"result"`
}

func runningPhase(session *Session) *PhaseRecord {
	if session == nil || session.OpenRound == nil {
		return nil
	}
	for index := len(session.OpenRound.PhaseHistory) - 1; index >= 0; index-- {
		if session.OpenRound.PhaseHistory[index].Status == "running" {
			return &session.OpenRound.PhaseHistory[index]
		}
	}
	return nil
}

func phaseSucceeded(round *RoundRecord, actor, phase string) bool {
	if round == nil {
		return false
	}
	for index := len(round.PhaseHistory) - 1; index >= 0; index-- {
		item := round.PhaseHistory[index]
		if item.Actor == actor && item.Phase == phase && item.Status == "succeeded" {
			return true
		}
	}
	return false
}

func buildAgentRequest(session *Session, round int, actor, phase string) (AgentRequest, map[string]any, error) {
	request := AgentRequest{
		Protocol: "roundtable-kernel.exec.v1",
		Actor:    actor,
		Phase:    phase,
		Round:    round,
		Session:  session,
	}
	switch phase {
	case "explore":
		return request, nil, nil
	case "propose":
		return request, nil, nil
	case "re-explore":
		if session.OpenRound == nil || session.OpenRound.Proposal == nil {
			return AgentRequest{}, nil, fmt.Errorf("re-explore requires an open-round proposal")
		}
		return request, map[string]any{"proposal_summary": session.OpenRound.Proposal.Summary}, nil
	case "review":
		if session.OpenRound == nil || session.OpenRound.Proposal == nil {
			return AgentRequest{}, nil, fmt.Errorf("review requires an open-round proposal")
		}
		request.Proposal = session.OpenRound.Proposal
		return request, map[string]any{"proposal_summary": session.OpenRound.Proposal.Summary}, nil
	case "adjudicate":
		if session.OpenRound == nil || session.OpenRound.Proposal == nil {
			return AgentRequest{}, nil, fmt.Errorf("adjudicate requires an open-round proposal")
		}
		request.Proposal = session.OpenRound.Proposal
		request.Findings = append([]Finding{}, session.OpenRound.CurrentFindings()...)
		return request, map[string]any{
			"proposal_summary": session.OpenRound.Proposal.Summary,
			"finding_count":    len(request.Findings),
		}, nil
	default:
		return AgentRequest{}, nil, fmt.Errorf("unsupported phase %s", phase)
	}
}

func previewNextStep(session *Session) (*Step, string, bool, error) {
	if session.Status.State == "failed" {
		return nil, "session_failed", true, nil
	}
	if session.Status.Converged || session.Status.State == "converged" {
		return nil, "converged", true, nil
	}
	if session.Status.State == "exhausted" {
		return nil, "exhausted", true, nil
	}
	if session.OpenRound != nil {
		if running := runningPhase(session); running != nil {
			request, _, err := buildAgentRequest(session, session.OpenRound.Index, running.Actor, running.Phase)
			if err != nil {
				return nil, "", false, err
			}
			return &Step{
				SessionID:    session.ID,
				Round:        session.OpenRound.Index,
				Actor:        running.Actor,
				Phase:        running.Phase,
				StartedAt:    running.StartedAt,
				InputSummary: running.InputSummary,
				Request:      request,
				Schema:       OutputSchemaForPhase(running.Phase),
			}, "phase_running", false, nil
		}
		round := session.OpenRound.Index
		if session.OpenRound.Proposal == nil {
			phase := "explore"
			if phaseSucceeded(session.OpenRound, session.Chair, "explore") {
				phase = "propose"
			}
			request, inputSummary, err := buildAgentRequest(session, round, session.Chair, phase)
			if err != nil {
				return nil, "", false, err
			}
			return &Step{
				SessionID:    session.ID,
				Round:        round,
				Actor:        session.Chair,
				Phase:        phase,
				InputSummary: inputSummary,
				Request:      request,
				Schema:       OutputSchemaForPhase(phase),
			}, "phase_ready", false, nil
		}
		for _, critic := range session.Critics {
			if !phaseSucceeded(session.OpenRound, critic, "re-explore") {
				request, inputSummary, err := buildAgentRequest(session, round, critic, "re-explore")
				if err != nil {
					return nil, "", false, err
				}
				return &Step{
					SessionID:    session.ID,
					Round:        round,
					Actor:        critic,
					Phase:        "re-explore",
					InputSummary: inputSummary,
					Request:      request,
					Schema:       OutputSchemaForPhase("re-explore"),
				}, "phase_ready", false, nil
			}
			if !phaseSucceeded(session.OpenRound, critic, "review") {
				request, inputSummary, err := buildAgentRequest(session, round, critic, "review")
				if err != nil {
					return nil, "", false, err
				}
				return &Step{
					SessionID:    session.ID,
					Round:        round,
					Actor:        critic,
					Phase:        "review",
					InputSummary: inputSummary,
					Request:      request,
					Schema:       OutputSchemaForPhase("review"),
				}, "phase_ready", false, nil
			}
		}
		if len(session.OpenRound.CurrentFindings()) > 0 && session.OpenRound.Verdict == nil {
			request, inputSummary, err := buildAgentRequest(session, round, session.Chair, "adjudicate")
			if err != nil {
				return nil, "", false, err
			}
			return &Step{
				SessionID:    session.ID,
				Round:        round,
				Actor:        session.Chair,
				Phase:        "adjudicate",
				InputSummary: inputSummary,
				Request:      request,
				Schema:       OutputSchemaForPhase("adjudicate"),
			}, "phase_ready", false, nil
		}
		return nil, "internal_transition", false, nil
	}
	if roundLimitExhausted(session.MaxRounds, session.Status.Round) {
		return nil, "exhausted", true, nil
	}
	nextRound := session.Status.Round + 1
	request, inputSummary, err := buildAgentRequest(session, nextRound, session.Chair, "explore")
	if err != nil {
		return nil, "", false, err
	}
	return &Step{
		SessionID:    session.ID,
		Round:        nextRound,
		Actor:        session.Chair,
		Phase:        "explore",
		InputSummary: inputSummary,
		Request:      request,
		Schema:       OutputSchemaForPhase("explore"),
	}, "phase_ready", false, nil
}

func persistSession(paths Paths, session *Session) error {
	return SaveSession(paths, session)
}

func advanceInternal(session *Session, paths Paths) error {
	for {
		step, reason, terminal, err := previewNextStep(session)
		if err != nil {
			return err
		}
		if terminal {
			if reason == "exhausted" && session.Status.State != "exhausted" {
				session.Status.State = "exhausted"
				if err := persistSession(paths, session); err != nil {
					return err
				}
			}
			return nil
		}
		if step != nil {
			return nil
		}
		if reason != "internal_transition" {
			return nil
		}
		round := session.OpenRound.Index
		if session.OpenRound.Verdict == nil && len(session.OpenRound.CurrentFindings()) == 0 {
			request, inputSummary, err := buildAgentRequest(session, round, session.Chair, "adjudicate")
			if err != nil {
				return err
			}
			_ = request
			if err := RecordPhaseStart(session, round, session.Chair, "adjudicate", inputSummary); err != nil {
				return err
			}
			if err := CompletePhase(session, round, session.Chair, "adjudicate", summarizeVerdict(nil), map[string]any{"verdict": nil}, 0, nil); err != nil {
				return err
			}
			if _, err := ApplyRound(session); err != nil {
				return err
			}
			if !session.Status.Converged && roundLimitExhausted(session.MaxRounds, session.Status.Round) {
				session.Status.State = "exhausted"
			}
			if err := persistSession(paths, session); err != nil {
				return err
			}
			continue
		}
		if session.OpenRound.Verdict != nil {
			if _, err := ApplyRound(session); err != nil {
				return err
			}
			if !session.Status.Converged && roundLimitExhausted(session.MaxRounds, session.Status.Round) {
				session.Status.State = "exhausted"
			}
			if err := persistSession(paths, session); err != nil {
				return err
			}
			continue
		}
		return nil
	}
}

func InitSession(paths Paths, specPath string, sessionID string, force bool) (*Session, string, error) {
	session, adapter, telemetryFile, err := bootstrapExecSession(paths, specPath, sessionID, force)
	if err != nil {
		return nil, "", err
	}
	_ = AppendTelemetryEvent(telemetryFile, map[string]any{
		"type":       "session_started",
		"session_id": session.ID,
		"adapter":    session.Adapter,
		"topic":      session.Topic,
		"chair":      session.Chair,
		"critics":    session.Critics,
		"max_rounds": session.MaxRounds,
		"mode":       "live",
	})
	seed, err := adapter.SeedEvidence(nil)
	if err != nil {
		return nil, "", err
	}
	if len(seed) > 0 {
		evidenceKeyMap := map[string]string{}
		if _, err := collectEvidenceBatches(session, evidenceKeyMap, seed, 0); err != nil {
			return nil, "", err
		}
	}
	if err := SaveSession(paths, session); err != nil {
		return nil, "", err
	}
	return session, telemetryFile, nil
}

func NextStep(paths Paths, sessionID string, actorFilter string) (*Session, *NextResult, error) {
	session, err := LoadSession(paths, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := advanceInternal(session, paths); err != nil {
		return nil, nil, err
	}
	summary := DeriveSessionSummary(session)
	step, reason, terminal, err := previewNextStep(session)
	if err != nil {
		return nil, nil, err
	}
	if terminal {
		return session, &NextResult{Ready: false, Terminal: true, Reason: reason, Summary: summary}, nil
	}
	if step == nil {
		return session, &NextResult{Ready: false, Terminal: false, Reason: reason, Summary: summary}, nil
	}
	if actorFilter != "" && step.Actor != actorFilter {
		return session, &NextResult{Ready: false, Terminal: false, Reason: "waiting_for_other_actor", Summary: summary, Step: step}, nil
	}
	if step.StartedAt == "" {
		if err := RecordPhaseStart(session, step.Round, step.Actor, step.Phase, step.InputSummary); err != nil {
			return nil, nil, err
		}
		if err := SaveSession(paths, session); err != nil {
			return nil, nil, err
		}
		running := runningPhase(session)
		if running == nil {
			return nil, nil, fmt.Errorf("phase issuance failed for %s/%s", step.Actor, step.Phase)
		}
		step.StartedAt = running.StartedAt
	}
	return session, &NextResult{Ready: true, Terminal: false, Reason: reason, Summary: DeriveSessionSummary(session), Step: step}, nil
}

func PeekNextStep(paths Paths, sessionID string, actorFilter string) (*Session, *NextResult, error) {
	session, err := LoadSession(paths, sessionID)
	if err != nil {
		return nil, nil, err
	}
	if err := advanceInternal(session, paths); err != nil {
		return nil, nil, err
	}
	summary := DeriveSessionSummary(session)
	step, reason, terminal, err := previewNextStep(session)
	if err != nil {
		return nil, nil, err
	}
	if actorFilter != "" && step != nil && step.Actor != actorFilter {
		return session, &NextResult{Ready: false, Terminal: false, Reason: "waiting_for_other_actor", Summary: summary, Step: step}, nil
	}
	return session, &NextResult{Ready: step != nil, Terminal: terminal, Reason: reason, Summary: summary, Step: step}, nil
}

func phaseDurationMS(startedAt string) int64 {
	if startedAt == "" {
		return 0
	}
	start, err := time.Parse("2006-01-02T15:04:05.000Z", startedAt)
	if err != nil {
		return 0
	}
	return time.Since(start).Milliseconds()
}

func decodeApplyInput(path string) (ApplyInput, error) {
	input := ApplyInput{}
	if path == "" || path == "-" {
		return input, ReadJSONStdin(&input)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return input, err
	}
	err = json.Unmarshal(data, &input)
	return input, err
}

func ApplyStep(paths Paths, sessionID string, input ApplyInput) (*Session, error) {
	session, err := LoadSession(paths, sessionID)
	if err != nil {
		return nil, err
	}
	if session.OpenRound == nil {
		return nil, fmt.Errorf("session has no open round")
	}
	running := runningPhase(session)
	if running == nil {
		return nil, fmt.Errorf("session has no running phase")
	}
	round := session.OpenRound.Index
	if input.StartedAt != "" && input.StartedAt != running.StartedAt {
		return nil, fmt.Errorf("apply started_at does not match the running phase")
	}
	if input.Round != 0 && input.Round != round {
		return nil, fmt.Errorf("apply round does not match the running phase")
	}
	if input.Actor != "" && input.Actor != running.Actor {
		return nil, fmt.Errorf("apply actor does not match the running phase")
	}
	if input.Phase != "" && input.Phase != running.Phase {
		return nil, fmt.Errorf("apply phase does not match the running phase")
	}
	durationMS := phaseDurationMS(running.StartedAt)
	switch running.Phase {
	case "explore", "re-explore":
		batches, err := EvidenceBatchesFromResult(input.Result, running.Actor, running.Phase)
		if err != nil {
			return nil, err
		}
		evidenceKeyMap := map[string]string{}
		added, err := collectEvidenceBatches(session, evidenceKeyMap, batches, round)
		if err != nil {
			return nil, err
		}
		if err := NoteRoundEvidence(session, round, added); err != nil {
			return nil, err
		}
		if err := CompletePhase(session, round, running.Actor, running.Phase, summarizeEvidenceBatches(batches), map[string]any{"evidence_added": added}, durationMS, nil); err != nil {
			return nil, err
		}
	case "propose":
		proposal, err := ProposalFromResult(input.Result)
		if err != nil {
			return nil, err
		}
		if err := RegisterProposal(session, round, proposal); err != nil {
			return nil, err
		}
		if err := CompletePhase(session, round, running.Actor, running.Phase, summarizeProposal(proposal), map[string]any{"proposal": proposal}, durationMS, nil); err != nil {
			return nil, err
		}
	case "review":
		findings, err := FindingsFromResult(input.Result)
		if err != nil {
			return nil, err
		}
		if err := AppendRoundFindings(session, round, findings); err != nil {
			return nil, err
		}
		if err := CompletePhase(session, round, running.Actor, running.Phase, summarizeFindings(findings), map[string]any{"findings_against_proposal": findings}, durationMS, nil); err != nil {
			return nil, err
		}
	case "adjudicate":
		verdict, err := VerdictFromResult(input.Result)
		if err != nil {
			return nil, err
		}
		if err := RegisterVerdict(session, round, verdict); err != nil {
			return nil, err
		}
		if err := CompletePhase(session, round, running.Actor, running.Phase, summarizeVerdict(verdict), map[string]any{"verdict": verdict}, durationMS, nil); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported running phase %s", running.Phase)
	}
	if err := SaveSession(paths, session); err != nil {
		return nil, err
	}
	if err := advanceInternal(session, paths); err != nil {
		return nil, err
	}
	return session, nil
}
