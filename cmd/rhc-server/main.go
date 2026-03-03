package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/coreos/go-systemd/v22/activation"
	govarlink "github.com/emersion/go-varlink"

	server "github.com/redhatinsights/rhc/internal/rhc-server"
	"github.com/redhatinsights/rhc/varlink/internalapi"
)

const (
	socketPath     = "/run/rhc/com.redhat.rhc"
	socketDirPerms = 0755
	socketPerms    = 0666
)

func main() {
	if err := run(); err != nil {
		slog.Error("rhc-server error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Create backend
	backend := server.NewBackend()

	// Create registry and register the internal API
	registry := govarlink.NewRegistry(&govarlink.RegistryOptions{
		Vendor:  "Red Hat",
		Product: "rhc",
		Version: server.Version,
		URL:     "https://github.com/redhatinsights/rhc",
	})

	// Register internal API
	handler := internalapi.Handler{Backend: backend}
	handler.Register(registry)

	// Create server
	varlinkServer := &govarlink.Server{
		Handler: registry,
	}

	// Try to get listener from systemd socket activation first
	listener, err := getListener()
	if err != nil {
		return fmt.Errorf("failed to get listener: %w", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			slog.Error("Error closing listener", "error", err)
		}
	}()

	slog.Info("rhc-server starting", "version", server.Version)
	slog.Info("Listening on socket", "address", listener.Addr())

	// Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		errChan <- varlinkServer.Serve(listener)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	case sig := <-sigChan:
		slog.Info("Received signal, shutting down gracefully", "signal", sig)
		cancel()
	}

	slog.Info("rhc-server stopped")
	return nil
}

// trySystemdActivation attempts to get a listener from systemd socket activation
func trySystemdActivation() (net.Listener, error) {
	listeners, err := activation.Listeners()
	if err != nil {
		return nil, fmt.Errorf("failed to get systemd listeners: %w", err)
	}

	if len(listeners) == 0 {
		return nil, nil // No systemd socket available
	}

	slog.Info("Using systemd socket activation")
	return listeners[0], nil
}

// ensureSocketDirectory creates the directory for the socket if it doesn't exist
func ensureSocketDirectory(path string) error {
	dirPath := filepath.Dir(path)
	if err := os.MkdirAll(dirPath, socketDirPerms); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}
	return nil
}

// createUnixSocket creates a unix socket at the specified path
func createUnixSocket(path string) (net.Listener, error) {
	slog.Info("Creating unix socket", "path", path)

	// Ensure the directory exists
	if err := ensureSocketDirectory(path); err != nil {
		return nil, err
	}

	// Remove existing socket file if it exists
	if err := os.RemoveAll(path); err != nil {
		return nil, fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create unix socket
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("failed to create unix socket: %w", err)
	}

	// Set socket permissions
	if err := os.Chmod(path, socketPerms); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("failed to set socket permissions: %w", err)
	}

	return listener, nil
}

// getListener tries to get a listener from systemd socket activation,
// falls back to creating a unix socket
func getListener() (net.Listener, error) {
	// Try systemd socket activation first
	listener, err := trySystemdActivation()
	if err != nil {
		return nil, err
	}
	if listener != nil {
		return listener, nil
	}

	// Fall back to creating our own unix socket
	return createUnixSocket(socketPath)
}
