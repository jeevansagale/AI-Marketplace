// Package session_test contains unit and integration tests for the session lifecycle.
//
// Unit tests (TestSessionLifecycle*, TestConcurrent*) use an in-memory mock
// provider and never make real HTTP calls, so they pass without any API keys.
//
// Integration tests (TestIntegration*) require a real Gemini API key set via
// GEMINI_API_KEY or OMNI_GEMINI_API_KEY environment variables. They are
// automatically skipped when no key is available.
package session_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/x402-marketplace/catalog"
	"github.com/x402-marketplace/gateway/internal/ledger"
	"github.com/x402-marketplace/gateway/internal/provider"
	"github.com/x402-marketplace/gateway/internal/session"
	"github.com/x402-marketplace/gateway/internal/settlement"
	"github.com/x402-marketplace/gateway/internal/wallet"
)

// ─────────────────────────────────────────────────────────────────────────────
// Mock provider — fully in-memory, no HTTP, deterministic.
// ─────────────────────────────────────────────────────────────────────────────

type mockProvider struct {
	kind catalog.ProviderKind
	// streamErr, if non-nil, causes Stream() to return this error.
	streamErr error
	// imageErr, if non-nil, causes GenerateImage() to return this error.
	imageErr error
}

func (m *mockProvider) Kind() catalog.ProviderKind { return m.kind }

func (m *mockProvider) Stream(_ context.Context, req provider.Request, emit provider.StreamFunc) (provider.Usage, error) {
	if m.streamErr != nil {
		return provider.Usage{}, m.streamErr
	}
	// Emit a small deterministic response and return real-looking usage.
	chunks := []string{"Hello, ", "world", "!"}
	for _, c := range chunks {
		if err := emit("data: {\"choices\":[{\"delta\":{\"content\":" + fmt.Sprintf("%q", c) + "}}]}\n\n"); err != nil {
			return provider.Usage{}, err
		}
	}
	return provider.Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
		LatencyMS:        int64(10 * time.Millisecond),
		Requests:         1,
	}, nil
}

func (m *mockProvider) GenerateImage(_ context.Context, req provider.ImageRequest) (provider.ImageResult, provider.Usage, error) {
	if m.imageErr != nil {
		return provider.ImageResult{}, provider.Usage{}, m.imageErr
	}
	return provider.ImageResult{URL: "https://example.com/generated-test-image.png"},
		provider.Usage{PromptTokens: 5, CompletionTokens: 1024, TotalTokens: 1029, LatencyMS: 50, Requests: 1},
		nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Setup helpers
// ─────────────────────────────────────────────────────────────────────────────

// newMockService creates a fully wired Service backed by in-memory mock providers.
// No real API keys or network are needed.
func newMockService() *session.Service {
	cat := catalog.Default()
	lg := ledger.New()
	wallets := wallet.NewService(map[string]int64{wallet.DemoWalletID: 1000_00})
	reg := provider.NewRegistryFromProviders(map[catalog.ProviderKind]provider.Provider{
		catalog.ProviderKindGemini: &mockProvider{kind: catalog.ProviderKindGemini},
		catalog.ProviderKindOpenAI: &mockProvider{kind: catalog.ProviderKindOpenAI},
	})
	settle := settlement.NewInternalDriver(lg)
	return session.NewService(wallets, lg, reg, cat, settle)
}

// loadEnv attempts to load a .env file by walking up from cwd to project root.
func loadEnv() {
	dir, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		envPath := dir + "/.env"
		if _, err := os.Stat(envPath); err == nil {
			_ = godotenv.Load(envPath)
			return
		}
		dir = dir + "/.."
	}
}

// realGeminiKey returns the Gemini API key from env, or "" if absent.
// The key must be a direct Gemini API key (AIza…) not a proxy credential.
func realGeminiKey() string {
	loadEnv()
	for _, name := range []string{"GEMINI_API_KEY", "OMNI_GEMINI_API_KEY"} {
		if k := os.Getenv(name); k != "" {
			return k
		}
	}
	return ""
}

// newLiveService creates a Service backed by the real Gemini HTTP adapter.
func newLiveService(t *testing.T) *session.Service {
	t.Helper()
	geminiKey := realGeminiKey()
	if geminiKey == "" {
		t.Skip("no real Gemini API key found in env (GEMINI_API_KEY / OMNI_GEMINI_API_KEY) — skipping integration test")
	}
	cat := catalog.Default()
	lg := ledger.New()
	wallets := wallet.NewService(map[string]int64{wallet.DemoWalletID: 1000_00})
	reg := provider.NewRegistry(cat, "", geminiKey)
	settle := settlement.NewInternalDriver(lg)
	return session.NewService(wallets, lg, reg, cat, settle)
}

// ─────────────────────────────────────────────────────────────────────────────
// Unit Tests — no network, always pass with mock provider
// ─────────────────────────────────────────────────────────────────────────────

// TestSessionLifecycleAndClosedImmutability verifies the full lifecycle:
//  1. Create → StageReserved
//  2. Stream → streaming completes, session is StageClosed
//  3. Re-streaming the closed session returns ErrSessionClosed
func TestSessionLifecycleAndClosedImmutability(t *testing.T) {
	svc := newMockService()

	// 1. Create Session
	sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash", 512, 10)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if sess.Stage != session.StageReserved {
		t.Errorf("Expected stage reserved, got %s", sess.Stage)
	}

	// 2. Stream Session (mock provider — no real HTTP)
	var chunks []string
	summary, err := svc.Stream(
		context.Background(),
		sess.ID,
		[]provider.Message{{Role: "user", Content: "Test"}},
		512,
		func(chunk string) error {
			chunks = append(chunks, chunk)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	if summary.SessionID != sess.ID {
		t.Errorf("Expected summary session ID %s, got %s", sess.ID, summary.SessionID)
	}
	if len(chunks) == 0 {
		t.Error("Expected at least one emitted chunk from mock provider")
	}

	// 3. Verify Session is now CLOSED (immutable terminal state)
	closedSess, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if closedSess.Stage != session.StageClosed {
		t.Errorf("Expected final stage closed, got %s", closedSess.Stage)
	}

	// 4. Attempting to re-stream a CLOSED session MUST return ErrSessionClosed
	_, err = svc.Stream(
		context.Background(),
		sess.ID,
		[]provider.Message{{Role: "user", Content: "Re-stream attempt"}},
		512,
		func(chunk string) error { return nil },
	)
	if err == nil {
		t.Fatal("Expected error when streaming closed session, got nil")
	}
	if !errors.Is(err, session.ErrSessionClosed) {
		t.Errorf("Expected ErrSessionClosed, got: %v", err)
	}
}

// TestConcurrentStreamRacePrevention verifies that when N goroutines race to
// stream the same session simultaneously, EXACTLY 1 succeeds and N-1 fail with
// ErrSessionClosed or a stage-transition error (not a data race).
func TestConcurrentStreamRacePrevention(t *testing.T) {
	svc := newMockService()

	sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash", 512, 10)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	const workers = 10
	var wg sync.WaitGroup
	errChan := make(chan error, workers)
	successChan := make(chan *session.Summary, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			sum, streamErr := svc.Stream(
				context.Background(),
				sess.ID,
				[]provider.Message{{Role: "user", Content: fmt.Sprintf("Worker %d", workerID)}},
				512,
				func(chunk string) error { return nil },
			)
			if streamErr != nil {
				errChan <- streamErr
			} else {
				successChan <- sum
			}
		}(i)
	}

	wg.Wait()
	close(errChan)
	close(successChan)

	var successes int
	for range successChan {
		successes++
	}
	var rejections int
	for range errChan {
		rejections++
	}

	if successes != 1 {
		t.Errorf("Expected EXACTLY 1 successful stream under concurrent access, got %d", successes)
	}
	if rejections != workers-1 {
		t.Errorf("Expected %d rejected concurrent attempts, got %d", workers-1, rejections)
	}
}

// TestImageSessionLifecycle verifies image generation lifecycle using mock provider.
func TestImageSessionLifecycle(t *testing.T) {
	svc := newMockService()

	sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash-image", 512)
	if err != nil {
		t.Fatalf("Create image session failed: %v", err)
	}

	summary, err := svc.GenerateImage(context.Background(), sess.ID, "A futuristic cyberpunk city at night")
	if err != nil {
		t.Fatalf("GenerateImage failed: %v", err)
	}
	if summary.SessionID != sess.ID {
		t.Errorf("Expected summary session ID %s, got %s", sess.ID, summary.SessionID)
	}
	if summary.ImageURL == "" {
		t.Error("Expected generated image URL in summary, got empty string")
	}

	finalSess, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get final session failed: %v", err)
	}
	if finalSess.Stage != session.StageClosed {
		t.Errorf("Expected final stage closed after image generation, got %s", finalSess.Stage)
	}
}

// TestStreamProviderError verifies that when the provider returns an error,
// the session is marked failed — not left in streaming stage.
func TestStreamProviderError(t *testing.T) {
	cat := catalog.Default()
	lg := ledger.New()
	wallets := wallet.NewService(map[string]int64{wallet.DemoWalletID: 1000_00})
	providerErr := errors.New("mock: upstream provider timeout")
	reg := provider.NewRegistryFromProviders(map[catalog.ProviderKind]provider.Provider{
		catalog.ProviderKindGemini: &mockProvider{kind: catalog.ProviderKindGemini, streamErr: providerErr},
	})
	svc := session.NewService(wallets, lg, reg, cat, settlement.NewInternalDriver(lg))

	sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash", 256, 10)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, err = svc.Stream(
		context.Background(),
		sess.ID,
		[]provider.Message{{Role: "user", Content: "Will fail"}},
		256,
		func(chunk string) error { return nil },
	)
	if err == nil {
		t.Fatal("Expected stream error from failing mock provider, got nil")
	}

	// Session should be in failed or closed stage after provider error.
	failedSess, getErr := svc.Get(sess.ID)
	if getErr != nil {
		t.Fatalf("Get failed after stream error: %v", getErr)
	}
	if failedSess.Stage != session.StageFailed && failedSess.Stage != session.StageClosed {
		t.Errorf("Expected stage failed or closed after provider error, got %s", failedSess.Stage)
	}
}

// TestMultipleIndependentSessions verifies that a new session can be created
// from the same wallet after a previous session closes — sessions are independent.
func TestMultipleIndependentSessions(t *testing.T) {
	svc := newMockService()

	for i := 0; i < 3; i++ {
		sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash", 128, 5)
		if err != nil {
			t.Fatalf("Create session %d failed: %v", i+1, err)
		}
		summary, err := svc.Stream(
			context.Background(),
			sess.ID,
			[]provider.Message{{Role: "user", Content: fmt.Sprintf("Session %d", i+1)}},
			128,
			func(chunk string) error { return nil },
		)
		if err != nil {
			t.Fatalf("Stream session %d failed: %v", i+1, err)
		}
		if summary.SessionID != sess.ID {
			t.Errorf("Session %d: mismatched summary ID", i+1)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration Tests — require a real direct Gemini API key (AIza…)
// Automatically skipped if no key is set in the environment.
// ─────────────────────────────────────────────────────────────────────────────

func TestIntegrationGeminiStream(t *testing.T) {
	svc := newLiveService(t) // skips if no key

	sess, err := svc.Create(wallet.DemoWalletID, "gemini-2.5-flash", 128, 3)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var output string
	summary, err := svc.Stream(
		context.Background(),
		sess.ID,
		[]provider.Message{{Role: "user", Content: "Reply with exactly: ok"}},
		128,
		func(chunk string) error {
			output += chunk
			return nil
		},
	)
	if err != nil {
		// 429 RESOURCE_EXHAUSTED is expected on the free tier (20 req/day limit).
		// Skip rather than fail so CI passes with a free-tier key.
		errStr := err.Error()
		if contains(errStr, "429") || contains(errStr, "RESOURCE_EXHAUSTED") || contains(errStr, "quota") {
			t.Skipf("Skipping: Gemini free-tier quota exhausted during integration test: %v", err)
		}
		t.Fatalf("Stream failed: %v", err)
	}
	if output == "" {
		t.Error("Expected non-empty streaming output from real Gemini API")
	}
	if summary.PromptTokens == 0 {
		t.Error("Expected non-zero prompt token count in summary")
	}

	finalSess, err := svc.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if finalSess.Stage != session.StageClosed {
		t.Errorf("Expected stage closed after integration stream, got %s", finalSess.Stage)
	}
}

// contains is a string containment helper for test assertions.
func contains(s, substr string) bool { return strings.Contains(s, substr) }
