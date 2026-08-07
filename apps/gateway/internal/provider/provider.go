// Package provider defines the AI provider abstraction.
//
// The gateway never knows which upstream (mock, OpenAI, Gemini) serves a
// request. The Registry maps a catalog provider to an implementation; new
// providers only need to implement this package's Provider interface.
package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/x402-marketplace/catalog"
	"github.com/x402-marketplace/gateway/internal/mockprovider"
)

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

// StreamFunc receives normalized OpenAI-compatible SSE chunks. The final chunk
// is a UsageChunk event carrying metering data.
type StreamFunc func(chunk string) error

// Provider is implemented by every AI provider adapter.
type Provider interface {
	// Stream performs a streaming completion, calling emit for each chunk.
	// It returns the internal usage breakdown and handles cancellation via ctx.
	Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error)
	// Kind identifies the implementation for routing/debug.
	Kind() catalog.ProviderKind
}

// UsageChunk is the terminal SSE event emitted by adapters. It is
// OpenAI-convention styled so the frontend parses one format everywhere.
const (
	eventChunk = "content"
	eventDone  = "done"
)

// chunkEvent is the normalized SSE payload (OpenAI-compatible).
func chunkEvent(model, text string) string {
	return `data: {"id":"chatcmpl-x","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"content":` + jsonStr(text) + `},"finish_reason":null}]}` + "\n\n"
}

func doneEvent(u Usage) string {
	b, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-x", "object": "chat.completion.chunk", "choices": []map[string]any{{"index": 0, "delta": map[string]any{}, "finish_reason": "stop"}},
		"usage": map[string]int{"prompt_tokens": u.PromptTokens, "completion_tokens": u.CompletionTokens, "total_tokens": u.TotalTokens},
	})
	return "data: " + string(b) + "\n\n"
}

func jsonStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// MockAdapter serves deterministic demo completions locally (no network).
type MockAdapter struct{}

// NewMockAdapter returns the demo adapter.
func NewMockAdapter() *MockAdapter { return &MockAdapter{} }

// Kind implements Provider.
func (m *MockAdapter) Kind() catalog.ProviderKind { return catalog.ProviderKindMock }

// Stream implements Provider with a ~60 tok/s word-by-word stream.
func (m *MockAdapter) Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error) {
	text := mockprovider.CorpusText(req.Model)
	words := strings.Fields(text)
	start := time.Now()
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()

	emit(chunkEvent(req.Model, "")) // role preface
	for _, w := range words {
		select {
		case <-ctx.Done():
			return Usage{}, ctx.Err()
		case <-ticker.C:
			if err := emit(chunkEvent(req.Model, w+" ")); err != nil {
				return Usage{}, err
			}
		}
	}
	return Usage{
		PromptTokens:     128,
		CompletionTokens: len(words) * 8,
		TotalTokens:      128 + len(words)*8,
		LatencyMS:        time.Since(start).Milliseconds(),
		Requests:         1,
	}, nil
}

// HTTPAdapter streams from OpenAI-compatible and Gemini HTTP APIs. It sets a
// per-request timeout, honors cancellation, and retries connection failures.
type HTTPAdapter struct {
	kind    catalog.ProviderKind
	baseURL string
	apiKey  string
	client  *http.Client
	timeout time.Duration
}

// NewOpenAIAdapter creates an adapter for OpenAI-compatible /chat/completions.
func NewOpenAIAdapter(baseURL, apiKey string) *HTTPAdapter {
	return &HTTPAdapter{kind: catalog.ProviderKindOpenAI, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: &http.Client{}, timeout: 60 * time.Second}
}

// NewGeminiAdapter creates an adapter for the Gemini generateContent API.
func NewGeminiAdapter(baseURL, apiKey string) *HTTPAdapter {
	return &HTTPAdapter{kind: catalog.ProviderKindGemini, baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: &http.Client{}, timeout: 60 * time.Second}
}

// Kind implements Provider.
func (h *HTTPAdapter) Kind() catalog.ProviderKind { return h.kind }

// Stream implements Provider.
func (h *HTTPAdapter) Stream(ctx context.Context, req Request, emit StreamFunc) (Usage, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout)
	defer cancel()
	start := time.Now()

	var body any
	var url string
	switch h.kind {
	case catalog.ProviderKindGemini:
		url = h.baseURL + "/models/" + req.Model + ":streamGenerateContent?alt=sse&key=" + h.apiKey
		body = map[string]any{
			"contents": []map[string]any{{"role": "user", "parts": []map[string]any{{"text": lastUserText(req.Messages)}}}},
			"generationConfig": map[string]any{"maxOutputTokens": req.MaxTokens, "streaming": true},
		}
	default: // openai
		url = h.baseURL + "/chat/completions"
		msgs := make([]map[string]string, 0, len(req.Messages))
		for _, m := range req.Messages {
			msgs = append(msgs, map[string]string{"role": m.Role, "content": m.Content})
		}
		body = map[string]any{"model": req.Model, "messages": msgs, "stream": true, "max_tokens": req.MaxTokens}
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ { // connection retries
		promptTokens := estimatePromptTokens(req.Messages)
		prompt := promptTokens
		completion := 0
		err := h.streamOnce(ctx, url, body, emit, &completion)
		if err == nil {
			return Usage{PromptTokens: prompt, CompletionTokens: completion, TotalTokens: prompt + completion, LatencyMS: time.Since(start).Milliseconds(), Requests: attempt + 1}, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			break // cancellation/timeout is not retryable
		}
		select {
		case <-ctx.Done():
			return Usage{}, ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	return Usage{}, fmt.Errorf("provider: stream failed after retries: %w", lastErr)
}

func (h *HTTPAdapter) streamOnce(ctx context.Context, url string, body any, emit StreamFunc, completion *int) error {
	b, _ := json.Marshal(body)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(b)))
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", "application/json")
	if h.kind != catalog.ProviderKindGemini {
		r.Header.Set("Authorization", "Bearer "+h.apiKey)
	}
	resp, err := h.client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("provider: status %d: %s", resp.StatusCode, string(body))
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
		var ev struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
			Candidates []struct {
				Content struct {
					Parts []struct {
						Text string `json:"text"`
					} `json:"parts"`
				} `json:"content"`
				FinishReason string `json:"finishReason"`
			} `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			continue
		}
		if len(ev.Choices) > 0 {
			if ev.Choices[0].Delta.Content != "" {
				*completion += len(strings.Fields(ev.Choices[0].Delta.Content))
				if err := emit(chunkEvent("", ev.Choices[0].Delta.Content)); err != nil {
					return err
				}
			}
		}
		if len(ev.Candidates) > 0 && len(ev.Candidates[0].Content.Parts) > 0 {
			text := ev.Candidates[0].Content.Parts[0].Text
			*completion += len(strings.Fields(text))
			if err := emit(chunkEvent("", text)); err != nil {
				return err
			}
		}
		if ev.Usage != nil {
			*completion = ev.Usage.CompletionTokens
		}
	}
	return scanner.Err()
}

func estimatePromptTokens(msgs []Message) int {
	n := 0
	for _, m := range msgs {
		n += len(strings.Fields(m.Content))
	}
	return n + len(msgs)*4
}

func lastUserText(msgs []Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == "user" {
			return msgs[i].Content
		}
	}
	if len(msgs) > 0 {
		return msgs[len(msgs)-1].Content
	}
	return ""
}

// Registry selects a concrete Provider for a catalog provider.
type Registry struct {
	byKind map[catalog.ProviderKind]Provider
}

// NewRegistry wires adapters. A missing API key falls back to the mock
// adapter so the demo always runs without credentials.
func NewRegistry(cat catalog.Catalog, openAIKey, geminiKey string) *Registry {
	r := &Registry{byKind: map[catalog.ProviderKind]Provider{}}
	r.byKind[catalog.ProviderKindMock] = NewMockAdapter()
	if openAIKey != "" {
		r.byKind[catalog.ProviderKindOpenAI] = NewOpenAIAdapter("https://api.openai.com/v1", openAIKey)
	} else {
		r.byKind[catalog.ProviderKindOpenAI] = NewMockAdapter()
	}
	if geminiKey != "" {
		r.byKind[catalog.ProviderKindGemini] = NewGeminiAdapter("https://generativelanguage.googleapis.com/v1beta", geminiKey)
	} else {
		r.byKind[catalog.ProviderKindGemini] = NewMockAdapter()
	}
	// Ensure every catalog provider kind is covered.
	for _, p := range cat.Providers {
		if _, ok := r.byKind[p.Kind]; !ok {
			r.byKind[p.Kind] = NewMockAdapter()
		}
	}
	return r
}

// Select returns the adapter for a catalog provider.
func (r *Registry) Select(p catalog.Provider) (Provider, error) {
	if ad, ok := r.byKind[p.Kind]; ok {
		return ad, nil
	}
	return nil, fmt.Errorf("provider registry: no adapter for %q", p.Kind)
}