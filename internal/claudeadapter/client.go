// Package claudeadapter sends one explicit bounded non-streaming Messages request.
package claudeadapter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jeonghanlee/repocue/internal/promptcache"
)

const DefaultEndpoint = "https://api.anthropic.com/v1/messages"

type Result struct {
	Attempted      bool
	RequestWritten bool
	StartedAt      time.Time
	FinishedAt     time.Time
	DurationMS     float64
	HTTPStatus     *int
	RequestID      *string
	Body           []byte
	BodyComplete   bool
	Error          error
}

func ValidateEndpoint(endpoint string) error {
	value, err := url.Parse(endpoint)
	if err != nil || value.Scheme != "https" || value.User != nil || value.RawQuery != "" || value.Fragment != "" ||
		value.Path != "/v1/messages" {
		return errors.New("endpoint must be HTTPS /v1/messages without credentials or query")
	}
	host := value.Hostname()
	ip := net.ParseIP(host)
	if endpoint != DefaultEndpoint && host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("endpoint must be the official API or an explicit HTTPS loopback fixture")
	}
	return nil
}

// Send never retries or follows redirects. The credential exists only in the
// outgoing authentication header. Raw bytes containing it are withheld.
func Send(ctx context.Context, endpoint, key, apiVersion string, limits promptcache.Limits, body []byte) (result Result) {
	result.StartedAt = time.Now().UTC()
	defer func() {
		result.FinishedAt = time.Now().UTC()
		result.DurationMS = float64(result.FinishedAt.Sub(result.StartedAt)) / float64(time.Millisecond)
	}()
	if err := ValidateEndpoint(endpoint); err != nil {
		result.Error = err
		return
	}
	if key == "" || strings.ContainsAny(key, "\r\n") {
		result.Error = errors.New("ANTHROPIC_API_KEY is missing or invalid")
		return
	}
	if limits.TimeoutMS < 1 || limits.TimeoutMS > 300000 || limits.MaxResponseBytes < 64 || limits.MaxResponseBytes > 8388608 {
		result.Error = errors.New("invalid frozen HTTP limits")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Duration(limits.TimeoutMS)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		result.Error = errors.New("cannot construct HTTP request")
		return
	}
	request.Header.Set("content-type", "application/json")
	request.Header.Set("anthropic-version", apiVersion)
	request.Header.Set("x-api-key", key)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	var written atomic.Bool
	trace := &httptrace.ClientTrace{WroteRequest: func(info httptrace.WroteRequestInfo) { written.Store(info.Err == nil) }}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	result.StartedAt = time.Now().UTC()
	result.Attempted = true
	response, err := client.Do(request)
	result.RequestWritten = written.Load()
	if err != nil {
		result.Error = errors.New("HTTP request failed or was cancelled")
		return
	}
	defer response.Body.Close()
	status := response.StatusCode
	result.HTTPStatus = &status
	requestID := response.Header.Get("request-id")
	if requestID != "" && len(requestID) <= 256 && !strings.Contains(requestID, key) {
		result.RequestID = &requestID
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, int64(limits.MaxResponseBytes)+1))
	result.BodyComplete = err == nil && len(raw) <= limits.MaxResponseBytes
	if len(raw) > limits.MaxResponseBytes {
		raw = raw[:limits.MaxResponseBytes]
		result.Error = errors.New("response exceeds frozen byte limit")
	}
	if err != nil {
		result.Error = errors.New("response body incomplete or cancelled")
	}
	if bytes.Contains(raw, []byte(key)) {
		result.Error = errors.New("response contains API credential; raw capture withheld")
		result.BodyComplete = false
		return
	}
	result.Body = raw
	if status < 200 || status >= 300 {
		result.Error = errors.New("HTTP response was not successful")
	}
	return
}
