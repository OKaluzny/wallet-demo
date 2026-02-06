package wallet

import (
	"context"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
	"github.com/tyler-smith/go-bip39"
)

func testSeed(t *testing.T) []byte {
	t.Helper()
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	seed := bip39.NewSeed(mnemonic, "")
	return seed
}

func testSeed2(t *testing.T) []byte {
	t.Helper()
	mnemonic := "zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo zoo wrong"
	seed := bip39.NewSeed(mnemonic, "")
	return seed
}

func allGenerators() []Generator {
	return []Generator{
		NewETHGenerator(),
		NewBTCGenerator(),
		NewTRXGenerator(),
	}
}

func TestGenerators_Network(t *testing.T) {
	tests := []struct {
		gen     Generator
		network models.Network
	}{
		{NewETHGenerator(), models.NetworkETH},
		{NewBTCGenerator(), models.NetworkBTC},
		{NewTRXGenerator(), models.NetworkTRX},
	}
	for _, tt := range tests {
		if got := tt.gen.Network(); got != tt.network {
			t.Errorf("Network() = %v, want %v", got, tt.network)
		}
	}
}

func TestGenerators_Deterministic(t *testing.T) {
	seed := testSeed(t)
	for _, gen := range allGenerators() {
		t.Run(string(gen.Network()), func(t *testing.T) {
			addr1, err := gen.GenerateFromSeed(seed, 0)
			if err != nil {
				t.Fatal(err)
			}
			addr2, err := gen.GenerateFromSeed(seed, 0)
			if err != nil {
				t.Fatal(err)
			}
			if addr1.Address != addr2.Address {
				t.Errorf("same seed+index produced different addresses: %s vs %s", addr1.Address, addr2.Address)
			}
			if addr1.PublicKey != addr2.PublicKey {
				t.Errorf("same seed+index produced different public keys: %s vs %s", addr1.PublicKey, addr2.PublicKey)
			}
		})
	}
}

func TestGenerators_DifferentSeeds(t *testing.T) {
	seed1 := testSeed(t)
	seed2 := testSeed2(t)
	for _, gen := range allGenerators() {
		t.Run(string(gen.Network()), func(t *testing.T) {
			addr1, err := gen.GenerateFromSeed(seed1, 0)
			if err != nil {
				t.Fatal(err)
			}
			addr2, err := gen.GenerateFromSeed(seed2, 0)
			if err != nil {
				t.Fatal(err)
			}
			if addr1.Address == addr2.Address {
				t.Error("different seeds produced same address")
			}
		})
	}
}

func TestGenerators_DifferentIndices(t *testing.T) {
	seed := testSeed(t)
	for _, gen := range allGenerators() {
		t.Run(string(gen.Network()), func(t *testing.T) {
			addr1, err := gen.GenerateFromSeed(seed, 0)
			if err != nil {
				t.Fatal(err)
			}
			addr2, err := gen.GenerateFromSeed(seed, 1)
			if err != nil {
				t.Fatal(err)
			}
			if addr1.Address == addr2.Address {
				t.Error("different indices produced same address")
			}
		})
	}
}

func TestETHGenerator_AddressFormat(t *testing.T) {
	seed := testSeed(t)
	gen := NewETHGenerator()
	addr, err := gen.GenerateFromSeed(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(addr.Address, "0x") {
		t.Errorf("ETH address should start with 0x, got %s", addr.Address)
	}
	if len(addr.Address) != 42 {
		t.Errorf("ETH address should be 42 chars, got %d: %s", len(addr.Address), addr.Address)
	}
	// Check hex validity (after 0x)
	if _, err := hex.DecodeString(addr.Address[2:]); err != nil {
		t.Errorf("ETH address is not valid hex: %s", addr.Address)
	}
}

func TestBTCGenerator_AddressFormat(t *testing.T) {
	seed := testSeed(t)
	gen := NewBTCGenerator()
	addr, err := gen.GenerateFromSeed(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(addr.Address, "1") {
		t.Errorf("BTC P2PKH address should start with 1, got %s", addr.Address)
	}
	if len(addr.Address) < 25 || len(addr.Address) > 34 {
		t.Errorf("BTC address length should be 25-34, got %d: %s", len(addr.Address), addr.Address)
	}
}

func TestTRXGenerator_AddressFormat(t *testing.T) {
	seed := testSeed(t)
	gen := NewTRXGenerator()
	addr, err := gen.GenerateFromSeed(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(addr.Address, "T") {
		t.Errorf("TRX address should start with T, got %s", addr.Address)
	}
}

func TestETHGenerator_PublicKeyFormat(t *testing.T) {
	seed := testSeed(t)
	gen := NewETHGenerator()
	addr, err := gen.GenerateFromSeed(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := hex.DecodeString(addr.PublicKey)
	if err != nil {
		t.Fatalf("public key is not valid hex: %s", addr.PublicKey)
	}
	if len(pubBytes) != 65 {
		t.Errorf("uncompressed public key should be 65 bytes, got %d", len(pubBytes))
	}
	if pubBytes[0] != 0x04 {
		t.Errorf("uncompressed public key should start with 0x04, got 0x%02x", pubBytes[0])
	}
}

func TestBTCGenerator_PublicKeyFormat(t *testing.T) {
	seed := testSeed(t)
	gen := NewBTCGenerator()
	addr, err := gen.GenerateFromSeed(seed, 0)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := hex.DecodeString(addr.PublicKey)
	if err != nil {
		t.Fatalf("public key is not valid hex: %s", addr.PublicKey)
	}
	if len(pubBytes) != 33 {
		t.Errorf("compressed public key should be 33 bytes, got %d", len(pubBytes))
	}
	if pubBytes[0] != 0x02 && pubBytes[0] != 0x03 {
		t.Errorf("compressed public key should start with 0x02 or 0x03, got 0x%02x", pubBytes[0])
	}
}

func TestSigners_Sign(t *testing.T) {
	signers := []struct {
		name   string
		signer Signer
	}{
		{"ETH", NewETHSigner(1)},
		{"BTC", NewBTCSigner(true)},
		{"TRX", NewTRXSigner()},
	}

	for _, tt := range signers {
		t.Run(tt.name, func(t *testing.T) {
			tx := &models.Transaction{
				Network: models.NetworkETH,
				From:    "0xfrom",
				To:      "0xto",
				Amount:  big.NewInt(1000),
				Nonce:   0,
			}
			signed, err := tt.signer.Sign(context.Background(), tx, []byte("fake-private-key"))
			if err != nil {
				t.Fatal(err)
			}
			if !signed.Signed {
				t.Error("transaction should be signed")
			}
			if signed.TxHash == "" {
				t.Error("TxHash should not be empty")
			}
		})
	}
}
