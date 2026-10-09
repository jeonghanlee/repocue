package promptcache

import (
	"math/big"
	"strings"
	"time"
)

// CostUSD is independent of Codex subset accounting and credit estimates.
func CostUSD(usage Usage, rates *Rates, provider Provider, responseModel string, at time.Time) *string {
	if rates == nil || responseModel != provider.Model || ValidateRates(*rates, provider, at) != nil {
		return nil
	}
	counts := []*int64{usage.OrdinaryInput, usage.CacheWrite5m, usage.CacheWrite1h, usage.CacheRead, usage.Output}
	prices := []string{rates.Input, rates.Write5m, rates.Write1h, rates.Read, rates.Output}
	total := new(big.Rat)
	for i, count := range counts {
		if count == nil || *count < 0 || *count > MaxCounter {
			return nil
		}
		rate, ok := new(big.Rat).SetString(prices[i])
		if !ok || rate.Sign() < 0 {
			return nil
		}
		total.Add(total, new(big.Rat).Mul(rate, new(big.Rat).SetInt64(*count)))
	}
	total.Quo(total, new(big.Rat).SetInt64(1000000))
	value := Decimal(total.FloatString(18))
	return &value
}
func AddUSD(values ...*string) *string {
	total := new(big.Rat)
	for _, value := range values {
		if value == nil {
			return nil
		}
		part, ok := new(big.Rat).SetString(*value)
		if !ok {
			return nil
		}
		total.Add(total, part)
	}
	value := Decimal(total.FloatString(18))
	return &value
}
func DifferenceUSD(left, right *string) *string {
	if left == nil || right == nil {
		return nil
	}
	a, okA := new(big.Rat).SetString(*left)
	b, okB := new(big.Rat).SetString(*right)
	if !okA || !okB {
		return nil
	}
	value := Decimal(new(big.Rat).Sub(a, b).FloatString(18))
	return &value
}
func PositiveUSD(value *string) bool {
	if value == nil {
		return false
	}
	amount, ok := new(big.Rat).SetString(*value)
	return ok && amount.Sign() > 0
}
func Decimal(value string) string {
	if strings.Contains(value, ".") {
		value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	}
	if value == "" || value == "-0" {
		return "0"
	}
	return value
}
