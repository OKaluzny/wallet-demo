package tx

import (
	"context"
	"math/big"
	"testing"

	"github.com/OKaluzny/wallet-demo/internal/storage"
	"github.com/OKaluzny/wallet-demo/pkg/models"
)

// mockSigner implements wallet.Signer for testing.
type mockSigner struct{}

func (m *mockSigner) Sign(ctx context.Context, tx *models.Transaction, privateKey []byte) (*models.Transaction, error) {
	tx.TxHash = "0xmockhash"
	tx.Signed = true
	tx.RawSigned = []byte("signed")
	return tx, nil
}

func newTestBuilder() *Builder {
	b := NewBuilder(
		BuilderConfig{
			MaxRetries: 3,
			Fees: map[models.Network]*big.Int{
				models.NetworkETH: big.NewInt(21_000 * 20_000_000_000),
				models.NetworkBTC: big.NewInt(10_000),
				models.NetworkTRX: big.NewInt(1_000_000),
			},
		},
		storage.NewMemoryNonceStore(),
		storage.NewMemoryTxStore(),
	)
	b.RegisterSigner(models.NetworkETH, &mockSigner{})
	b.RegisterSigner(models.NetworkBTC, &mockSigner{})
	b.RegisterSigner(models.NetworkTRX, &mockSigner{})
	return b
}

func TestBuilder_Idempotency(t *testing.T) {
	b := newTestBuilder()
	ctx := context.Background()

	req := SendRequest{
		IdempotencyKey: "key-1",
		Network:        models.NetworkETH,
		From:           "0xfrom",
		To:             "0xto",
		Amount:         big.NewInt(1000),
		PrivateKey:     []byte("pk"),
	}

	tx1, err := b.Send(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	tx2, err := b.Send(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	if tx1.TxHash != tx2.TxHash {
		t.Errorf("idempotent requests should return same tx, got %s vs %s", tx1.TxHash, tx2.TxHash)
	}
}

func TestBuilder_DifferentKeys(t *testing.T) {
	b := newTestBuilder()
	ctx := context.Background()

	tx1, err := b.Send(ctx, SendRequest{
		IdempotencyKey: "key-a",
		Network:        models.NetworkETH,
		From:           "0xfrom",
		To:             "0xto",
		Amount:         big.NewInt(1000),
		PrivateKey:     []byte("pk-a"),
	})
	if err != nil {
		t.Fatal(err)
	}

	tx2, err := b.Send(ctx, SendRequest{
		IdempotencyKey: "key-b",
		Network:        models.NetworkETH,
		From:           "0xfrom",
		To:             "0xto",
		Amount:         big.NewInt(2000),
		PrivateKey:     []byte("pk-b"),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Different amounts should result in different nonces at minimum
	if tx1.Nonce == tx2.Nonce {
		t.Error("different requests from same address should have different nonces")
	}
}

func TestBuilder_NonceIncrement(t *testing.T) {
	b := newTestBuilder()
	ctx := context.Background()

	var nonces []uint64
	for i := 0; i < 3; i++ {
		tx, err := b.Send(ctx, SendRequest{
			IdempotencyKey: "nonce-" + string(rune('0'+i)),
			Network:        models.NetworkETH,
			From:           "0xaddr",
			To:             "0xto",
			Amount:         big.NewInt(100),
			PrivateKey:     []byte("pk"),
		})
		if err != nil {
			t.Fatal(err)
		}
		nonces = append(nonces, tx.Nonce)
	}

	for i := 1; i < len(nonces); i++ {
		if nonces[i] != nonces[i-1]+1 {
			t.Errorf("nonce should increment: nonces[%d]=%d, nonces[%d]=%d", i-1, nonces[i-1], i, nonces[i])
		}
	}
}

func TestBuilder_NoSigner(t *testing.T) {
	b := NewBuilder(BuilderConfig{}, storage.NewMemoryNonceStore(), storage.NewMemoryTxStore())
	// No signers registered

	_, err := b.Send(context.Background(), SendRequest{
		IdempotencyKey: "no-signer",
		Network:        models.NetworkETH,
		From:           "0xfrom",
		To:             "0xto",
		Amount:         big.NewInt(100),
		PrivateKey:     []byte("pk"),
	})

	if err == nil {
		t.Error("expected error when no signer is registered")
	}
}

func TestBuilder_FeeEstimation(t *testing.T) {
	b := newTestBuilder()

	tests := []struct {
		network models.Network
		fee     int64
	}{
		{models.NetworkETH, 21_000 * 20_000_000_000},
		{models.NetworkBTC, 10_000},
		{models.NetworkTRX, 1_000_000},
	}

	for _, tt := range tests {
		t.Run(string(tt.network), func(t *testing.T) {
			got := b.estimateFee(tt.network)
			want := big.NewInt(tt.fee)
			if got.Cmp(want) != 0 {
				t.Errorf("estimateFee(%s) = %v, want %v", tt.network, got, want)
			}
		})
	}
}
