// Pay-per-use AI Marketplace — Control Plane (Hackathon MVP).
// Serves the marketplace catalog + provider registry for the dashboard.
// The catalog comes from the shared module so pricing and routing decisions
// are never duplicated between the control plane and the gateway.
package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/x402-marketplace/catalog"
)

func main() {
	addr := envOr("OMNI_HTTP_ADDR", ":8081")
	logger := log.New(os.Stdout, "[controlplane] ", log.LstdFlags)

	cat := catalog.Default()

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
		data := make([]openAIModel, 0, len(cat.Models))
		for _, m := range cat.ActiveModels() {
			data = append(data, openAIModel{"model", 1710000000, m.ProviderName})
		}
		writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
	})

	// Marketplace provider registry (dashboard).
	mux.HandleFunc("GET /api/v1/providers", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"providers": cat.Providers})
	})

	// Marketplace catalog with full model detail (dashboard).
	mux.HandleFunc("GET /api/v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"models": cat.ActiveModels()})
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