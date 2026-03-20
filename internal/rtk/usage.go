package rtk

import (
	"bufio"
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

const resultMetaKey = "_rtk"

type PhaseUsage struct {
	Provider                 string            `json:"provider,omitempty"`
	Model                    string            `json:"model,omitempty"`
	InputTokens              *int64            `json:"input_tokens,omitempty"`
	OutputTokens             *int64            `json:"output_tokens,omitempty"`
	CacheReadInputTokens     *int64            `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens *int64            `json:"cache_creation_input_tokens,omitempty"`
	CostUSD                  *float64          `json:"cost_usd,omitempty"`
	CostSource               string            `json:"cost_source,omitempty"`
	Models                   []PhaseModelUsage `json:"models,omitempty"`
}

type PhaseModelUsage struct {
	Model                    string   `json:"model"`
	InputTokens              *int64   `json:"input_tokens,omitempty"`
	OutputTokens             *int64   `json:"output_tokens,omitempty"`
	CacheReadInputTokens     *int64   `json:"cache_read_input_tokens,omitempty"`
	CacheCreationInputTokens *int64   `json:"cache_creation_input_tokens,omitempty"`
	CostUSD                  *float64 `json:"cost_usd,omitempty"`
}

type AgentPhaseResult struct {
	Value any
	Usage *PhaseUsage
}

func AttachUsage(result map[string]any, usage *PhaseUsage) map[string]any {
	usage = normalizePhaseUsage(usage)
	if usage == nil {
		return result
	}
	enriched := cloneMap(result)
	enriched[resultMetaKey] = map[string]any{
		"usage": usage,
	}
	return enriched
}

func ResultUsage(result map[string]any) *PhaseUsage {
	if result == nil {
		return nil
	}
	metaRaw, ok := result[resultMetaKey]
	if !ok || metaRaw == nil {
		return nil
	}
	metaMap, ok := metaRaw.(map[string]any)
	if !ok {
		return nil
	}
	usageRaw, ok := metaMap["usage"]
	if !ok || usageRaw == nil {
		return nil
	}
	data, err := json.Marshal(usageRaw)
	if err != nil {
		return nil
	}
	usage := &PhaseUsage{}
	if err := json.Unmarshal(data, usage); err != nil {
		return nil
	}
	return normalizePhaseUsage(usage)
}

func SemanticResult(result map[string]any) map[string]any {
	if result == nil {
		return nil
	}
	if _, ok := result[resultMetaKey]; !ok {
		return result
	}
	semantic := make(map[string]any, len(result)-1)
	for key, value := range result {
		if key == resultMetaKey {
			continue
		}
		semantic[key] = value
	}
	return semantic
}

func ClaudePhaseUsage(resultEvent map[string]any, configuredModel string) *PhaseUsage {
	usage := &PhaseUsage{
		Provider: "claude",
		Model:    strings.TrimSpace(configuredModel),
	}
	applyTokenUsage(usage, mapValue(resultEvent["usage"]), "cache_read_input_tokens")
	usage.CostUSD = float64Value(resultEvent["total_cost_usd"])
	if usage.CostUSD != nil {
		usage.CostSource = "provider"
	}
	usage.Models = claudeModelUsage(mapValue(resultEvent["modelUsage"]))
	if len(usage.Models) == 1 {
		usage.Model = usage.Models[0].Model
	} else if len(usage.Models) > 1 {
		usage.Model = ""
	}
	return normalizePhaseUsage(usage)
}

func CodexPhaseUsage(stdout string, configuredModel string) *PhaseUsage {
	usage := &PhaseUsage{
		Provider: "codex",
		Model:    strings.TrimSpace(configuredModel),
	}
	scanner := bufio.NewScanner(bytes.NewBufferString(stdout))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		event := map[string]any{}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event["type"] != "turn.completed" {
			continue
		}
		applyTokenUsage(usage, mapValue(event["usage"]), "cached_input_tokens")
	}
	if usage.Model != "" && hasTokenUsage(usage) {
		usage.Models = []PhaseModelUsage{{
			Model:                    usage.Model,
			InputTokens:              cloneInt64Ptr(usage.InputTokens),
			OutputTokens:             cloneInt64Ptr(usage.OutputTokens),
			CacheReadInputTokens:     cloneInt64Ptr(usage.CacheReadInputTokens),
			CacheCreationInputTokens: cloneInt64Ptr(usage.CacheCreationInputTokens),
		}}
	}
	if usage.Model != "" {
		usage.CostUSD = EstimateOpenAICostUSD(usage.Model, usage)
		if usage.CostUSD != nil {
			usage.CostSource = "official_pricing_estimate_tokens_only"
			if len(usage.Models) == 1 {
				usage.Models[0].CostUSD = cloneFloat64Ptr(usage.CostUSD)
			}
		}
	}
	return normalizePhaseUsage(usage)
}

func claudeModelUsage(raw map[string]any) []PhaseModelUsage {
	if len(raw) == 0 {
		return nil
	}
	models := make([]string, 0, len(raw))
	for model := range raw {
		models = append(models, model)
	}
	sort.Strings(models)
	usage := make([]PhaseModelUsage, 0, len(models))
	for _, model := range models {
		entry := mapValue(raw[model])
		item := PhaseModelUsage{
			Model:                    model,
			InputTokens:              int64Value(entry["inputTokens"]),
			OutputTokens:             int64Value(entry["outputTokens"]),
			CacheReadInputTokens:     int64Value(entry["cacheReadInputTokens"]),
			CacheCreationInputTokens: int64Value(entry["cacheCreationInputTokens"]),
			CostUSD:                  float64Value(entry["costUSD"]),
		}
		if hasModelUsage(item) {
			usage = append(usage, item)
		}
	}
	if len(usage) == 0 {
		return nil
	}
	return usage
}

func applyTokenUsage(usage *PhaseUsage, raw map[string]any, cachedInputKey string) {
	if usage == nil || len(raw) == 0 {
		return
	}
	addInt64(&usage.InputTokens, int64Value(raw["input_tokens"]))
	addInt64(&usage.OutputTokens, int64Value(raw["output_tokens"]))
	addInt64(&usage.CacheReadInputTokens, int64Value(raw[cachedInputKey]))
	addInt64(&usage.CacheCreationInputTokens, int64Value(raw["cache_creation_input_tokens"]))
}

func normalizePhaseUsage(usage *PhaseUsage) *PhaseUsage {
	if usage == nil {
		return nil
	}
	copy := &PhaseUsage{
		Provider:                 strings.TrimSpace(usage.Provider),
		Model:                    strings.TrimSpace(usage.Model),
		InputTokens:              cloneInt64Ptr(usage.InputTokens),
		OutputTokens:             cloneInt64Ptr(usage.OutputTokens),
		CacheReadInputTokens:     cloneInt64Ptr(usage.CacheReadInputTokens),
		CacheCreationInputTokens: cloneInt64Ptr(usage.CacheCreationInputTokens),
		CostUSD:                  cloneFloat64Ptr(usage.CostUSD),
		CostSource:               strings.TrimSpace(usage.CostSource),
	}
	for _, model := range usage.Models {
		if !hasModelUsage(model) {
			continue
		}
		copy.Models = append(copy.Models, PhaseModelUsage{
			Model:                    strings.TrimSpace(model.Model),
			InputTokens:              cloneInt64Ptr(model.InputTokens),
			OutputTokens:             cloneInt64Ptr(model.OutputTokens),
			CacheReadInputTokens:     cloneInt64Ptr(model.CacheReadInputTokens),
			CacheCreationInputTokens: cloneInt64Ptr(model.CacheCreationInputTokens),
			CostUSD:                  cloneFloat64Ptr(model.CostUSD),
		})
	}
	if copy.Model == "" && len(copy.Models) == 1 {
		copy.Model = copy.Models[0].Model
	}
	if copy.CostUSD != nil && copy.CostSource == "" {
		copy.CostSource = "provider"
	}
	if !hasPhaseUsage(copy) {
		return nil
	}
	return copy
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func mapValue(value any) map[string]any {
	mapped, _ := value.(map[string]any)
	return mapped
}

func int64Value(value any) *int64 {
	switch typed := value.(type) {
	case int:
		return int64Ptr(int64(typed))
	case int64:
		return int64Ptr(typed)
	case float64:
		return int64Ptr(int64(typed))
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		return int64Ptr(parsed)
	default:
		return nil
	}
}

func float64Value(value any) *float64 {
	switch typed := value.(type) {
	case float64:
		return float64Ptr(typed)
	case float32:
		return float64Ptr(float64(typed))
	case int:
		return float64Ptr(float64(typed))
	case int64:
		return float64Ptr(float64(typed))
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return nil
		}
		return float64Ptr(parsed)
	default:
		return nil
	}
}

func addInt64(target **int64, value *int64) {
	if value == nil {
		return
	}
	if *target == nil {
		*target = int64Ptr(0)
	}
	**target += *value
}

func cloneInt64Ptr(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func hasTokenUsage(usage *PhaseUsage) bool {
	return usage != nil && (usage.InputTokens != nil || usage.OutputTokens != nil || usage.CacheReadInputTokens != nil || usage.CacheCreationInputTokens != nil)
}

func hasModelUsage(model PhaseModelUsage) bool {
	return model.InputTokens != nil || model.OutputTokens != nil || model.CacheReadInputTokens != nil || model.CacheCreationInputTokens != nil || model.CostUSD != nil
}

func hasPhaseUsage(usage *PhaseUsage) bool {
	return usage != nil && (hasTokenUsage(usage) || usage.CostUSD != nil || len(usage.Models) > 0)
}
