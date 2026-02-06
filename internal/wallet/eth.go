package wallet

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/OKaluzny/wallet-demo/pkg/models"
	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/tyler-smith/go-bip32"
	"golang.org/x/crypto/sha3"
)

// ETHGenerator generates Ethereum addresses using BIP-44 derivation.
// Derivation path: m/44'/60'/0'/0/{index}
type ETHGenerator struct{}

// NewETHGenerator returns a new Ethereum address generator.
func NewETHGenerator() *ETHGenerator {
	return &ETHGenerator{}
}

// Network returns the Ethereum network identifier.
func (g *ETHGenerator) Network() models.Network {
	return models.NetworkETH
}

// GenerateFromSeed derives an Ethereum address from a BIP-39 seed.
func (g *ETHGenerator) GenerateFromSeed(seed []byte, index uint32) (*models.DerivedAddress, error) {
	path := fmt.Sprintf("m/44'/60'/0'/0/%d", index)

	key, err := deriveKey(seed, 60, index)
	if err != nil {
		return nil, fmt.Errorf("derive key: %w", err)
	}

	// Get public key from private key using secp256k1
	privKey, pubKey := btcec.PrivKeyFromBytes(key[:32])
	_ = privKey
	pubBytes := pubKey.SerializeUncompressed()

	// Ethereum address = last 20 bytes of Keccak256(publicKey)
	hash := keccak256(pubBytes[1:]) // skip 0x04 prefix
	address := fmt.Sprintf("0x%s", hex.EncodeToString(hash[12:]))

	return &models.DerivedAddress{
		Network:        models.NetworkETH,
		Address:        address,
		DerivationPath: path,
		PublicKey:      hex.EncodeToString(pubBytes),
	}, nil
}

// ETHSigner signs Ethereum transactions (EIP-155 replay protection).
// In production, this would call HSM for signing.
type ETHSigner struct {
	chainID *big.Int
}

// NewETHSigner returns a new Ethereum transaction signer with the given chain ID.
func NewETHSigner(chainID int64) *ETHSigner {
	return &ETHSigner{chainID: big.NewInt(chainID)}
}

// Sign signs an Ethereum transaction with EIP-155 replay protection.
func (s *ETHSigner) Sign(ctx context.Context, tx *models.Transaction, privateKey []byte) (*models.Transaction, error) {
	// Build RLP-encoded transaction (simplified)
	// In production: use go-ethereum types.NewTransaction + types.SignTx
	txData := encodeTxForSigning(tx, s.chainID)
	txHash := keccak256(txData)

	tx.TxHash = fmt.Sprintf("0x%s", hex.EncodeToString(txHash))
	tx.Signed = true
	tx.RawSigned = txData // simplified; would be actual signed RLP

	return tx, nil
}

// --- helpers ---

// deriveKey derives a child private key from a BIP-39 seed using BIP-32/BIP-44.
// Path: m/44'/{coinType}'/0'/0/{index}
func deriveKey(seed []byte, coinType uint32, index uint32) ([]byte, error) {
	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("master key: %w", err)
	}

	// m/44'
	purpose, err := masterKey.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, fmt.Errorf("derive purpose: %w", err)
	}

	// m/44'/{coinType}'
	coin, err := purpose.NewChildKey(bip32.FirstHardenedChild + coinType)
	if err != nil {
		return nil, fmt.Errorf("derive coin: %w", err)
	}

	// m/44'/{coinType}'/0'
	account, err := coin.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, fmt.Errorf("derive account: %w", err)
	}

	// m/44'/{coinType}'/0'/0
	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, fmt.Errorf("derive change: %w", err)
	}

	// m/44'/{coinType}'/0'/0/{index}
	child, err := change.NewChildKey(index)
	if err != nil {
		return nil, fmt.Errorf("derive child: %w", err)
	}

	return child.Key, nil
}

func keccak256(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	h.Write(data)
	return h.Sum(nil)
}

func encodeTxForSigning(tx *models.Transaction, chainID *big.Int) []byte {
	// Simplified RLP encoding for demo
	// Production: use go-ethereum/rlp package
	var data []byte
	data = append(data, byte(tx.Nonce))
	data = append(data, tx.Amount.Bytes()...)
	data = append(data, []byte(tx.To)...)
	data = append(data, chainID.Bytes()...)
	return data
}
