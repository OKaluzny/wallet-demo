package listener

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/olehkaliuzhnyi/wallet-demo/internal/storage"
	"github.com/olehkaliuzhnyi/wallet-demo/pkg/models"
)

// mockFetcher simulates a blockchain that produces blocks on demand.
type mockFetcher struct {
	mu     sync.Mutex
	blocks map[uint64]*BlockData
	head   uint64
}

func newMockFetcher() *mockFetcher {
	return &mockFetcher{blocks: make(map[uint64]*BlockData)}
}

func (f *mockFetcher) addBlock(b *BlockData) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.blocks[b.Number] = b
	if b.Number > f.head {
		f.head = b.Number
	}
}

func (f *mockFetcher) LatestBlockNumber(ctx context.Context) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.head, nil
}

func (f *mockFetcher) GetBlock(ctx context.Context, number uint64) (*BlockData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	b, ok := f.blocks[number]
	if !ok {
		return &BlockData{Number: number, Hash: fmt.Sprintf("hash-%d", number)}, nil
	}
	return b, nil
}

func newTestListener() (*PollingListener, *storage.MemoryWatchStore, *mockFetcher) {
	ws := storage.NewMemoryWatchStore()
	f := newMockFetcher()
	l := NewPollingListener(models.NetworkETH, 50*time.Millisecond, ws, f, PollingConfig{ConfirmationDepth: 3})
	return l, ws, f
}

func TestPollingListener_WatchUnwatch(t *testing.T) {
	l, ws, _ := newTestListener()

	if err := l.WatchAddress("0xabc"); err != nil {
		t.Fatal(err)
	}
	if err := l.WatchAddress("0xdef"); err != nil {
		t.Fatal(err)
	}

	addrs, _ := ws.List()
	if len(addrs) != 2 {
		t.Errorf("expected 2 watched addresses, got %d", len(addrs))
	}

	if err := l.UnwatchAddress("0xabc"); err != nil {
		t.Fatal(err)
	}

	addrs, _ = ws.List()
	if len(addrs) != 1 {
		t.Errorf("expected 1 watched address after unwatch, got %d", len(addrs))
	}
}

func TestPollingListener_Events(t *testing.T) {
	l, _, f := newTestListener()

	if err := l.WatchAddress("0xtest"); err != nil {
		t.Fatal(err)
	}

	// Add a block with a matching transaction
	f.addBlock(&BlockData{
		Number: 1,
		Hash:   "hash-1",
		Txs: []BlockTx{
			{Hash: "tx-1", From: "0xsender", To: "0xtest", Amount: big.NewInt(1000)},
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := l.Start(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-l.Events():
		if event.Network != models.NetworkETH {
			t.Errorf("expected ETH network, got %s", event.Network)
		}
		if event.To != "0xtest" {
			t.Errorf("expected event.To=0xtest, got %s", event.To)
		}
		if event.Confirmed {
			t.Error("event should not be confirmed yet")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	cancel()
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestPollingListener_Stop(t *testing.T) {
	l, _, _ := newTestListener()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := l.Start(ctx); err != nil {
		t.Fatal(err)
	}

	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}

	_, ok := <-l.Events()
	if ok {
		t.Error("events channel should be closed after Stop")
	}
}

func TestPollingListener_Confirmation(t *testing.T) {
	l, _, f := newTestListener()
	// ConfirmationDepth = 3

	if err := l.WatchAddress("0xaddr"); err != nil {
		t.Fatal(err)
	}

	// Block 1: has a matching tx
	f.addBlock(&BlockData{
		Number: 1, Hash: "h1",
		Txs: []BlockTx{{Hash: "tx1", From: "0xsender", To: "0xaddr", Amount: big.NewInt(100)}},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := l.Start(ctx); err != nil {
		t.Fatal(err)
	}

	// First event: unconfirmed
	select {
	case ev := <-l.Events():
		if ev.Confirmed {
			t.Error("first event should be unconfirmed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for unconfirmed event")
	}

	// Add blocks 2, 3, 4 to reach confirmation depth
	for i := uint64(2); i <= 4; i++ {
		f.addBlock(&BlockData{Number: i, Hash: fmt.Sprintf("h%d", i)})
	}

	// Wait for confirmation event
	select {
	case ev := <-l.Events():
		if !ev.Confirmed {
			t.Error("expected confirmed event after depth reached")
		}
		if ev.TxHash != "tx1" {
			t.Errorf("expected tx1, got %s", ev.TxHash)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for confirmed event")
	}

	cancel()
	if err := l.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestPollingListener_Reorg(t *testing.T) {
	// Use manual poll calls instead of Start() to avoid races on lastBlock.
	ws := storage.NewMemoryWatchStore()
	f := newMockFetcher()
	l := NewPollingListener(models.NetworkETH, time.Hour, ws, f, PollingConfig{ConfirmationDepth: 3})

	if err := l.WatchAddress("0xaddr"); err != nil {
		t.Fatal(err)
	}

	// Block 1 with a tx
	f.addBlock(&BlockData{
		Number: 1, Hash: "h1-original",
		Txs: []BlockTx{{Hash: "tx1", From: "0xsender", To: "0xaddr", Amount: big.NewInt(100)}},
	})

	ctx := context.Background()

	// Manually poll to process block 1
	if err := l.poll(ctx); err != nil {
		t.Fatal(err)
	}

	// Drain unconfirmed event
	select {
	case ev := <-l.Events():
		if ev.Reorged {
			t.Error("first event should not be reorged")
		}
		if ev.TxHash != "tx1" {
			t.Errorf("expected tx1, got %s", ev.TxHash)
		}
	default:
		t.Fatal("expected an event after poll")
	}

	// Simulate reorg: replace block 1 with different hash + add block 2 so head advances.
	// The fetcher decrements head to force re-processing via a new block 2.
	f.addBlock(&BlockData{
		Number: 1, Hash: "h1-reorged",
		Txs: []BlockTx{{Hash: "tx1-new", From: "0xsender", To: "0xaddr", Amount: big.NewInt(200)}},
	})
	// We need the listener to re-check block 1. Set lastBlock back to 0 (safe, no goroutine running).
	l.lastBlock = 0

	// Poll again — will re-fetch block 1, detect hash change, emit reorg + new event
	if err := l.poll(ctx); err != nil {
		t.Fatal(err)
	}

	// Collect events: expect reorg event for tx1 and new event for tx1-new
	var gotReorg, gotNew bool
	for i := 0; i < 10; i++ {
		select {
		case ev := <-l.Events():
			if ev.Reorged && ev.TxHash == "tx1" {
				gotReorg = true
			}
			if !ev.Reorged && ev.TxHash == "tx1-new" {
				gotNew = true
			}
		default:
		}
		if gotReorg && gotNew {
			break
		}
	}

	if !gotReorg {
		t.Error("expected reorg event for tx1")
	}
	if !gotNew {
		t.Error("expected new event for tx1-new")
	}
}

func TestManager_RegisterAndWatchAddress(t *testing.T) {
	handler := func(event models.BlockEvent) error { return nil }
	mgr := NewManager(handler)

	ws := storage.NewMemoryWatchStore()
	f := newMockFetcher()
	l := NewPollingListener(models.NetworkETH, 50*time.Millisecond, ws, f, PollingConfig{ConfirmationDepth: 3})
	mgr.RegisterListener(models.NetworkETH, l)

	if err := mgr.WatchAddress(models.NetworkETH, "0xaddr"); err != nil {
		t.Fatal(err)
	}

	found, _ := ws.Contains("0xaddr")
	if !found {
		t.Error("address should be in watched list after WatchAddress")
	}
}

func TestManager_StartAllStopAll(t *testing.T) {
	var handlerCalled atomic.Int64

	handler := func(event models.BlockEvent) error {
		handlerCalled.Add(1)
		return nil
	}

	mgr := NewManager(handler)

	ws := storage.NewMemoryWatchStore()
	f := newMockFetcher()
	l := NewPollingListener(models.NetworkETH, 50*time.Millisecond, ws, f, PollingConfig{ConfirmationDepth: 3})
	if err := l.WatchAddress("0xaddr"); err != nil {
		t.Fatal(err)
	}

	// Add a block with tx so handler gets called
	f.addBlock(&BlockData{
		Number: 1, Hash: "h1",
		Txs: []BlockTx{{Hash: "tx1", From: "0xsender", To: "0xaddr", Amount: big.NewInt(100)}},
	})

	mgr.RegisterListener(models.NetworkETH, l)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := mgr.StartAll(ctx); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)
	mgr.StopAll()

	if handlerCalled.Load() == 0 {
		t.Error("handler should have been called at least once")
	}
}

func TestManager_UnknownNetwork(t *testing.T) {
	handler := func(event models.BlockEvent) error { return nil }
	mgr := NewManager(handler)

	err := mgr.WatchAddress(models.NetworkBTC, "1abc")
	if err == nil {
		t.Error("expected error for unregistered network")
	}
}
