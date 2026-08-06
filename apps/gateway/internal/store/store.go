// Package store keeps 402 nonce replay protection and channel balances.
// Hackathon MVP: in-memory with TTL. Production: Redis (fast path) +
// Postgres nonce journal (authoritative, per spec §1.3 idempotency).
package store

import (
	"sync"
	"time"
)

type ChannelState struct {
	ChannelID  string
	Escrow     string
	Balance    uint64 // locked micro-USDC remaining
	Spent      uint64
	LatestNonce uint64
}

type Store struct {
	mu       sync.RWMutex
	nonces   map[string]time.Time // nonce -> expiry
	channels map[string]*ChannelState
}

func New() *Store {
	return &Store{
		nonces:   make(map[string]time.Time),
		channels: make(map[string]*ChannelState),
	}
}

// ClaimNonce atomically records a nonce. Returns false on replay within TTL.
func (s *Store) ClaimNonce(nonce string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if exp, ok := s.nonces[nonce]; ok && time.Now().Before(exp) {
		return false
	}
	s.nonces[nonce] = time.Now().Add(ttl)
	return true
}

// UpsertChannel stores channel state (idempotent).
func (s *Store) UpsertChannel(c *ChannelState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.channels[c.ChannelID] = c
}

func (s *Store) GetChannel(id string) (*ChannelState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.channels[id]
	if !ok || c == nil {
		return nil, false
	}
	cp := *c
	return &cp, true
}

// Debit attempts to spend amount from the channel. Atomic.
func (s *Store) Debit(channelID string, amount uint64) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.channels[channelID]
	if !ok {
		return 0, false
	}
	if amount > c.Balance {
		return c.Balance, false
	}
	c.Balance -= amount
	c.Spent += amount
	c.LatestNonce++
	return c.Balance, true
}

// SeedDemo populates the demo consumer channel for the hackathon demo.
func (s *Store) SeedDemo() {
	s.UpsertChannel(&ChannelState{
		ChannelID:  "40000000-0000-0000-0000-000000000001",
		Escrow:     "DEMOWALLET000000000000000000000000000000000",
		Balance:    100_000_000, // 100 USDC
		Spent:      4_300_000,
		LatestNonce: 42,
	})
}