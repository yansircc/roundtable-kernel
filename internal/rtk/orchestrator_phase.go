package rtk

import (
	"errors"
	"fmt"
	"time"
)

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
			if item.Key == "" {
				continue
			}
			if _, ok := evidenceKeyMap[item.Key]; ok {
				return nil, fmt.Errorf("duplicate evidence key %s", item.Key)
			}
			evidenceKeyMap[item.Key] = added[index].ID
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
	for _, batch := range batches {
		itemCount += len(batch.Items)
	}
	return map[string]any{
		"batch_count": len(batches),
		"item_count":  itemCount,
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
	Fn            func() (AgentPhaseResult, error)
	Paths         Paths
}

func runPhase(spec phaseRun) (any, error) {
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
		if errors.Is(err, ErrSessionStopped) {
			_ = syncStoppedSession(spec.Paths, spec.Session)
			_ = AppendTelemetryEvent(spec.TelemetryFile, map[string]any{
				"type":        "phase_stopped",
				"session_id":  spec.Session.ID,
				"round":       spec.Round,
				"actor":       spec.Actor,
				"phase":       spec.Phase,
				"adapter":     spec.Adapter,
				"duration_ms": time.Since(start).Milliseconds(),
			})
			return nil, ErrSessionStopped
		}
		if spec.Round > 0 {
			_ = CompletePhase(spec.Session, spec.Round, spec.Actor, spec.Phase, nil, nil, nil, time.Since(start).Milliseconds(), err)
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
		outputSummary = spec.Summarize(result.Value)
	}
	var artifact map[string]any
	if spec.OnSuccess != nil {
		artifact, err = spec.OnSuccess(result.Value)
		if err != nil {
			return nil, err
		}
	}
	if spec.Round > 0 {
		if err := CompletePhase(spec.Session, spec.Round, spec.Actor, spec.Phase, outputSummary, artifact, result.Usage, time.Since(start).Milliseconds(), nil); err != nil {
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
	return result.Value, nil
}
