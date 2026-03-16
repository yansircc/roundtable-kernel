package rtk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ParsedArgs struct {
	Values      map[string]string
	Flags       map[string]bool
	Positionals []string
}

func ParseArgs(argv []string) ParsedArgs {
	args := ParsedArgs{
		Values:      map[string]string{},
		Flags:       map[string]bool{},
		Positionals: []string{},
	}
	for index := 0; index < len(argv); index++ {
		arg := argv[index]
		if !strings.HasPrefix(arg, "--") {
			args.Positionals = append(args.Positionals, arg)
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		if index+1 >= len(argv) || strings.HasPrefix(argv[index+1], "--") {
			args.Flags[key] = true
			continue
		}
		args.Values[key] = argv[index+1]
		index++
	}
	return args
}

func (p ParsedArgs) Has(key string) bool {
	_, okValue := p.Values[key]
	_, okFlag := p.Flags[key]
	return okValue || okFlag
}

func (p ParsedArgs) Value(key string) string {
	return p.Values[key]
}

func Ensure(value string, message string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s", message)
	}
	return nil
}

type AgentRequest struct {
	Protocol string    `json:"protocol"`
	Actor    string    `json:"actor"`
	Phase    string    `json:"phase"`
	Round    int       `json:"round"`
	Session  *Session  `json:"session"`
	Proposal *Proposal `json:"proposal"`
	Findings []Finding `json:"findings"`
}

func ReadJSONStdin(target any) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func CompactSession(session *Session) map[string]any {
	if session == nil {
		return nil
	}
	compactFindings := func(round *RoundRecord) []map[string]any {
		findings := []map[string]any{}
		for _, finding := range roundFindings(round) {
			findings = append(findings, map[string]any{
				"id":           finding.ID,
				"critic":       finding.Critic,
				"severity":     finding.Severity,
				"basis":        finding.Basis,
				"summary":      finding.Summary,
				"evidence_ids": finding.EvidenceIDs,
			})
		}
		return findings
	}
	rounds := []map[string]any{}
	for index := range session.Rounds {
		round := &session.Rounds[index]
		proposal := ""
		if round.Proposal != nil {
			proposal = round.Proposal.Summary
		}
		verdict := any(nil)
		if round.Verdict != nil {
			verdict = round.Verdict.Summary
		}
		rounds = append(rounds, map[string]any{
			"index":                     round.Index,
			"proposal":                  proposal,
			"findings_against_proposal": compactFindings(round),
			"verdict":                   verdict,
		})
	}
	var openRound any
	if session.OpenRound != nil {
		proposal := ""
		if session.OpenRound.Proposal != nil {
			proposal = session.OpenRound.Proposal.Summary
		}
		verdict := any(nil)
		if session.OpenRound.Verdict != nil {
			verdict = session.OpenRound.Verdict.Summary
		}
		phaseHistory := []map[string]any{}
		for _, phase := range session.OpenRound.PhaseHistory {
			phaseHistory = append(phaseHistory, map[string]any{
				"actor":          phase.Actor,
				"phase":          phase.Phase,
				"status":         phase.Status,
				"output_summary": phase.OutputSummary,
			})
		}
		openRound = map[string]any{
			"index":                     session.OpenRound.Index,
			"proposal":                  proposal,
			"findings_against_proposal": compactFindings(session.OpenRound),
			"verdict":                   verdict,
			"phase_history":             phaseHistory,
			"error":                     session.OpenRound.Error,
		}
	}
	var adjudicated any
	if session.AdjudicatedProposal != nil {
		adjudicated = session.AdjudicatedProposal
	}
	evidence := []map[string]any{}
	for _, item := range session.Evidence {
		evidence = append(evidence, map[string]any{
			"id":        item.ID,
			"phase":     item.Phase,
			"round":     item.Round,
			"source":    item.Source,
			"statement": item.Statement,
		})
	}
	return map[string]any{
		"id":                   session.ID,
		"topic":                session.Topic,
		"chair":                session.Chair,
		"critics":              session.Critics,
		"max_rounds":           session.MaxRounds,
		"status":               session.Status,
		"adjudicated_proposal": adjudicated,
		"evidence":             evidence,
		"rounds":               rounds,
		"open_round":           openRound,
	}
}

func OutputSchemaForPhase(phase string) map[string]any {
	evidenceItem := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"source":    map[string]any{"type": "string"},
			"kind":      map[string]any{"type": "string"},
			"statement": map[string]any{"type": "string"},
			"excerpt":   map[string]any{"type": "string"},
		},
		"required": []string{"source", "kind", "statement", "excerpt"},
	}
	proposal := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"summary":    map[string]any{"type": "string"},
			"claims":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"acceptance": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"summary", "claims", "acceptance"},
	}
	finding := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"id":               map[string]any{"type": "string"},
			"critic":           map[string]any{"type": "string"},
			"severity":         map[string]any{"type": "string", "enum": []string{"high", "medium", "low"}},
			"basis":            map[string]any{"type": "string", "enum": []string{"supported", "gap"}},
			"summary":          map[string]any{"type": "string"},
			"rationale":        map[string]any{"type": "string"},
			"suggested_change": map[string]any{"type": "string"},
			"evidence_ids":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"id", "critic", "severity", "basis", "summary", "rationale", "suggested_change", "evidence_ids"},
	}
	verdict := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"summary":          map[string]any{"type": "string"},
			"revised_proposal": map[string]any{"anyOf": []any{map[string]any{"type": "null"}, proposal}},
			"decisions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"properties": map[string]any{
						"finding_id":  map[string]any{"type": "string"},
						"disposition": map[string]any{"type": "string", "enum": []string{"accept", "reject"}},
						"rationale":   map[string]any{"type": "string"},
						"evidence_ids": map[string]any{
							"type":  "array",
							"items": map[string]any{"type": "string"},
						},
					},
					"required": []string{"finding_id", "disposition", "rationale", "evidence_ids"},
				},
			},
		},
		"required": []string{"summary", "revised_proposal", "decisions"},
	}
	switch phase {
	case "explore", "re-explore":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"items": map[string]any{"type": "array", "items": evidenceItem},
			},
			"required": []string{"items"},
		}
	case "propose":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"proposal": proposal,
			},
			"required": []string{"proposal"},
		}
	case "review":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"findings": map[string]any{"type": "array", "items": finding},
			},
			"required": []string{"findings"},
		}
	case "adjudicate":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"verdict": map[string]any{"anyOf": []any{map[string]any{"type": "null"}, verdict}},
			},
			"required": []string{"verdict"},
		}
	default:
		panic(fmt.Sprintf("unsupported phase %s", phase))
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
	task := []string{}
	switch request.Phase {
	case "explore":
		task = []string{
			"Inspect the workspace read-only and gather baseline evidence relevant to the topic.",
			"Return up to 5 evidence items.",
			"If nothing new matters, return {\"items\":[]}.",
		}
	case "re-explore":
		task = []string{
			"Challenge the current direction and gather only targeted evidence that may change the proposal.",
			"Return up to 5 evidence items.",
			"If the ledger is already sufficient, return {\"items\":[]}.",
		}
	case "propose":
		task = []string{
			"Produce the best current proposal.",
			"Keep it compressed: one summary, a few core claims, a few acceptance criteria.",
			"Do not mention evidence IDs in the proposal object.",
		}
	case "review":
		task = []string{
			fmt.Sprintf("Review the current proposal as critic %s.", request.Actor),
			fmt.Sprintf("Use finding ids of the form \"%s:F1\", \"%s:F2\", ...", request.Actor, request.Actor),
			"Report only real findings against the proposal.",
			"If there are no findings, return {\"findings\":[]}.",
		}
	case "adjudicate":
		task = []string{
			"Adjudicate the current findings.",
			"Return exactly one decision per finding.",
			"If accepted findings change the plan, revise the proposal. Otherwise you may keep revised_proposal null.",
			"If there are no findings, return {\"verdict\":null}.",
		}
	default:
		panic(fmt.Sprintf("unsupported phase %s", request.Phase))
	}
	lines := append([]string{}, sharedRules...)
	lines = append(lines, "", "Task:")
	for _, line := range task {
		lines = append(lines, "- "+line)
	}
	lines = append(lines, "", "Context JSON:")
	contextJSON, _ := json.MarshalIndent(contextPayload, "", "  ")
	lines = append(lines, string(contextJSON))
	return strings.Join(lines, "\n")
}

type TempSchemaHandle struct {
	File string
	Dir  string
}

func WriteTempSchema(schema any) (*TempSchemaHandle, error) {
	dir, err := os.MkdirTemp("", "roundtable-schema-")
	if err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "schema.json")
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		return nil, err
	}
	return &TempSchemaHandle{File: file, Dir: dir}, nil
}

func (h *TempSchemaHandle) Cleanup() {
	if h == nil {
		return
	}
	_ = os.Remove(h.File)
	_ = os.Remove(h.Dir)
}

func PrintJSON(value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(append(data, '\n'))
	return err
}
