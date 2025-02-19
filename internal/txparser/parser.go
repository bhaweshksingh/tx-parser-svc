package txparser

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// Parser is the interface exposing the key methods needed by external users.
type Parser interface {
	// GetCurrentBlock returns the last parsed block number (as an int).
	GetCurrentBlock() int

	// Subscribe adds an address to the watch list.
	Subscribe(address string) bool

	// GetTransactions returns transactions (inbound/outbound) for an address.
	GetTransactions(address string) []Transaction

	// StartParsing starts a background loop that fetches new blocks,
	// parses transactions, and updates the store until the context is canceled.
	StartParsing(ctx context.Context, pollInterval time.Duration)
}

// EthParser is a concrete implementation of Parser interface.
type EthParser struct {
	client JSONRPCClient // for calling Ethereum JSON-RPC
	store  Store         // in-memory store
	logger *slog.Logger

	mu           sync.RWMutex // for synchronizing currentBlock
	parseRunning bool
}

// NewEthParser returns a new EthParser with the given JSONRPCClient and MemoryStore.
func NewEthParser(client JSONRPCClient, store Store, logger *slog.Logger) *EthParser {
	if logger == nil {
		logger = slog.Default()
	}
	return &EthParser{
		client: client,
		store:  store,
		logger: logger,
	}
}

// StartParsing runs a background loop that continuously processes the next block.
func (p *EthParser) StartParsing(ctx context.Context, pollInterval time.Duration) {
	p.mu.Lock()
	if p.parseRunning {
		p.logger.Warn("Parser loop is already running; ignoring second StartParsing call")
		p.mu.Unlock()
		return
	}
	p.parseRunning = true
	p.mu.Unlock()

	p.logger.Info("Background parser loop started", "interval", pollInterval.String())

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("Context canceled, stopping parser loop.")
			return
		default:
			err := p.processNextBlock()
			if err != nil {
				p.logger.Error("Error processing next block", "err", err)
			}
			time.Sleep(pollInterval)
		}
	}
}

// processNextBlock fetches the next block from the chain, parses it, and stores relevant txs.
func (p *EthParser) processNextBlock() error {
	currentBlock := p.GetCurrentBlock()

	// Retrieve latest on-chain block
	latestBlockHex, err := p.client.BlockNumber()
	if err != nil {
		return fmt.Errorf("failed to get block number: %w", err)
	}

	latestBlockDecimal, err := hexToInt64(latestBlockHex)
	if err != nil {
		return fmt.Errorf("failed converting block hex to int64: %w", err)
	}

	if int64(currentBlock) >= latestBlockDecimal {
		p.logger.Debug("Already at or past the chain tip",
			"latest", latestBlockDecimal,
			"current", currentBlock,
		)
		return nil
	}

	nextBlock := currentBlock + 1
	blockData, err := p.client.GetBlockByNumber(int64(nextBlock))
	if err != nil {
		return fmt.Errorf("failed to fetch block data for block %d: %w", nextBlock, err)
	}

	transactions := parseTransactions(blockData)
	p.storeTransactions(transactions)

	p.mu.Lock()
	p.store.SetCurrentBlock(nextBlock)
	p.mu.Unlock()

	p.logger.Info("Parsed block",
		"block", nextBlock,
		"tx_count", len(transactions),
	)
	return nil
}

// parseTransactions transforms JSON-RPC block result into our Transaction type.
func parseTransactions(block BlockResponse) []Transaction {
	var txs []Transaction
	for _, tx := range block.Result.Transactions {
		txs = append(txs, Transaction{
			Hash:  tx.Hash,
			From:  tx.From,
			To:    tx.To,
			Value: tx.Value,
			Block: hexToInt64OrZero(block.Result.Number),
		})
	}
	return txs
}

// storeTransactions stores transactions if from/to addresses are subscribed.
func (p *EthParser) storeTransactions(txs []Transaction) {
	for _, tx := range txs {
		if p.store.IsSubscribed(tx.From) {
			p.store.AddTransaction(tx.From, tx)
		}
		if p.store.IsSubscribed(tx.To) {
			p.store.AddTransaction(tx.To, tx)
		}
	}
}

// Subscribe adds an address to the subscription set.
func (p *EthParser) Subscribe(address string) bool {
	return p.store.Subscribe(address)
}

// GetTransactions returns all transactions for a given address.
func (p *EthParser) GetTransactions(address string) []Transaction {
	return p.store.GetTransactions(address)
}

// GetCurrentBlock returns the in-memory current block number.
func (p *EthParser) GetCurrentBlock() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.store.GetCurrentBlock()
}

// hexToInt64 converts a "0x..." hex string to int64.
func hexToInt64(h string) (int64, error) {
	if len(h) > 2 && h[:2] == "0x" {
		h = h[2:]
	}
	val, err := strconv.ParseInt(h, 16, 64)
	if err != nil {
		return 0, err
	}
	return val, nil
}

// hexToInt64OrZero returns 0 if hex parsing fails.
func hexToInt64OrZero(h string) int64 {
	v, _ := hexToInt64(h)
	return v
}
