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
	Category        string  `json:"category"` // chat | vision
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
	geminiBase := "https://generativelanguage.googleapis.com/v1beta"
	openAIBase := "https://api.openai.com/v1"
	return []Provider{
		{ID: "10000000-0000-0000-0000-000000000001", Name: "Gemini", Slug: "gemini", Kind: ProviderKindGemini, BaseURL: geminiBase, APIKeyEnv: "OMNI_GEMINI_API_KEY", Wallet: "TESTNETWALLETGEMINI00000000000000000000000000", Status: "active", Reputation: 0.98},
		{ID: "10000000-0000-0000-0000-000000000002", Name: "OpenAI", Slug: "openai", Kind: ProviderKindOpenAI, BaseURL: openAIBase, APIKeyEnv: "OMNI_OPENAI_API_KEY", Wallet: "TESTNETWALLETOPENAI00000000000000000000000000", Status: "active", Reputation: 0.99},
	}
}

// defaultModels returns the canonical model catalog in paise (INR) per 1K tokens.
// Model slugs are verified against GET /v1beta/models with a live API key.
// Last verified: 2026-08-13.
func defaultModels() []Model {
	return []Model{
		// ── Gemini (verified live against GET /v1beta/models 2026-08-13) ──────
		// gemini-2.5-flash: fast, low-cost, 1M context.
		{"20000000-0000-0000-0000-000000000001", "10000000-0000-0000-0000-000000000001", "Gemini 2.5 Flash", "gemini-2.5-flash", "chat", "Fast and affordable multimodal model with 1M token context.", 1048576, 10, 2, 3, "Gemini", 380, 96.0, true},
		// gemini-2.5-flash-lite: cheapest option for lightweight tasks.
		{"20000000-0000-0000-0000-000000000002", "10000000-0000-0000-0000-000000000001", "Gemini 2.5 Flash Lite", "gemini-2.5-flash-lite", "chat", "Fastest and most affordable Gemini model for high-frequency workloads.", 1048576, 5, 1, 2, "Gemini", 220, 140.0, true},
		// gemini-2.5-pro: highest-intelligence Gemini model.
		{"20000000-0000-0000-0000-000000000003", "10000000-0000-0000-0000-000000000001", "Gemini 2.5 Pro", "gemini-2.5-pro", "chat", "Most intelligent Gemini model for complex reasoning tasks.", 2097152, 30, 7, 20, "Gemini", 800, 40.0, true},
		// ── OpenAI ───────────────────────────────────────────────────────────
		{"20000000-0000-0000-0000-000000000004", "10000000-0000-0000-0000-000000000002", "GPT-4o", "gpt-4o", "chat", "Flagship high-intelligence multimodal flagship model.", 128000, 20, 4, 12, "OpenAI", 400, 80.0, true},
		{"20000000-0000-0000-0000-000000000005", "10000000-0000-0000-0000-000000000002", "GPT-4o Mini", "gpt-4o-mini", "chat", "Affordable, fast small model for lightweight tasks.", 128000, 5, 1, 2, "OpenAI", 180, 160.0, true},
		{"20000000-0000-0000-0000-000000000006", "10000000-0000-0000-0000-000000000002", "GPT-3.5 Turbo", "gpt-3.5-turbo", "chat", "Legacy fast text model.", 16385, 5, 1, 2, "OpenAI", 200, 140.0, true},
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

// FindModel returns the model for a slug, ID, or name and whether it exists.
func (c Catalog) FindModel(identifier string) (Model, bool) {
	// 1. Exact slug match
	for _, m := range c.Models {
		if m.Slug == identifier {
			return m, true
		}
	}
	// 2. Exact ID match
	for _, m := range c.Models {
		if m.ID == identifier {
			return m, true
		}
	}
	// 3. Case-insensitive display name match fallback
	for _, m := range c.Models {
		if strings.EqualFold(m.Name, identifier) {
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
