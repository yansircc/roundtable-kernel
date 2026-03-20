package rtk

import "encoding/json"

func EvidenceBatchesFromResult(result map[string]any, actor string, phase string) ([]EvidenceBatch, error) {
	result = SemanticResult(result)
	itemsRaw, ok := result["items"]
	if !ok || itemsRaw == nil {
		return []EvidenceBatch{}, nil
	}
	data, err := json.Marshal(itemsRaw)
	if err != nil {
		return nil, err
	}
	items := []EvidenceInput{}
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []EvidenceBatch{}, nil
	}
	collectedBy := actor
	if value, ok := result["collected_by"].(string); ok && value != "" {
		collectedBy = value
	}
	return []EvidenceBatch{{Items: items, CollectedBy: collectedBy, Phase: phase}}, nil
}

func ProposalFromResult(result map[string]any) (*Proposal, error) {
	result = SemanticResult(result)
	proposalRaw := result["proposal"]
	if proposalRaw == nil {
		proposalRaw = result
	}
	data, err := json.Marshal(proposalRaw)
	if err != nil {
		return nil, err
	}
	proposal := &Proposal{}
	return proposal, json.Unmarshal(data, proposal)
}

func FindingsFromResult(result map[string]any) ([]Finding, error) {
	result = SemanticResult(result)
	findingsRaw := result["findings"]
	if findingsRaw == nil {
		return []Finding{}, nil
	}
	data, err := json.Marshal(findingsRaw)
	if err != nil {
		return nil, err
	}
	findings := []Finding{}
	return findings, json.Unmarshal(data, &findings)
}

func VerdictFromResult(result map[string]any) (*Verdict, error) {
	result = SemanticResult(result)
	verdictRaw := result["verdict"]
	if verdictRaw == nil {
		if len(result) == 0 {
			return nil, nil
		}
		verdictRaw = result
	}
	data, err := json.Marshal(verdictRaw)
	if err != nil {
		return nil, err
	}
	verdict := &Verdict{}
	return verdict, json.Unmarshal(data, verdict)
}
