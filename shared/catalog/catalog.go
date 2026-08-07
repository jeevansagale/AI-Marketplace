// Package catalog provides the canonical AI marketplace catalog: providers,
// models, INR pricing, session-cost estimation, and provider routing.
//
// Both the gateway and the control plane import this package so pricing and
// routing decisions are never duplicated across services.
package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// ProviderKind identifies the upstream AI provider implementation.
type ProviderKind string

const (
	ProviderKindGemini ProviderKind = "gemini"
	ProviderKindOpenAI ProviderKind = "openai"
	ProviderKindMock   ProviderKind = "mock"
)

// Provider is an AI provider registered on the marketplace.
type Provider struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Slug       string       `json:"slug"`
	Kind       ProviderKind `json:"kind"`
	BaseURL    string       `json:"base_url"`
	APIKeyEnv  string       `json:"-"` // env var holding the provider API key (never serialized)
	Wallet     string       `json:"wallet_address"`
	Status     string       `json:"status"` // active | paused | suspended
	Reputation float64      `json:"reputation"`
}

// Model is a purchasable AI model. All prices are in paise (INR, 1/100).
// PricePrompt / PriceCompletion are per 1K tokens.
type Model struct {
	ID              string  `json:"id"`
	ProviderID      string  `json:"provider_id"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Category        string  `json:"category"` // chat | vision | ocr | embeddings | audio
	Description     string  `json:"description"`
	ContextWindow   int     `json:"context_window"`
	PriceBase       int64   `json:"price_base_paise"`       // flat per-request fee
	PricePrompt     int64   `json:"price_prompt_paise"`     // per 1K prompt tokens
	PriceCompletion int64   `json:"price_completion_paise"` // per 1K completion tokens
	ProviderName    string  `json:"provider"`
	LatencyMs       int     `json:"p90_latency_ms"`
	Throughput      float64 `json:"throughput_tps"`
	IsActive        bool    `json:"is_active"`
}

// Catalog bundles providers and models for a single fetch.
type Catalog struct {
	Providers []Provider `json:"providers"`
	Models    []Model    `json:"models"`
}

// Default returns the canonical marketplace catalog.
func Default() Catalog {
	return Catalog{Providers: defaultProviders(), Models: defaultModels()}
}

// ConvertUSDCToPaise converts existing micro-USDC pricing to paise using a
// fixed demo rate of 1 USDC = 83.00 INR. Migration helper only.
func ConvertUSDCToPaise(microUSDC int64) int64 {
	if microUSDC <= 0 {
		return 0
	}
	const perPaise = 83.0 * 1_000_000 // micro-USDC per rupee
	return int64(float64(microUSDC) / perPaise * 100)
}

func defaultProviders() []Provider {
	base := "https://generativelanguage.googleapis.com/v1beta"
	openAIBase := "https://api.openai.com/v1"
	return []Provider{
		{ID: "10000000-0000-0000-0000-000000000001", Name: "Vertex Labs", Slug: "vertexlabs", Kind: ProviderKindOpenAI, BaseURL: openAIBase, APIKeyEnv: "OMNI_OPENAI_API_KEY", Wallet: "TESTNETWALLETNOVA000000000000000000000000000000", Status: "active", Reputation: 0.98},
		{ID: "10000000-0000-0000-0000-000000000002", Name: "Gemini Works", Slug: "geminiworks", Kind: ProviderKindGemini, BaseURL: base, APIKeyEnv: "OMNI_GEMINI_API_KEY", Wallet: "TESTNETWALLETALGO00000000000000000000000000000", Status: "active", Reputation: 0.96},
		{ID: "10000000-0000-0000-0000-000000000003", Name: "TensorGrid", Slug: "tensorgrid", Kind: ProviderKindOpenAI, BaseURL: openAIBase, APIKeyEnv: "OMNI_OPENAI_API_KEY", Wallet: "TESTNETWALLETTENSOR000000000000000000000000000", Status: "active", Reputation: 0.94},
		{ID: "10000000-0000-0000-0000-000000000004", Name: "VisionForge", Slug: "visionforge", Kind: ProviderKindOpenAI, BaseURL: openAIBase, APIKeyEnv: "OMNI_OPENAI_API_KEY", Wallet: "TESTNETWALLETVISION000000000000000000000000000", Status: "active", Reputation: 0.92},
		{ID: "10000000-0000-0000-0000-000000000005", Name: "VectorHaven", Slug: "vectorhaven", Kind: ProviderKindOpenAI, BaseURL: openAIBase, APIKeyEnv: "OMNI_OPENAI_API_KEY", Wallet: "TESTNETWALLETVECTOR000000000000000000000000000", Status: "active", Reputation: 0.97},
		{ID: "10000000-0000-0000-0000-000000000006", Name: "AudioWave", Slug: "audiowave", Kind: ProviderKindMock, BaseURL: "", APIKeyEnv: "", Wallet: "TESTNETWALLETAUDIO0000000000000000000000000000", Status: "paused", Reputation: 0.89},
	}
}

// defaultModels returns the canonical model catalog in paise (INR) per 1K tokens.
func defaultModels() []Model {
	return []Model{
		{"20000000-0000-0000-0000-000000000001", "10000000-0000-0000-0000-000000000001", "Vertex Llama 3.3 70B", "vertex-llama-3.3-70b-instruct", "chat", "Flagship Llama 3.3 70B - best quality per rupee.", 131072, 12, 1, 4, "Vertex Labs", 380, 85.4, true},
		{"20000000-0000-0000-0000-000000000002", "10000000-0000-0000-0000-000000000001", "Vertex Llama 3.1 8B", "vertex-llama-3.1-8b-instruct", "chat", "Fast 8B instruct for cost-sensitive workloads.", 131072, 6, 1, 2, "Vertex Labs", 140, 220.0, true},
		{"20000000-0000-0000-0000-000000000003", "10000000-0000-0000-0000-000000000002", "Gemini 2.0 Flash", "gemini-2.0-flash", "chat", "Low-latency Gemini generation.", 1048576, 10, 2, 3, "Gemini Works", 480, 96.0, true},
		{"20000000-0000-0000-0000-000000000004", "10000000-0000-0000-0000-000000000003", "TensorGrid Mixtral 8x7B", "tensorgrid-mixtral-8x7b", "chat", "Sparse MoE - high throughput.", 32768, 8, 1, 3, "TensorGrid", 210, 150.0, true},
		{"20000000-0000-0000-0000-000000000005", "10000000-0000-0000-0000-000000000001", "Vertex Qwen 2.5 72B", "vertex-qwen-2.5-72b", "chat", "Multilingual 72B assistant.", 131072, 12, 1, 4, "Vertex Labs", 470, 62.0, true},
		{"20000000-0000-0000-0000-000000000006", "10000000-0000-0000-0000-000000000004", "VisionForge GPT-4o Vision", "visionforge-gpt-4o-vision-class", "vision", "Image understanding and captioning.", 128000, 50, 5, 8, "VisionForge", 850, 25.0, true},
		{"20000000-0000-0000-0000-000000000007", "10000000-0000-0000-0000-000000000004", "VisionForge YOLO OCR", "visionforge-yolo-v11-ocr", "ocr", "Scene-text OCR with bounding boxes.", 4096, 30, 4, 6, "VisionForge", 620, 30.0, true},
		{"20000000-0000-0000-0000-000000000008", "10000000-0000-0000-0000-000000000005", "VectorHaven Text Embedding 3", "vectorhaven-text-embedding-3", "embeddings", "768-dim dense embeddings for RAG.", 8192, 4, 1, 0, "VectorHaven", 90, 400.0, true},
		{"20000000-0000-0000-0000-000000000009", "10000000-0000-0000-0000-000000000005", "VectorHaven BGE-M3", "vectorhaven-bge-m3", "embeddings", "Multilingual embedding model.", 8192, 4, 1, 0, "VectorHaven", 100, 380.0, true},
		{"20000000-0000-0000-0000-000000000010", "10000000-0000-0000-0000-000000000006", "AudioWave Whisper v3", "audiowave-whisper-large-v3", "audio", "Speech-to-text transcription.", 262144, 60, 10, 0, "AudioWave", 1200, 12.0, false},
	}
}

// FindProvider returns the provider for a slug and whether it exists.
func (c Catalog) FindProvider(slug string) (Provider, bool) {
	for _, p := range c.Providers {
		if p.Slug == slug {
			return p, true
		}
	}
	return Provider{}, false
}

// FindProviderByID returns the provider for a UUID and whether it exists.
func (c Catalog) FindProviderByID(id string) (Provider, bool) {
	for _, p := range c.Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// FindModel returns the model for a slug and whether it exists.
func (c Catalog) FindModel(slug string) (Model, bool) {
	for _, m := range c.Models {
		if m.Slug == slug {
			return m, true
		}
	}
	return Model{}, false
}

// ProviderForModel resolves the active provider that serves a model.
func (c Catalog) ProviderForModel(modelSlug string) (Provider, Model, error) {
	m, ok := c.FindModel(modelSlug)
	if !ok {
		return Provider{}, Model{}, fmt.Errorf("catalog: unknown model %q", modelSlug)
	}
	p, ok := c.FindProviderByID(m.ProviderID)
	if !ok {
		return Provider{}, Model{}, fmt.Errorf("catalog: unknown provider %q", m.ProviderID)
	}
	if p.Status != "active" {
		return Provider{}, Model{}, fmt.Errorf("catalog: provider %q is %s", p.Name, p.Status)
	}
	if !m.IsActive {
		return Provider{}, Model{}, fmt.Errorf("catalog: model %q is inactive", modelSlug)
	}
	return p, m, nil
}

// EstimateSessionCost computes the projected session cost in paise for a
// request (prompt tokens + max completion tokens). This is what the wallet
// reserves before a conversation starts.
func (m Model) EstimateSessionCost(promptTokens, maxCompletionTokens int) int64 {
	if promptTokens < 0 || maxCompletionTokens < 0 {
		return 0
	}
	base := m.PriceBase
	prompt := int64(promptTokens) * m.PricePrompt / 1000
	completion := int64(maxCompletionTokens) * m.PriceCompletion / 1000
	return base + prompt + completion
}

// FormatINR renders paise as Indian-rupee currency, e.g. 237 -> "₹2.37".
// Kept here so every service formats money identically.
func FormatINR(paise int64) string {
	if paise < 0 {
		return "-" + FormatINR(-paise)
	}
	whole := paise / 100
	frac := paise % 100
	return fmt.Sprintf("₹%d.%02d", whole, frac)
}

// ActiveModels returns active models sorted by provider then name.
func (c Catalog) ActiveModels() []Model {
	out := make([]Model, 0, len(c.Models))
	for _, m := range c.Models {
		if m.IsActive {
			out = append(out, m)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProviderName != out[j].ProviderName {
			return out[i].ProviderName < out[j].ProviderName
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// NormalizeModelSlug strips a leading provider/ prefix if present so the
// gateway can accept OpenAI-style "org/model" ids.
func NormalizeModelSlug(slug string) string {
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		return slug[i+1:]
	}
	return slug
}