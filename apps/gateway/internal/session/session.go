// Package session implements the deterministic session reservation lifecycle.
//
// Workflow: User creates session → Estimated cost is reserved from wallet →
// Session moves to "reserved" → Stream/Image request executes atomically →
// Session moves to "streaming" / "generating" → Actual cost is metered & charged →
// Session reaches "settled" / "closed".
//
// A closed session can NEVER be reservable or re-streamed again.
package session

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/x402-marketplace/catalog"
	"github.com/x402-marketplace/gateway/internal/ledger"
	"github.com/x402-marketplace/gateway/internal/provider"
	"github.com/x402-marketplace/gateway/internal/settlement"
	"github.com/x402-marketplace/gateway/internal/wallet"
)

// Stage represents a deterministic session-timeline state.
type Stage string

const (
	StageRequested  Stage = "requested"
	StageAuthorized Stage = "authorized" // wallet payment authorization
	StageReserved   Stage = "reserved"   // wallet hold created
	StageStreaming  Stage = "streaming"  // provider streaming text
	StageGenerating Stage = "generating" // provider generating image
	StageCompleted  Stage = "completed"  // stream / generation finished
	StageSettled    Stage = "settled"    // provider payout recorded
	StageRefunded   Stage = "refunded"   // unused reserve returned
	StageClosed     Stage = "closed"     // session finalized, immutable
	StageFailed     Stage = "failed"     // stream/charge failure, reserve released
)

// ErrSessionClosed is returned when an operation targets a completed or closed session.
var ErrSessionClosed = errors.New("session: session is closed and cannot be modified")

// Event is one timeline step with a human-readable label.
type Event struct {
	Stage Stage     `json:"stage"`
	Label string    `json:"label"`
	At    time.Time `json:"at"`
	Note  string    `json:"note,omitempty"`
}

// Summary is returned on completion for the client.
type Summary struct {
	SessionID    string `json:"session_id"`
	Reserved     int64  `json:"reserved_paise"`
	ActualCost   int64  `json:"actual_cost_paise"`
	Refunded     int64  `json:"refunded_paise"`
	PromptTokens int    `json:"prompt_tokens"`
	Completion   int    `json:"completion_tokens"`
	LatencyMS    int64  `json:"latency_ms"`
	ImageURL     string `json:"image_url,omitempty"`
}

// Service owns session state, concurrency locks, and lifecycle transitions.
type Service struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	next     int

	wallet *wallet.Service
	ledger *ledger.Ledger
	reg    *provider.Registry
	settle settlement.Driver
	cat    catalog.Catalog
}

// Session is a live conversation or generation record.
type Session struct {
	ID           string         `json:"id"`
	WalletID     string         `json:"wallet_id"`
	ModelID      string         `json:"model_id"`
	ModelSlug    string         `json:"model_slug"`
	ModelName    string         `json:"model_name"`
	ProviderID   string         `json:"provider_id"`
	ProviderName string         `json:"provider_name"`
	Reservation  string         `json:"reservation_id"`
	Stage        Stage          `json:"stage"`
	Estimated    int64          `json:"estimated_paise"`
	ActualCost   int64          `json:"actual_cost_paise"`
	Refunded     int64          `json:"refunded_paise"`
	Timeline     []Event        `json:"timeline"`
	Usage        provider.Usage `json:"usage"`
	ImageURL     string         `json:"image_url,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// NewService wires the session engine to wallet, ledger, provider registry, catalog, and settlement driver.
func NewService(w *wallet.Service, l *ledger.Ledger, reg *provider.Registry, cat catalog.Catalog, settle settlement.Driver) *Service {
	return &Service{
		sessions: map[string]*Session{},
		wallet:   w,
		ledger:   l,
		reg:      reg,
		settle:   settle,
		cat:      cat,
	}
}

// ProviderRegistry returns the underlying provider registry.
func (s *Service) ProviderRegistry() *provider.Registry {
	return s.reg
}

// Create validates the model, estimates cost accurately based on prompt size,
// reserves wallet funds, and returns a new session.
func (s *Service) Create(walletID, modelSlug string, maxTokens int, estimatedPromptTokens ...int) (*Session, error) {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	promptTokens := 128
	if len(estimatedPromptTokens) > 0 && estimatedPromptTokens[0] > 0 {
		promptTokens = estimatedPromptTokens[0]
	}

	prov, mdl, err := s.cat.ProviderForModel(modelSlug)
	if err != nil {
		return nil, err
	}

	estimated := mdl.EstimateSessionCost(promptTokens, maxTokens)
	resID, err := s.wallet.Reserve(walletID, estimated, "session:"+modelSlug)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.next++
	id := fmt.Sprintf("ses_%d", s.next)
	now := time.Now().UTC()
	sess := &Session{
		ID:           id,
		WalletID:     walletID,
		ModelID:      mdl.ID,
		ModelSlug:    mdl.Slug,
		ModelName:    mdl.Name,
		ProviderID:   prov.ID,
		ProviderName: prov.Name,
		Reservation:  resID,
		Stage:        StageReserved,
		Estimated:    estimated,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	sess.push(StageRequested, "Request received", "model="+mdl.Name)
	sess.push(StageAuthorized, "Wallet authorized", catalog.FormatINR(estimated)+" reserved")
	sess.push(StageReserved, "Reservation created", resID)
	s.sessions[id] = sess

	s.ledger.Append(ledger.Entry{
		WalletID:    walletID,
		Kind:        ledger.KindReserve,
		Amount:      -estimated,
		Reservation: resID,
		ProviderID:  prov.ID,
		ModelID:     mdl.ID,
		Reference:   id,
		Status:      "reserved",
	})
	return s.snapshotLocked(sess), nil
}

// Stream executes text streaming using atomic state transition.
func (s *Service) Stream(ctx context.Context, sessionID string, messages []provider.Message, maxTokens int, emit func(chunk string) error) (*Summary, error) {
	sess, err := s.transitionStage(sessionID, StageReserved, StageStreaming, "Streaming", "")
	if err != nil {
		return nil, err
	}

	prov, mdl, err := s.cat.ProviderForModel(sess.ModelSlug)
	if err != nil {
		s.fail(sessionID, err)
		return nil, err
	}
	adapter, err := s.reg.Select(prov)
	if err != nil {
		s.fail(sessionID, err)
		return nil, err
	}

	usage, err := adapter.Stream(ctx, provider.Request{
		Model:     mdl.Slug,
		Messages:  messages,
		MaxTokens: maxTokens,
	}, emit)
	if err != nil {
		// Attempt automatic failover to another active provider if primary quota is exhausted
		if pe, ok := err.(*provider.ProviderError); ok && (pe.IsQuotaExhausted || pe.Type == "quota_exceeded") {
			if fbProv, fbMdl, fbErr := s.cat.FallbackProvider(prov.Kind, mdl.Category); fbErr == nil {
				if fbAdapter, selectErr := s.reg.Select(fbProv); selectErr == nil {
					s.pushEvent(sessionID, StageStreaming, "Provider Failover", fmt.Sprintf("Primary %s quota exhausted. Failing over to %s (%s)", prov.Name, fbProv.Name, fbMdl.Name))
					fbUsage, fbStreamErr := fbAdapter.Stream(ctx, provider.Request{
						Model:     fbMdl.Slug,
						Messages:  messages,
						MaxTokens: maxTokens,
					}, emit)
					if fbStreamErr == nil {
						s.updateSessionProvider(sessionID, fbProv, fbMdl)
						return s.complete(sessionID, fbUsage, "")
					}
				}
			}
		}
		s.fail(sessionID, err)
		return nil, err
	}

	return s.complete(sessionID, usage, "")
}

// GenerateImage executes image generation for a session.
func (s *Service) GenerateImage(ctx context.Context, sessionID string, prompt string) (*Summary, error) {
	sess, err := s.transitionStage(sessionID, StageReserved, StageGenerating, "Generating Image", "")
	if err != nil {
		return nil, err
	}

	prov, mdl, err := s.cat.ProviderForModel(sess.ModelSlug)
	if err != nil {
		s.fail(sessionID, err)
		return nil, err
	}
	adapter, err := s.reg.Select(prov)
	if err != nil {
		s.fail(sessionID, err)
		return nil, err
	}

	result, usage, err := adapter.GenerateImage(ctx, provider.ImageRequest{
		Model:  mdl.Slug,
		Prompt: prompt,
	})
	if err != nil {
		s.fail(sessionID, err)
		return nil, err
	}

	imgURL := result.URL
	if imgURL == "" && result.B64JSON != "" {
		imgURL = "data:image/png;base64," + result.B64JSON
	}

	return s.complete(sessionID, usage, imgURL)
}

// transitionStage performs a thread-safe atomic stage transition under a single write lock.
func (s *Service) transitionStage(id string, expectedStage, targetStage Stage, label, note string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("session: unknown session " + id)
	}

	if sess.Stage == StageClosed || sess.Stage == StageSettled || sess.Stage == StageFailed {
		return nil, fmt.Errorf("session %s is closed (stage %s): %w", id, sess.Stage, ErrSessionClosed)
	}

	if sess.Stage != expectedStage {
		return nil, fmt.Errorf("session %s is not reservable (stage %s)", id, sess.Stage)
	}

	sess.Stage = targetStage
	sess.push(targetStage, label, note)
	sess.UpdatedAt = time.Now().UTC()
	return s.snapshotLocked(sess), nil
}

// complete charges actual cost, refunds remainder, records ledger + settlement, and closes the session.
func (s *Service) complete(sessionID string, usage provider.Usage, imageURL string) (*Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil, errors.New("session: unknown session " + sessionID)
	}

	mdl, ok := s.cat.FindModel(sess.ModelSlug)
	if !ok {
		mdl = catalog.Model{}
	}

	actual := mdl.PriceBase +
		int64(usage.PromptTokens)*mdl.PricePrompt/1000 +
		int64(usage.CompletionTokens)*mdl.PriceCompletion/1000
	if actual > sess.Estimated || actual <= 0 {
		if sess.Estimated > 0 {
			actual = sess.Estimated
		} else {
			actual = 10 // minimum 10 paise
		}
	}

	refund, err := s.wallet.Charge(sess.WalletID, sess.Reservation, actual)
	if err != nil {
		s.failLocked(sess, err)
		return nil, err
	}

	sess.ActualCost = actual
	sess.Refunded = refund
	sess.Usage = usage
	sess.ImageURL = imageURL
	sess.push(StageCompleted, "Completed", "charged "+catalog.FormatINR(actual))
	sess.Stage = StageCompleted

	s.ledger.Append(ledger.Entry{
		WalletID:    sess.WalletID,
		Kind:        ledger.KindSettle,
		Amount:      -actual,
		Reservation: sess.Reservation,
		ProviderID:  sess.ProviderID,
		ModelID:     sess.ModelID,
		Reference:   sess.ID,
		Status:      "settled",
	})

	if refund > 0 {
		sess.push(StageRefunded, "Refund", catalog.FormatINR(refund)+" returned")
		s.ledger.Append(ledger.Entry{
			WalletID:    sess.WalletID,
			Kind:        ledger.KindRefund,
			Amount:      refund,
			Reservation: sess.Reservation,
			ProviderID:  sess.ProviderID,
			Reference:   sess.ID,
			Status:      "refunded",
		})
	}

	sess.push(StageSettled, "Settled", "provider payout recorded")
	sess.push(StageClosed, "Closed", "session finalized")
	sess.Stage = StageClosed
	sess.UpdatedAt = time.Now().UTC()

	_ = s.settle.Settle(sess.ProviderID, actual, sess.ID)

	return &Summary{
		SessionID:    sess.ID,
		Reserved:     sess.Estimated,
		ActualCost:   actual,
		Refunded:     refund,
		PromptTokens: usage.PromptTokens,
		Completion:   usage.CompletionTokens,
		LatencyMS:    usage.LatencyMS,
		ImageURL:     imageURL,
	}, nil
}

// fail performs thread-safe session failure handling.
func (s *Service) fail(sessionID string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	s.failLocked(sess, err)
}

func (s *Service) failLocked(sess *Session, err error) {
	if sess.Stage == StageClosed || sess.Stage == StageFailed {
		return
	}
	sess.push(StageFailed, "Failed", err.Error())
	sess.Stage = StageFailed
	sess.UpdatedAt = time.Now().UTC()
	_ = s.wallet.Refund(sess.WalletID, sess.Reservation)
}

func (s *Session) push(st Stage, label, note string) {
	s.Timeline = append(s.Timeline, Event{Stage: st, Label: label, At: time.Now().UTC(), Note: note})
}

func (s *Service) snapshotLocked(sess *Session) *Session {
	cp := *sess
	cp.Timeline = append([]Event(nil), sess.Timeline...)
	return &cp
}

// Get returns a thread-safe session snapshot.
func (s *Service) Get(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sess, ok := s.sessions[id]
	if !ok {
		return nil, errors.New("session: unknown session " + id)
	}
	return s.snapshotLocked(sess), nil
}

// List returns sessions for a wallet, newest first.
func (s *Service) List(walletID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []Session
	for _, sess := range s.sessions {
		if sess.WalletID == walletID {
			out = append(out, *s.snapshotLocked(sess))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Service) pushEvent(sessionID string, st Stage, label, note string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionID]; ok {
		sess.push(st, label, note)
	}
}

func (s *Service) updateSessionProvider(sessionID string, p catalog.Provider, m catalog.Model) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sess, ok := s.sessions[sessionID]; ok {
		sess.ProviderID = p.ID
		sess.ProviderName = p.Name
		sess.ModelID = m.ID
		sess.ModelSlug = m.Slug
		sess.ModelName = m.Name
	}
}
