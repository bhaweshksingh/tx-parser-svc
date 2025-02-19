package txparser

import (
	"context"
	"testing"
	"time"

	"log/slog"
)

// TestMemoryStore ensures our in-memory store logic works as expected.
func TestMemoryStore(t *testing.T) {
	store := NewMemoryStore()

	if store.GetCurrentBlock() != 0 {
		t.Errorf("expected CurrentBlock=0, got %d", store.GetCurrentBlock())
	}

	addr := "0x1234"
	subscribed := store.Subscribe(addr)
	if !subscribed {
		t.Errorf("expected new subscription to return true, got false")
	}
	// Re-subscribe
	if store.Subscribe(addr) {
		t.Errorf("expected repeat subscription to return false, got true")
	}

	tx := Transaction{Hash: "0xabc", From: addr, To: "0x5678", Value: "0x1", Block: 100}
	store.AddTransaction(addr, tx)
	txs := store.GetTransactions(addr)
	if len(txs) != 1 {
		t.Errorf("expected 1 tx, got %d", len(txs))
	}
	if txs[0].Hash != "0xabc" {
		t.Errorf("expected hash=0xabc, got %s", txs[0].Hash)
	}
}

// mockClient is a stub JSONRPCClient for testing parser logic.
type mockClient struct {
	latestBlock string
	blocks      map[int64]BlockResponse
}

func (m *mockClient) BlockNumber() (string, error) {
	return m.latestBlock, nil
}
func (m *mockClient) GetBlockByNumber(blockNum int64) (BlockResponse, error) {
	return m.blocks[blockNum], nil
}

// TestParser verifies the parser processes blocks and stores transactions for subscribed addresses.
func TestParser(t *testing.T) {
	mc := &mockClient{
		latestBlock: "0x3", // decimal 3
		blocks: map[int64]BlockResponse{
			1: {
				Result: struct {
					Number       string  `json:"number"`
					Hash         string  `json:"hash"`
					Transactions []RawTx `json:"transactions"`
				}{
					Number: "0x1",
					Hash:   "0xblock1",
					Transactions: []RawTx{
						{Hash: "0xtx1", From: "0xABCDEF", To: "0x123", Value: "0x10"},
						{Hash: "0xtx2", From: "0x555", To: "0x666", Value: "0x20"},
					},
				},
			},
			2: {
				Result: struct {
					Number       string  `json:"number"`
					Hash         string  `json:"hash"`
					Transactions []RawTx `json:"transactions"`
				}{
					Number: "0x2",
					Hash:   "0xblock2",
					Transactions: []RawTx{
						{Hash: "0xtx3", From: "0x123", To: "0xABCDEF", Value: "0x15"},
					},
				},
			},
			3: {
				Result: struct {
					Number       string  `json:"number"`
					Hash         string  `json:"hash"`
					Transactions []RawTx `json:"transactions"`
				}{
					Number:       "0x3",
					Hash:         "0xblock3",
					Transactions: []RawTx{},
				},
			},
		},
	}

	store := NewMemoryStore()
	logger := slog.New(slog.NewTextHandler(nil, nil)) // minimal logger to pass in
	parser := NewEthParser(mc, store, logger)

	// Subscribe to address "0x123" so we only track those.
	parser.Subscribe("0x123")

	// Manually process next blocks
	if err := parser.processNextBlock(); err != nil {
		t.Fatalf("processNextBlock block1 error: %v", err)
	}
	if err := parser.processNextBlock(); err != nil {
		t.Fatalf("processNextBlock block2 error: %v", err)
	}
	if err := parser.processNextBlock(); err != nil {
		t.Fatalf("processNextBlock block3 error: %v", err)
	}

	// Confirm we've updated to block 3
	if parser.GetCurrentBlock() != 3 {
		t.Errorf("expected current block=3, got %d", parser.GetCurrentBlock())
	}

	// We subscribed to "0x123", so let's see which txs we got
	txs := parser.GetTransactions("0x123")
	if len(txs) != 2 {
		t.Errorf("expected 2 transactions for 0x123, got %d", len(txs))
	}

	// Now test the StartParsing loop with a real context (optional).
	ctx, cancel := context.WithCancel(context.Background())
	go parser.StartParsing(ctx, 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	cancel() // ensure no panic or hang
}
