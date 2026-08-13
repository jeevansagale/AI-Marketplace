// Marketplace API Gateway — Hackathon v2.
// Implements the HTTP 402 Payment Required loop:
//   1. Client → POST /v1/chat/completions (no proof) → 402 + X-402-Payment-Required
//   2. Client → retry with X-402-Payment-Proof → validate → bill → stream to provider
// Plus the v2 wallet/session API:
//   - GET  /api/catalog                 marketplace catalog (INR pricing)
//   - GET  /api/wallet                  current/reserved/available balance
//   - GET  /api/wallet/transactions     user transaction history
//   - POST /api/topup                   create payment order (payment abstraction)
//   - POST /api/topup/verify            verify callback and credit wallet
//   - POST /api/sessions                create session reservation
//   - POST /api/sessions/{id}/stream    SSE stream with metering + settlement + refund
//   - GET  /api/sessions                session list for wallet
//   - GET  /api/ledger                  immutable ledger (read-only)
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

	"github.com/joho/godotenv"
	"github.com/x402-marketplace/catalog"
	"github.com/x402-marketplace/gateway/internal/ledger"
	"github.com/x402-marketplace/gateway/internal/payments"
	"github.com/x402-marketplace/gateway/internal/provider"
	"github.com/x402-marketplace/gateway/internal/session"
	"github.com/x402-marketplace/gateway/internal/settlement"
	"github.com/x402-marketplace/gateway/internal/store"
	"github.com/x402-marketplace/gateway/internal/wallet"
	"github.com/x402-marketplace/gateway/internal/x402"
)

const (
	// HeaderBalance reports remaining channel balance after billing.
	headerBalance = "X-402-Balance"
	// headerSpent reports what the request cost.
	headerSpent = "X-402-Spent"

	// Demo flat-rate per inference call (micro-USDC). Kept for the legacy
	// x402 channel flow; new sessions use INR reservation pricing.
	costPerCall uint64 = 2500

	challengeTTL  = 2 * time.Minute
	nonceTTL      = 5 * time.Minute
	network       = "algorand-testnet"
	escrowAccount = "DEMOWALLET000000000000000000000000000000000"
)

var requestID atomic.Uint64

type gateway struct {
	st      *store.Store
	logger  *log.Logger
	cat     catalog.Catalog
	wallets *wallet.Service
	ledger  *ledger.Ledger
	sess    *session.Service
	pay     *payments.DemoProvider
}

func main() {
	logger := log.New(os.Stdout, "[gateway] ", log.LstdFlags)

	// Load .env automatically on local startup.
	_ = godotenv.Load()            // Try current directory
	_ = godotenv.Load("../.env")   // Try parent directory
	_ = godotenv.Load("../../.env") // Try grandparent directory

	logger.Println("✓ Loaded .env (if present)")

	addr := envOr("OMNI_HTTP_ADDR", ":8080")

	st := store.New()
	st.SeedDemo()

	cat := catalog.Default()
	lg := ledger.New()

	// Seed the demo INR wallet (₹100.00).
	wallets := wallet.NewService(map[string]int64{wallet.DemoWalletID: 100_00})

	geminiKey := os.Getenv("OMNI_GEMINI_API_KEY")
	if geminiKey == "" {
		geminiKey = os.Getenv("GEMINI_API_KEY")
	}
	openAIKey := os.Getenv("OMNI_OPENAI_API_KEY")
	if openAIKey == "" {
		openAIKey = os.Getenv("OPENAI_API_KEY")
	}

	if geminiKey == "" {
		logger.Fatal("FATAL:\nOMNI_GEMINI_API_KEY is missing.\n\nCreate a .env file or export the environment variable.")
	} else {
		logger.Println("✓ Gemini API key detected")
	}

	if openAIKey == "" {
		logger.Fatal("FATAL:\nOMNI_OPENAI_API_KEY is missing.\n\nCreate a .env file or export the environment variable.")
	} else {
		logger.Println("✓ OpenAI API key detected")
	}

	reg := provider.NewRegistry(cat, openAIKey, geminiKey)
	settle := settlement.NewInternalDriver(lg)
	sess := session.NewService(wallets, lg, reg, cat, settle)
	pay := payments.NewDemoProvider()

	g := &gateway{st: st, logger: logger, cat: cat, wallets: wallets, ledger: lg, sess: sess, pay: pay}

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

	// Legacy OpenAI-compatible surface (402 flow preserved).
	mux.HandleFunc("GET /v1/models", g.handleModels)
	mux.HandleFunc("POST /v1/chat/completions", g.handleChatCompletions)

	// v2 API surface.
	mux.HandleFunc("GET /api/catalog", g.handleCatalog)
	mux.HandleFunc("GET /api/wallet", g.handleWallet)
	mux.HandleFunc("GET /api/wallet/transactions", g.handleWalletTransactions)
	mux.HandleFunc("POST /api/topup", g.handleTopUp)
	mux.HandleFunc("POST /api/topup/verify", g.handleTopUpVerify)
	mux.HandleFunc("POST /api/sessions", g.handleSessionCreate)
	mux.HandleFunc("POST /api/sessions/{id}/stream", g.handleSessionStream)
	mux.HandleFunc("GET /api/sessions", g.handleSessionList)
	mux.HandleFunc("GET /api/ledger", g.handleLedger)

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

// ---- v2 API ----

// handleCatalog returns the marketplace catalog with INR pricing.
func (g *gateway) handleCatalog(w http.ResponseWriter, r *http.Request) {
	models := g.cat.ActiveModels()
	for i := range models {
		models[i].Description = strings.TrimSpace(models[i].Description)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"providers": g.cat.Providers,
		"models":    models,
	})
}

func (g *gateway) handleWallet(w http.ResponseWriter, r *http.Request) {
	bal, err := g.wallets.Balance(wallet.DemoWalletID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, bal)
}

func (g *gateway) handleWalletTransactions(w http.ResponseWriter, r *http.Request) {
	tx, err := g.wallets.Transactions(wallet.DemoWalletID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"transactions": tx})
}

// handleTopUp creates a payment order through the payment abstraction.
func (g *gateway) handleTopUp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AmountINR float64 `json:"amount_inr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	amountPaise := int64(req.AmountINR * 100)
	if amountPaise <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New("amount_inr must be positive"))
		return
	}
	order, err := g.pay.CreateOrder(amountPaise, wallet.DemoWalletID)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"order_id":     order.ID,
		"amount_paise": order.AmountPaise,
		"status":       order.Status,
		"checkout_url": order.Checkout,
	})
}

// handleTopUpVerify processes the simulated payment webhook. The wallet is
// credited only after the provider verifies the order.
func (g *gateway) handleTopUpVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	ok, err := g.pay.Verify(req.OrderID, []byte("webhook:"+req.OrderID))
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusUnauthorized, payments.ErrInvalidCallback)
		return
	}
	// Wallet credited only after successful verification.
	// Look up the order amount from the provider (never trust client input).
	_amount := g.orderAmount(req.OrderID)
	if _amount <= 0 {
		writeErr(w, http.StatusBadRequest, errors.New("unknown order"))
		return
	}
	if err := g.wallets.TopUp(wallet.DemoWalletID, _amount, "topup:"+req.OrderID); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	g.ledger.Append(ledger.Entry{
		Kind:      ledger.KindTopUp,
		WalletID:  wallet.DemoWalletID,
		Amount:    _amount,
		Reference: req.OrderID,
		Status:    "credited",
	})
	writeJSON(w, http.StatusOK, map[string]any{"status": "verified", "credited_paise": _amount})
}

func (g *gateway) orderAmount(orderID string) int64 {
	// Demo provider stores order amounts; a real integration would query
	// the payment provider API.
	return g.pay.OrderAmount(orderID)
}

// handleSessionCreate reserves wallet funds for an estimated session cost.
func (g *gateway) handleSessionCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Model == "" {
		writeErr(w, http.StatusBadRequest, errors.New("model is required"))
		return
	}
	slug := catalog.NormalizeModelSlug(req.Model)
	sess, err := g.sess.Create(wallet.DemoWalletID, slug, req.MaxTokens)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// handleSessionStream streams provider tokens as SSE, then settles and refunds.
func (g *gateway) handleSessionStream(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Messages  []provider.Message `json:"messages"`
		MaxTokens int                `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	writeSSE := func(ev string) error {
		_, err := fmt.Fprint(w, ev+"\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		return err
	}

	// Emit provider chunks verbatim (OpenAI-compatible SSE).
	sum, err := g.sess.Stream(r.Context(), id, req.Messages, req.MaxTokens, func(chunk string) error {
		return writeSSE(strings.TrimSuffix(strings.TrimSuffix(chunk, "\n"), "\n"))
	})
	if err != nil {
		_ = writeSSE(`data: {"error":"` + strings.ReplaceAll(err.Error(), `"`, `\"`) + `"}`)
		return
	}
	b, _ := json.Marshal(map[string]any{
		"summary": sum,
		"usage":   sum,
	})
	_ = writeSSE("data: " + string(b))
	_ = writeSSE("data: [DONE]")
}

func (g *gateway) handleSessionList(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"sessions": g.sess.List(wallet.DemoWalletID)})
}

func (g *gateway) handleLedger(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": g.ledger.List(),
		"len":     g.ledger.Len(),
	})
}

// ---- Legacy x402 surface (preserved unchanged) ----

// handleModels lists marketplace models (OpenAI-compatible).
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

	w.Header().Set(headerSpent, fmt.Sprintf("%d", costPerCall))

	// Decode request to stream from real provider
	var req struct {
		Model     string             `json:"model"`
		Messages  []provider.Message `json:"messages"`
		MaxTokens int                `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
		return
	}

	slug := catalog.NormalizeModelSlug(req.Model)
	p, m, err := g.cat.ProviderForModel(slug)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":%q}}`, err.Error()), http.StatusBadRequest)
		return
	}

	ad, err := g.sess.ProviderRegistry().Select(p)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":{"message":%q}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if req.MaxTokens <= 0 {
		req.MaxTokens = 512
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	writeSSE := func(chunk string) error {
		_, err := fmt.Fprint(w, chunk)
		if flusher != nil {
			flusher.Flush()
		}
		return err
	}

	_, _ = ad.Stream(r.Context(), provider.Request{
		Model:     m.Slug,
		Messages:  req.Messages,
		MaxTokens: req.MaxTokens,
	}, writeSSE)
	_ = writeSSE("data: [DONE]\n\n")
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

	if !g.st.ClaimNonce(p.Nonce, nonceTTL) {
		return errors.New("nonce replay detected")
	}

	ch, ok := g.st.GetChannel(p.ChannelID)
	if !ok {
		return errors.New("unknown channel")
	}

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

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": err.Error(),
			"status":  status,
		},
	})
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
