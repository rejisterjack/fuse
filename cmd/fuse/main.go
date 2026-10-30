package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rejisterjack/fuse/internal/alert"
	"github.com/rejisterjack/fuse/internal/budget"
	"github.com/rejisterjack/fuse/internal/config"
	"github.com/rejisterjack/fuse/internal/dashboard"
	"github.com/rejisterjack/fuse/internal/proxy"
	"github.com/rejisterjack/fuse/internal/store"
)

func main() {
	configPath := flag.String("config", "config.example.yaml", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	db, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Printf("warning: postgres unavailable (%v); continuing with in-memory budgets only", err)
	} else {
		defer db.Close()
	}

	tracker := budget.NewTracker(cfg.DefaultSoftUSD, cfg.DefaultHardUSD)
	notifier := &alert.Notifier{SlackWebhookURL: cfg.SlackWebhookURL}

	proxyHandler, err := proxy.New(cfg.UpstreamBaseURL, cfg.UpstreamAPIKey, tracker)
	if err != nil {
		log.Fatalf("proxy: %v", err)
	}
	proxyHandler.OnSoftHit = notifier.SoftHit

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	(&dashboard.Pages{Tracker: tracker}).Mount(mux)
	mux.Handle("/v1/", proxyHandler)
	mux.Handle("/v1", proxyHandler)

	srv := &http.Server{Addr: cfg.ListenAddr, Handler: mux}
	go func() {
		fmt.Fprintf(os.Stderr, "fuse listening on %s (upstream %s)\n", cfg.ListenAddr, cfg.UpstreamBaseURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
