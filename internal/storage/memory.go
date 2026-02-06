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

func NewMemoryNonceStore() *MemoryNonceStore {
	return &MemoryNonceStore{nonces: make(map[string]uint64)}
}

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

func NewMemoryTxStore() *MemoryTxStore {
	return &MemoryTxStore{txs: make(map[string]*models.Transaction)}
}

func (s *MemoryTxStore) Get(idempotencyKey string) (*models.Transaction, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.txs[idempotencyKey], nil
}

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

func NewMemoryWatchStore() *MemoryWatchStore {
	return &MemoryWatchStore{addrs: make(map[string]bool)}
}

func (s *MemoryWatchStore) Add(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addrs[address] = true
	return nil
}

func (s *MemoryWatchStore) Remove(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.addrs, address)
	return nil
}

func (s *MemoryWatchStore) List() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.addrs))
	for addr := range s.addrs {
		result = append(result, addr)
	}
	return result, nil
}

func (s *MemoryWatchStore) Contains(address string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addrs[address], nil
}
