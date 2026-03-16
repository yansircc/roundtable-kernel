package rtk

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
)

type execAgentSpec struct {
	Cmd       []string          `json:"cmd"`
	Cwd       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
	TimeoutMS int               `json:"timeout_ms"`
}

type execSpec struct {
	Topic     string   `json:"topic"`
	Chair     string   `json:"chair"`
	Critics   []string `json:"critics"`
	MaxRounds int      `json:"max_rounds"`
	SeedBatch *struct {
		Actor       string                `json:"actor"`
		CollectedBy string                `json:"collected_by"`
		Items       []fixtureEvidenceItem `json:"items"`
	} `json:"seed_batch"`
	Agent  execAgentSpec            `json:"agent"`
	Actors map[string]execAgentSpec `json:"actors"`
}

type execAdapter struct {
	spec          execSpec
	specDir       string
	telemetryFile string
}

func newExecAdapter(specPath string, telemetryFile string) (Adapter, error) {
	spec := execSpec{}
	resolved := filepath.Clean(specPath)
	if err := readJSONFile(resolved, &spec); err != nil {
		return nil, err
	}
	if spec.Topic == "" {
		return nil, fmt.Errorf("exec spec.topic must be a non-empty string")
	}
	if spec.Chair == "" {
		return nil, fmt.Errorf("exec spec.chair must be a non-empty string")
	}
	if spec.MaxRounds <= 0 {
		return nil, fmt.Errorf("exec spec.max_rounds must be a positive integer")
	}
	if len(spec.Agent.Cmd) == 0 {
		return nil, fmt.Errorf("exec spec.agent must define a non-empty cmd array")
	}
	return &execAdapter{spec: spec, specDir: filepath.Dir(resolved), telemetryFile: telemetryFile}, nil
}

func (e *execAdapter) Kind() string { return "exec" }

func (e *execAdapter) Metadata() AdapterMetadata {
	return AdapterMetadata{
		Topic:     e.spec.Topic,
		Chair:     e.spec.Chair,
		Critics:   append([]string{}, e.spec.Critics...),
		MaxRounds: e.spec.MaxRounds,
	}
}

func (e *execAdapter) resolveAgent(actor string) execAgentSpec {
	merged := e.spec.Agent
	if override, ok := e.spec.Actors[actor]; ok {
		if len(override.Cmd) > 0 {
			merged.Cmd = override.Cmd
		}
		if override.Cwd != "" {
			merged.Cwd = override.Cwd
		}
		if override.Env != nil {
			if merged.Env == nil {
				merged.Env = map[string]string{}
			}
			for key, value := range override.Env {
				merged.Env[key] = value
			}
		}
		if override.TimeoutMS > 0 {
			merged.TimeoutMS = override.TimeoutMS
		}
	}
	if merged.TimeoutMS <= 0 {
		merged.TimeoutMS = 60000
	}
	if merged.Env == nil {
		merged.Env = map[string]string{}
	}
	return merged
}

func (e *execAdapter) resolveCwd(raw string) string {
	if raw == "" {
		return e.specDir
	}
	if filepath.IsAbs(raw) {
		return raw
	}
	return filepath.Join(e.specDir, raw)
}

func payloadSessionID(payload map[string]any) any {
	if session, ok := payload["session"].(*Session); ok && session != nil {
		return session.ID
	}
	if session, ok := payload["session"].(map[string]any); ok {
		return session["id"]
	}
	return nil
}

func (e *execAdapter) invoke(ctx context.Context, actor string, payload map[string]any) (map[string]any, error) {
	agent := e.resolveAgent(actor)
	env := map[string]string{}
	for key, value := range agent.Env {
		env[key] = value
	}
	if e.telemetryFile != "" {
		env["ROUNDTABLE_TELEMETRY_FILE"] = e.telemetryFile
	}
	env["ROUNDTABLE_ADAPTER_KIND"] = "exec"
	return RunJSONCommand(CommandOptions{
		Cmd:     agent.Cmd,
		Cwd:     e.resolveCwd(agent.Cwd),
		Env:     env,
		Timeout: durationMS(agent.TimeoutMS),
		Telemetry: &CommandTelemetry{
			File: e.telemetryFile,
			Context: map[string]any{
				"session_id": payloadSessionID(payload),
				"round":      payload["round"],
				"actor":      actor,
				"phase":      payload["phase"],
				"adapter":    "exec",
				"source":     "exec_adapter",
			},
		},
		Input: payload,
	})
}

func asEvidenceBatches(result map[string]any, actor string, phase string) ([]EvidenceBatch, error) {
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

func (e *execAdapter) SeedEvidence(ctx context.Context) ([]EvidenceBatch, error) {
	_ = ctx
	if e.spec.SeedBatch == nil || len(e.spec.SeedBatch.Items) == 0 {
		return []EvidenceBatch{}, nil
	}
	collectedBy := e.spec.SeedBatch.CollectedBy
	if collectedBy == "" {
		if e.spec.SeedBatch.Actor != "" {
			collectedBy = e.spec.SeedBatch.Actor
		} else {
			collectedBy = e.spec.Chair
		}
	}
	return []EvidenceBatch{{
		Items:       toEvidenceInput(e.spec.SeedBatch.Items),
		CollectedBy: collectedBy,
		Phase:       "seed",
	}}, nil
}

func (e *execAdapter) CollectEvidence(ctx context.Context, args CollectEvidenceArgs) ([]EvidenceBatch, error) {
	result, err := e.invoke(ctx, args.Actor, map[string]any{
		"protocol": "roundtable-kernel.exec.v1",
		"actor":    args.Actor,
		"phase":    args.Phase,
		"round":    args.Round,
		"session":  args.Session,
	})
	if err != nil {
		return nil, err
	}
	return asEvidenceBatches(result, args.Actor, args.Phase)
}

func (e *execAdapter) Propose(ctx context.Context, args ProposeArgs) (*Proposal, error) {
	result, err := e.invoke(ctx, e.spec.Chair, map[string]any{
		"protocol": "roundtable-kernel.exec.v1",
		"actor":    e.spec.Chair,
		"phase":    "propose",
		"round":    args.Round,
		"session":  args.Session,
	})
	if err != nil {
		return nil, err
	}
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

func (e *execAdapter) Review(ctx context.Context, args ReviewArgs) ([]Finding, error) {
	result, err := e.invoke(ctx, args.Critic, map[string]any{
		"protocol": "roundtable-kernel.exec.v1",
		"actor":    args.Critic,
		"phase":    "review",
		"round":    args.Round,
		"session":  args.Session,
		"proposal": args.Proposal,
	})
	if err != nil {
		return nil, err
	}
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

func (e *execAdapter) Adjudicate(ctx context.Context, args AdjudicateArgs) (*Verdict, error) {
	if len(args.Findings) == 0 {
		return nil, nil
	}
	result, err := e.invoke(ctx, e.spec.Chair, map[string]any{
		"protocol": "roundtable-kernel.exec.v1",
		"actor":    e.spec.Chair,
		"phase":    "adjudicate",
		"round":    args.Round,
		"session":  args.Session,
		"proposal": args.Proposal,
		"findings": args.Findings,
	})
	if err != nil {
		return nil, err
	}
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
