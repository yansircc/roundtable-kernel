package rtk

import (
	"strings"
	"testing"
)

func TestValidateFindingSupportedRequiresEvidenceIDs(t *testing.T) {
	t.Parallel()

	finding := &Finding{
		ID:              "F1",
		Critic:          "critic",
		Severity:        "medium",
		Basis:           "supported",
		Summary:         "summary",
		Rationale:       "rationale",
		SuggestedChange: "change",
	}

	err := ValidateFinding(finding, map[string]Evidence{"E1": {ID: "E1"}}, "finding")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "must be non-empty for supported findings") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFindingGapRejectsEvidenceIDs(t *testing.T) {
	t.Parallel()

	finding := &Finding{
		ID:              "F1",
		Critic:          "critic",
		Severity:        "low",
		Basis:           "gap",
		Summary:         "summary",
		Rationale:       "rationale",
		SuggestedChange: "change",
		EvidenceIDs:     []string{"E1"},
	}

	err := ValidateFinding(finding, map[string]Evidence{"E1": {ID: "E1"}}, "finding")
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "must be empty for gap findings") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRoundRequiresVerdictForMaterialFindings(t *testing.T) {
	t.Parallel()

	session, err := NewSession("test-round", "topic", "chair", []string{"critic"}, intPtr(1), "exec")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := ensureOpenRound(session, 1); err != nil {
		t.Fatalf("ensureOpenRound: %v", err)
	}
	if err := RegisterProposal(session, 1, &Proposal{Summary: "proposal"}); err != nil {
		t.Fatalf("RegisterProposal: %v", err)
	}
	session.OpenRound.FindingsAgainstProposal = []Finding{{
		ID:              "F1",
		Critic:          "critic",
		Severity:        "high",
		Basis:           "gap",
		Summary:         "summary",
		Rationale:       "rationale",
		SuggestedChange: "change",
		EvidenceIDs:     nil,
	}}
	refreshOpenRoundSummary(session)

	_, err = ApplyRound(session)
	if err == nil {
		t.Fatal("expected ApplyRound to require a verdict")
	}
	if !strings.Contains(err.Error(), "material findings require a verdict") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRoundConvergesWithoutMaterialFindings(t *testing.T) {
	t.Parallel()

	session, err := NewSession("test-converge", "topic", "chair", nil, intPtr(1), "exec")
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if _, err := ensureOpenRound(session, 1); err != nil {
		t.Fatalf("ensureOpenRound: %v", err)
	}
	if err := RegisterProposal(session, 1, &Proposal{Summary: "proposal"}); err != nil {
		t.Fatalf("RegisterProposal: %v", err)
	}

	closed, err := ApplyRound(session)
	if err != nil {
		t.Fatalf("ApplyRound: %v", err)
	}
	if !session.Status.Converged {
		t.Fatal("expected session to converge")
	}
	if session.Status.State != "converged" {
		t.Fatalf("unexpected state: %s", session.Status.State)
	}
	if session.AdjudicatedProposal == nil || session.AdjudicatedProposal.Summary != "proposal" {
		t.Fatalf("unexpected adjudicated proposal: %#v", session.AdjudicatedProposal)
	}
	if closed.Index != 1 {
		t.Fatalf("unexpected closed round index: %d", closed.Index)
	}
}
