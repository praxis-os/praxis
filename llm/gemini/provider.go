// SPDX-License-Identifier: Apache-2.0

// Package gemini provides an [llm.Provider] for the Google Gemini API.
//
// The Gemini API uses a distinct request/response format from OpenAI. This
// provider handles the full mapping between praxis's provider-agnostic types
// and Gemini's generateContent endpoint.
//
// Usage:
//
//	p := gemini.New("AIza...",
//	    gemini.WithDefaultModel("gemini-2.0-flash"),
//	)
//	orch, _ := orchestrator.New(p)
package gemini

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	praxiserrors "github.com/praxis-os/praxis/errors"
	"github.com/praxis-os/praxis/llm"
)

const (
	// defaultModel is the Gemini model used when LLMRequest.Model is empty.
	defaultModel = "gemini-2.0-flash"

	// defaultBaseURL is the canonical Gemini API base URL.
	defaultBaseURL = "https://generativelanguage.googleapis.com"

	// providerName is the canonical name returned by Name().
	providerName = "gemini"
)

// Provider calls the Google Gemini generateContent API and implements
// [llm.Provider].
//
// Construct via [New]. Provider is safe for concurrent use.
type Provider struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	defaultModel string
}

// defaultHTTPClient returns an HTTP client with a tuned transport for
// concurrent workloads.
func defaultHTTPClient() *http.Client {
	t, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		t = &http.Transport{}
	}
	tc := t.Clone()
	tc.MaxIdleConns = 100
	tc.MaxIdleConnsPerHost = 10
	return &http.Client{
		Timeout:   120 * time.Second,
		Transport: tc,
	}
}

// New constructs a Provider with the given API key and optional configuration.
//
// The API key is sent as a query parameter (?key=...). Use [WithBaseURL] to
// override the target endpoint for testing.
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:       apiKey,
		baseURL:      defaultBaseURL,
		httpClient:   defaultHTTPClient(),
		defaultModel: defaultModel,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns "gemini", the canonical provider name used by
// budget.PriceProvider lookups.
func (p *Provider) Name() string { return providerName }

// SupportsParallelToolCalls reports whether the provider can process multiple
// tool calls returned in a single response concurrently.
// Gemini can return multiple function calls in one response.
func (p *Provider) SupportsParallelToolCalls() bool { return true }

// Capabilities returns a snapshot of supported features for the Gemini
// provider. The snapshot is immutable for the provider's lifetime.
func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		SupportsStreaming:         false, // streaming not yet implemented
		SupportsParallelToolCalls: true,
		SupportsSystemPrompt:      true,
		SupportedStopReasons: []llm.StopReason{
			llm.StopReasonEndTurn,
			llm.StopReasonToolUse,
			llm.StopReasonMaxTokens,
		},
		MaxContextTokens: 1_048_576,
	}
}

// Complete sends req to the Gemini generateContent API and returns the full
// response. It respects context cancellation and deadlines.
//
// Errors are returned as typed [praxiserrors.TypedError] values:
//   - HTTP 400 → [praxiserrors.PermanentLLMError]
//   - HTTP 401/403 → [praxiserrors.PermanentLLMError]
//   - HTTP 429 → [praxiserrors.TransientLLMError]
//   - HTTP 500/503 → [praxiserrors.TransientLLMError]
//   - Other HTTP errors → [praxiserrors.PermanentLLMError]
func (p *Provider) Complete(ctx context.Context, req llm.LLMRequest) (llm.LLMResponse, error) {
	apiReq, model := toAPIRequest(req, p.defaultModel)

	body, err := json.Marshal(apiReq)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewPermanentLLMError(
			providerName, 0, fmt.Errorf("marshalling request: %w", err),
		)
	}

	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s",
		p.baseURL, model, p.apiKey)

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewPermanentLLMError(
			providerName, 0, fmt.Errorf("creating HTTP request: %w", err),
		)
	}
	httpReq.Header.Set("content-type", "application/json")

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return llm.LLMResponse{}, praxiserrors.NewCancellationError(
				praxiserrors.CancellationKindHard,
				fmt.Errorf("gemini request cancelled: %w", ctx.Err()),
			)
		}
		return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
			providerName, 0, fmt.Errorf("executing HTTP request: %w", err),
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		errBody, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if err != nil {
			return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
				providerName, resp.StatusCode, fmt.Errorf("reading error body: %w", err),
			)
		}
		return llm.LLMResponse{}, p.mapHTTPError(resp, errBody)
	}

	var apiResp geminiResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
			providerName, resp.StatusCode, fmt.Errorf("decoding response: %w", err),
		)
	}

	result := fromAPIResponse(apiResp)

	// Detect tool use from response parts — Gemini uses finishReason "STOP"
	// even when returning function calls.
	for _, part := range result.Message.Parts {
		if part.Type == llm.PartTypeToolCall {
			result.StopReason = llm.StopReasonToolUse
			break
		}
	}

	return result, nil
}

// Stream sends req to the Gemini API. Because streaming is not yet
// implemented in this adapter, it delegates to [Provider.Complete] and
// delivers the result as a single final chunk.
//
// The returned channel is always closed before Stream returns an error.
func (p *Provider) Stream(ctx context.Context, req llm.LLMRequest) (<-chan llm.LLMStreamChunk, error) {
	ch := make(chan llm.LLMStreamChunk, 1)

	go func() {
		defer close(ch)

		resp, err := p.Complete(ctx, req)
		if err != nil {
			ch <- llm.LLMStreamChunk{Err: err}
			return
		}
		ch <- llm.LLMStreamChunk{
			Final:    true,
			Response: &resp,
		}
	}()

	return ch, nil
}

// mapHTTPError converts a non-200 HTTP response into a typed praxis error.
func (p *Provider) mapHTTPError(resp *http.Response, body []byte) error {
	var apiErr geminiErrorEnvelope
	msg := string(body)
	if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
		msg = apiErr.Error.Message
	}

	cause := fmt.Errorf("gemini API error: %s", msg)
	sc := resp.StatusCode

	switch {
	case sc == http.StatusBadRequest: // 400
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusUnauthorized: // 401
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusForbidden: // 403
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusNotFound: // 404 — model not found
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusTooManyRequests: // 429
		return praxiserrors.NewTransientLLMError(providerName, sc,
			withRetryAfter(resp, cause),
		)

	case sc >= 500:
		return praxiserrors.NewTransientLLMError(providerName, sc, cause)

	default:
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)
	}
}

// withRetryAfter wraps cause with the Retry-After header value when present.
func withRetryAfter(resp *http.Response, cause error) error {
	ra := resp.Header.Get("retry-after")
	if ra == "" {
		return cause
	}
	secs, err := strconv.ParseFloat(ra, 64)
	if err != nil {
		return fmt.Errorf("%w (retry-after: %s)", cause, ra)
	}
	return fmt.Errorf("%w (retry-after: %.0fs)", cause, secs)
}
