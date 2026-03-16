package rtk

import (
	"context"
	"fmt"
)

type fixtureEvidenceItem struct {
	Key       string `json:"key"`
	Source    string `json:"source"`
	Kind      string `json:"kind"`
	Statement string `json:"statement"`
	Excerpt   string `json:"excerpt"`
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
	RevisedProposal *Proposal         `json:"revised_proposal"`
	Decisions       []fixtureDecision `json:"decisions"`
}

type fixtureRound struct {
	EvidenceBatches         []fixtureEvidenceBatch `json:"evidence_batches"`
	Proposal                *Proposal              `json:"proposal"`
	FindingsAgainstProposal []fixtureFinding       `json:"findings_against_proposal"`
	FindingsLegacy          []fixtureFinding       `json:"findings"`
	Verdict                 *fixtureVerdict        `json:"verdict"`
}

type fixtureFile struct {
	Topic     string                `json:"topic"`
	Chair     string                `json:"chair"`
	Critics   []string              `json:"critics"`
	MaxRounds int                   `json:"max_rounds"`
	SeedBatch *fixtureEvidenceBatch `json:"seed_batch"`
	Rounds    []fixtureRound        `json:"rounds"`
}

type fixtureAdapter struct {
	fixture fixtureFile
}

func newFixtureAdapter(path string) (Adapter, error) {
	fixture := fixtureFile{}
	if err := readJSONFile(path, &fixture); err != nil {
		return nil, err
	}
	if len(fixture.Critics) == 0 {
		fixture.Critics = uniqueCritics(fixture.Rounds)
	}
	if fixture.Chair == "" {
		fixture.Chair = "chair"
	}
	return &fixtureAdapter{fixture: fixture}, nil
}

func uniqueCritics(rounds []fixtureRound) []string {
	seen := map[string]bool{}
	critics := []string{}
	for _, round := range rounds {
		findings := round.FindingsAgainstProposal
		if len(findings) == 0 {
			findings = round.FindingsLegacy
		}
		for _, finding := range findings {
			if finding.Critic == "" || seen[finding.Critic] {
				continue
			}
			seen[finding.Critic] = true
			critics = append(critics, finding.Critic)
		}
	}
	return critics
}

func toEvidenceInput(items []fixtureEvidenceItem) []EvidenceInput {
	result := make([]EvidenceInput, 0, len(items))
	for _, item := range items {
		result = append(result, EvidenceInput{
			Key:       item.Key,
			Source:    item.Source,
			Kind:      item.Kind,
			Statement: item.Statement,
			Excerpt:   item.Excerpt,
		})
	}
	return result
}

func (f *fixtureAdapter) Kind() string { return "fixture" }

func (f *fixtureAdapter) Metadata() AdapterMetadata {
	maxRounds := f.fixture.MaxRounds
	if maxRounds <= 0 {
		maxRounds = len(f.fixture.Rounds)
	}
	return AdapterMetadata{
		Topic:     f.fixture.Topic,
		Chair:     f.fixture.Chair,
		Critics:   append([]string{}, f.fixture.Critics...),
		MaxRounds: maxRounds,
	}
}

func (f *fixtureAdapter) SeedEvidence(ctx context.Context) ([]EvidenceBatch, error) {
	_ = ctx
	if f.fixture.SeedBatch == nil || len(f.fixture.SeedBatch.Items) == 0 {
		return []EvidenceBatch{}, nil
	}
	collectedBy := f.fixture.SeedBatch.CollectedBy
	if collectedBy == "" {
		if f.fixture.SeedBatch.Actor != "" {
			collectedBy = f.fixture.SeedBatch.Actor
		} else {
			collectedBy = f.fixture.Chair
		}
	}
	return []EvidenceBatch{{
		Items:       toEvidenceInput(f.fixture.SeedBatch.Items),
		CollectedBy: collectedBy,
		Phase:       "seed",
	}}, nil
}

func (f *fixtureAdapter) roundAt(round int) (*fixtureRound, error) {
	index := round - 1
	if index < 0 || index >= len(f.fixture.Rounds) {
		return nil, fmt.Errorf("fixture has no round %d", round)
	}
	return &f.fixture.Rounds[index], nil
}

func (f *fixtureAdapter) CollectEvidence(ctx context.Context, args CollectEvidenceArgs) ([]EvidenceBatch, error) {
	_ = ctx
	round, err := f.roundAt(args.Round)
	if err != nil {
		return nil, err
	}
	batches := []EvidenceBatch{}
	for _, batch := range round.EvidenceBatches {
		if batch.Phase != args.Phase || batch.Actor != args.Actor || len(batch.Items) == 0 {
			continue
		}
		collectedBy := batch.CollectedBy
		if collectedBy == "" {
			collectedBy = args.Actor
		}
		batches = append(batches, EvidenceBatch{
			Items:       toEvidenceInput(batch.Items),
			CollectedBy: collectedBy,
			Phase:       args.Phase,
		})
	}
	return batches, nil
}

func (f *fixtureAdapter) Propose(ctx context.Context, args ProposeArgs) (*Proposal, error) {
	_ = ctx
	round, err := f.roundAt(args.Round)
	if err != nil {
		return nil, err
	}
	return round.Proposal, nil
}

func replaceEvidenceKeys(keys []string, evidenceKeyMap map[string]string) ([]string, error) {
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		mapped, ok := evidenceKeyMap[key]
		if !ok {
			return nil, fmt.Errorf("unknown evidence key %s", key)
		}
		result = append(result, mapped)
	}
	return result, nil
}

func (f *fixtureAdapter) Review(ctx context.Context, args ReviewArgs) ([]Finding, error) {
	_ = ctx
	round, err := f.roundAt(args.Round)
	if err != nil {
		return nil, err
	}
	fixtureFindings := round.FindingsAgainstProposal
	if len(fixtureFindings) == 0 {
		fixtureFindings = round.FindingsLegacy
	}
	findings := []Finding{}
	for _, finding := range fixtureFindings {
		if finding.Critic != args.Critic {
			continue
		}
		evidenceIDs, err := replaceEvidenceKeys(finding.EvidenceKeys, args.EvidenceKeyMap)
		if err != nil {
			return nil, err
		}
		findings = append(findings, Finding{
			ID:              finding.ID,
			Critic:          finding.Critic,
			Severity:        finding.Severity,
			Basis:           finding.Basis,
			Summary:         finding.Summary,
			Rationale:       finding.Rationale,
			SuggestedChange: finding.SuggestedChange,
			EvidenceIDs:     evidenceIDs,
		})
	}
	return findings, nil
}

func (f *fixtureAdapter) Adjudicate(ctx context.Context, args AdjudicateArgs) (*Verdict, error) {
	_ = ctx
	round, err := f.roundAt(args.Round)
	if err != nil {
		return nil, err
	}
	if round.Verdict == nil {
		return nil, nil
	}
	decisions := make([]Decision, 0, len(round.Verdict.Decisions))
	for _, decision := range round.Verdict.Decisions {
		evidenceIDs, err := replaceEvidenceKeys(decision.EvidenceKeys, args.EvidenceKeyMap)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, Decision{
			FindingID:   decision.FindingID,
			Disposition: decision.Disposition,
			Rationale:   decision.Rationale,
			EvidenceIDs: evidenceIDs,
		})
	}
	return &Verdict{
		Summary:         round.Verdict.Summary,
		RevisedProposal: round.Verdict.RevisedProposal,
		Decisions:       decisions,
	}, nil
}
