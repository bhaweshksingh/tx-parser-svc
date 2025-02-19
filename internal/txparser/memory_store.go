package txparser

import "sync"

type Store interface {
	Subscribe(address string) bool
	IsSubscribed(address string) bool
	AddTransaction(address string, tx Transaction)
	GetTransactions(address string) []Transaction
	SetCurrentBlock(block int)
	GetCurrentBlock() int
}

// MemoryStore holds subscriptions and transactions in memory.
type MemoryStore struct {
	mu           sync.RWMutex
	CurrentBlock int
	subscribed   map[string]bool
	transactions map[string][]Transaction
}

func (m *MemoryStore) GetCurrentBlock() int {
	return m.CurrentBlock
}

func (m *MemoryStore) SetCurrentBlock(block int) {
	m.CurrentBlock = block
}

// NewMemoryStore returns a new in-memory store.
func NewMemoryStore() Store {
	return &MemoryStore{
		subscribed:   make(map[string]bool),
		transactions: make(map[string][]Transaction),
	}
}

// Subscribe adds an address to the subscription set.
// Returns true if subscribed newly, false if already subscribed.
func (m *MemoryStore) Subscribe(address string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscribed[address] {
		return false
	}
	m.subscribed[address] = true
	m.transactions[address] = []Transaction{}
	return true
}

// IsSubscribed checks if an address is subscribed.
func (m *MemoryStore) IsSubscribed(address string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subscribed[address]
}

// AddTransaction appends a transaction to an address’s list if subscribed.
func (m *MemoryStore) AddTransaction(address string, tx Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscribed[address] {
		m.transactions[address] = append(m.transactions[address], tx)
	}
}

// GetTransactions returns the transactions for a given address.
func (m *MemoryStore) GetTransactions(address string) []Transaction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	txs, ok := m.transactions[address]
	if !ok {
		return []Transaction{}
	}

	// Return a copy to avoid external mutation
	cp := make([]Transaction, len(txs))
	copy(cp, txs)
	return cp
}
