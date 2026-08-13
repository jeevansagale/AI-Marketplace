//go:build ignore
// +build ignore

// verify_models.go — run this to list every model your Gemini API key can access.
//
// Usage (from repo root):
//   cd apps/gateway
//   go run ../../scripts/verify_models.go
//
// Or with explicit key:
//   OMNI_GEMINI_API_KEY=AIza... go run ../../scripts/verify_models.go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/joho/godotenv"
)

func main() {
	// Try loading .env from common locations.
	for _, p := range []string{".env", "../.env", "../../.env"} {
		if err := godotenv.Load(p); err == nil {
			fmt.Fprintf(os.Stderr, "Loaded %s\n", p)
			break
		}
	}

	key := os.Getenv("OMNI_GEMINI_API_KEY")
	if key == "" {
		key = os.Getenv("GEMINI_API_KEY")
	}
	if key == "" {
		log.Fatal("OMNI_GEMINI_API_KEY is not set. Add it to .env or export it.")
	}

	url := "https://generativelanguage.googleapis.com/v1beta/models?key=" + key
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("GET /v1beta/models failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		log.Fatalf("API returned %d: %v", resp.StatusCode, body)
	}

	var result struct {
		Models []struct {
			Name                       string   `json:"name"`
			DisplayName                string   `json:"displayName"`
			Description                string   `json:"description"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Fatalf("decode failed: %v", err)
	}

	// Filter to only models supporting generateContent.
	type row struct {
		slug        string
		displayName string
		methods     string
	}
	var rows []row
	for _, m := range result.Models {
		slug := strings.TrimPrefix(m.Name, "models/")
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" || method == "streamGenerateContent" {
				supportsGenerate = true
				break
			}
		}
		if supportsGenerate {
			rows = append(rows, row{
				slug:        slug,
				displayName: m.DisplayName,
				methods:     strings.Join(m.SupportedGenerationMethods, ", "),
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].slug < rows[j].slug })

	fmt.Println("\n=== Models supporting generateContent / streamGenerateContent ===")
	fmt.Printf("%-40s %-30s %s\n", "SLUG", "DISPLAY NAME", "SUPPORTED METHODS")
	fmt.Println(strings.Repeat("─", 110))
	for _, r := range rows {
		fmt.Printf("%-40s %-30s %s\n", r.slug, r.displayName, r.methods)
	}
	fmt.Printf("\nTotal: %d models\n", len(rows))

	// Cross-check against our catalog slugs.
	catalogSlugs := []string{
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-2.5-pro",
	}
	fmt.Println("\n=== Catalog cross-check ===")
	for _, slug := range catalogSlugs {
		found := false
		for _, r := range rows {
			if r.slug == slug {
				found = true
				fmt.Printf("  ✅ %-35s — VALID\n", slug)
				break
			}
		}
		if !found {
			fmt.Printf("  ❌ %-35s — NOT FOUND or does not support generateContent\n", slug)
		}
	}
}
