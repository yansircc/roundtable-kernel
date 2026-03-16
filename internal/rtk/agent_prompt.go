package rtk

import (
	"encoding/json"
	"fmt"
	"strings"
)

func phaseTask(request AgentRequest) []string {
	switch request.Phase {
	case "explore":
		return []string{
			"Inspect the workspace read-only and gather baseline evidence relevant to the topic.",
			"Return up to 5 evidence items.",
			"If nothing new matters, return {\"items\":[]}.",
		}
	case "re-explore":
		return []string{
			"Challenge the current direction and gather only targeted evidence that may change the proposal.",
			"Return up to 5 evidence items.",
			"If the ledger is already sufficient, return {\"items\":[]}.",
		}
	case "propose":
		return []string{
			"Produce the best current proposal.",
			"Keep it compressed: one summary, a few core claims, a few acceptance criteria.",
			"Do not mention evidence IDs in the proposal object.",
		}
	case "review":
		return []string{
			fmt.Sprintf("Review the current proposal as critic %s.", request.Actor),
			fmt.Sprintf("Use finding ids of the form \"%s:F1\", \"%s:F2\", ...", request.Actor, request.Actor),
			"Report only real findings against the proposal.",
			"If there are no findings, return {\"findings\":[]}.",
		}
	case "adjudicate":
		return []string{
			"Adjudicate the current findings.",
			"Return exactly one decision per finding.",
			"If accepted findings change the plan, revise the proposal. Otherwise you may keep revised_proposal null.",
			"If there are no findings, return {\"verdict\":null}.",
		}
	default:
		panic(fmt.Sprintf("unsupported phase %s", request.Phase))
	}
}

func PromptForRequest(request AgentRequest) string {
	contextPayload := map[string]any{
		"protocol": request.Protocol,
		"actor":    request.Actor,
		"phase":    request.Phase,
		"round":    request.Round,
		"session":  CompactSession(request.Session),
		"proposal": request.Proposal,
		"findings": request.Findings,
	}
	sharedRules := []string{
		"You are participating in a local roundtable.",
		"Return only JSON that matches the provided schema.",
		"Do not use markdown fences or explanatory prose.",
		"Prefer derivation over enumeration.",
		"Never invent evidence IDs.",
		"A supported finding or decision must cite evidence_ids from session.evidence.",
		"A gap finding must use basis='gap' and evidence_ids=[].",
		"Severity belongs on findings, not on verdict decisions.",
	}
	lines := append([]string{}, sharedRules...)
	lines = append(lines, "", "Task:")
	for _, line := range phaseTask(request) {
		lines = append(lines, "- "+line)
	}
	lines = append(lines, "", "Context JSON:")
	contextJSON, _ := json.MarshalIndent(contextPayload, "", "  ")
	lines = append(lines, string(contextJSON))
	return strings.Join(lines, "\n")
}
