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

	"cpa-monitor/server/application"
	"cpa-monitor/server/internal/config"
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

	appRuntime, err := application.New(*projectRoot)
	if err != nil {
		log.Fatal(err)
	}
	defer appRuntime.Close()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	appRuntime.Start(ctx)

	server := &http.Server{
		Addr:              *addr,
		Handler:           appRuntime.Handler(),
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
	log.Printf("CPA Monitor listening on http://%s (project root: %s)", *addr, appRuntime.Root())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
