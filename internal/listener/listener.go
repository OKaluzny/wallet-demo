package listener

import (
	"context"
	"fmt"
	"log/slog"
	"math/big"
	"time"

	"github.com/olehkaliuzhnyi/wallet-demo/internal/storage"
	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// BlockListener defines the interface for monitoring blockchain addresses.
// Each network provides its own implementation (polling or WebSocket).
type BlockListener interface {
	// Start begins listening for transactions to watched addresses
	Start(ctx context.Context) error

	// Stop gracefully shuts down the listener
	Stop() error

	// WatchAddress adds an address to the watch list
	WatchAddress(address string) error

	// UnwatchAddress removes an address from the watch list
	UnwatchAddress(address string) error

	// Events returns a channel of detected block events
	Events() <-chan models.BlockEvent
}

// EventHandler processes detected blockchain events.
// In production: update balances, send notifications, trigger webhooks.
type EventHandler func(event models.BlockEvent) error

// BlockData represents the data returned by a block fetcher.
type BlockData struct {
	Number uint64
	Hash   string
	Txs    []BlockTx
}

// BlockTx represents a transaction within a block.
type BlockTx struct {
	Hash   string
	From   string
	To     string
	Amount *big.Int
}

// BlockFetcher abstracts the chain RPC calls for block data.
// In production: wraps eth_blockNumber + eth_getBlockByNumber, etc.
type BlockFetcher interface {
	// LatestBlockNumber returns the current chain head.
	LatestBlockNumber(ctx context.Context) (uint64, error)
	// GetBlock returns block data (hash + transactions) by block number.
	GetBlock(ctx context.Context, number uint64) (*BlockData, error)
}

// ----- Generic polling-based listener (works for any JSON-RPC chain) -----

// PollingConfig holds configuration for the polling listener.
type PollingConfig struct {
	ConfirmationDepth uint64 // blocks required before marking tx as confirmed
}

// PollingListener implements BlockListener using periodic block polling.
// Tracks block hashes to detect chain reorganizations.
type PollingListener struct {
	network      models.Network
	pollInterval time.Duration
	events       chan models.BlockEvent
	watchStore   storage.WatchStore
	fetcher      BlockFetcher
	cfg          PollingConfig
	lastBlock    uint64
	// blockHashes tracks recent block number -> hash for reorg detection.
	// Kept for the last confirmationDepth+1 blocks.
	blockHashes map[uint64]string
	// pendingEvents stores unconfirmed events keyed by block number for reorg rollback.
	pendingEvents map[uint64][]models.BlockEvent
	logger        *slog.Logger
	cancel        context.CancelFunc
	done          chan struct{}
}

func NewPollingListener(network models.Network, pollInterval time.Duration, ws storage.WatchStore, fetcher BlockFetcher, cfg PollingConfig) *PollingListener {
	if cfg.ConfirmationDepth == 0 {
		cfg.ConfirmationDepth = 12
	}
	return &PollingListener{
		network:       network,
		pollInterval:  pollInterval,
		events:        make(chan models.BlockEvent, 100),
		watchStore:    ws,
		fetcher:       fetcher,
		cfg:           cfg,
		blockHashes:   make(map[uint64]string),
		pendingEvents: make(map[uint64][]models.BlockEvent),
		done:          make(chan struct{}),
		logger:        slog.Default().With("component", "listener", "network", string(network)),
	}
}

func (l *PollingListener) Start(ctx context.Context) error {
	ctx, l.cancel = context.WithCancel(ctx)

	l.logger.Info("starting block listener",
		"poll_interval", l.pollInterval,
		"confirmation_depth", l.cfg.ConfirmationDepth,
	)

	go l.pollLoop(ctx)
	return nil
}

func (l *PollingListener) Stop() error {
	if l.cancel != nil {
		l.cancel()
	}
	<-l.done // wait for pollLoop to exit
	close(l.events)
	l.logger.Info("listener stopped")
	return nil
}

func (l *PollingListener) WatchAddress(address string) error {
	if err := l.watchStore.Add(address); err != nil {
		return err
	}
	l.logger.Info("watching address", "address", address)
	return nil
}

func (l *PollingListener) UnwatchAddress(address string) error {
	if err := l.watchStore.Remove(address); err != nil {
		return err
	}
	l.logger.Info("unwatched address", "address", address)
	return nil
}

func (l *PollingListener) Events() <-chan models.BlockEvent {
	return l.events
}

func (l *PollingListener) pollLoop(ctx context.Context) {
	defer close(l.done)
	ticker := time.NewTicker(l.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := l.poll(ctx); err != nil {
				l.logger.Error("poll failed", "error", err)
			}
		}
	}
}

func (l *PollingListener) poll(ctx context.Context) error {
	latest, err := l.fetcher.LatestBlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("latest block: %w", err)
	}

	// Process all blocks from lastBlock+1 to latest
	for num := l.lastBlock + 1; num <= latest; num++ {
		if err := l.processBlock(ctx, num); err != nil {
			return fmt.Errorf("process block %d: %w", num, err)
		}
	}

	// Check for newly confirmed events
	l.checkConfirmations(ctx, latest)

	return nil
}

func (l *PollingListener) processBlock(ctx context.Context, number uint64) error {
	block, err := l.fetcher.GetBlock(ctx, number)
	if err != nil {
		return fmt.Errorf("get block: %w", err)
	}

	// Reorg detection: check if stored hash differs from what we just fetched
	if prevHash, ok := l.blockHashes[number]; ok && prevHash != block.Hash {
		l.logger.Warn("chain reorganization detected",
			"block", number,
			"old_hash", prevHash,
			"new_hash", block.Hash,
		)
		// Invalidate all pending events from this block onward.
		// Use the highest block we have hashes for as upper bound.
		var maxStored uint64
		for bn := range l.blockHashes {
			if bn > maxStored {
				maxStored = bn
			}
		}
		l.handleReorg(ctx, number, maxStored)
	}

	// Store this block's hash
	l.blockHashes[number] = block.Hash
	l.lastBlock = number

	// Prune old block hashes beyond confirmation window
	if number > l.cfg.ConfirmationDepth+1 {
		delete(l.blockHashes, number-l.cfg.ConfirmationDepth-1)
	}

	// Match transactions against watched addresses
	addrs, err := l.watchStore.List()
	if err != nil {
		return fmt.Errorf("list watched: %w", err)
	}
	addrSet := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		addrSet[a] = true
	}

	for _, tx := range block.Txs {
		if addrSet[tx.To] || addrSet[tx.From] {
			event := models.BlockEvent{
				Network:     l.network,
				BlockNumber: number,
				TxHash:      tx.Hash,
				From:        tx.From,
				To:          tx.To,
				Amount:      tx.Amount,
				Confirmed:   false,
			}

			l.pendingEvents[number] = append(l.pendingEvents[number], event)

			l.logger.Info("detected transaction",
				"block", number,
				"tx", tx.Hash,
				"to", tx.To,
				"confirmed", false,
			)

			select {
			case l.events <- event:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}

	return nil
}

// handleReorg emits Reorged=true events for all pending events from reorgBlock to upTo,
// then removes them from pendingEvents so re-processing can produce fresh events.
func (l *PollingListener) handleReorg(ctx context.Context, reorgBlock uint64, upTo uint64) {
	for blockNum := reorgBlock; blockNum <= upTo; blockNum++ {
		events, ok := l.pendingEvents[blockNum]
		if !ok {
			continue
		}
		for _, ev := range events {
			ev.Reorged = true
			ev.Confirmed = false
			l.logger.Warn("reorg: invalidating event",
				"block", ev.BlockNumber,
				"tx", ev.TxHash,
			)
			select {
			case l.events <- ev:
			case <-ctx.Done():
				return
			}
		}
		delete(l.pendingEvents, blockNum)
		delete(l.blockHashes, blockNum)
	}
}

// checkConfirmations promotes pending events to confirmed once they have enough depth.
func (l *PollingListener) checkConfirmations(ctx context.Context, currentBlock uint64) {
	for blockNum, events := range l.pendingEvents {
		if currentBlock >= blockNum+l.cfg.ConfirmationDepth {
			for _, ev := range events {
				ev.Confirmed = true
				l.logger.Info("transaction confirmed",
					"block", ev.BlockNumber,
					"tx", ev.TxHash,
					"depth", currentBlock-blockNum,
				)
				select {
				case l.events <- ev:
				case <-ctx.Done():
					return
				}
			}
			delete(l.pendingEvents, blockNum)
		}
	}
}

// ----- Multi-chain listener manager -----

// Manager coordinates listeners across multiple networks.
type Manager struct {
	listeners map[models.Network]BlockListener
	handler   EventHandler
	logger    *slog.Logger
}

func NewManager(handler EventHandler) *Manager {
	return &Manager{
		listeners: make(map[models.Network]BlockListener),
		handler:   handler,
		logger:    slog.Default().With("component", "listener_manager"),
	}
}

func (m *Manager) RegisterListener(network models.Network, listener BlockListener) {
	m.listeners[network] = listener
}

// StartAll starts all registered listeners and routes events to the handler.
func (m *Manager) StartAll(ctx context.Context) error {
	for network, listener := range m.listeners {
		if err := listener.Start(ctx); err != nil {
			return fmt.Errorf("start %s listener: %w", network, err)
		}

		// Fan-in: route events from each listener to the common handler
		go func(net models.Network, l BlockListener) {
			for event := range l.Events() {
				if err := m.handler(event); err != nil {
					m.logger.Error("handle event failed",
						"network", net,
						"block", event.BlockNumber,
						"error", err,
					)
				}
			}
		}(network, listener)
	}

	m.logger.Info("all listeners started", "count", len(m.listeners))
	return nil
}

func (m *Manager) StopAll() {
	for network, listener := range m.listeners {
		if err := listener.Stop(); err != nil {
			m.logger.Error("stop listener failed", "network", network, "error", err)
		}
	}
}

// WatchAddress adds an address to the appropriate network listener.
func (m *Manager) WatchAddress(network models.Network, address string) error {
	l, ok := m.listeners[network]
	if !ok {
		return fmt.Errorf("no listener registered for %s", network)
	}
	return l.WatchAddress(address)
}
