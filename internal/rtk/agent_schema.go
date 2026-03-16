package rtk

import "fmt"

func evidenceItemSchema() map[string]any {
	return map[string]any{
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
}

func proposalSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"summary":    map[string]any{"type": "string"},
			"claims":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"acceptance": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		},
		"required": []string{"summary", "claims", "acceptance"},
	}
}

func findingSchema() map[string]any {
	return map[string]any{
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
}

func verdictSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"summary":          map[string]any{"type": "string"},
			"revised_proposal": map[string]any{"anyOf": []any{map[string]any{"type": "null"}, proposalSchema()}},
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
}

func OutputSchemaForPhase(phase string) map[string]any {
	switch phase {
	case "explore", "re-explore":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"items": map[string]any{"type": "array", "items": evidenceItemSchema()},
			},
			"required": []string{"items"},
		}
	case "propose":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"proposal": proposalSchema(),
			},
			"required": []string{"proposal"},
		}
	case "review":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"findings": map[string]any{"type": "array", "items": findingSchema()},
			},
			"required": []string{"findings"},
		}
	case "adjudicate":
		return map[string]any{
			"type":                 "object",
			"additionalProperties": false,
			"properties": map[string]any{
				"verdict": map[string]any{"anyOf": []any{map[string]any{"type": "null"}, verdictSchema()}},
			},
			"required": []string{"verdict"},
		}
	default:
		panic(fmt.Sprintf("unsupported phase %s", phase))
	}
}
