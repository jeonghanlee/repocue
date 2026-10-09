package evaluation

import (
	"math"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/taskbenchmark"
)

func TestTaskCreditCompleteCountersAndBounds(t *testing.T) {
	at := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	settings := taskbenchmark.Settings{Model: "fixture", SpeedTier: "standard"}
	rates := taskbenchmark.Rates{ID: "fixture", AsOf: "2026-10-07", Unit: "credits_per_million_tokens",
		Model: "fixture", SpeedTier: "standard", Input: 2, CachedInput: 0.2, Output: 4}
	pointer := func(value int64) *int64 { return &value }
	complete := func() taskbenchmark.Transport {
		return taskbenchmark.Transport{Status: "completed", UsageComplete: true, UsageEvents: []taskbenchmark.Usage{
			{Turn: 1, InputTokens: pointer(100), CachedInputTokens: pointer(30), OutputTokens: pointer(10)}}}
	}
	good := TaskCredit(complete(), &rates, settings, at)
	if good.Value == nil || math.Abs(*good.Value-0.000186) > 1e-12 {
		t.Fatalf("subset accounting: %+v", good)
	}
	for _, mode := range []string{"partial", "cache-missing", "cache-bounds", "reasoning-bounds", "negative", "overflow", "failed", "rate-missing", "tier", "future"} {
		t.Run(mode, func(t *testing.T) {
			transport, rate := complete(), rates
			ratePointer := &rate
			switch mode {
			case "partial":
				transport.UsageComplete = false
			case "cache-missing":
				transport.UsageEvents[0].CachedInputTokens = nil
			case "cache-bounds":
				transport.UsageEvents[0].CachedInputTokens = pointer(101)
			case "reasoning-bounds":
				transport.UsageEvents[0].ReasoningOutputTokens = pointer(11)
				transport.UsageEvents = append(transport.UsageEvents, taskbenchmark.Usage{Turn: 2, InputTokens: pointer(1), CachedInputTokens: pointer(0), OutputTokens: pointer(100), ReasoningOutputTokens: pointer(0)})
			case "negative":
				transport.UsageEvents[0].InputTokens = pointer(-1)
			case "overflow":
				transport.UsageEvents[0].InputTokens = pointer(math.MaxInt64)
			case "failed":
				transport.Status = "failed"
			case "rate-missing":
				ratePointer = nil
			case "tier":
				rate.SpeedTier = "fast"
			case "future":
				rate.AsOf = "2026-10-09"
			}
			cost := TaskCredit(transport, ratePointer, settings, at)
			if cost.Value != nil || cost.Status != "not_observed" {
				t.Fatalf("incomplete or invalid cost awarded: %+v", cost)
			}
		})
	}
}
