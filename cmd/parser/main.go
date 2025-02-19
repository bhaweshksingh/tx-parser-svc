package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/bhaweshksingh/tx-parser-svc/internal/txparser"
)

// main is the entrypoint for running the Tx Parser service.
// Key points:
//   - We create a structured slog.Logger.
//   - We pass a context to the parser for graceful shutdown.
func main() {
	// Create a structured logger using slog’s TextHandler (stdout).
	// In production, you might configure JSON or other outputs.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true, // includes file & line
		Level:     slog.LevelInfo,
	}))

	logger.Info("Starting Ethereum TX Parser...")

	// Create an in-memory store to track subscriptions and transactions.
	store := txparser.NewMemoryStore()

	// Create a JSON-RPC client for Ethereum (points to a public node).
	client := txparser.NewJSONRPCClient("https://ethereum-rpc.publicnode.com")

	// Create a parser instance that uses the JSON-RPC client and memory store.
	parser := txparser.NewEthParser(client, store, logger)

	// Create a cancellable context for controlling the background parser loop.
	ctx, cancel := context.WithCancel(context.Background())

	// Start the background routine to parse blocks every 3 seconds.
	go parser.StartParsing(ctx, 3*time.Second)

	// Create our HTTP server using the parser and logger.
	server := txparser.NewHTTPServer(parser, logger)
	srv := &http.Server{
		Addr:    ":8080",
		Handler: server.Router(),
	}

	// Start the HTTP server in a separate goroutine.
	go func() {
		logger.Info("Starting HTTP server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("HTTP server error", "err", err)
			cancel()
		}
	}()

	// Listen for system interrupts (Ctrl+C, SIGTERM) to allow graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	<-sigChan
	logger.Info("Received shutdown signal, attempting graceful shutdown...")

	// Cancel the background parser context.
	cancel()

	// Gracefully shut down the HTTP server with a 5-second timeout.
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error("Server shutdown error", "err", err)
	}

	logger.Info("Shutdown complete. Goodbye!")
	fmt.Println("Exiting.")
}
