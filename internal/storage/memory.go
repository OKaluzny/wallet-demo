package storage

import (
	"sync"

	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// MemoryNonceStore is an in-memory NonceStore.
type MemoryNonceStore struct {
	mu     sync.Mutex
	nonces map[string]uint64
}

// NewMemoryNonceStore returns a new in-memory NonceStore.
func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{nonces: make(map[string]uint64)}
}

// GetAndIncrement atomically returns the current nonce and increments it.
func (s *MemoryNonceStore) GetAndIncrement(address string) (uint64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := s.nonces[address]
	s.nonces[address] = n + 1
	return n, nil
}

// MemoryTxStore is an in-memory TxStore.
type MemoryTxStore struct {
	mu  sync.RWMutex
	txs map[string]*models.Transaction
}

// NewMemoryTxStore returns a new in-memory TxStore.
func NewMemoryTxStore() *MemoryTxStore {
	return &MemoryTxStore{txs: make(map[string]*models.Transaction)}
}

// Get returns a transaction by idempotency key, or nil if not found.
func (s *MemoryTxStore) Get(idempotencyKey string) (*models.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.txs[idempotencyKey], nil
}

// Put stores a transaction by idempotency key.
func (s *MemoryTxStore) Put(idempotencyKey string, tx *models.Transaction) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.txs[idempotencyKey] = tx
	return nil
}

// MemoryWatchStore is an in-memory WatchStore.
type MemoryWatchStore struct {
	mu    sync.RWMutex
	addrs map[string]bool
}

// NewMemoryWatchStore returns a new in-memory WatchStore.
func NewMemoryWatchStore() *MemoryWatchStore {
	return &MemoryWatchStore{addrs: make(map[string]bool)}
}

// Add registers an address for watching.
func (s *MemoryWatchStore) Add(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addrs[address] = true
	return nil
}

// Remove unregisters an address from watching.
func (s *MemoryWatchStore) Remove(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.addrs, address)
	return nil
}

// List returns all watched addresses.
func (s *MemoryWatchStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.addrs))
	for addr := range s.addrs {
		result = append(result, addr)
	}
	return result, nil
}

// Contains checks if an address is being watched.
func (s *MemoryWatchStore) Contains(address string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addrs[address], nil
}
