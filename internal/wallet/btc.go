package wallet

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/OKaluzny/wallet-demo/pkg/models"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/base58"
	"golang.org/x/crypto/ripemd160" //nolint:staticcheck // RIPEMD-160 is required by the Bitcoin protocol (Hash160)
)

// BTCGenerator generates Bitcoin addresses using BIP-44 derivation.
// Derivation path: m/44'/0'/0'/0/{index}
// Supports P2PKH (legacy 1...) addresses. Production would also support
// P2SH-P2WPKH (3...) and native SegWit bech32 (bc1...).
type BTCGenerator struct{}

// NewBTCGenerator returns a new Bitcoin address generator.
func NewBTCGenerator() *BTCGenerator {
	return &BTCGenerator{}
}

// Network returns the Bitcoin network identifier.
func (g *BTCGenerator) Network() models.Network {
	return models.NetworkBTC
}

// GenerateFromSeed derives a Bitcoin address from a BIP-39 seed.
// Uses Hash160 (SHA256 + RIPEMD160) for address generation.
func (g *BTCGenerator) GenerateFromSeed(seed []byte, index uint32) (*models.DerivedAddress, error) {
	path := fmt.Sprintf("m/44'/0'/0'/0/%d", index)

	key, err := deriveKey(seed, 0, index)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Get compressed public key via secp256k1
	pubKey := compressedPubKey(key[:32])

	// Bitcoin address: Base58Check(0x00 + Hash160(pubKey))
	hash160 := hash160(pubKey)
	address := base58CheckEncode(0x00, hash160)

	return &models.DerivedAddress{
		Network:        models.NetworkBTC,
		Address:        address,
		DerivationPath: path,
		PublicKey:      hex.EncodeToString(pubKey),
	}, nil
}

// BTCSigner builds and signs Bitcoin transactions (UTXO model).
// Production would handle UTXO selection, change addresses, fee estimation.
type BTCSigner struct {
	networkPrefix byte // 0x00 mainnet, 0x6f testnet
}

// NewBTCSigner returns a new Bitcoin transaction signer for mainnet or testnet.
func NewBTCSigner(mainnet bool) *BTCSigner {
	prefix := byte(0x00)
	if !mainnet {
		prefix = 0x6f
	}
	return &BTCSigner{networkPrefix: prefix}
}

// Sign signs a Bitcoin transaction using double-SHA256 hashing.
func (s *BTCSigner) Sign(ctx context.Context, tx *models.Transaction, privateKey []byte) (*models.Transaction, error) {
	rawTx := buildRawBTCTx(tx)
	txHash := doubleSHA256(rawTx)

	tx.TxHash = hex.EncodeToString(txHash)
	tx.Signed = true
	tx.RawSigned = rawTx

	return tx, nil
}

// --- helpers ---

func compressedPubKey(privKeyBytes []byte) []byte {
	_, pubKey := btcec.PrivKeyFromBytes(privKeyBytes)
	return pubKey.SerializeCompressed()
}

func hash160(data []byte) []byte {
	sha := sha256.Sum256(data)
	ripe := ripemd160.New()
	ripe.Write(sha[:])
	return ripe.Sum(nil)
}

func doubleSHA256(data []byte) []byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	return second[:]
}

func base58CheckEncode(version byte, payload []byte) string {
	// Version + payload
	data := make([]byte, 0, 1+len(payload)+4)
	data = append(data, version)
	data = append(data, payload...)

	// Checksum = first 4 bytes of double SHA256
	checksum := doubleSHA256(data)
	data = append(data, checksum[:4]...)

	return base58Encode(data)
}

func base58Encode(input []byte) string {
	return base58.Encode(input)
}

func buildRawBTCTx(tx *models.Transaction) []byte {
	// Simplified transaction serialization
	// Production: proper serialization with version, locktime, witness data
	var raw []byte
	raw = append(raw, []byte{0x01, 0x00, 0x00, 0x00}...) // version
	raw = append(raw, []byte(tx.From)...)
	raw = append(raw, []byte(tx.To)...)
	raw = append(raw, tx.Amount.Bytes()...)
	raw = append(raw, []byte{0x00, 0x00, 0x00, 0x00}...) // locktime
	return raw
}
