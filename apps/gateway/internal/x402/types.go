// Package x402 implements the HTTP 402 Payment Required handshake.
// Wire format follows the API Specification: Base64-JSON challenge in
// X-402-Payment-Required, Base64-JSON proof in X-402-Payment-Proof.
package x402

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const (
	HeaderPaymentRequired = "X-402-Payment-Required"
	HeaderPaymentProof    = "X-402-Payment-Proof"

	DefaultUSDCASAID = 31566704
)

// Challenge is the server->client invoice returned with HTTP 402.
type Challenge struct {
	Schema           string   `json:"schema"`                     // "x402.invoice/1.0"
	Network          string   `json:"network"`                    // "algorand-testnet"
	Escrow           string   `json:"escrow"`                     // escrow account address
	AssetID          uint64   `json:"assetId"`                    // USDC ASA id
	Amount           uint64   `json:"amount"`                     // micro-USDC C_max
	Nonce            string   `json:"nonce"`                      // replay protection id
	ExpiresAt        string   `json:"expiresAt"`                  // RFC3339
	Metadata         Metadata `json:"metadata,omitempty"`         // model/channel context
	PaymentRecipient string   `json:"paymentRecipient,omitempty"` // provider wallet
}

type Metadata struct {
	Model     string `json:"model"`
	ChannelID string `json:"channelId"`
}

// Proof is the client->server payment proof returned in the retry.
type Proof struct {
	Schema    string `json:"schema"` // "x402.proof/1.0"
	Nonce     string `json:"nonce"`  // echoes challenge nonce
	ChannelID string `json:"channelId"`
	Sender    string `json:"sender"`    // consumer Algorand address
	Signature string `json:"signature"` // base64 Ed25519 sig over canonical bytes
	SignedAt  string `json:"signedAt"`  // RFC3339
	Hint      string `json:"hint"`      // "ed25519"
}

// EncodeChallenge base64-encodes the challenge JSON for the header.
func EncodeChallenge(c Challenge) (string, error) {
	b, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeChallenge parses a base64 X-402-Payment-Required header.
func DecodeChallenge(hdr string) (Challenge, error) {
	var c Challenge
	b, err := base64.StdEncoding.DecodeString(hdr)
	if err != nil {
		return c, fmt.Errorf("x402: bad challenge base64: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, fmt.Errorf("x402: bad challenge json: %w", err)
	}
	return c, nil
}

// EncodeProof base64-encodes the proof JSON for the header.
func EncodeProof(p Proof) (string, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// DecodeProof parses a base64 X-402-Payment-Proof header.
func DecodeProof(hdr string) (Proof, error) {
	var p Proof
	b, err := base64.StdEncoding.DecodeString(hdr)
	if err != nil {
		return p, fmt.Errorf("x402: bad proof base64: %w", err)
	}
	if err := json.Unmarshal(b, &p); err != nil {
		return p, fmt.Errorf("x402: bad proof json: %w", err)
	}
	return p, nil
}

// CanonicalBytes is the byte string the consumer signs:
// nonce || channelId || sender || amount || escrow || assetId (length-prefixed).
func CanonicalBytes(c Challenge, p Proof) []byte {
	var buf []byte
	for _, s := range []string{p.Nonce, p.ChannelID, p.Sender, fmt.Sprintf("%d", c.Amount), c.Escrow, fmt.Sprintf("%d", c.AssetID)} {
		buf = append(buf, byte(len(s)))
		buf = append(buf, s...)
	}
	return buf
}

// IsExpired returns true when the challenge is past its expiry.
func IsExpired(c Challenge, now time.Time) (bool, error) {
	if c.ExpiresAt == "" {
		return false, errors.New("x402: challenge missing expiresAt")
	}
	t, err := time.Parse(time.RFC3339, c.ExpiresAt)
	if err != nil {
		return false, fmt.Errorf("x402: bad expiresAt: %w", err)
	}
	return now.After(t), nil
}
