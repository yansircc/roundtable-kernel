package main

import (
	"encoding/json"
	"fmt"
	"os"

	"roundtable-kernel/internal/rtk"
)

type fixtureEvidenceItem struct {
	Key       string `json:"key"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Statement string `json:"statement"`
	Excerpt   string `json:"excerpt"`
	Phase     string `json:"phase,omitempty"`
}

type fixtureEvidenceBatch struct {
	Phase       string                `json:"phase"`
	Actor       string                `json:"actor"`
	CollectedBy string                `json:"collected_by"`
	Items       []fixtureEvidenceItem `json:"items"`
}

type fixtureFinding struct {
	ID              string   `json:"id"`
	Critic          string   `json:"critic"`
	Severity        string   `json:"severity"`
	Basis           string   `json:"basis"`
	Summary         string   `json:"summary"`
	Rationale       string   `json:"rationale"`
	SuggestedChange string   `json:"suggested_change"`
	EvidenceKeys    []string `json:"evidence_keys"`
}

type fixtureDecision struct {
	FindingID    string   `json:"finding_id"`
	Disposition  string   `json:"disposition"`
	Rationale    string   `json:"rationale"`
	EvidenceKeys []string `json:"evidence_keys"`
}

type fixtureVerdict struct {
	Summary         string            `json:"summary"`
	RevisedProposal *rtk.Proposal     `json:"revised_proposal"`
	Decisions       []fixtureDecision `json:"decisions"`
}

type scenarioRound struct {
	EvidenceBatches         []fixtureEvidenceBatch `json:"evidence_batches"`
	Proposal                *rtk.Proposal          `json:"proposal"`
	FindingsAgainstProposal []fixtureFinding       `json:"findings_against_proposal"`
	FindingsLegacy          []fixtureFinding       `json:"findings"`
	Verdict                 *fixtureVerdict        `json:"verdict"`
}

type scenarioFile struct {
	SeedBatch *fixtureEvidenceBatch `json:"seed_batch"`
	Rounds    []scenarioRound       `json:"rounds"`
}

func readJSONFile(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func toEvidenceInput(items []fixtureEvidenceItem) []rtk.EvidenceInput {
	result := make([]rtk.EvidenceInput, 0, len(items))
	for _, item := range items {
		result = append(result, rtk.EvidenceInput{
			Key:       item.Key,
			Source:    item.Source,
			Kind:      item.Kind,
			Statement: item.Statement,
			Excerpt:   item.Excerpt,
		})
	}
	return result
}

func allEvidenceEntries(fixture scenarioFile) []fixtureEvidenceItem {
	entries := []fixtureEvidenceItem{}
	if fixture.SeedBatch != nil {
		for _, item := range fixture.SeedBatch.Items {
			item.Phase = "seed"
			entries = append(entries, item)
		}
	}
	for _, round := range fixture.Rounds {
		for _, batch := range round.EvidenceBatches {
			for _, item := range batch.Items {
				item.Phase = batch.Phase
				entries = append(entries, item)
			}
		}
	}
	return entries
}

func evidenceIDByKey(session *rtk.Session, evidenceIndex map[string]fixtureEvidenceItem, key string) (string, error) {
	target, ok := evidenceIndex[key]
	if !ok {
		return "", fmt.Errorf("unknown scenario evidence key %s", key)
	}
	for _, item := range session.Evidence {
		if item.Source == target.Source && item.Statement == target.Statement && item.Phase == target.Phase {
			return item.ID, nil
		}
	}
	return "", fmt.Errorf("session does not yet contain evidence for key %s", key)
}

func mapFinding(finding fixtureFinding, session *rtk.Session, evidenceIndex map[string]fixtureEvidenceItem) (rtk.Finding, error) {
	evidenceIDs := []string{}
	for _, key := range finding.EvidenceKeys {
		evidenceID, err := evidenceIDByKey(session, evidenceIndex, key)
		if err != nil {
			return rtk.Finding{}, err
		}
		evidenceIDs = append(evidenceIDs, evidenceID)
	}
	return rtk.Finding{
		ID:              finding.ID,
		Critic:          finding.Critic,
		Severity:        finding.Severity,
		Basis:           finding.Basis,
		Summary:         finding.Summary,
		Rationale:       finding.Rationale,
		SuggestedChange: finding.SuggestedChange,
		EvidenceIDs:     evidenceIDs,
	}, nil
}

func mapVerdict(verdict *fixtureVerdict, session *rtk.Session, evidenceIndex map[string]fixtureEvidenceItem) (*rtk.Verdict, error) {
	if verdict == nil {
		return nil, nil
	}
	decisions := []rtk.Decision{}
	for _, decision := range verdict.Decisions {
		evidenceIDs := []string{}
		for _, key := range decision.EvidenceKeys {
			evidenceID, err := evidenceIDByKey(session, evidenceIndex, key)
			if err != nil {
				return nil, err
			}
			evidenceIDs = append(evidenceIDs, evidenceID)
		}
		decisions = append(decisions, rtk.Decision{
			FindingID:   decision.FindingID,
			Disposition: decision.Disposition,
			Rationale:   decision.Rationale,
			EvidenceIDs: evidenceIDs,
		})
	}
	return &rtk.Verdict{
		Summary:         verdict.Summary,
		RevisedProposal: verdict.RevisedProposal,
		Decisions:       decisions,
	}, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/mock-agent <scenario.json>")
		os.Exit(1)
	}
	fixture := scenarioFile{}
	if err := readJSONFile(os.Args[1], &fixture); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	request := rtk.AgentRequest{}
	if err := rtk.ReadJSONStdin(&request); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	index := request.Round - 1
	if index < 0 || index >= len(fixture.Rounds) {
		fmt.Fprintf(os.Stderr, "scenario has no round %d\n", request.Round)
		os.Exit(1)
	}
	round := fixture.Rounds[index]
	evidenceIndex := map[string]fixtureEvidenceItem{}
	for _, item := range allEvidenceEntries(fixture) {
		evidenceIndex[item.Key] = item
	}
	switch request.Phase {
	case "explore", "re-explore":
		items := []rtk.EvidenceInput{}
		for _, batch := range round.EvidenceBatches {
			if batch.Phase != request.Phase || batch.Actor != request.Actor {
				continue
			}
			items = append(items, toEvidenceInput(batch.Items)...)
		}
		_ = rtk.PrintJSON(map[string]any{"items": items, "collected_by": request.Actor})
	case "propose":
		_ = rtk.PrintJSON(map[string]any{"proposal": round.Proposal})
	case "review":
		findings := []rtk.Finding{}
		fixtureFindings := round.FindingsAgainstProposal
		if len(fixtureFindings) == 0 {
			fixtureFindings = round.FindingsLegacy
		}
		for _, finding := range fixtureFindings {
			if finding.Critic != request.Actor {
				continue
			}
			mapped, err := mapFinding(finding, request.Session, evidenceIndex)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				os.Exit(1)
			}
			findings = append(findings, mapped)
		}
		_ = rtk.PrintJSON(map[string]any{"findings": findings})
	case "adjudicate":
		verdict, err := mapVerdict(round.Verdict, request.Session, evidenceIndex)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		_ = rtk.PrintJSON(map[string]any{"verdict": verdict})
	default:
		fmt.Fprintf(os.Stderr, "unsupported phase %s\n", request.Phase)
		os.Exit(1)
	}
}
