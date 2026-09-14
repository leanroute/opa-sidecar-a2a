// Package common holds test-only ed25519 keypairs and chain-building
// helpers shared between the planner and executor demos.
//
// The keys here are hardcoded for a stable, reproducible demo. NEVER use
// them anywhere except this example.
package common

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
)

// Principal identifies one of the three agents in this demo.
type Principal struct {
	ID         string
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// Deterministic seeds so runs are reproducible and the sidecar's trust
// dir doesn't have to be regenerated between runs. Each seed is exactly
// ed25519.SeedSize (32) bytes — the readable label is padded/truncated
// by seedFromLabel().
var (
	userSeed     = seedFromLabel("USER_SEED_DEMO_ONLY_DO_NOT_USE_IN_PROD")
	plannerSeed  = seedFromLabel("PLANNER_SEED_DEMO_ONLY_DO_NOT_USE_IN_PROD")
	executorSeed = seedFromLabel("EXECUTOR_SEED_DEMO_ONLY_DO_NOT_USE_IN_PROD")
)

// seedFromLabel returns a deterministic 32-byte seed for a given label.
// Copies the label bytes into a 32-byte buffer, truncating or zero-padding
// as needed. The result is stable across runs and platforms so the sidecar's
// trust dir never drifts.
func seedFromLabel(label string) []byte {
	b := make([]byte, ed25519.SeedSize) // 32
	copy(b, label)
	return b
}

// NewPrincipal derives a Principal from a fixed seed. Test-only.
func NewPrincipal(id string, seed []byte) Principal {
	priv := ed25519.NewKeyFromSeed(seed)
	return Principal{
		ID:         id,
		PublicKey:  priv.Public().(ed25519.PublicKey),
		PrivateKey: priv,
	}
}

// User, Planner, Executor return the three principals used by the demo.
func User() Principal     { return NewPrincipal("did:web:user.example.com", userSeed) }
func Planner() Principal  { return NewPrincipal("did:web:planner.example.com", plannerSeed) }
func Executor() Principal { return NewPrincipal("did:web:executor.example.com", executorSeed) }

// TrustPEM emits a PEM-encoded ed25519 public key that the sidecar can
// load from its trust dir.
func TrustPEM(p Principal) []byte {
	block := &pem.Block{
		Type:    "PUBLIC KEY",
		Headers: map[string]string{"kid": p.ID},
		Bytes:   p.PublicKey,
	}
	return pem.EncodeToMemory(block)
}

// RandomKey returns a fresh keypair. Used to simulate an attacker who
// forges a signature with a key the sidecar doesn't trust.
func RandomKey() (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("ed25519 keygen: %v", err))
	}
	return pub, priv
}

// LookupPublicKey returns the ed25519 public key for a demo principal ID.
// Only knows about the three hardcoded demo principals. Real deployments
// would consult the sidecar's trust dir or an IdP.
func LookupPublicKey(id string) ed25519.PublicKey {
	switch id {
	case User().ID:
		return User().PublicKey
	case Planner().ID:
		return Planner().PublicKey
	case Executor().ID:
		return Executor().PublicKey
	}
	return nil
}

