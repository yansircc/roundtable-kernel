package rtk

import "strings"

type tokenPricing struct {
	Input                       float64
	CachedInput                 *float64
	Output                      float64
	LongContextInputThreshold   *int64
	LongContextInputMultiplier  float64
	LongContextOutputMultiplier float64
}

func EstimateOpenAICostUSD(model string, usage *PhaseUsage) *float64 {
	pricing, ok := openAIModelPricing(strings.TrimSpace(model))
	if !ok || usage == nil {
		return nil
	}
	inputTokens := int64ValueOrZero(usage.InputTokens)
	cachedTokens := int64ValueOrZero(usage.CacheReadInputTokens)
	if cachedTokens > inputTokens {
		cachedTokens = inputTokens
	}
	uncachedTokens := inputTokens - cachedTokens
	outputTokens := int64ValueOrZero(usage.OutputTokens)
	inputRate := pricing.Input
	cachedRate := pricing.cachedInputRate()
	outputRate := pricing.Output
	if pricing.isLongContext(inputTokens) {
		inputRate *= pricing.longContextInputMultiplier()
		cachedRate *= pricing.longContextInputMultiplier()
		outputRate *= pricing.longContextOutputMultiplier()
	}
	total := (float64(uncachedTokens) * inputRate) + (float64(cachedTokens) * cachedRate) + (float64(outputTokens) * outputRate)
	return float64Ptr(total / 1_000_000.0)
}

func openAIModelPricing(model string) (tokenPricing, bool) {
	switch normalizeOpenAIModel(model) {
	case "gpt-5.4":
		return tokenPricing{
			Input:                       2.50,
			CachedInput:                 float64Ptr(0.25),
			Output:                      15.00,
			LongContextInputThreshold:   int64Ptr(272000),
			LongContextInputMultiplier:  2.0,
			LongContextOutputMultiplier: 1.5,
		}, true
	case "gpt-5.4-pro":
		return tokenPricing{
			Input:                       30.00,
			CachedInput:                 nil,
			Output:                      180.00,
			LongContextInputThreshold:   int64Ptr(272000),
			LongContextInputMultiplier:  2.0,
			LongContextOutputMultiplier: 1.5,
		}, true
	case "gpt-5.4-mini":
		return tokenPricing{Input: 0.75, CachedInput: float64Ptr(0.075), Output: 4.50}, true
	case "gpt-5.4-nano":
		return tokenPricing{Input: 0.20, CachedInput: float64Ptr(0.02), Output: 1.25}, true
	case "gpt-5.2":
		return tokenPricing{Input: 1.75, CachedInput: float64Ptr(0.175), Output: 14.00}, true
	case "gpt-5.2-pro":
		return tokenPricing{Input: 21.00, CachedInput: nil, Output: 168.00}, true
	case "gpt-5.1":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5-mini":
		return tokenPricing{Input: 0.25, CachedInput: float64Ptr(0.025), Output: 2.00}, true
	case "gpt-5-nano":
		return tokenPricing{Input: 0.05, CachedInput: float64Ptr(0.005), Output: 0.40}, true
	case "gpt-5.3-codex":
		return tokenPricing{Input: 1.75, CachedInput: float64Ptr(0.175), Output: 14.00}, true
	case "gpt-5.2-codex":
		return tokenPricing{Input: 1.75, CachedInput: float64Ptr(0.175), Output: 14.00}, true
	case "gpt-5.1-codex-max":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5.1-codex":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5-codex":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5.1-codex-mini":
		return tokenPricing{Input: 0.25, CachedInput: float64Ptr(0.025), Output: 2.00}, true
	case "codex-mini-latest":
		return tokenPricing{Input: 1.50, CachedInput: float64Ptr(0.375), Output: 6.00}, true
	case "gpt-5.3-chat-latest":
		return tokenPricing{Input: 1.75, CachedInput: float64Ptr(0.175), Output: 14.00}, true
	case "gpt-5.2-chat-latest":
		return tokenPricing{Input: 1.75, CachedInput: float64Ptr(0.175), Output: 14.00}, true
	case "gpt-5.1-chat-latest":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	case "gpt-5-chat-latest":
		return tokenPricing{Input: 1.25, CachedInput: float64Ptr(0.125), Output: 10.00}, true
	default:
		return tokenPricing{}, false
	}
}

func normalizeOpenAIModel(model string) string {
	model = strings.TrimSpace(model)
	switch {
	case strings.HasPrefix(model, "gpt-5.4-pro-"):
		return "gpt-5.4-pro"
	case strings.HasPrefix(model, "gpt-5.4-mini-"):
		return "gpt-5.4-mini"
	case strings.HasPrefix(model, "gpt-5.4-nano-"):
		return "gpt-5.4-nano"
	case strings.HasPrefix(model, "gpt-5.4-"):
		return "gpt-5.4"
	case strings.HasPrefix(model, "gpt-5.2-pro-"):
		return "gpt-5.2-pro"
	case strings.HasPrefix(model, "gpt-5.2-"):
		return "gpt-5.2"
	case strings.HasPrefix(model, "gpt-5.1-") && !strings.HasPrefix(model, "gpt-5.1-codex"):
		return "gpt-5.1"
	case strings.HasPrefix(model, "gpt-5-mini-"):
		return "gpt-5-mini"
	case strings.HasPrefix(model, "gpt-5-nano-"):
		return "gpt-5-nano"
	case strings.HasPrefix(model, "gpt-5-") && !strings.HasPrefix(model, "gpt-5-codex"):
		return "gpt-5"
	default:
		return model
	}
}

func (p tokenPricing) cachedInputRate() float64 {
	if p.CachedInput == nil {
		return p.Input
	}
	return *p.CachedInput
}

func (p tokenPricing) isLongContext(inputTokens int64) bool {
	return p.LongContextInputThreshold != nil && inputTokens > *p.LongContextInputThreshold
}

func (p tokenPricing) longContextInputMultiplier() float64 {
	if p.LongContextInputMultiplier == 0 {
		return 1
	}
	return p.LongContextInputMultiplier
}

func (p tokenPricing) longContextOutputMultiplier() float64 {
	if p.LongContextOutputMultiplier == 0 {
		return 1
	}
	return p.LongContextOutputMultiplier
}

func int64ValueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
