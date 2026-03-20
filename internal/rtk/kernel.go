package rtk

import (
	"fmt"
	"regexp"
	"time"
)

var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func nowISO() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
}

func makeEvidenceID(session *Session) string {
	return fmt.Sprintf("E%d", len(session.Evidence)+1)
}

func evidenceIndex(session *Session) map[string]Evidence {
	index := make(map[string]Evidence, len(session.Evidence))
	for _, item := range session.Evidence {
		index[item.ID] = item
	}
	return index
}

func findingIndex(findings []Finding) map[string]Finding {
	index := make(map[string]Finding, len(findings))
	for _, item := range findings {
		index[item.ID] = item
	}
	return index
}

func ensureOpenRound(session *Session, round int) (*RoundRecord, error) {
	if err := invariant(round > 0, "open round index must be a positive integer"); err != nil {
		return nil, err
	}
	if session.OpenRound == nil {
		session.OpenRound = &RoundRecord{
			Index:                   round,
			Proposal:                nil,
			EvidenceAdded:           []string{},
			FindingsAgainstProposal: []Finding{},
			ReviewSummary:           Counts{},
			Verdict:                 nil,
			PhaseHistory:            []PhaseRecord{},
			CreatedAt:               nowISO(),
			UpdatedAt:               nowISO(),
			Error:                   nil,
		}
	}
	if session.OpenRound.Index != round {
		return nil, fmt.Errorf("session already has open round %d", session.OpenRound.Index)
	}
	return session.OpenRound, nil
}

func refreshOpenRoundSummary(session *Session) {
	if session.OpenRound == nil {
		return
	}
	counts := FindingCounts(session.OpenRound.CurrentFindings())
	session.OpenRound.ReviewSummary = counts
	session.OpenRound.UpdatedAt = nowISO()
	session.Status.Round = session.OpenRound.Index
	session.Status.UnresolvedHigh = counts.High
	session.Status.UnresolvedMedium = counts.Medium
}

func roundPhaseIndex(roundRecord *RoundRecord, actor, phase, status string) int {
	for index := len(roundRecord.PhaseHistory) - 1; index >= 0; index-- {
		item := roundRecord.PhaseHistory[index]
		if item.Actor == actor && item.Phase == phase && item.Status == status {
			return index
		}
	}
	return -1
}

func NewSession(id, topic, chair string, critics []string, maxRounds *int, adapter string) (*Session, error) {
	if err := validateString(id, "session id"); err != nil {
		return nil, err
	}
	if !sessionIDPattern.MatchString(id) {
		return nil, fmt.Errorf("session id must match [A-Za-z0-9._-]+")
	}
	if err := validateString(topic, "topic"); err != nil {
		return nil, err
	}
	if err := validateString(chair, "chair"); err != nil {
		return nil, err
	}
	if maxRounds != nil {
		if err := invariant(*maxRounds > 0, "max_rounds must be a positive integer"); err != nil {
			return nil, err
		}
	}
	if err := validateString(adapter, "adapter"); err != nil {
		return nil, err
	}
	return &Session{
		Version:             1,
		ID:                  id,
		Topic:               topic,
		CreatedAt:           nowISO(),
		Chair:               chair,
		Critics:             append([]string{}, critics...),
		MaxRounds:           cloneIntPtr(maxRounds),
		Adapter:             adapter,
		Evidence:            []Evidence{},
		Rounds:              []RoundRecord{},
		OpenRound:           nil,
		AdjudicatedProposal: nil,
		Status: Status{
			Round:            0,
			Converged:        false,
			UnresolvedHigh:   0,
			UnresolvedMedium: 0,
			State:            "initialized",
			ActiveActor:      nil,
			ActivePhase:      nil,
			Error:            nil,
		},
	}, nil
}

type EvidenceInput struct {
	Key       string `json:"key,omitempty"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Statement string `json:"statement"`
	Excerpt   string `json:"excerpt"`
}

func AppendEvidence(session *Session, items []EvidenceInput, collectedBy, phase string, round int) ([]Evidence, error) {
	if err := validateString(collectedBy, "collectedBy"); err != nil {
		return nil, err
	}
	if err := validateString(phase, "phase"); err != nil {
		return nil, err
	}
	if err := invariant(round >= 0, "round must be a non-negative integer"); err != nil {
		return nil, err
	}
	added := make([]Evidence, 0, len(items))
	for _, item := range items {
		evidence := Evidence{
			ID:          makeEvidenceID(session),
			Source:      item.Source,
			Kind:        item.Kind,
			Phase:       phase,
			Statement:   item.Statement,
			Excerpt:     item.Excerpt,
			CollectedBy: collectedBy,
			Round:       round,
			CreatedAt:   nowISO(),
		}
		if err := ValidateEvidence(&evidence, "evidence"); err != nil {
			return nil, err
		}
		session.Evidence = append(session.Evidence, evidence)
		added = append(added, evidence)
	}
	return added, nil
}

func startRound(session *Session, round int) (*RoundRecord, error) {
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return nil, err
	}
	session.Status.Round = round
	session.Status.Converged = false
	session.Status.UnresolvedHigh = roundRecord.ReviewSummary.High
	session.Status.UnresolvedMedium = roundRecord.ReviewSummary.Medium
	session.Status.State = "running"
	session.Status.ActiveActor = nil
	session.Status.ActivePhase = nil
	session.Status.Error = nil
	roundRecord.UpdatedAt = nowISO()
	return roundRecord, nil
}

func RecordPhaseStart(session *Session, round int, actor, phase string, inputSummary map[string]any) error {
	roundRecord, err := startRound(session, round)
	if err != nil {
		return err
	}
	roundRecord.PhaseHistory = append(roundRecord.PhaseHistory, PhaseRecord{
		Actor:         actor,
		Phase:         phase,
		Status:        "running",
		InputSummary:  inputSummary,
		OutputSummary: nil,
		Artifact:      nil,
		Usage:         nil,
		StartedAt:     nowISO(),
		CompletedAt:   nil,
		DurationMS:    nil,
		Error:         nil,
	})
	roundRecord.UpdatedAt = nowISO()
	session.Status.ActiveActor = stringPtr(actor)
	session.Status.ActivePhase = stringPtr(phase)
	session.Status.Error = nil
	return nil
}

func CompletePhase(session *Session, round int, actor, phase string, outputSummary map[string]any, artifact map[string]any, usage *PhaseUsage, durationMS int64, phaseErr error) error {
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return err
	}
	index := roundPhaseIndex(roundRecord, actor, phase, "running")
	if index < 0 {
		return fmt.Errorf("no running phase record for %s/%s in round %d", actor, phase, round)
	}
	status := "succeeded"
	var errorInfo *SessionError
	if phaseErr != nil {
		status = "failed"
		errorInfo = &SessionError{
			Message: phaseErr.Error(),
			At:      nowISO(),
		}
	}
	completedAt := nowISO()
	roundRecord.PhaseHistory[index] = PhaseRecord{
		Actor:         roundRecord.PhaseHistory[index].Actor,
		Phase:         roundRecord.PhaseHistory[index].Phase,
		Status:        status,
		InputSummary:  roundRecord.PhaseHistory[index].InputSummary,
		OutputSummary: outputSummary,
		Artifact:      artifact,
		Usage:         normalizePhaseUsage(usage),
		StartedAt:     roundRecord.PhaseHistory[index].StartedAt,
		CompletedAt:   &completedAt,
		DurationMS:    int64Ptr(durationMS),
		Error:         errorInfo,
	}
	roundRecord.UpdatedAt = completedAt
	if phaseErr != nil {
		session.Status.ActiveActor = stringPtr(actor)
		session.Status.ActivePhase = stringPtr(phase)
		session.Status.Error = &SessionError{Message: phaseErr.Error(), At: completedAt}
	} else {
		session.Status.ActiveActor = nil
		session.Status.ActivePhase = nil
		session.Status.Error = nil
	}
	return nil
}

func NoteRoundEvidence(session *Session, round int, evidenceAdded []string) error {
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return err
	}
	seen := make(map[string]bool, len(roundRecord.EvidenceAdded))
	for _, item := range roundRecord.EvidenceAdded {
		seen[item] = true
	}
	for _, evidenceID := range evidenceAdded {
		if err := validateString(evidenceID, "evidence_added[]"); err != nil {
			return err
		}
		if !seen[evidenceID] {
			roundRecord.EvidenceAdded = append(roundRecord.EvidenceAdded, evidenceID)
			seen[evidenceID] = true
		}
	}
	roundRecord.UpdatedAt = nowISO()
	return nil
}

func RegisterProposal(session *Session, round int, proposal *Proposal) error {
	if err := ValidateProposal(proposal, "proposal"); err != nil {
		return err
	}
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return err
	}
	roundRecord.Proposal = proposal
	roundRecord.UpdatedAt = nowISO()
	return nil
}

func AppendRoundFindings(session *Session, round int, findings []Finding) error {
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return err
	}
	evidence := evidenceIndex(session)
	seen := make(map[string]bool, len(roundRecord.CurrentFindings()))
	for _, item := range roundRecord.CurrentFindings() {
		seen[item.ID] = true
	}
	for index, finding := range findings {
		copy := finding
		if err := ValidateFinding(&copy, evidence, fmt.Sprintf("open_round.findings_against_proposal[%d]", index)); err != nil {
			return err
		}
		if seen[copy.ID] {
			return fmt.Errorf("duplicate finding id %s", copy.ID)
		}
		seen[copy.ID] = true
		roundRecord.FindingsAgainstProposal = append(roundRecord.FindingsAgainstProposal, copy)
	}
	refreshOpenRoundSummary(session)
	return nil
}

func RegisterVerdict(session *Session, round int, verdict *Verdict) error {
	roundRecord, err := ensureOpenRound(session, round)
	if err != nil {
		return err
	}
	if verdict == nil {
		roundRecord.Verdict = nil
		roundRecord.UpdatedAt = nowISO()
		return nil
	}
	if verdict.RevisedProposal != nil {
		if err := ValidateProposal(verdict.RevisedProposal, "verdict.revised_proposal"); err != nil {
			return err
		}
	}
	findings := roundRecord.CurrentFindings()
	findingByID := findingIndex(findings)
	evidence := evidenceIndex(session)
	if len(verdict.Decisions) != len(findings) {
		return fmt.Errorf("verdict must contain exactly one decision for every finding")
	}
	seen := map[string]bool{}
	for index, decision := range verdict.Decisions {
		copy := decision
		if err := ValidateDecision(&copy, findingByID, evidence, fmt.Sprintf("open_round.verdict.decisions[%d]", index)); err != nil {
			return err
		}
		if seen[copy.FindingID] {
			return fmt.Errorf("duplicate decision for finding %s", copy.FindingID)
		}
		seen[copy.FindingID] = true
	}
	roundRecord.Verdict = verdict
	roundRecord.UpdatedAt = nowISO()
	return nil
}

func MarkSessionFailed(session *Session, round int, actor, phase string, failure error) {
	message := "unknown error"
	if failure != nil {
		message = failure.Error()
	}
	at := nowISO()
	session.Status.Round = round
	if session.Status.Round == 0 && session.OpenRound != nil {
		session.Status.Round = session.OpenRound.Index
	}
	session.Status.State = "failed"
	if actor != "" {
		session.Status.ActiveActor = stringPtr(actor)
	}
	if phase != "" {
		session.Status.ActivePhase = stringPtr(phase)
	}
	session.Status.Error = &SessionError{Message: message, At: at}
	if session.OpenRound != nil {
		session.OpenRound.Error = &SessionError{
			Message: message,
			Actor:   stringPtr(actor),
			Phase:   stringPtr(phase),
			At:      at,
		}
		session.OpenRound.UpdatedAt = at
	}
}

func ApplyRound(session *Session) (*RoundRecord, error) {
	if session.OpenRound == nil {
		return nil, fmt.Errorf("session has no open round to apply")
	}
	roundRecord := session.OpenRound
	if err := ValidateProposal(roundRecord.Proposal, "open_round.proposal"); err != nil {
		return nil, err
	}
	findings := append([]Finding{}, roundRecord.CurrentFindings()...)
	counts := FindingCounts(findings)
	material := make([]Finding, 0, counts.Material)
	for _, finding := range findings {
		if IsMaterialFinding(finding) {
			material = append(material, finding)
		}
	}
	if roundRecord.Verdict == nil && len(material) > 0 {
		return nil, fmt.Errorf("material findings require a verdict before the round can close")
	}
	closed := RoundRecord{
		Index:                   roundRecord.Index,
		Proposal:                roundRecord.Proposal,
		EvidenceAdded:           append([]string{}, roundRecord.EvidenceAdded...),
		FindingsAgainstProposal: findings,
		ReviewSummary:           counts,
		Verdict:                 roundRecord.Verdict,
		PhaseHistory:            append([]PhaseRecord{}, roundRecord.PhaseHistory...),
		CreatedAt:               roundRecord.CreatedAt,
		UpdatedAt:               nowISO(),
	}
	session.Rounds = append(session.Rounds, closed)
	session.OpenRound = nil
	if closed.Verdict != nil && closed.Verdict.RevisedProposal != nil {
		session.AdjudicatedProposal = closed.Verdict.RevisedProposal
	} else {
		session.AdjudicatedProposal = closed.Proposal
	}
	session.Status.Round = closed.Index
	session.Status.Converged = counts.Material == 0
	session.Status.UnresolvedHigh = counts.High
	session.Status.UnresolvedMedium = counts.Medium
	if counts.Material == 0 {
		session.Status.State = "converged"
	} else {
		session.Status.State = "needs_revision"
	}
	session.Status.ActiveActor = nil
	session.Status.ActivePhase = nil
	session.Status.Error = nil
	return &closed, nil
}
