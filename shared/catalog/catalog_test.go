package catalog

import "testing"

func TestFormatINR(t *testing.T) {
	cases := map[int64]string{
		0:     "₹0.00",
		100:   "₹1.00",
		237:   "₹2.37",
		10000: "₹100.00",
		-50:   "-₹0.50",
	}
	for in, want := range cases {
		if got := FormatINR(in); got != want {
			t.Errorf("FormatINR(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestEstimateSessionCost(t *testing.T) {
	m := Model{PriceBase: 6, PricePrompt: 1, PriceCompletion: 2}
	got := m.EstimateSessionCost(100, 512)
	want := int64(6) + 100*1/1000 + 512*2/1000
	if got != want {
		t.Errorf("EstimateSessionCost = %d, want %d", got, want)
	}
}

func TestProviderForModel(t *testing.T) {
	c := Default()
	p, m, err := c.ProviderForModel("gemini-2.0-flash")
	if err != nil {
		t.Fatal(err)
	}
	if p.Kind != ProviderKindGemini {
		t.Errorf("expected gemini provider, got %s", p.Kind)
	}
	if m.Slug != "gemini-2.0-flash" {
		t.Errorf("unexpected model %s", m.Slug)
	}
	if _, _, err := c.ProviderForModel("does-not-exist"); err == nil {
		t.Error("expected error for unknown model")
	}
}
