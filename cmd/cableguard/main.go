package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wyw14/cry-103/internal/api"
)

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	services, err := buildServices(cfg)
	if err != nil {
		return fmt.Errorf("build services: %w", err)
	}
	server := &http.Server{
		Addr: cfg.Listen, Handler: api.NewServer(services).Handler(),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second,
		WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errChannel := make(chan error, 1)
	go func() {
		log.Printf("CableGuard listening on %s", cfg.Listen)
		errChannel <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case serveErr := <-errChannel:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return serveErr
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
