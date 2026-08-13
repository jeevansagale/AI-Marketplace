// Package session implements the session reservation lifecycle that replaces
// per-request charging.
//
// Workflow: user chooses a model → estimated cost is reserved from the wallet →
// tokens stream → internal usage is metered → the actual cost is charged and the
// remainder refunded. Every stage is recorded on an immutable timeline and the
// ledger, and provider earnings flow through the settlement driver.
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

// Stage is a live session-timeline state.
type Stage string

const (
	StageRequested  Stage = "requested"
	StageAuthorized Stage = "authorized" // wallet payment authorization (402-inspired handshake)
	StageReserved   Stage = "reserved"   // wallet hold created
	StageStreaming  Stage = "streaming"  // provider streaming
	StageCompleted  Stage = "completed"  // stream finished
	StageSettled    Stage = "settled"    // provider payout recorded
	StageRefunded   Stage = "refunded"   // unused reserve returned
	StageFailed     Stage = "failed"     // stream/charge failure, reserve released
)

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
}

// Service owns session state and lifecycle.
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

// Session is a live conversation record.
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
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// NewService wires the session engine to the wallet, ledger, provider
// registry, catalog and settlement driver.
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

// Create validates the model, reserves the estimated cost, and returns a
// session the client can stream against.
func (s *Service) Create(walletID, modelSlug string, maxTokens int) (*Session, error) {
	if maxTokens <= 0 {
		maxTokens = 512
	}
	prov, mdl, err := s.cat.ProviderForModel(modelSlug)
	if err != nil {
		return nil, err
	}
	estimated := mdl.EstimateSessionCost(64, maxTokens)

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
	return s.snapshot(sess), nil
}

// Stream runs the provider stream and meters usage. On success the session is
// settled immediately: charged at actual cost, refunded the remainder, and the
// provider earnings recorded through the settlement driver.
func (s *Service) Stream(ctx context.Context, sessionID string, messages []provider.Message, maxTokens int, emit func(chunk string) error) (*Summary, error) {
	sess, err := s.stored(sessionID)
	if err != nil {
		return nil, err
	}
	if sess.Stage != StageReserved {
		return nil, fmt.Errorf("session %s is not reservable (stage %s)", sessionID, sess.Stage)
	}

	prov, mdl, err := s.cat.ProviderForModel(sess.ModelSlug)
	if err != nil {
		s.fail(sess, err)
		return nil, err
	}
	adapter, err := s.reg.Select(prov)
	if err != nil {
		s.fail(sess, err)
		return nil, err
	}

	s.setStage(sess, StageStreaming, "Streaming", string(adapter.Kind()))
	usage, err := adapter.Stream(ctx, provider.Request{
		Model:     mdl.Slug,
		Messages:  messages,
		MaxTokens: maxTokens,
	}, emit)
	if err != nil {
		s.fail(sess, err)
		return nil, err
	}

	return s.complete(sess, usage)
}

// complete charges actual cost, refunds remainder, records ledger + settlement.
func (s *Service) complete(sess *Session, usage provider.Usage) (*Summary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mdl, ok := s.cat.FindModel(sess.ModelSlug)
	if !ok {
		mdl = catalog.Model{}
	}
	actual := mdl.PriceBase +
		int64(usage.PromptTokens)*mdl.PricePrompt/1000 +
		int64(usage.CompletionTokens)*mdl.PriceCompletion/1000
	if actual > sess.Estimated {
		actual = sess.Estimated
	}

	refund, err := s.wallet.Charge(sess.WalletID, sess.Reservation, actual)
	if err != nil {
		s.failLocked(sess, err)
		return nil, err
	}

	sess.ActualCost = actual
	sess.Refunded = refund
	sess.Usage = usage
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
	sess.Stage = StageSettled
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
	}, nil
}

// stored returns the live session pointer for internal mutation.
func (s *Service) stored(id string) (*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sess, ok := s.sessions[id]; ok {
		return sess, nil
	}
	return nil, errors.New("session: unknown session " + id)
}

// snapshot returns a copy safe for concurrent reads.
func (s *Service) snapshot(sess *Session) *Session {
	cp := *sess
	cp.Timeline = append([]Event(nil), sess.Timeline...)
	return &cp
}

// Get returns a session snapshot.
func (s *Service) Get(id string) (*Session, error) {
	sess, err := s.stored(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot(sess), nil
}

// List returns sessions for a wallet, newest first (sorted by CreatedAt descending).
func (s *Service) List(walletID string) []Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Session
	for _, sess := range s.sessions {
		if sess.WalletID == walletID {
			out = append(out, *s.snapshot(sess))
		}
	}
	// Sort by CreatedAt descending — map iteration order is random.
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *Service) fail(sess *Session, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failLocked(sess, err)
}

func (s *Service) failLocked(sess *Session, err error) {
	sess.push(StageFailed, "Failed", err.Error())
	sess.Stage = StageFailed
	sess.UpdatedAt = time.Now().UTC()
	_ = s.wallet.Refund(sess.WalletID, sess.Reservation)
}

func (s *Session) push(st Stage, label, note string) {
	s.Timeline = append(s.Timeline, Event{Stage: st, Label: label, At: time.Now().UTC(), Note: note})
}

func (s *Service) setStage(sess *Session, st Stage, label, note string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess.Stage = st
	sess.push(st, label, note)
	sess.UpdatedAt = time.Now().UTC()
}
