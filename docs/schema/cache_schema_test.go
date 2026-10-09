package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCacheEmbeddedContractsAndPublicExamples(t *testing.T) {
	for _, name := range []string{"benchmark", "request", "observation", "answer", "evaluation"} {
		contract := "cache-" + name + "-v1.schema.json"
		if _, err := Contract(contract); err != nil {
			t.Fatal(err)
		}
		if err := Validate(contract, []byte(`{}`)); err == nil {
			t.Fatalf("empty %s contract accepted", name)
		}
	}
	for _, example := range []struct{ file, contract string }{
		{"prompt-cache-v1.json", "cache-benchmark-v1.schema.json"},
		{"prompt-cache-answer-v1.json", "cache-answer-v1.schema.json"},
	} {
		raw, err := os.ReadFile(filepath.Join("..", "benchmarks", example.file))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(example.contract, raw); err != nil {
			t.Fatal(err)
		}
		for _, mutation := range []string{"version", "unknown"} {
			var value map[string]any
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			if mutation == "version" {
				value["schema_version"] = "repocue/cache-unknown-999"
			} else {
				value["unknown"] = true
			}
			bad, _ := json.Marshal(value)
			if err := Validate(example.contract, bad); err == nil {
				t.Fatalf("%s %s accepted", example.file, mutation)
			}
		}
	}
}
