package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha512"
	"encoding/base32"
	"testing"
)

func makeTestAlgorandAddress(publicKey ed25519.PublicKey) string {
	checksum := sha512.Sum512_256(publicKey)
	addressBytes := append(append([]byte{}, publicKey...), checksum[len(checksum)-4:]...)
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(addressBytes)
}

func TestVerifyAlgorandSignature(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	address := makeTestAlgorandAddress(publicKey)
	payload := []byte("signed payment challenge")
	signature := ed25519.Sign(privateKey, payload)
	encodedSignature := base64.StdEncoding.EncodeToString(signature)

	if !verifyAlgorandSignature(address, encodedSignature, payload) {
		t.Fatal("valid Ed25519 proof should verify")
	}
	if verifyAlgorandSignature(address, encodedSignature, []byte("different challenge")) {
		t.Fatal("signature must not verify for a different payload")
	}
	if verifyAlgorandSignature(address, encodedSignature, nil) {
		t.Fatal("signature must not verify for an empty payload")
	}
}

func TestVerifyAlgorandSignatureRejectsInvalidAddress(t *testing.T) {
	if verifyAlgorandSignature("not-an-algorand-address", "invalid", []byte("payload")) {
		t.Fatal("invalid sender address must be rejected")
	}
}