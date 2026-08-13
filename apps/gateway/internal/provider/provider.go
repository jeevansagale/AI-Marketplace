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

// Provider is implemented by every AI provider adapter.
type Provider interface {
	// Stream performs a streaming completion, calling emit for each SSE chunk.
	// It returns the internal usage breakdown and honors ctx cancellation.
	Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error)
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
		// Endpoint for streaming; ?alt=sse enables Server-Sent Events.
		url := h.baseURL + "/models/" + req.Model + ":streamGenerateContent?alt=sse&key=" + h.apiKey

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
		return fmt.Errorf("provider: status %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024)

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

