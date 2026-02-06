package tx

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/olehkaliuzhnyi/wallet-demo/internal/storage"
	"github.com/olehkaliuzhnyi/wallet-demo/internal/wallet"
	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// BuilderConfig holds configurable parameters for the transaction builder.
type BuilderConfig struct {
	MaxRetries int
	Fees       map[models.Network]*big.Int
}

// Builder constructs and manages transaction lifecycle.
// Handles nonce management, fee estimation, signing, broadcast, and confirmation.
type Builder struct {
	signers    map[models.Network]wallet.Signer
	nonceStore storage.NonceStore
	txStore    storage.TxStore
	logger     *slog.Logger
	cfg        BuilderConfig
}

// NewBuilder creates a new transaction builder with the given config and stores.
func NewBuilder(cfg BuilderConfig, nonces storage.NonceStore, txs storage.TxStore) *Builder {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.Fees == nil {
		cfg.Fees = make(map[models.Network]*big.Int)
	}
	return &Builder{
		signers:    make(map[models.Network]wallet.Signer),
		nonceStore: nonces,
		txStore:    txs,
		logger:     slog.Default().With("component", "tx_builder"),
		cfg:        cfg,
	}
}

// RegisterSigner registers a transaction signer for a specific network.
func (b *Builder) RegisterSigner(network models.Network, signer wallet.Signer) {
	b.signers[network] = signer
}

// SendRequest represents a request to send a transaction.
type SendRequest struct {
	IdempotencyKey string // prevents duplicate sends
	Network        models.Network
	From           string
	To             string
	Amount         *big.Int
	Data           []byte // smart contract call data (ETH/TRX)
	PrivateKey     []byte // in production: replaced by HSM key reference
}

// Send builds, signs, and "broadcasts" a transaction with idempotency.
func (b *Builder) Send(ctx context.Context, req SendRequest) (*models.Transaction, error) {
	// Idempotency check — prevent duplicate sends
	existing, err := b.txStore.Get(req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("tx store get: %w", err)
	}
	if existing != nil {
		b.logger.Info("duplicate request, returning existing tx",
			"idempotency_key", req.IdempotencyKey,
			"tx_hash", existing.TxHash,
		)
		return existing, nil
	}

	// Nonce management (for account-model chains like ETH, TRX)
	nonce, err := b.nonceStore.GetAndIncrement(req.From)
	if err != nil {
		return nil, fmt.Errorf("nonce store: %w", err)
	}

	// Build transaction
	tx := &models.Transaction{
		Network: req.Network,
		From:    req.From,
		To:      req.To,
		Amount:  req.Amount,
		Nonce:   nonce,
		Data:    req.Data,
		Fee:     b.estimateFee(req.Network),
	}

	b.logger.Info("building transaction",
		"network", tx.Network,
		"from", tx.From,
		"to", tx.To,
		"amount", tx.Amount,
		"nonce", tx.Nonce,
	)

	// Sign
	signer, ok := b.signers[req.Network]
	if !ok {
		return nil, fmt.Errorf("no signer for network %s", req.Network)
	}

	signed, err := signer.Sign(ctx, tx, req.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}

	// Broadcast with retry
	if err := b.broadcastWithRetry(ctx, signed, b.cfg.MaxRetries); err != nil {
		return nil, fmt.Errorf("broadcast: %w", err)
	}

	// Store for idempotency
	if err := b.txStore.Put(req.IdempotencyKey, signed); err != nil {
		return nil, fmt.Errorf("tx store put: %w", err)
	}

	return signed, nil
}

func (b *Builder) estimateFee(network models.Network) *big.Int {
	if fee, ok := b.cfg.Fees[network]; ok {
		return new(big.Int).Set(fee)
	}
	return big.NewInt(0)
}

func (b *Builder) broadcastWithRetry(ctx context.Context, tx *models.Transaction, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := b.broadcast(ctx, tx)
		if err == nil {
			b.logger.Info("transaction broadcast successful",
				"tx_hash", tx.TxHash,
				"attempt", attempt,
			)
			return nil
		}

		lastErr = err
		b.logger.Warn("broadcast attempt failed",
			"attempt", attempt,
			"max_retries", maxRetries,
			"error", err,
		)

		// Exponential backoff
		select {
		case <-time.After(time.Duration(attempt*attempt) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return fmt.Errorf("all %d broadcast attempts failed: %w", maxRetries, lastErr)
}

func (b *Builder) broadcast(ctx context.Context, tx *models.Transaction) error {
	// In production:
	// ETH: eth_sendRawTransaction
	// BTC: sendrawtransaction
	// TRX: wallet/broadcasttransaction
	b.logger.Info("broadcasting transaction",
		"network", tx.Network,
		"tx_hash", tx.TxHash,
	)
	return nil // simulated success
}
