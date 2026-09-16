// docker-dashboard: an interactive, LAN-only Docker dashboard.
package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

//go:embed static/*
var uiFS embed.FS

// Injected at build time via -ldflags "-X main.version=... -X main.buildTime=...".
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	if len(os.Args) > 1 && os.Args[1] == "-version" {
		fmt.Printf("docker-dashboard %s (build %s)\n", version, buildTime)
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	docker, err := newDocker(cfg.DockerHost)
	if err != nil {
		log.Fatalf("docker: %v", err)
	}

	srv := newServer(cfg, docker)

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("docker-dashboard listening on %s (dockerd: %s)", cfg.Addr, docker.hostName)
		errCh <- httpSrv.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case <-stop:
		log.Printf("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}
}
