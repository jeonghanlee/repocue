package promptcache

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jeonghanlee/repocue/internal/taskcontext"
)

func TestCacheUsageDisjointAndNullable(t *testing.T) {
	for _, test := range []struct {
		name, usage, ttl, total, cost, activity string
		invalid                                 bool
	}{
		{"mixed", `{"input_tokens":10,"cache_creation_input_tokens":30,"cache_read_input_tokens":40,"output_tokens":5,"cache_creation":{"ephemeral_5m_input_tokens":20,"ephemeral_1h_input_tokens":10}}`, "5m", "80", "0.000069", "creation-and-reuse", false},
		{"single TTL", `{"input_tokens":10,"cache_creation_input_tokens":30,"cache_read_input_tokens":0,"output_tokens":5}`, "1h", "40", "0.00008", "creation", false},
		{"unallocated", `{"input_tokens":10,"cache_creation_input_tokens":30,"cache_read_input_tokens":0,"output_tokens":5}`, "", "40", "", "creation", false},
		{"missing read", `{"input_tokens":10,"cache_creation_input_tokens":0,"output_tokens":5}`, "5m", "", "", "not_observed", false},
		{"zero", `{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0}`, "5m", "0", "0", "none-observed", false},
		{"lifetime mismatch", `{"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":20,"ephemeral_1h_input_tokens":11}}`, "5m", "", "", "", true},
		{"partial overflow", `{"cache_creation_input_tokens":30,"cache_creation":{"ephemeral_5m_input_tokens":31}}`, "5m", "", "", "", true},
		{"negative", `{"input_tokens":-1}`, "5m", "", "", "", true},
		{"fractional", `{"output_tokens":1.1}`, "5m", "", "", "", true},
		{"unsafe integer", `{"input_tokens":9007199254740992}`, "5m", "", "", "", true},
		{"sum overflow", `{"input_tokens":9007199254740991,"cache_creation_input_tokens":1,"cache_read_input_tokens":0}`, "5m", "", "", "", true},
		{"duplicate", `{"input_tokens":0,"input_tokens":1}`, "5m", "", "", "", true},
		{"thinking subset", `{"output_tokens":1,"output_tokens_details":{"thinking_tokens":2}}`, "5m", "", "", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte(`{"model":"claude-sonnet-4-6","usage":` + test.usage + `}`)
			response, err := DecodeResponse(raw, test.ttl)
			if (err != nil) != test.invalid {
				t.Fatalf("validity: %v", err)
			}
			if test.invalid {
				return
			}
			total := ""
			if response.Usage.TotalInput != nil {
				value, _ := json.Marshal(*response.Usage.TotalInput)
				total = string(value)
			}
			if total != test.total || response.Usage.Activity() != test.activity {
				t.Fatalf("total/activity: %s %s", total, response.Usage.Activity())
			}
			rates := Rates{ID: "synthetic", AsOf: "2026-10-08", Unit: "usd_per_million_tokens", Model: "claude-sonnet-4-6", APIVersion: "2023-06-01", Input: "1", Write5m: "1.25", Write1h: "2", Read: "0.1", Output: "2"}
			got := CostUSD(response.Usage, &rates, Provider{Model: rates.Model, APIVersion: rates.APIVersion, TTL: test.ttl}, rates.Model, time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC))
			cost := ""
			if got != nil {
				cost = *got
			}
			if cost != test.cost {
				t.Fatalf("cost: %s want %s", cost, test.cost)
			}
			if CostUSD(response.Usage, &rates, Provider{Model: rates.Model, APIVersion: rates.APIVersion}, "different-model", time.Now()) != nil {
				t.Fatal("response-model mismatch cost accepted")
			}
		})
	}
}

func TestCacheDecimalPrecisionAndCompleteTotals(t *testing.T) {
	a, b := "0.000000000000000001", "1.000000000000000001"
	if got := AddUSD(&a, &b); got == nil || *got != "1.000000000000000002" {
		t.Fatalf("decimal precision: %v", got)
	}
	if AddUSD(&a, nil) != nil {
		t.Fatal("partial favorable subtotal accepted")
	}
	if Decimal("10") != "10" || Decimal("10.000") != "10" {
		t.Fatal("integer trailing zero lost")
	}
	if got := DifferenceUSD(&a, &b); got == nil || *got != "-1" {
		t.Fatalf("negative difference: %v", got)
	}
}

func TestCacheContractStrictDecode(t *testing.T) {
	valid := `{"schema_version":"repocue/cache-answer-1","task_id":"sample","required_ids":[],"source_ids":[],"claims":[],"uncertainties":[]}`
	for _, raw := range []string{valid + ` {}`, strings.Replace(valid, `"task_id":"sample"`, `"task_id":"sample","task_id":"other"`, 1), strings.Replace(valid, `"task_id":"sample"`, `"task_id":"sample","extra":true`, 1), strings.Replace(valid, "cache-answer-1", "cache-answer-2", 1)} {
		var answer Answer
		if err := taskcontext.DecodeStrict([]byte(raw), "cache-answer-v1.schema.json", &answer); err == nil {
			t.Fatalf("invalid answer accepted: %s", raw)
		}
	}
	var answer Answer
	if err := taskcontext.DecodeStrict([]byte(valid), "cache-answer-v1.schema.json", &answer); err != nil {
		t.Fatal(err)
	}
}
