// Package mockprovider implements the demo AI provider.
// Serves OpenAI-compatible /v1/chat/completions with SSE streaming,
// progressing at a deterministic ~60 tokens/second for the live demo.
package mockprovider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// Demo corpus: deterministic per-model responses that "feel" real.
	corpus = []string{
		"Algorand's Pure Proof-of-Stake consensus achieves finality in under four seconds with zero fork risk.",
		"HTTP 402 Payment Required is the native payment primitive for autonomous machine-to-machine commerce.",
		"State channels let AI agents stream inference results while paying per token off-chain.",
		"USDC on Algorand settles microtransactions at a fraction of a cent in transaction fees.",
		"x402 turns every API endpoint into a self-serve, pay-per-use machine.",
	}
)

// Handler serves POST /v1/chat/completions.
// Non-stream returns a single completion; stream returns SSE chunks.
func Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model       string          `json:"model"`
			Stream      bool            `json:"stream"`
			Messages    []chatMessage   `json:"messages"`
			MaxTokens   int             `json:"max_tokens"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":{"message":"bad request","type":"invalid_request_error"}}`, http.StatusBadRequest)
			return
		}

		if req.Stream {
			streamChat(w, req.Model)
			return
		}
		nonStreamChat(w, req.Model)
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func pickText(model string) string {
	// Deterministic-ish selection for demo consistency.
	idx := 0
	for _, c := range model {
		idx = (idx + int(c)) % len(corpus)
	}
	return corpus[idx]
}

func nonStreamChat(w http.ResponseWriter, model string) {
	text := pickText(model) + " (non-stream)"
	body := map[string]any{
		"id":      "chatcmpl-mock-001",
		"object":  "chat.completion",
		"model":   model,
		"choices": []map[string]any{{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": text,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     128,
			"completion_tokens": 64,
			"total_tokens":      192,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}

func streamChat(w http.ResponseWriter, model string) {
	text := pickText(model)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	write := func(s string) bool {
		_, err := fmt.Fprint(w, s)
		if flusher != nil {
			flusher.Flush()
		}
		return err == nil
	}

	// Preface chunk with role metadata (OpenAI convention).
	pre := `data: {"id":"chatcmpl-mock-001","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"role":"assistant","content":""},"finish_reason":null}]}` + "\n\n"
	if !write(pre) {
		return
	}

	words := strings.Fields(text)
	for _, word := range words {
		if !write(`data: {"id":"chatcmpl-mock-001","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{"content":"` + word + ` "},"finish_reason":null}]}` + "\n\n") {
			return
		}
		time.Sleep(120 * time.Millisecond) // ~8 words/s ≈ 60 tok/s
	}

	_ = write(`data: {"id":"chatcmpl-mock-001","object":"chat.completion.chunk","model":"` + model + `","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}` + "\n\n")
	_ = write("data: [DONE]\n\n")
}
