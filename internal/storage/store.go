package storage

import "github.com/olehkaliuzhnyi/wallet-demo/pkg/models"

// NonceStore manages per-address nonce state.
type NonceStore interface {
	// GetAndIncrement atomically returns the current nonce and increments it.
	GetAndIncrement(address string) (uint64, error)
}

// TxStore provides idempotent transaction storage.
type TxStore interface {
	// Get returns a previously stored transaction by idempotency key, or nil if not found.
	Get(idempotencyKey string) (*models.Transaction, error)
	// Put stores a transaction keyed by idempotency key.
	Put(idempotencyKey string, tx *models.Transaction) error
}

// WatchStore manages the set of watched addresses.
type WatchStore interface {
	// Add adds an address to the watch set.
	Add(address string) error
	// Remove removes an address from the watch set.
	Remove(address string) error
	// List returns all currently watched addresses.
	List() ([]string, error)
	// Contains checks if an address is in the watch set.
	Contains(address string) (bool, error)
}
