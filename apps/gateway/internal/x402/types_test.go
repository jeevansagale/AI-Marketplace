package x402

import (
	"strings"
	"testing"
	"time"
)

func TestChallengeRoundTrip(t *testing.T) {
	c := Challenge{
		Schema:    "x402.invoice/1.0",
		Network:   "algorand-testnet",
		Escrow:    "ESCROWADDR",
		AssetID:   31566704,
		Amount:    2500,
		Nonce:     "n-123",
		ExpiresAt: time.Now().Add(time.Minute).UTC().Format(time.RFC3339),
	}
	hdr, err := EncodeChallenge(c)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if hdr == "" {
		t.Fatal("empty header")
	}
	got, err := DecodeChallenge(hdr)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Nonce != c.Nonce || got.Amount != c.Amount {
		t.Fatalf("roundtrip mismatch: %+v vs %+v", got, c)
	}
}

func TestProofRoundTrip(t *testing.T) {
	p := Proof{Schema: "x402.proof/1.0", Nonce: "n-123", Sender: "SENDERADDR", Signature: "sig"}
	hdr, err := EncodeProof(p)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := DecodeProof(hdr)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Sender != p.Sender {
		t.Fatalf("mismatch: %+v", got)
	}
}

func TestDecodeGarbage(t *testing.T) {
	if _, err := DecodeChallenge("!!!not-json!!!"); err == nil {
		t.Fatal("expected error for garbage challenge")
	}
	if _, err := DecodeProof("###"); err == nil {
		t.Fatal("expected error for garbage proof")
	}
}

func TestCanonicalBytesDeterministic(t *testing.T) {
	c := Challenge{Amount: 100, Escrow: "ESC", AssetID: 31566704}
	p := Proof{Nonce: "n1", ChannelID: "ch1", Sender: "S"}
	a := CanonicalBytes(c, p)
	b := CanonicalBytes(c, p)
	if !strings.EqualFold(string(a), string(b)) {
		t.Fatal("canonical bytes not deterministic")
	}
}

func TestIsExpired(t *testing.T) {
	c := Challenge{ExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)}
	e, err := IsExpired(c, time.Now())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !e {
		t.Fatal("expected expired")
	}
}
