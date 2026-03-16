package rtk

import (
	"fmt"
	"strings"
)

var (
	materialSeverities = map[string]bool{"high": true, "medium": true}
	allSeverities      = map[string]bool{"high": true, "medium": true, "low": true}
	findingBasis       = map[string]bool{"supported": true, "gap": true}
	decisions          = map[string]bool{"accept": true, "reject": true}
)

func invariant(condition bool, message string) error {
	if !condition {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func validateString(value string, path string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must be a non-empty string", path)
	}
	return nil
}

func ValidateProposal(proposal *Proposal, path string) error {
	if path == "" {
		path = "proposal"
	}
	if proposal == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	if err := validateString(proposal.Summary, path+".summary"); err != nil {
		return err
	}
	return nil
}

func ValidateEvidence(evidence *Evidence, path string) error {
	if path == "" {
		path = "evidence"
	}
	if evidence == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	if err := validateString(evidence.ID, path+".id"); err != nil {
		return err
	}
	if err := validateString(evidence.Source, path+".source"); err != nil {
		return err
	}
	if err := validateString(evidence.Kind, path+".kind"); err != nil {
		return err
	}
	if err := validateString(evidence.Phase, path+".phase"); err != nil {
		return err
	}
	if err := validateString(evidence.Statement, path+".statement"); err != nil {
		return err
	}
	if err := validateString(evidence.Excerpt, path+".excerpt"); err != nil {
		return err
	}
	if err := validateString(evidence.CollectedBy, path+".collected_by"); err != nil {
		return err
	}
	if err := invariant(evidence.Round >= 0, path+".round must be a non-negative integer"); err != nil {
		return err
	}
	return validateString(evidence.CreatedAt, path+".created_at")
}

func ValidateFinding(finding *Finding, evidenceIndex map[string]Evidence, path string) error {
	if path == "" {
		path = "finding"
	}
	if finding == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	if err := validateString(finding.ID, path+".id"); err != nil {
		return err
	}
	if err := validateString(finding.Critic, path+".critic"); err != nil {
		return err
	}
	if err := validateString(finding.Summary, path+".summary"); err != nil {
		return err
	}
	if err := validateString(finding.Rationale, path+".rationale"); err != nil {
		return err
	}
	if err := validateString(finding.SuggestedChange, path+".suggested_change"); err != nil {
		return err
	}
	if !allSeverities[finding.Severity] {
		return fmt.Errorf("%s.severity must be one of high|medium|low", path)
	}
	if !findingBasis[finding.Basis] {
		return fmt.Errorf("%s.basis must be one of supported|gap", path)
	}
	if finding.Basis == "supported" && len(finding.EvidenceIDs) == 0 {
		return fmt.Errorf("%s.evidence_ids must be non-empty for supported findings", path)
	}
	if finding.Basis == "gap" && len(finding.EvidenceIDs) != 0 {
		return fmt.Errorf("%s.evidence_ids must be empty for gap findings", path)
	}
	for index, evidenceID := range finding.EvidenceIDs {
		if err := validateString(evidenceID, fmt.Sprintf("%s.evidence_ids[%d]", path, index)); err != nil {
			return err
		}
		if _, ok := evidenceIndex[evidenceID]; !ok {
			return fmt.Errorf("%s.evidence_ids[%d] references unknown evidence %s", path, index, evidenceID)
		}
	}
	return nil
}

func ValidateDecision(decision *Decision, findingIndex map[string]Finding, evidenceIndex map[string]Evidence, path string) error {
	if path == "" {
		path = "decision"
	}
	if decision == nil {
		return fmt.Errorf("%s must be an object", path)
	}
	if err := validateString(decision.FindingID, path+".finding_id"); err != nil {
		return err
	}
	if !decisions[decision.Disposition] {
		return fmt.Errorf("%s.disposition must be one of accept|reject", path)
	}
	if err := validateString(decision.Rationale, path+".rationale"); err != nil {
		return err
	}
	finding, ok := findingIndex[decision.FindingID]
	if !ok {
		return fmt.Errorf("%s.finding_id references unknown finding %s", path, decision.FindingID)
	}
	if finding.Basis == "supported" && len(decision.EvidenceIDs) == 0 {
		return fmt.Errorf("%s.evidence_ids must be non-empty for supported findings", path)
	}
	for index, evidenceID := range decision.EvidenceIDs {
		if err := validateString(evidenceID, fmt.Sprintf("%s.evidence_ids[%d]", path, index)); err != nil {
			return err
		}
		if _, ok := evidenceIndex[evidenceID]; !ok {
			return fmt.Errorf("%s.evidence_ids[%d] references unknown evidence %s", path, index, evidenceID)
		}
	}
	return nil
}

func FindingCounts(findings []Finding) Counts {
	counts := Counts{}
	for _, finding := range findings {
		counts.Total++
		switch finding.Severity {
		case "high":
			counts.High++
		case "medium":
			counts.Medium++
		case "low":
			counts.Low++
		}
		if materialSeverities[finding.Severity] {
			counts.Material++
		}
		if finding.Basis == "gap" {
			counts.Gaps++
		}
	}
	return counts
}

func IsMaterialFinding(finding Finding) bool {
	return materialSeverities[finding.Severity]
}
