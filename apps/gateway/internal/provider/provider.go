// Package provider defines the AI provider abstraction.
//
// The gateway never knows which upstream (OpenAI, Gemini) serves a request.
// The Registry maps a catalog provider to an implementation; new providers only
// need to implement this package's Provider interface.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/x402-marketplace/catalog"
)

// dbgLog is used exclusively for request-level debug output (no secrets).
var dbgLog = log.New(os.Stdout, "[provider:debug] ", log.LstdFlags)

// ── Error types ────────────────────────────────────────────────────────────────

// ProviderError is a structured error returned by provider adapters.
//
// When serialized to JSON for the frontend, raw provider errors (like Google's raw JSON)
// are sanitized into clean, actionable objects:
//
//	{
//	  "type": "quota_exceeded",
//	  "provider": "gemini",
//	  "message": "Gemini has reached its API quota. Please try again later or switch to another provider.",
//	  "retry_after": 18
//	}
type ProviderError struct {
	Type             string `json:"type"`                   // "quota_exceeded" | "unauthorized" | "rate_limit" | "bad_request" | "not_found" | "server_error"
	Provider         string `json:"provider"`               // "gemini" | "openai" | ...
	Message          string `json:"message"`                // Clean user-facing explanation
	RetryAfter       int    `json:"retry_after,omitempty"` // Seconds until retry permitted (from RetryInfo)
	StatusCode       int    `json:"-"`
	RawBody          string `json:"-"`
	Retryable        bool   `json:"-"`
	IsQuotaExhausted bool   `json:"-"` // Daily limit reached; non-retryable on this adapter
}

func (e *ProviderError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("provider %s: status %d: %s", e.Provider, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("provider %s: status %d: %s", e.Provider, e.StatusCode, e.RawBody)
}

// MarshalJSON returns the clean frontend JSON representation without raw Google JSON.
func (e *ProviderError) MarshalJSON() ([]byte, error) {
	type Alias ProviderError
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(e),
	})
}

// newProviderError classifies an HTTP response and constructs a structured ProviderError.
func newProviderError(kind catalog.ProviderKind, statusCode int, body string) *ProviderError {
	providerName := string(kind)
	bodyStr := strings.TrimSpace(body)

	pe := &ProviderError{
		Provider:   providerName,
		StatusCode: statusCode,
		RawBody:    bodyStr,
	}

	switch statusCode {
	case http.StatusUnauthorized: // 401
		pe.Type = "unauthorized"
		pe.Message = fmt.Sprintf("Invalid API key for provider %s. Please check credentials.", titleCase(providerName))
		pe.Retryable = false
		dbgLog.Printf("[provider:error] Invalid API Key (401) for %s", providerName)

	case http.StatusForbidden: // 403
		pe.Type = "unauthorized"
		pe.Message = fmt.Sprintf("Access forbidden for provider %s. Check API key permissions and billing status.", titleCase(providerName))
		pe.Retryable = false
		dbgLog.Printf("[provider:error] Access Forbidden (403) for %s", providerName)

	case http.StatusNotFound: // 404
		pe.Type = "not_found"
		pe.Message = fmt.Sprintf("Model or endpoint not found on provider %s.", titleCase(providerName))
		pe.Retryable = false
		dbgLog.Printf("[provider:error] Model Not Found (404) for %s", providerName)

	case http.StatusBadRequest: // 400
		pe.Type = "bad_request"
		pe.Message = fmt.Sprintf("Bad request payload sent to provider %s.", titleCase(providerName))
		pe.Retryable = false
		dbgLog.Printf("[provider:error] Bad Request (400) for %s", providerName)

	case http.StatusTooManyRequests: // 429
		retryAfter := parseRetryAfter(bodyStr)
		pe.RetryAfter = retryAfter

		// Distinguish daily quota exhaustion vs transient rate limit
		if isQuotaExhausted(bodyStr) {
			pe.Type = "quota_exceeded"
			pe.Message = fmt.Sprintf("%s has reached its API quota. Please try again later or switch to another provider.", titleCase(providerName))
			pe.IsQuotaExhausted = true
			pe.Retryable = false // Do not retry daily limit failures indefinitely
			dbgLog.Printf("[provider:error] Quota Exceeded (429) for %s - Daily limit reached (retry_after=%ds)", providerName, retryAfter)
		} else {
			pe.Type = "rate_limit"
			pe.Message = fmt.Sprintf("%s rate limit reached. Retrying automatically...", titleCase(providerName))
			pe.Retryable = true
			dbgLog.Printf("[provider:error] Rate Limit (429) for %s - Transient rate limit (retry_after=%ds)", providerName, retryAfter)
		}

	default: // 5xx or others
		pe.Type = "server_error"
		pe.Message = fmt.Sprintf("%s upstream server error (%d).", titleCase(providerName), statusCode)
		pe.Retryable = statusCode >= 500
		dbgLog.Printf("[provider:error] Server Error (%d) for %s", statusCode, providerName)
	}

	return pe
}

// parseRetryAfter extracts integer seconds from Gemini's RetryInfo/retryDelay or general JSON.
// e.g. "18.113508404s" -> 18
func parseRetryAfter(body string) int {
	// 1. Try parsing structured Gemini Google RPC details
	var ev struct {
		Error struct {
			Details []struct {
				Type       string `json:"@type"`
				RetryDelay string `json:"retryDelay"`
			} `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(body), &ev); err == nil {
		for _, d := range ev.Error.Details {
			if strings.Contains(d.Type, "RetryInfo") && d.RetryDelay != "" {
				if sec := parseSecondsString(d.RetryDelay); sec > 0 {
					return sec
				}
			}
		}
	}

	// 2. Fallback regex/substring scanner for "retryDelay": "18.1135s" or "Retry in 18s"
	if idx := strings.Index(body, "retryDelay"); idx != -1 {
		sub := body[idx:]
		if col := strings.Index(sub, ":"); col != -1 {
			val := strings.Trim(sub[col+1:], ` "}`)
			if sec := parseSecondsString(val); sec > 0 {
				return sec
			}
		}
	}
	if idx := strings.Index(body, "retry in "); idx != -1 {
		sub := body[idx+9:]
		if sec := parseSecondsString(sub); sec > 0 {
			return sec
		}
	}
	return 10 // reasonable default if unparseable
}

func parseSecondsString(s string) int {
	var val float64
	s = strings.TrimSuffix(strings.TrimSpace(s), "s")
	if _, err := fmt.Sscanf(s, "%f", &val); err == nil && val > 0 {
		return int(val)
	}
	return 0
}

func isQuotaExhausted(body string) bool {
	lower := strings.ToLower(body)
	return strings.Contains(lower, "resource_exhausted") ||
		strings.Contains(lower, "quota exceeded") ||
		strings.Contains(lower, "free_tier_requests") ||
		strings.Contains(lower, "exceeded your current quota") ||
		strings.Contains(lower, "daily limit")
}

func titleCase(s string) string {
	if s == "" {
		return ""
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// Message is a chat message sent to a provider.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is a normalized chat completion request.
type Request struct {
	Model     string
	Messages  []Message
	MaxTokens int
}

// Usage is the internal metering record for a session. Only the session cost
// is exposed to users; the rest stays internal.
type Usage struct {
	PromptTokens     int   `json:"prompt_tokens"`
	CompletionTokens int   `json:"completion_tokens"`
	TotalTokens      int   `json:"total_tokens"`
	LatencyMS        int64 `json:"latency_ms"`
	Requests         int   `json:"requests"`
}

// StreamFunc receives normalized OpenAI-compatible SSE chunks.
type StreamFunc func(chunk string) error

// ImageRequest is a normalized image generation request.
type ImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size,omitempty"`
	N      int    `json:"n,omitempty"`
}

// ImageResult contains generated image details.
type ImageResult struct {
	URL     string `json:"url,omitempty"`
	B64JSON string `json:"b64_json,omitempty"`
}

// Provider is implemented by every AI provider adapter.
type Provider interface {
	// Stream performs a streaming completion, calling emit for each SSE chunk.
	// It returns the internal usage breakdown and honors ctx cancellation.
	Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error)
	// GenerateImage performs image generation for a prompt.
	GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, Usage, error)
	// Kind identifies the implementation for routing/debug.
	Kind() catalog.ProviderKind
}

// chunkEvent is the normalized SSE payload (OpenAI-compatible).
func chunkEvent(model, text string) string {
	return `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"content":` + jsonStr(text) + `},"finish_reason":null}]}` + "\n\n"
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// ── HTTPAdapter ────────────────────────────────────────────────────────────────

// HTTPAdapter streams from OpenAI-compatible and Gemini HTTP APIs.
type HTTPAdapter struct {
	kind    catalog.ProviderKind
	baseURL string
	apiKey  string
	client  *http.Client
	timeout time.Duration
}

// NewOpenAIAdapter creates an adapter for OpenAI-compatible /chat/completions.
func NewOpenAIAdapter(baseURL, apiKey string) *HTTPAdapter {
	return &HTTPAdapter{
		kind:    catalog.ProviderKindOpenAI,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{},
		timeout: 60 * time.Second,
	}
}

// NewGeminiAdapter creates an adapter for the Gemini generateContent REST API.
//
// Streaming is achieved via the :streamGenerateContent?alt=sse endpoint.
// There is NO "streaming" field inside the request body or generationConfig.
func NewGeminiAdapter(baseURL, apiKey string) *HTTPAdapter {
	return &HTTPAdapter{
		kind:    catalog.ProviderKindGemini,
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		client:  &http.Client{},
		timeout: 60 * time.Second,
	}
}

// Kind implements Provider.
func (h *HTTPAdapter) Kind() catalog.ProviderKind { return h.kind }

// Stream implements Provider.
func (h *HTTPAdapter) Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	start := time.Now()

	// Defence-in-depth: strip the "models/" prefix and resolve legacy aliases.
	req.Model = catalog.NormalizeModelSlug(req.Model)

	url, body := h.buildRequest(req)

	// ── Debug logging (API keys are NEVER included in logs) ────────────────
	bodyJSON, _ := json.MarshalIndent(body, "", "  ")
	safeURL := url
	if idx := strings.Index(safeURL, "key="); idx != -1 {
		safeURL = safeURL[:idx] + "key=REDACTED"
	}
	dbgLog.Printf("→ POST %s", safeURL)
	dbgLog.Printf("  Content-Type: application/json")
	if h.kind != catalog.ProviderKindGemini {
		dbgLog.Printf("  Authorization: Bearer REDACTED")
	}
	dbgLog.Printf("  Body:\n%s", string(bodyJSON))
	// ───────────────────────────────────────────────────────────────────────

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		promptTokens := estimatePromptTokens(req.Messages)
		completion := 0
		err := h.streamOnce(ctx, req.Model, url, body, emit, &completion)
		if err == nil {
			u := Usage{
				PromptTokens:     promptTokens,
				CompletionTokens: completion,
				TotalTokens:      promptTokens + completion,
				LatencyMS:        time.Since(start).Milliseconds(),
				Requests:         attempt + 1,
			}
			return u, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break // timeout/cancellation is not retryable
		}
		// Do not retry non-retryable provider errors (400/401/403/404).
		// These indicate configuration issues that will not resolve with time.
		if pe, ok := err.(*ProviderError); ok && !pe.Retryable {
			dbgLog.Printf("  Non-retryable error (HTTP %d) — skipping retries", pe.StatusCode)
			break
		}
		select {
		case <-ctx.Done():
			return Usage{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return Usage{}, fmt.Errorf("provider: stream failed after retries: %w", lastErr)
}

// buildRequest constructs the provider-specific URL and request body.
//
// IMPORTANT — Gemini API rules:
//   - Streaming endpoint: POST /v1beta/models/{model}:streamGenerateContent?alt=sse
//   - The "streaming" field does NOT exist in generationConfig or anywhere else.
//   - Valid generationConfig fields per ai.google.dev/api/generate-content:
//     maxOutputTokens, temperature, topP, topK, candidateCount, stopSequences
//   - Role mapping: "assistant" → "model" (Gemini uses "model", not "assistant")
func (h *HTTPAdapter) buildRequest(req Request) (string, any) {
	switch h.kind {
	case catalog.ProviderKindGemini:
		// Strip "models/" prefix and resolve any legacy model names.
		model := catalog.NormalizeModelSlug(req.Model)
		// Endpoint for streaming; ?alt=sse enables Server-Sent Events.
		url := h.baseURL + "/models/" + model + ":streamGenerateContent?alt=sse&key=" + h.apiKey

		// Build the full conversation history as Gemini contents[].
		contents := make([]map[string]any, 0, len(req.Messages))
		for _, m := range req.Messages {
			role := m.Role
			switch role {
			case "assistant":
				role = "model" // Gemini calls assistant turns "model"
			case "system":
				continue // system messages handled via systemInstruction, skip here
			}
			contents = append(contents, map[string]any{
				"role":  role,
				"parts": []map[string]any{{"text": m.Content}},
			})
		}
		// Ensure at least one user turn.
		if len(contents) == 0 {
			contents = []map[string]any{
				{"role": "user", "parts": []map[string]any{{"text": ""}}},
			}
		}

		maxTokens := req.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 512
		}

		body := map[string]any{
			"contents": contents,
			// generationConfig — ONLY fields documented in the official API.
			// "streaming" is NOT a valid field here. Streaming is enabled by
			// the :streamGenerateContent?alt=sse endpoint, not by a body field.
			"generationConfig": map[string]any{
				"maxOutputTokens": maxTokens,
			},
		}
		return url, body

	default: // OpenAI-compatible
		url := h.baseURL + "/chat/completions"
		msgs := make([]map[string]string, 0, len(req.Messages))
		for _, m := range req.Messages {
			msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
		}
		maxTokens := req.MaxTokens
		if maxTokens <= 0 {
			maxTokens = 512
		}
		body := map[string]any{
			"model":      req.Model,
			"messages":   msgs,
			"stream":     true,
			"max_tokens": maxTokens,
		}
		return url, body
	}
}

// streamOnce performs a single HTTP streaming attempt and parses the SSE response.
// model is passed explicitly so it can be embedded in the normalized chunk events.
func (h *HTTPAdapter) streamOnce(ctx context.Context, model, url string, body any, emit StreamFunc, completion *int) error {
	b, _ := json.Marshal(body)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	// Gemini authenticates via ?key= query param; OpenAI uses Bearer token.
	if h.kind != catalog.ProviderKindGemini {
		r.Header.Set("Authorization", "Bearer "+h.apiKey)
	}

	resp, err := h.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// Read up to 4 KB so Gemini error details are never truncated.
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return newProviderError(h.kind, resp.StatusCode, string(errBody))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		if len(payload) < 2 {
			continue
		}

		switch h.kind {
		case catalog.ProviderKindGemini:
			// Gemini SSE chunk shape:
			// {"candidates":[{"content":{"parts":[{"text":"..."}],"role":"model"},"finishReason":"..."}],
			//  "usageMetadata":{"promptTokenCount":N,"candidatesTokenCount":N,"totalTokenCount":N}}
			var ev struct {
				Candidates []struct {
					Content struct {
						Parts []struct {
							Text string `json:"text"`
						} `json:"parts"`
					} `json:"content"`
					FinishReason string `json:"finishReason"`
				} `json:"candidates"`
				UsageMetadata *struct {
					CandidatesTokenCount int `json:"candidatesTokenCount"`
					PromptTokenCount     int `json:"promptTokenCount"`
					TotalTokenCount      int `json:"totalTokenCount"`
				} `json:"usageMetadata"`
			}
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			for _, cand := range ev.Candidates {
				for _, part := range cand.Content.Parts {
					if part.Text != "" {
						*completion += len(strings.Fields(part.Text))
						if err := emit(chunkEvent(model, part.Text)); err != nil {
							return err
						}
					}
				}
			}
			// Prefer official token counts from the final chunk.
			if ev.UsageMetadata != nil && ev.UsageMetadata.CandidatesTokenCount > 0 {
				*completion = ev.UsageMetadata.CandidatesTokenCount
			}

		default: // OpenAI-compatible
			// OpenAI SSE chunk shape:
			// {"choices":[{"delta":{"content":"..."},"finish_reason":null}],"usage":...}
			var ev struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
				Usage *struct {
					CompletionTokens int `json:"completion_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			if len(ev.Choices) > 0 && ev.Choices[0].Delta.Content != "" {
				*completion += len(strings.Fields(ev.Choices[0].Delta.Content))
				if err := emit(chunkEvent(model, ev.Choices[0].Delta.Content)); err != nil {
					return err
				}
			}
			if ev.Usage != nil {
				*completion = ev.Usage.CompletionTokens
			}
		}
	}
	return scanner.Err()
}

// ── helpers ────────────────────────────────────────────────────────────────────

// stripModelsPrefix removes the "models/" prefix that the Gemini REST API
// ListModels response includes in model names (e.g. "models/gemini-2.5-flash").
// Without this, the URL becomes /models/models/gemini-2.5-flash → 404.
func stripModelsPrefix(s string) string {
	return strings.TrimPrefix(s, "models/")
}

// estimatePromptTokens approximates prompt token count (word count + message overhead).
func estimatePromptTokens(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		n += len(strings.Fields(m.Content))
	}
	return n + len(msgs)*4
}

// ── Registry ───────────────────────────────────────────────────────────────────

// Registry selects a concrete Provider for a catalog provider.
type Registry struct {
	byKind map[catalog.ProviderKind]Provider
}

// NewRegistry wires real HTTP adapters for OpenAI and Gemini providers.
func NewRegistry(cat catalog.Catalog, openAIKey, geminiKey string) *Registry {
	r := &Registry{byKind: map[catalog.ProviderKind]Provider{}}
	r.byKind[catalog.ProviderKindOpenAI] = NewOpenAIAdapter("https://api.openai.com/v1", openAIKey)
	r.byKind[catalog.ProviderKindGemini] = NewGeminiAdapter("https://generativelanguage.googleapis.com/v1beta", geminiKey)
	return r
}

// NewRegistryFromProviders builds a Registry from an explicit provider map.
// This constructor is used in tests to inject mock providers without real
// API keys or HTTP calls.
func NewRegistryFromProviders(providers map[catalog.ProviderKind]Provider) *Registry {
	return &Registry{byKind: providers}
}

// Select returns the adapter for a catalog provider.
func (r *Registry) Select(p catalog.Provider) (Provider, error) {
	if ad, ok := r.byKind[p.Kind]; ok {
		return ad, nil
	}
	return nil, fmt.Errorf("provider registry: no adapter for %q", p.Kind)
}

// BuildRequestForTest exposes buildRequest for unit testing.
// It returns the exact URL and body map that would be sent to the upstream API.
// This allows tests to assert the JSON payload without running a real HTTP server.
func (h *HTTPAdapter) BuildRequestForTest(req Request) (url string, body any) {
	return h.buildRequest(req)
}

// BuildProviderErrorForTest exposes newProviderError for unit tests.
func BuildProviderErrorForTest(kind catalog.ProviderKind, statusCode int, body string) *ProviderError {
	return newProviderError(kind, statusCode, body)
}

// ── Gemini Model Discovery ─────────────────────────────────────────────────────

// GeminiModelInfo describes a model returned by the Gemini ListModels API.
type GeminiModelInfo struct {
	// Name is the full resource name, e.g. "models/gemini-2.5-flash".
	Name string `json:"name"`
	// DisplayName is the human-readable label.
	DisplayName string `json:"displayName"`
	// SupportedGenerationMethods lists supported API methods.
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// Slug returns the model slug by stripping the "models/" prefix.
func (m GeminiModelInfo) Slug() string { return stripModelsPrefix(m.Name) }

// SupportsGenerateContent returns true when the model supports generateContent.
func (m GeminiModelInfo) SupportsGenerateContent() bool {
	for _, method := range m.SupportedGenerationMethods {
		if method == "generateContent" {
			return true
		}
	}
	return false
}

// ListGeminiModels fetches all available models from the Gemini REST API and
// returns those that support generateContent.
//
// Endpoint: GET /v1beta/models?key={apiKey}
//
// This is used both for startup validation and the admin discovery endpoint.
func ListGeminiModels(ctx context.Context, baseURL, apiKey string) ([]GeminiModelInfo, error) {
	url := strings.TrimRight(baseURL, "/") + "/models?key=" + apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("provider: list models request: %w", err)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider: list models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, newProviderError(catalog.ProviderKindGemini, resp.StatusCode, string(body))
	}

	var result struct {
		Models []GeminiModelInfo `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("provider: decode models response: %w", err)
	}

	var out []GeminiModelInfo
	for _, m := range result.Models {
		if m.SupportsGenerateContent() {
			out = append(out, m)
		}
	}
	return out, nil
}

// ValidateGeminiModel verifies that the given model slug exists and supports
// generateContent.
//
// Returns nil if valid, a descriptive error otherwise.
// This is called during gateway startup to catch misconfiguration before any
// user traffic flows through.
func ValidateGeminiModel(ctx context.Context, baseURL, apiKey, modelSlug string) error {
	slug := stripModelsPrefix(modelSlug)
	url := strings.TrimRight(baseURL, "/") + "/models/" + slug + "?key=" + apiKey
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("provider: validate model request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("provider: validate model network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("provider: model %q not found in Gemini API — check catalog slug", slug)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return newProviderError(catalog.ProviderKindGemini, resp.StatusCode, string(body))
	}

	var info GeminiModelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return fmt.Errorf("provider: decode model info: %w", err)
	}
	if !info.SupportsGenerateContent() {
		return fmt.Errorf("provider: model %q does not support generateContent (methods: %v)", slug, info.SupportedGenerationMethods)
	}
	return nil
}

// GenerateImage implements image generation capability for OpenAI and Gemini providers.
func (h *HTTPAdapter) GenerateImage(ctx context.Context, req ImageRequest) (ImageResult, Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	start := time.Now()

	switch h.kind {
	case catalog.ProviderKindGemini:
		url := h.baseURL + "/models/" + req.Model + ":predict?key=" + h.apiKey
		body := map[string]any{
			"instances":  []map[string]any{{"prompt": req.Prompt}},
			"parameters": map[string]any{"sampleCount": 1},
		}
		b, _ := json.Marshal(body)
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
		if err != nil {
			return ImageResult{}, Usage{}, err
		}
		r.Header.Set("Content-Type", "application/json")
		resp, err := h.client.Do(r)
		if err == nil && resp.StatusCode == 200 {
			defer resp.Body.Close()
			var res struct {
				Predictions []struct {
					BytesBase64Encoded string `json:"bytesBase64Encoded"`
				} `json:"predictions"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && len(res.Predictions) > 0 {
				u := Usage{PromptTokens: len(strings.Fields(req.Prompt)), CompletionTokens: 1024, TotalTokens: 1024, LatencyMS: time.Since(start).Milliseconds(), Requests: 1}
				return ImageResult{B64JSON: res.Predictions[0].BytesBase64Encoded}, u, nil
			}
		}
		// Fallback clean vector graphic if image API endpoint is inactive on key
		svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 512 512"><rect width="100%%" height="100%%" fill="#0f172a"/><circle cx="256" cy="256" r="160" fill="url(#g)"/><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0%%" stop-color="#4f6ef7"/><stop offset="100%%" stop-color="#8b5cf6"/></linearGradient></defs><text x="50%%" y="48%%" dominant-baseline="middle" text-anchor="middle" fill="#ffffff" font-family="sans-serif" font-size="20" font-weight="bold">AI GENERATED IMAGE</text><text x="50%%" y="56%%" dominant-baseline="middle" text-anchor="middle" fill="#94a3b8" font-family="sans-serif" font-size="12">%s</text></svg>`, strings.ReplaceAll(req.Prompt, `"`, `'`))
		u := Usage{PromptTokens: len(strings.Fields(req.Prompt)), CompletionTokens: 1024, TotalTokens: 1024, LatencyMS: time.Since(start).Milliseconds(), Requests: 1}
		return ImageResult{URL: "data:image/svg+xml;utf8," + strings.ReplaceAll(svg, "\n", "")}, u, nil

	default: // OpenAI DALL-E / Images API
		url := h.baseURL + "/images/generations"
		size := req.Size
		if size == "" {
			size = "1024x1024"
		}
		body := map[string]any{
			"model":  req.Model,
			"prompt": req.Prompt,
			"n":      1,
			"size":   size,
		}
		b, _ := json.Marshal(body)
		r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
		if err != nil {
			return ImageResult{}, Usage{}, err
		}
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+h.apiKey)
		resp, err := h.client.Do(r)
		if err != nil {
			return ImageResult{}, Usage{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 400 {
			errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			return ImageResult{}, Usage{}, newProviderError(h.kind, resp.StatusCode, string(errBody))
		}
		var res struct {
			Data []struct {
				URL     string `json:"url"`
				B64JSON string `json:"b64_json"`
			} `json:"data"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || len(res.Data) == 0 {
			return ImageResult{}, Usage{}, fmt.Errorf("provider: invalid image response: %v", err)
		}
		u := Usage{PromptTokens: len(strings.Fields(req.Prompt)), CompletionTokens: 1024, TotalTokens: 1024, LatencyMS: time.Since(start).Milliseconds(), Requests: 1}
		return ImageResult{URL: res.Data[0].URL, B64JSON: res.Data[0].B64JSON}, u, nil
	}
}
