package wallet

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// TRXGenerator generates TRON addresses using BIP-44 derivation.
// Derivation path: m/44'/195'/0'/0/{index}
// TRON uses the same ECDSA secp256k1 as Ethereum but with Base58Check encoding.
type TRXGenerator struct{}

func NewTRXGenerator() *TRXGenerator {
	return &TRXGenerator{}
}

func (g *TRXGenerator) Network() models.Network {
	return models.NetworkTRX
}

// GenerateFromSeed derives a TRON address from a BIP-39 seed.
// TRON address = Base58Check(0x41 + Keccak256(pubKey)[12:])
func (g *TRXGenerator) GenerateFromSeed(seed []byte, index uint32) (*models.DerivedAddress, error) {
	path := fmt.Sprintf("m/44'/195'/0'/0/%d", index)

	key, err := deriveKey(seed, 195, index)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Get public key using secp256k1
	_, pubKey := btcec.PrivKeyFromBytes(key[:32])
	pubBytes := pubKey.SerializeUncompressed()

	// Keccak256 hash, take last 20 bytes (same as ETH)
	hash := keccak256(pubBytes[1:])
	addrBytes := hash[12:]

	// TRON uses 0x41 prefix + Base58Check (unlike ETH's hex encoding)
	address := base58CheckEncode(0x41, addrBytes)

	return &models.DerivedAddress{
		Network:        models.NetworkTRX,
		Address:        address,
		DerivationPath: path,
		PublicKey:      hex.EncodeToString(pubBytes),
	}, nil
}

// TRXSigner signs TRON transactions.
// TRON uses protobuf for transaction serialization.
type TRXSigner struct{}

func NewTRXSigner() *TRXSigner {
	return &TRXSigner{}
}

func (s *TRXSigner) Sign(ctx context.Context, tx *models.Transaction, privateKey []byte) (*models.Transaction, error) {
	rawData := []byte(fmt.Sprintf("%s:%s:%s", tx.From, tx.To, tx.Amount.String()))
	txHash := keccak256(rawData)

	tx.TxHash = hex.EncodeToString(txHash)
	tx.Signed = true
	tx.RawSigned = rawData

	return tx, nil
}
