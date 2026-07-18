package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cpa-monitor/server/internal/config"
	"cpa-monitor/server/internal/httpapi"
	"cpa-monitor/server/internal/scraper"
	"cpa-monitor/server/internal/subscriptions"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	defaultRoot := filepath.Clean(filepath.Join(cwd, ".."))
	defaultAddr := os.Getenv("CPA_MONITOR_ADDR")
	if defaultAddr == "" {
		defaultAddr = config.DefaultListenAddr
	}
	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	projectRoot := flag.String("project-root", defaultRoot, "project root containing data and k12 directories")
	flag.Parse()

	root, err := filepath.Abs(*projectRoot)
	if err != nil {
		log.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		log.Fatal(err)
	}
	settings, err := config.NewStore(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		log.Fatal(err)
	}
	subs, err := subscriptions.NewManager(filepath.Join(root, "k12"), filepath.Join(dataDir, "subscription_checks.json"), settings)
	if err != nil {
		log.Fatal(err)
	}
	monitor, err := httpapi.NewMonitor(filepath.Join(dataDir, "offers.json"), filepath.Join(dataDir, "alerts.json"), settings, scraper.NewClient())
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	monitor.Start(ctx)

	server := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.NewServer(settings, monitor, subs),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()
	log.Printf("CPA Monitor listening on http://%s (project root: %s)", *addr, root)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
