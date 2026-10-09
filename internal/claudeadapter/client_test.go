package claudeadapter

import "testing"

func TestCacheEndpointBoundary(t *testing.T) {
	for _, value := range []string{DefaultEndpoint, "https://127.0.0.1:8443/v1/messages", "https://[::1]:8443/v1/messages", "https://localhost:8443/v1/messages"} {
		if err := ValidateEndpoint(value); err != nil {
			t.Errorf("valid endpoint %s: %v", value, err)
		}
	}
	for _, value := range []string{"http://127.0.0.1/v1/messages", "https://example.invalid/v1/messages", "https://api.anthropic.com:443/v1/messages", "https://key@localhost/v1/messages", "https://localhost/v1/messages?key=value", "https://localhost/v1/messages#fragment", "https://localhost/elsewhere", "://invalid"} {
		if err := ValidateEndpoint(value); err == nil {
			t.Errorf("invalid endpoint accepted: %s", value)
		}
	}
}
