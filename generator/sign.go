package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// Feed signing: an ed25519 detached signature over the exact bytes of
// v1/index.json, published as v1/index.json.sig. Service documents are
// covered transitively — the index carries their SHA-256, so one signature
// authenticates the whole tree. ed25519 is deterministic, which keeps the
// .sig byte-stable across incremental republishes of an unchanged index.
//
// The .sig format is "<keyid>:<base64 signature>\n", where keyid is the
// first 4 bytes (8 hex chars) of SHA-256 over the public key — consumers pin
// a keyid->pubkey map, making rotation an additive change.
//
// This signs at CI, so it defends the distribution layer (Pages/CDN
// tampering); it cannot defend against compromise of the repo or workflow
// that holds the key.

// signIndex produces the index.json.sig contents for indexBytes using a
// hex-encoded 32-byte ed25519 seed.
func signIndex(seedHex string, indexBytes []byte) ([]byte, error) {
	seed, err := hex.DecodeString(seedHex)
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("signing key must be %d hex-encoded bytes", ed25519.SeedSize)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, indexBytes)
	return []byte(keyID(priv.Public().(ed25519.PublicKey)) + ":" + base64.StdEncoding.EncodeToString(sig) + "\n"), nil
}

func keyID(pub ed25519.PublicKey) string {
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:4])
}

// printKeygen mints a fresh signing keypair for rotation (-keygen).
func printKeygen() error {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	fmt.Printf("seed  = %s   (Actions secret FEED_SIGNING_KEY; keep out of git)\n", hex.EncodeToString(priv.Seed()))
	fmt.Printf("pub   = %s   (pin in consumers)\n", hex.EncodeToString(pub))
	fmt.Printf("keyid = %s\n", keyID(pub))
	return nil
}
