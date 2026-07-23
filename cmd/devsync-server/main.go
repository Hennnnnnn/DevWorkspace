package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Hennnnnnn/DevWorkspace/internal/db"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/config"
	httpsrv "github.com/Hennnnnnn/DevWorkspace/internal/server/http"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "serve":
			runServe()
		default:
			log.Fatalf("unknown command %q (want: serve)", os.Args[1])
		}
		return
	}
	runServe()
}

func mustStore(ctx context.Context) (*store.Store, *config.Config) {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if err := db.Migrate(ctx, cfg.Driver, cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	st, err := store.New(ctx, cfg.Driver, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	return st, cfg
}

func runServe() {
	ctx := context.Background()
	log.Println("running migrations...")
	st, cfg := mustStore(ctx)
	defer st.Close()

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           httpsrv.New(st).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
