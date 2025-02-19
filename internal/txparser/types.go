package txparser

// Transaction is the internal representation of an Ethereum transaction
type Transaction struct {
	Hash  string `json:"hash"`
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
	Block int64  `json:"block"`
}
