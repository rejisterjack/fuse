package cost

// EstimateUSD returns an approximate USD cost from token counts.
// Pricing is illustrative OpenAI-class rates and will diverge from real invoices.
func EstimateUSD(model string, promptTokens, completionTokens int) float64 {
	inPerM, outPerM := rates(model)
	return (float64(promptTokens)/1_000_000)*inPerM + (float64(completionTokens)/1_000_000)*outPerM
}

func rates(model string) (inputPerMillion, outputPerMillion float64) {
	switch model {
	case "gpt-4o", "claude-3-5-sonnet", "claude-sonnet-4":
		return 3.0, 15.0
	case "gpt-4o-mini", "claude-3-5-haiku":
		return 0.15, 0.60
	default:
		return 1.0, 3.0
	}
}
