package store

import (
	"testing"
	"time"
)

func TestClaimNonceReplay(t *testing.T) {
	s := New()
	if !s.ClaimNonce("n1", time.Minute) {
		t.Fatal("first claim should succeed")
	}
	if s.ClaimNonce("n1", time.Minute) {
		t.Fatal("replay within TTL must be rejected")
	}
}

func TestNonceTTLExpiry(t *testing.T) {
	s := New()
	if !s.ClaimNonce("n2", -time.Second) {
		t.Fatal("claim with expired TTL should succeed")
	}
	if !s.ClaimNonce("n2", time.Minute) {
		t.Fatal("re-claim after TTL expiry allowed")
	}
}

func TestDebit(t *testing.T) {
	s := New()
	s.UpsertChannel(&ChannelState{ChannelID: "ch", Balance: 100, LatestNonce: 0})
	bal, ok := s.Debit("ch", 30)
	if !ok || bal != 70 {
		t.Fatalf("expected 70, got %d ok=%v", bal, ok)
	}
	if _, ok := s.Debit("ch", 100); ok {
		t.Fatal("overdraw must fail")
	}
}
