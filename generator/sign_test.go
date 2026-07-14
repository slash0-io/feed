package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

const (
	testSeed = "8bd77a54826ea365f0161a341fd7b3ff5b3d5bde6bee73791d8100a12eef0f21"
)

func TestSignIndexVerifies(t *testing.T) {
	body := []byte(`{"schemaVersion":1}` + "\n")
	sig, err := signIndex(testSeed, body)
	if err != nil {
		t.Fatal(err)
	}
	keyid, b64, ok := strings.Cut(strings.TrimSpace(string(sig)), ":")
	if !ok {
		t.Fatalf("sig format: %q", sig)
	}
	seed, _ := hex.DecodeString(testSeed)
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	if keyid != keyID(pub) {
		t.Errorf("keyid = %s, want %s", keyid, keyID(pub))
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, body, raw) {
		t.Error("signature does not verify")
	}
	if ed25519.Verify(pub, append(body, ' '), raw) {
		t.Error("signature verified tampered body")
	}

	// Deterministic: byte-stable index bytes yield a byte-stable .sig, so
	// incremental republishes stay deploy-skippable.
	sig2, err := signIndex(testSeed, body)
	if err != nil {
		t.Fatal(err)
	}
	if string(sig) != string(sig2) {
		t.Error("signing is not deterministic")
	}
}

func TestSignIndexRejectsBadKey(t *testing.T) {
	for _, seed := range []string{"", "abc", "zz" + testSeed[2:]} {
		if _, err := signIndex(seed, []byte("x")); err == nil {
			t.Errorf("seed %q: want error", seed)
		}
	}
}
