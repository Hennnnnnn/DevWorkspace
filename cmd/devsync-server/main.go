package main

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose

	"github.com/Hennnnnnn/DevWorkspace/internal/crypto"
	"github.com/Hennnnnnn/DevWorkspace/internal/db"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/config"
	httpsrv "github.com/Hennnnnnn/DevWorkspace/internal/server/http"
	"github.com/Hennnnnnn/DevWorkspace/internal/server/store"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "create-admin":
			runCreateAdmin(os.Args[2:])
		case "serve":
			runServe()
		default:
			log.Fatalf("unknown command %q (want: serve | create-admin)", os.Args[1])
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
	if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	st, err := store.New(ctx, cfg.DatabaseURL)
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

// runCreateAdmin bootstraps the first admin from server-side shell access.
// Reads the user's device registration data from stdin flags: it activates an
// existing pending user+device and flags them admin. Zero race window: requires
// shell on the server. Usage: devsync-server create-admin <username> <fingerprint>
func runCreateAdmin(args []string) {
	if len(args) < 2 {
		log.Fatalf("usage: devsync-server create-admin <username> <fingerprint>")
	}
	username, fingerprint := args[0], args[1]
	ctx := context.Background()
	st, _ := mustStore(ctx)
	defer st.Close()

	device, user, err := st.GetDeviceByFingerprint(ctx, fingerprint)
	if err != nil {
		log.Fatalf("no device with fingerprint %s — run `devsync register` first", fingerprint)
	}
	if user.Username != username {
		log.Fatalf("fingerprint belongs to %q not %q", user.Username, username)
	}
	// Sanity: fingerprint must match the stored signing key.
	if crypto.Fingerprint(ed25519.PublicKey(device.SignPubKey)) != fingerprint {
		log.Fatalf("stored key does not match fingerprint (corrupt?)")
	}
	if err := st.SetUserStatus(ctx, user.ID, "active"); err != nil {
		log.Fatalf("activate user: %v", err)
	}
	if err := st.SetDeviceStatus(ctx, device.ID, "active"); err != nil {
		log.Fatalf("activate device: %v", err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE users SET is_admin=TRUE WHERE id=$1`, user.ID); err != nil {
		log.Fatalf("set admin: %v", err)
	}
	fmt.Printf("admin %q activated (device %s)\n", username, base64.RawStdEncoding.EncodeToString([]byte(device.ID))[:8])
}
