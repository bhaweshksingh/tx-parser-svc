package txparser

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// NewHTTPServer constructs a new HTTP server with the given parser and slog logger.
func NewHTTPServer(parser Parser, logger *slog.Logger) *HTTPServer {
	if logger == nil {
		logger = slog.Default()
	}
	return &HTTPServer{
		parser: parser,
		logger: logger,
	}
}

// HTTPServer holds the parser and exposes handlers.
type HTTPServer struct {
	parser Parser
	logger *slog.Logger
}

// Router configures our endpoints with net/http’s ServeMux.
func (s *HTTPServer) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/current-block", s.handleCurrentBlock)
	mux.HandleFunc("/subscribe", s.handleSubscribe)
	mux.HandleFunc("/transactions", s.handleGetTransactions)
	return mux
}

// handleCurrentBlock returns the last parsed block.
func (s *HTTPServer) handleCurrentBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	block := s.parser.GetCurrentBlock()
	s.writeJSON(w, http.StatusOK, map[string]int{"currentBlock": block})
}

// handleSubscribe handles POST /subscribe { "address": "0x1234..." }
func (s *HTTPServer) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	type subReq struct {
		Address string `json:"address"`
	}
	var req subReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Error("Failed to decode JSON in subscribe", "err", err)
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}
	if req.Address == "" {
		http.Error(w, "address is required", http.StatusBadRequest)
		return
	}
	subscribed := s.parser.Subscribe(req.Address)
	s.writeJSON(w, http.StatusOK, map[string]bool{"subscribed": subscribed})
}

// handleGetTransactions handles GET /transactions?address=0x1234
func (s *HTTPServer) handleGetTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "only GET is allowed", http.StatusMethodNotAllowed)
		return
	}
	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "address is required", http.StatusBadRequest)
		return
	}
	txs := s.parser.GetTransactions(address)
	s.writeJSON(w, http.StatusOK, txs)
}

// writeJSON is a helper to marshal and write JSON with a given status code.
func (s *HTTPServer) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.logger.Error("Failed to encode JSON response", "err", err)
	}
}
