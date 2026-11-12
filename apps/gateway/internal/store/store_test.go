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
func TestIssuedChallengeNonceIsSingleUse(t *testing.T) {
	s := New()
	expiresAt := time.Now().Add(time.Minute)
	if !s.RegisterChallengeNonce("issued", expiresAt) {
		t.Fatal("expected challenge nonce registration to succeed")
	}
	if !s.ConsumeChallengeNonce("issued", time.Now()) {
		t.Fatal("expected issued nonce to be consumed")
	}
	if s.ConsumeChallengeNonce("issued", time.Now()) {
		t.Fatal("challenge nonce must not be reusable")
	}
}

func TestUnissuedChallengeNonceIsRejected(t *testing.T) {
	s := New()
	if s.ConsumeChallengeNonce("not-issued", time.Now()) {
		t.Fatal("unissued nonce must be rejected")
	}
}

func TestExpiredChallengeNonceIsRejected(t *testing.T) {
	s := New()
	expiresAt := time.Now().Add(-time.Second)
	if s.RegisterChallengeNonce("expired", time.Now().Add(-time.Minute)) {
		t.Fatal("already expired nonce must not be registered")
	}
	if s.ConsumeChallengeNonce("expired", expiresAt) {
		t.Fatal("expired nonce must not be consumed")
	}
}
