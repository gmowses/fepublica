// Command api serves the public HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gmowses/fepublica/internal/api"
	"github.com/gmowses/fepublica/internal/config"
	"github.com/gmowses/fepublica/internal/logging"
	"github.com/gmowses/fepublica/internal/store"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		// For convenience in docker-compose we reuse the api binary to apply
		// the embedded schema. The initial migration is run by Postgres itself
		// through the docker-entrypoint-initdb.d volume, so this subcommand is
		// a placeholder that simply verifies connectivity.
		if err := migrate(ctx); err != nil {
			fmt.Fprintln(os.Stderr, "migrate error:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		// For the api binary we tolerate a missing TRANSPARENCIA_API_KEY since
		// it is not used by API-serving code paths. Bubble up any other error.
		return err
	}
	log := logging.New(cfg.Log.Level, cfg.Log.Format, "api")
	log.Info().
		Str("version", version).
		Str("base_url", cfg.API.BaseURL).
		Msg("api: starting")

	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()

	srv := api.New(st, log, version, cfg.API.BaseURL)
	addr := net.JoinHostPort(cfg.API.Host, strconv.Itoa(cfg.API.Port))
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info().Str("addr", addr).Msg("api: listening")
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}

func migrate(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, cfg.Postgres.DSN())
	if err != nil {
		return err
	}
	defer st.Close()
	// Verify at least one source exists (seeded by 001_initial.sql).
	sources, err := st.ListSources(ctx)
	if err != nil {
		return fmt.Errorf("migrate: verify sources: %w", err)
	}
	if len(sources) == 0 {
		return fmt.Errorf("migrate: no sources seeded — did 001_initial.sql run?")
	}
	fmt.Printf("migrate: ok (%d sources seeded)\n", len(sources))
	return nil
}

func runHealthcheck() int {
	// Called from Dockerfile HEALTHCHECK. A 200 from /health is enough.
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://127.0.0.1:8080/health")
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
