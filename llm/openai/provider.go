// SPDX-License-Identifier: Apache-2.0

package openai

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
	// defaultModel is the OpenAI model used when LLMRequest.Model is empty.
	defaultModel = "gpt-4o"

	// defaultBaseURL is the canonical OpenAI API base URL.
	defaultBaseURL = "https://api.openai.com"

	// providerName is the canonical name returned by Name().
	providerName = "openai"
)

// Provider calls the OpenAI Chat Completions API and implements [llm.Provider].
//
// Construct via [New]. Provider is safe for concurrent use.
type Provider struct {
	apiKey       string
	baseURL      string
	httpClient   *http.Client
	defaultModel string
}

// New constructs a Provider with the given API key and optional configuration.
//
// The API key is sent via the Authorization: Bearer request header. Use
// [WithBaseURL] to override the target endpoint for Azure OpenAI deployments or
// testing.
func New(apiKey string, opts ...Option) *Provider {
	p := &Provider{
		apiKey:       apiKey,
		baseURL:      defaultBaseURL,
		httpClient:   &http.Client{Timeout: 120 * time.Second},
		defaultModel: defaultModel,
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns "openai", the canonical provider name used by
// budget.PriceProvider lookups.
func (p *Provider) Name() string { return providerName }

// SupportsParallelToolCalls reports whether the provider can process multiple
// tool calls returned in a single response concurrently.
// OpenAI returns multiple tool_calls in one response, so this is true.
func (p *Provider) SupportsParallelToolCalls() bool { return true }

// Capabilities returns a snapshot of supported features for the OpenAI
// provider. The snapshot is immutable for the provider's lifetime.
func (p *Provider) Capabilities() llm.Capabilities {
	return llm.Capabilities{
		SupportsStreaming:          false, // streaming not yet implemented
		SupportsParallelToolCalls: true,
		SupportsSystemPrompt:      true,
		SupportedStopReasons: []llm.StopReason{
			llm.StopReasonEndTurn,
			llm.StopReasonToolUse,
			llm.StopReasonMaxTokens,
		},
		MaxContextTokens: 128_000,
	}
}

// Complete sends req to the OpenAI Chat Completions API and returns the full
// response. It respects context cancellation and deadlines.
//
// Errors are returned as typed [praxiserrors.TypedError] values:
//   - HTTP 401/400/403 → [praxiserrors.PermanentLLMError]
//   - HTTP 429 → [praxiserrors.TransientLLMError]
//   - HTTP 500/502/503 → [praxiserrors.TransientLLMError]
//   - Other HTTP errors → [praxiserrors.PermanentLLMError]
func (p *Provider) Complete(ctx context.Context, req llm.LLMRequest) (llm.LLMResponse, error) {
	apiReq, err := toAPIRequest(req, p.defaultModel)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewPermanentLLMError(
			providerName, 0, fmt.Errorf("building request: %w", err),
		)
	}

	body, err := json.Marshal(apiReq)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewPermanentLLMError(
			providerName, 0, fmt.Errorf("marshalling request: %w", err),
		)
	}

	httpReq, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewPermanentLLMError(
			providerName, 0, fmt.Errorf("creating HTTP request: %w", err),
		)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		// Network-level or context cancellation error.
		if ctx.Err() != nil {
			return llm.LLMResponse{}, praxiserrors.NewCancellationError(
				praxiserrors.CancellationKindHard,
				fmt.Errorf("openai request cancelled: %w", ctx.Err()),
			)
		}
		return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
			providerName, 0, fmt.Errorf("executing HTTP request: %w", err),
		)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
			providerName, resp.StatusCode, fmt.Errorf("reading response body: %w", err),
		)
	}

	if resp.StatusCode != http.StatusOK {
		return llm.LLMResponse{}, p.mapHTTPError(resp, respBody)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return llm.LLMResponse{}, praxiserrors.NewTransientLLMError(
			providerName, resp.StatusCode, fmt.Errorf("decoding response: %w", err),
		)
	}

	return fromAPIResponse(apiResp), nil
}

// Stream sends req to the OpenAI API. Because streaming is not yet implemented
// in this adapter, it delegates to [Provider.Complete] and delivers the result
// as a single final chunk.
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
	// Attempt to decode the OpenAI error envelope for a richer message.
	var apiErr apiError
	msg := string(body)
	if jsonErr := json.Unmarshal(body, &apiErr); jsonErr == nil && apiErr.Error.Message != "" {
		msg = apiErr.Error.Message
	}

	cause := fmt.Errorf("openai API error: %s", msg)
	sc := resp.StatusCode

	switch {
	case sc == http.StatusUnauthorized: // 401
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusBadRequest: // 400
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusForbidden: // 403
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusNotFound: // 404 — model not found or bad path
		return praxiserrors.NewPermanentLLMError(providerName, sc, cause)

	case sc == http.StatusUnprocessableEntity: // 422
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

// withRetryAfter wraps cause with the Retry-After header value when present,
// returning a new error that includes the retry delay hint.
func withRetryAfter(resp *http.Response, cause error) error {
	ra := resp.Header.Get("retry-after")
	if ra == "" {
		return cause
	}
	secs, err := strconv.ParseFloat(ra, 64)
	if err != nil {
		// Non-numeric Retry-After (e.g., HTTP-date) — include raw value.
		return fmt.Errorf("%w (retry-after: %s)", cause, ra)
	}
	return fmt.Errorf("%w (retry-after: %.0fs)", cause, secs)
}
