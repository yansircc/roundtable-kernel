package rtk

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestProposalFromResultIgnoresUsageMetadata(t *testing.T) {
	t.Parallel()

	result := AttachUsage(map[string]any{
		"proposal": map[string]any{
			"summary":    "proposal",
			"claims":     []string{"claim"},
			"acceptance": []string{"acceptance"},
		},
	}, &PhaseUsage{
		Provider:     "claude",
		Model:        "claude-haiku-4-5-20251001",
		InputTokens:  int64Ptr(18),
		OutputTokens: int64Ptr(27),
		CostUSD:      float64Ptr(0.01),
		CostSource:   "provider",
	})

	usage := ResultUsage(result)
	if usage == nil || usage.CostUSD == nil || *usage.CostUSD != 0.01 {
		t.Fatalf("unexpected usage: %#v", usage)
	}

	proposal, err := ProposalFromResult(result)
	if err != nil {
		t.Fatalf("ProposalFromResult: %v", err)
	}
	if proposal.Summary != "proposal" {
		t.Fatalf("unexpected proposal: %#v", proposal)
	}

	semantic := SemanticResult(result)
	if _, ok := semantic[resultMetaKey]; ok {
		t.Fatalf("reserved metadata leaked into semantic result: %#v", semantic)
	}
}

func TestCodexPhaseUsageSumsTurnCompletedEvents(t *testing.T) {
	t.Parallel()

	stdout := `{"type":"thread.started","thread_id":"t1"}
{"type":"turn.completed","usage":{"input_tokens":100,"cached_input_tokens":40,"output_tokens":5}}
{"type":"turn.completed","usage":{"input_tokens":7,"cached_input_tokens":3,"output_tokens":11}}`

	usage := CodexPhaseUsage(stdout, "gpt-5.4")
	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.Model != "gpt-5.4" {
		t.Fatalf("unexpected model: %#v", usage)
	}
	if usage.InputTokens == nil || *usage.InputTokens != 107 {
		t.Fatalf("unexpected input tokens: %#v", usage)
	}
	if usage.CacheReadInputTokens == nil || *usage.CacheReadInputTokens != 43 {
		t.Fatalf("unexpected cached input tokens: %#v", usage)
	}
	if usage.OutputTokens == nil || *usage.OutputTokens != 16 {
		t.Fatalf("unexpected output tokens: %#v", usage)
	}
	if len(usage.Models) != 1 || usage.Models[0].Model != "gpt-5.4" {
		t.Fatalf("unexpected model breakdown: %#v", usage.Models)
	}
	if usage.CostUSD == nil {
		t.Fatalf("expected cost estimate: %#v", usage)
	}
	if usage.CostSource != "official_pricing_estimate_tokens_only" {
		t.Fatalf("unexpected cost source: %#v", usage)
	}
	expected := (float64(107-43)*2.50 + float64(43)*0.25 + float64(16)*15.00) / 1_000_000.0
	if math.Abs(*usage.CostUSD-expected) > 1e-12 {
		t.Fatalf("unexpected estimated cost: got %f want %f", *usage.CostUSD, expected)
	}
}

func TestClaudePhaseUsagePrefersResolvedModelAndProviderCost(t *testing.T) {
	t.Parallel()

	usage := ClaudePhaseUsage(map[string]any{
		"total_cost_usd": 0.0102174,
		"usage": map[string]any{
			"input_tokens":                18.0,
			"cache_creation_input_tokens": 5004.0,
			"cache_read_input_tokens":     14054.0,
			"output_tokens":               277.0,
		},
		"modelUsage": map[string]any{
			"claude-haiku-4-5-20251001": map[string]any{
				"inputTokens":              797.0,
				"outputTokens":             352.0,
				"cacheReadInputTokens":     14054.0,
				"cacheCreationInputTokens": 5004.0,
				"costUSD":                  0.0102174,
			},
		},
	}, "haiku")
	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.Model != "claude-haiku-4-5-20251001" {
		t.Fatalf("unexpected model: %#v", usage)
	}
	if usage.CostUSD == nil || *usage.CostUSD != 0.0102174 {
		t.Fatalf("unexpected cost: %#v", usage)
	}
	if usage.InputTokens == nil || *usage.InputTokens != 18 {
		t.Fatalf("unexpected input tokens: %#v", usage)
	}
	if usage.CacheReadInputTokens == nil || *usage.CacheReadInputTokens != 14054 {
		t.Fatalf("unexpected cache read tokens: %#v", usage)
	}
}

func TestEstimateOpenAICostUSDUsesLongContextPricingForGPT54(t *testing.T) {
	t.Parallel()

	usage := &PhaseUsage{
		InputTokens:  int64Ptr(300000),
		OutputTokens: int64Ptr(1000),
	}
	cost := EstimateOpenAICostUSD("gpt-5.4", usage)
	if cost == nil {
		t.Fatal("expected cost")
	}
	expected := (float64(300000)*5.00 + float64(1000)*22.50) / 1_000_000.0
	if math.Abs(*cost-expected) > 1e-12 {
		t.Fatalf("unexpected long-context cost: got %f want %f", *cost, expected)
	}
}

func TestApplyStepPersistsUsageMetadata(t *testing.T) {
	paths := ResolvePaths(t.TempDir())
	specPath := filepath.Join(paths.Root, "spec.json")
	spec := map[string]any{
		"topic":   "usage persistence",
		"chair":   "chair",
		"critics": []string{},
		"agent": map[string]any{
			"cmd": []string{"true"},
		},
	}
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if err := os.WriteFile(specPath, data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	session, _, err := InitSession(paths, specPath, "usage-live", true)
	if err != nil {
		t.Fatalf("InitSession: %v", err)
	}
	session, next, err := NextStep(paths, session.ID, "chair")
	if err != nil {
		t.Fatalf("NextStep: %v", err)
	}
	if next.Step == nil || next.Step.Phase != "explore" {
		t.Fatalf("unexpected step: %#v", next.Step)
	}

	phaseResult := AttachUsage(map[string]any{
		"items": []map[string]any{{
			"source":    "repo/file.go:1",
			"kind":      "reference",
			"statement": "fact",
			"excerpt":   "fact excerpt",
		}},
	}, &PhaseUsage{
		Provider:     "claude",
		Model:        "claude-haiku-4-5-20251001",
		InputTokens:  int64Ptr(18),
		OutputTokens: int64Ptr(27),
		CostUSD:      float64Ptr(0.01),
		CostSource:   "provider",
	})

	session, err = ApplyStep(paths, session.ID, ApplyInput{
		StartedAt: next.Step.StartedAt,
		Result:    phaseResult,
	})
	if err != nil {
		t.Fatalf("ApplyStep: %v", err)
	}
	if session.OpenRound == nil || len(session.OpenRound.PhaseHistory) == 0 {
		t.Fatalf("missing phase history: %#v", session.OpenRound)
	}
	entry := session.OpenRound.PhaseHistory[0]
	if entry.Usage == nil {
		t.Fatal("expected persisted usage")
	}
	if entry.Usage.Model != "claude-haiku-4-5-20251001" {
		t.Fatalf("unexpected usage: %#v", entry.Usage)
	}
	if entry.Usage.CostUSD == nil || *entry.Usage.CostUSD != 0.01 {
		t.Fatalf("unexpected cost: %#v", entry.Usage)
	}
	if entry.Usage.InputTokens == nil || *entry.Usage.InputTokens != 18 {
		t.Fatalf("unexpected input tokens: %#v", entry.Usage)
	}
}
