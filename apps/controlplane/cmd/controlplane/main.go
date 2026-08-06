// Pay-per-use AI Marketplace — Control Plane (Hackathon MVP).
// Serves the marketplace catalog + provider registry for the dashboard.
// Phase 0: in-memory catalog mirroring the canonical DB seed so the demo
// runs without Postgres. DB-backed read/write path lands in M2 (BE2).
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"
)

type model struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Context     int     `json:"context_window"`
	PricePrompt uint64  `json:"price_prompt_micro_usdc"`
	PriceComp   uint64  `json:"price_completion_micro_usdc"`
	Provider    string  `json:"provider"`
	LatencyMs   int     `json:"p90_latency_ms"`
	Throughput  float64 `json:"throughput_tps"`
	Active      bool    `json:"is_active"`
}

type provider struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Slug       string  `json:"slug"`
	BaseURL    string  `json:"base_url"`
	Wallet     string  `json:"wallet_address"`
	Status     string  `json:"status"`
	Reputation float64 `json:"reputation"`
}

// catalog mirrors infra/docker/postgres/init/0001_init.sql seed.
func seedCatalog() ([]provider, []model) {
	providers := []provider{
		{"10000000-0000-0000-0000-000000000001", "Nova Compute", "nova", "http://gateway:8080/provider/nova/v1", "TESTNETWALLETNOVA000000000000000000000000000000", "active", 0.98},
		{"10000000-0000-0000-0000-000000000002", "AlgoForge", "algoforge", "http://gateway:8080/provider/algoforge/v1", "TESTNETWALLETALGO00000000000000000000000000000", "active", 0.96},
		{"10000000-0000-0000-0000-000000000003", "TensorGrid", "tensorgrid", "http://gateway:8080/provider/tensorgrid/v1", "TESTNETWALLETTENSOR000000000000000000000000000", "active", 0.94},
		{"10000000-0000-0000-0000-000000000004", "VisionForge", "visionforge", "http://gateway:8080/provider/visionforge/v1", "TESTNETWALLETVISION000000000000000000000000000", "active", 0.92},
		{"10000000-0000-0000-0000-000000000005", "VectorHaven", "vectorhaven", "http://gateway:8080/provider/vectorhaven/v1", "TESTNETWALLETVECTOR000000000000000000000000000", "active", 0.97},
		{"10000000-0000-0000-0000-000000000006", "AudioWave", "audiowave", "http://gateway:8080/provider/audiowave/v1", "TESTNETWALLETAUDIO0000000000000000000000000000", "paused", 0.89},
	}

	models := []model{
		{"20000000-0000-0000-0000-000000000001", "market/llama-3.3-70b-instruct", "market-llama-3.3-70b-instruct", "chat", "Flagship Llama 3.3 70B — best quality per dollar.", 131072, 2000, 8000, "Nova Compute", 380, 85.4, true},
		{"20000000-0000-0000-0000-000000000002", "market/llama-3.1-8b-instruct", "market-llama-3.1-8b-instruct", "chat", "Fast 8B instruct for cost-sensitive workloads.", 131072, 1000, 3000, "Nova Compute", 140, 220.0, true},
		{"20000000-0000-0000-0000-000000000003", "algoforge/deepseek-r1-32b", "algoforge-deepseek-r1-32b", "chat", "Reasoning model with chain-of-thought.", 65536, 2000, 6000, "AlgoForge", 520, 48.2, true},
		{"20000000-0000-0000-0000-000000000004", "tensorgrid/mixtral-8x7b", "tensorgrid-mixtral-8x7b", "chat", "Sparse MoE — high throughput.", 32768, 1000, 4000, "TensorGrid", 210, 150.0, true},
		{"20000000-0000-0000-0000-000000000005", "market/qwen-2.5-72b", "market-qwen-2.5-72b", "chat", "Multilingual 72B assistant.", 131072, 2000, 7000, "Nova Compute", 470, 62.0, true},
		{"20000000-0000-0000-0000-000000000006", "visionforge/gpt-4o-vision-class", "visionforge-gpt-4o-vision-class", "vision", "Image understanding and captioning.", 128000, 10000, 15000, "VisionForge", 850, 25.0, true},
		{"20000000-0000-0000-0000-000000000007", "visionforge/yolo-v11-ocr", "visionforge-yolo-v11-ocr", "ocr", "Scene-text OCR with bounding boxes.", 4096, 8000, 12000, "VisionForge", 620, 30.0, true},
		{"20000000-0000-0000-0000-000000000008", "vectorhaven/text-embedding-3", "vectorhaven-text-embedding-3", "embeddings", "768-dim dense embeddings for RAG.", 8192, 1000, 0, "VectorHaven", 90, 400.0, true},
		{"20000000-0000-0000-0000-000000000009", "vectorhaven/bge-m3", "vectorhaven-bge-m3", "embeddings", "Multilingual embedding model.", 8192, 1000, 0, "VectorHaven", 100, 380.0, true},
		{"20000000-0000-0000-0000-000000000010", "audiowave/whisper-large-v3", "audiowave-whisper-large-v3", "audio", "Speech-to-text transcription.", 262144, 20000, 0, "AudioWave", 1200, 12.0, false},
	}
	return providers, models
}

func main() {
	addr := envOr("OMNI_HTTP_ADDR", ":8081")
	logger := log.New(os.Stdout, "[controlplane] ", log.LstdFlags)

	providers, models := seedCatalog()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "controlplane",
			"mode":    envOr("OMNI_MODE", "demo"),
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// OpenAI-compatible model list (same catalog as the gateway).
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, _ *http.Request) {
		type openAIModel struct {
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		}
		data := make([]openAIModel, 0, len(models))
		for _, m := range models {
			if !m.Active {
				continue
			}
			data = append(data, openAIModel{"model", 1710000000, m.Provider})
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
	})

	// Marketplace provider registry (dashboard).
	mux.HandleFunc("GET /api/v1/providers", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"providers": providers})
	})

	// Marketplace catalog with full model detail (dashboard cards).
	mux.HandleFunc("GET /api/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"models": models})
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           logMiddleware(logger, corsMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("controlplane listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server error: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func logMiddleware(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		logger.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
