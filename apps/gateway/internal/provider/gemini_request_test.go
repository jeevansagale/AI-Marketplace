package provider_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/x402-marketplace/catalog"
	"github.com/x402-marketplace/gateway/internal/provider"
)

// TestGeminiRequestBodyHasNoStreamingField is the definitive proof that the
// payload sent to Gemini does NOT contain a "streaming" field anywhere.
//
// Run with: go test ./internal/provider/ -v -run TestGeminiRequestBodyHasNoStreamingField
func TestGeminiRequestBodyHasNoStreamingField(t *testing.T) {
	adapter := provider.NewGeminiAdapter("https://generativelanguage.googleapis.com/v1beta", "TEST_KEY")

	req := provider.Request{
		Model: "gemini-2.0-flash",
		Messages: []provider.Message{
			{Role: "user", Content: "Hello, what is 2+2?"},
		},
		MaxTokens: 512,
	}

	url, body := adapter.BuildRequestForTest(req)

	// Serialize to JSON exactly as the HTTP client does.
	raw, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	t.Logf("=== REQUEST URL ===\n%s\n", url)
	t.Logf("=== REQUEST BODY ===\n%s\n", string(raw))

	// ── Assertions ────────────────────────────────────────────────────────

	// 1. URL must use the streaming endpoint.
	if !strings.Contains(url, ":streamGenerateContent") {
		t.Errorf("URL must contain :streamGenerateContent, got: %s", url)
	}
	if !strings.Contains(url, "alt=sse") {
		t.Errorf("URL must contain alt=sse, got: %s", url)
	}

	// 2. Body must NOT contain "streaming" anywhere.
	if strings.Contains(string(raw), `"streaming"`) {
		t.Errorf(`FAIL: JSON body contains "streaming" field:\n%s`, string(raw))
	}

	// 3. generationConfig must exist and contain only maxOutputTokens.
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}
	genCfg, ok := parsed["generationConfig"].(map[string]any)
	if !ok {
		t.Fatal("generationConfig is missing from request body")
	}
	if _, hasStreaming := genCfg["streaming"]; hasStreaming {
		t.Errorf(`FAIL: generationConfig contains "streaming" field`)
	}
	if _, hasMax := genCfg["maxOutputTokens"]; !hasMax {
		t.Errorf(`generationConfig is missing "maxOutputTokens"`)
	}
	t.Logf("generationConfig keys: %v", keys(genCfg))

	// 4. contents must be present and non-empty.
	contents, ok := parsed["contents"].([]any)
	if !ok || len(contents) == 0 {
		t.Error("contents array is missing or empty")
	}

	// 5. Provider type must be Gemini.
	if adapter.Kind() != catalog.ProviderKindGemini {
		t.Errorf("expected kind gemini, got %q", adapter.Kind())
	}

	t.Log("ALL ASSERTIONS PASSED — no streaming field in Gemini request body")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
