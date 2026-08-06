// Marketplace API Gateway — Hackathon MVP.
// Implements the HTTP 402 Payment Required loop:
//   1. Client → POST /v1/chat/completions (no proof) → 402 + X-402-Payment-Required
//   2. Client → retry with X-402-Payment-Proof → validate → bill → stream to provider
// Wire format per API_Specification.md (Base64-JSON headers, RFC 9457 body).
package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/x402-marketplace/gateway/internal/mockprovider"
	"github.com/x402-marketplace/gateway/internal/store"
	"github.com/x402-marketplace/gateway/internal/x402"
)

const (
	// HeaderBalance reports remaining channel balance after billing.
	headerBalance = "X-402-Balance"
	// headerSpent reports what the request cost.
	headerSpent = "X-402-Spent"

	// Demo flat-rate per inference call (micro-USDC). Real pricing is
	// per-token with streaming reconciliation (M2, per Implementation Plan).
	costPerCall uint64 = 2500

	challengeTTL  = 2 * time.Minute
	nonceTTL      = 5 * time.Minute
	network       = "algorand-testnet"
	escrowAccount = "DEMOWALLET000000000000000000000000000000000"
)

var requestID atomic.Uint64

type gateway struct {
	st     *store.Store
	logger *log.Logger
}

func main() {
	addr := envOr("OMNI_HTTP_ADDR", ":8080")
	logger := log.New(os.Stdout, "[gateway] ", log.LstdFlags)

	st := store.New()
	st.SeedDemo()

	g := &gateway{st: st, logger: logger}

	mux := http.NewServeMux()

	// Liveness / readiness for orchestrators.
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"service": "gateway",
			"mode":    envOr("OMNI_MODE", "demo"),
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// OpenAI-compatible surface.
	mux.HandleFunc("GET /v1/models", g.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", g.handleChatCompletions)

	srv := &http.Server{
		Addr:              addr,
		Handler:           logMiddleware(logger, corsMiddleware(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("gateway listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Fatalf("server error: %v", err)
	}
}

// handleModels lists marketplace models (OpenAI-compatible).
// Catalog mirrors infra/docker/postgres/init/0001_init.sql seed.
func (g *gateway) handleModels(w http.ResponseWriter, _ *http.Request) {
	models := []string{
		"market-llama-3.3-70b-instruct",
		"market-llama-3.1-8b-instruct",
		"algoforge-deepseek-r1-32b",
		"tensorgrid-mixtral-8x7b",
		"market-qwen-2.5-72b",
		"visionforge-gpt-4o-vision-class",
		"visionforge-yolo-v11-ocr",
		"vectorhaven-text-embedding-3",
		"vectorhaven-bge-m3",
		"audiowave-whisper-large-v3",
	}
	data := make([]map[string]any, 0, len(models))
	for _, id := range models {
		data = append(data, map[string]any{
			"id":       id,
			"object":   "model",
			"owned_by": "marketplace",
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// handleChatCompletions is the 402-gated inference endpoint.
func (g *gateway) handleChatCompletions(w http.ResponseWriter, r *http.Request) {
	rid := requestID.Add(1)
	ctx := fmt.Sprintf("req-%06d", rid)
	g.logger.Printf("ctx=%s POST /v1/chat/completions", ctx)

	proofHdr := r.Header.Get(x402.HeaderPaymentProof)
	if proofHdr == "" {
		g.issueChallenge(w, ctx)
		return
	}

	if err := g.verifyAndBill(w, r, proofHdr, ctx); err != nil {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":        "https://x402.invalid/problems/payment-required",
			"title":       "Payment rejected",
			"status":      http.StatusPaymentRequired,
			"detail":      err.Error(),
			"instance":    ctx,
			"retry-after": "2",
		})
		return
	}

	// Billed — proxy to the provider (in-process for MVP; out-of-process
	// routing + streaming reconciliation is M2).
	w.Header().Set(headerSpent, fmt.Sprintf("%d", costPerCall))
	mockprovider.Handler().ServeHTTP(w, r)
}

// issueChallenge returns 402 with a fresh invoice.
func (g *gateway) issueChallenge(w http.ResponseWriter, ctx string) {
	nonce, err := randomNonce()
	if err != nil {
		g.logger.Printf("ctx=%s nonce error: %v", ctx, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ch := x402.Challenge{
		Schema:    "x402.invoice/1.0",
		Network:   network,
		Escrow:    escrowAccount,
		AssetID:   x402.DefaultUSDCASAID,
		Amount:    costPerCall,
		Nonce:     nonce,
		ExpiresAt: time.Now().Add(challengeTTL).UTC().Format(time.RFC3339),
		Metadata: x402.Metadata{
			Model: "market-llama-3.1-8b-instruct",
		},
	}

	hdr, err := x402.EncodeChallenge(ch)
	if err != nil {
		g.logger.Printf("ctx=%s encode challenge: %v", ctx, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	g.logger.Printf("ctx=%s issued 402 nonce=%s amount=%d", ctx, nonce, costPerCall)
	w.Header().Set(x402.HeaderPaymentRequired, hdr)
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusPaymentRequired)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://x402.invalid/problems/payment-required",
		"title":  "Payment Required",
		"status": http.StatusPaymentRequired,
		"detail": "Include X-402-Payment-Proof with an Algorand USDC payment to continue.",
	})
}

// verifyAndBill validates proof, claims the nonce, and debits the channel.
func (g *gateway) verifyAndBill(w http.ResponseWriter, r *http.Request, proofHdr, ctx string) error {
	p, err := x402.DecodeProof(proofHdr)
	if err != nil {
		return fmt.Errorf("invalid proof: %w", err)
	}
	if p.Schema != "x402.proof/1.0" {
		return errors.New("unsupported proof schema")
	}
	if p.Hint != "" && p.Hint != "ed25519" {
		return errors.New("unsupported signature hint")
	}

	// Replay protection: nonce must be unique within TTL.
	if !g.st.ClaimNonce(p.Nonce, nonceTTL) {
		return errors.New("nonce replay detected")
	}

	// Channel must exist with funds.
	ch, ok := g.st.GetChannel(p.ChannelID)
	if !ok {
		return errors.New("unknown channel")
	}

	// Demo verification: sender is signed but for the hackathon we verify
	// the channel relationship; signature verification lands with AlgoKit
	// key handling in M2 (Implementation Plan §5.1).
	if strings.TrimSpace(p.Sender) == "" {
		return errors.New("missing sender")
	}

	bal, ok := g.st.Debit(p.ChannelID, costPerCall)
	if !ok {
		return fmt.Errorf("insufficient balance: have %d, need %d", ch.Balance, costPerCall)
	}

	g.logger.Printf("ctx=%s billed channel=%s nonce=%s balance=%d", ctx, ch.ChannelID, p.Nonce, bal)
	w.Header().Set(headerBalance, fmt.Sprintf("%d", bal))
	return nil
}

func randomNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-402-Payment-Proof")
		w.Header().Set("Access-Control-Expose-Headers", "X-402-Payment-Required, X-402-Payment-Proof, X-402-Balance, X-402-Spent")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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