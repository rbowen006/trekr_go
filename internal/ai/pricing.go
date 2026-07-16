package ai

// pricing mirrors rv_marketplace's Ai::Pricing (app/services/ai/pricing.rb):
// per-token USD rates keyed by model, and cost = input*rate_in + output*rate_out.
// The values are a contract shared with the ai_requests.estimated_cost_usd
// column, so they must stay byte-identical to the Rails RATES table.

type modelRates struct {
	input  float64
	output float64
}

var rates = map[string]modelRates{
	"claude-sonnet-4-6": {input: 0.000003, output: 0.000015},
	"claude-haiku-4-5":  {input: 0.0000008, output: 0.000004},
}

// CostFor returns the estimated USD cost of a call, or nil for an unknown model
// (mirrors Pricing.cost_for returning nil when RATES[model] is missing). The
// caller passes nil tokens through as a nil cost (parity with the Ruby guard
// that only computes cost when both token counts are present).
func CostFor(model string, inputTokens, outputTokens *int) *float64 {
	if inputTokens == nil || outputTokens == nil {
		return nil
	}
	r, ok := rates[model]
	if !ok {
		return nil
	}
	cost := float64(*inputTokens)*r.input + float64(*outputTokens)*r.output
	return &cost
}
