package txparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// JSONRPCClient is a minimal interface for Ethereum JSON-RPC calls
type JSONRPCClient interface {
	BlockNumber() (string, error)
	GetBlockByNumber(blockNum int64) (BlockResponse, error)
}

// RPCClient is a simple implementation of JSONRPCClient
type RPCClient struct {
	endpoint string
	client   *http.Client
}

// NewJSONRPCClient creates a new RPCClient
func NewJSONRPCClient(endpoint string) JSONRPCClient {
	return &RPCClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// rpcRequest is used to form the body of a JSON-RPC request
type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

type rpcResponseBlockNumber struct {
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Result  string `json:"result"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (r *RPCClient) BlockNumber() (string, error) {
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_blockNumber",
		Params:  []interface{}{},
		ID:      1,
	}

	respBody, err := r.doRequest(reqBody)
	if err != nil {
		return "", err
	}

	var blockResp rpcResponseBlockNumber
	if err := json.Unmarshal(respBody, &blockResp); err != nil {
		return "", err
	}
	if blockResp.Error != nil {
		return "", fmt.Errorf("rpc error: %s", blockResp.Error.Message)
	}

	return blockResp.Result, nil
}

// BlockResponse is the structure to hold block data returned by the JSON RPC
type BlockResponse struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Number       string  `json:"number"`
		Hash         string  `json:"hash"`
		Transactions []RawTx `json:"transactions"`
	} `json:"result"`
}

type RawTx struct {
	Hash  string `json:"hash"`
	From  string `json:"from"`
	To    string `json:"to"`
	Value string `json:"value"`
	// Potentially blockNumber, input, gas, etc. For brevity, only keep needed fields
}

// GetBlockByNumber retrieves a specific block's data (and transactions).
func (r *RPCClient) GetBlockByNumber(blockNum int64) (BlockResponse, error) {
	hexBlockNum := fmt.Sprintf("0x%x", blockNum)
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_getBlockByNumber",
		Params:  []interface{}{hexBlockNum, true},
		ID:      1,
	}
	respBody, err := r.doRequest(reqBody)
	if err != nil {
		return BlockResponse{}, fmt.Errorf("GetBlockByNumber request failed: %w", err)
	}

	var blockResp BlockResponse
	if err := json.Unmarshal(respBody, &blockResp); err != nil {
		return BlockResponse{}, fmt.Errorf("GetBlockByNumber unmarshal failed: %w", err)
	}
	return blockResp, nil
}

// doRequest performs the JSON-RPC HTTP call and returns raw bytes of the response.
func (r *RPCClient) doRequest(data interface{}) ([]byte, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("json marshal failed: %w", err)
	}

	req, err := http.NewRequest("POST", r.endpoint, bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("reading response body failed: %w", err)
	}
	return buf.Bytes(), nil
}
