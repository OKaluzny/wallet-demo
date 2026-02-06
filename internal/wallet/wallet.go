package wallet

import (
	"context"

	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// Generator defines the interface for address generation per network.
// Each network implements this to handle its own derivation logic.
type Generator interface {
	// Network returns which blockchain this generator supports
	Network() models.Network

	// GenerateFromSeed derives an address from HD seed bytes at the given index
	GenerateFromSeed(seed []byte, index uint32) (*models.DerivedAddress, error)
}

// Signer defines the interface for transaction signing.
// In production, this would delegate to HSM via PKCS#11 or cloud KMS API.
type Signer interface {
	// Sign signs a transaction and returns it with RawSigned populated
	Sign(ctx context.Context, tx *models.Transaction, privateKey []byte) (*models.Transaction, error)
}

// HSMSigner is a placeholder interface showing how HSM integration would look.
// In production: wraps PKCS#11 calls or cloud KMS (AWS CloudHSM, GCP Cloud KMS).
type HSMSigner interface {
	// SignWithHSM signs using a key reference (never exposing the private key)
	SignWithHSM(ctx context.Context, tx *models.Transaction, keyID string) (*models.Transaction, error)
}
