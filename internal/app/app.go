package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spolnik/arqboard/internal/config"
	"github.com/spolnik/arqboard/internal/db"
	httpapi "github.com/spolnik/arqboard/internal/http"
	"github.com/spolnik/arqboard/internal/mcpserver"
	"github.com/spolnik/arqboard/migrations"
)

type envLookup = config.LookupFunc

func Run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	return run(ctx, args, os.LookupEnv, stdout, stderr)
}

func run(ctx context.Context, args []string, lookup envLookup, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "serve":
		if err := serve(ctx, args[1:], lookup, stderr); err != nil {
			fmt.Fprintf(stderr, "serve: %v\n", err)
			return 1
		}
		return 0
	case "migrate":
		if err := migrate(ctx, lookup, stdout); err != nil {
			fmt.Fprintf(stderr, "migrate: %v\n", err)
			return 1
		}
		return 0
	case "mcp":
		if err := runMCP(ctx, args[1:], lookup, stderr); err != nil {
			fmt.Fprintf(stderr, "mcp: %v\n", err)
			return 1
		}
		return 0
	case "admin":
		if err := admin(ctx, args[1:], lookup, stdout); err != nil {
			fmt.Fprintf(stderr, "admin: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: arqboard <serve|migrate|mcp|admin>")
}

func serve(ctx context.Context, args []string, lookup envLookup, stderr io.Writer) error {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(lookup)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	logger.Info("applying migrations")
	if err := prepareDatabase(ctx, cfg.DatabaseURL); err != nil {
		logger.Error("database migration failed", "error", err)
		return err
	}
	logger.Info("database migrations ready")

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	router := httpapi.NewRouter(httpapi.Options{
		Readiness:  db.ReadinessChecker{Conn: pool},
		BoardStore: db.BoardStore{Conn: pool},
		AuthStore:  db.AuthStore{Conn: pool},
		TeamStore:  db.TeamStore{Conn: pool},
		StaticFS:   os.DirFS(cfg.WebDistDir),
		Logger:     logger,
	})

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("starting server", "addr", cfg.HTTPAddr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func prepareDatabase(ctx context.Context, databaseURL string) error {
	migrationFS, err := migrations.ForDriver(string(db.DriverForURL(databaseURL)))
	if err != nil {
		return err
	}
	return db.MigrateUp(ctx, databaseURL, migrationFS)
}

func migrate(ctx context.Context, lookup envLookup, stdout io.Writer) error {
	databaseURL, err := databaseURLFromEnv(lookup)
	if err != nil {
		return err
	}

	if err := prepareDatabase(ctx, databaseURL); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "migrations applied")
	return nil
}

func runMCP(ctx context.Context, args []string, lookup envLookup, stderr io.Writer) error {
	flags := flag.NewFlagSet("mcp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	if err := flags.Parse(args); err != nil {
		return err
	}

	databaseURL, err := databaseURLFromEnv(lookup)
	if err != nil {
		return err
	}

	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	logger.Info("applying migrations")
	if err := prepareDatabase(ctx, databaseURL); err != nil {
		logger.Error("database migration failed", "error", err)
		return err
	}
	logger.Info("database migrations ready")

	conn, err := db.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	logger.Info("starting mcp server", "transport", "stdio")
	server := mcpserver.New(db.BoardStore{Conn: conn})
	return server.Run(ctx, &sdkmcp.StdioTransport{})
}

func admin(ctx context.Context, args []string, lookup envLookup, stdout io.Writer) error {
	if len(args) == 0 || args[0] != "create-user" {
		return errors.New("usage: arqboard admin create-user --email EMAIL --password PASSWORD [--name NAME]")
	}

	flags := flag.NewFlagSet("admin create-user", flag.ContinueOnError)
	email := flags.String("email", "", "admin email address")
	password := flags.String("password", "", "admin password")
	name := flags.String("name", "", "admin display name")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}

	databaseURL, err := databaseURLFromEnv(lookup)
	if err != nil {
		return err
	}

	pool, err := db.Open(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	userID, err := db.CreateAdminUser(ctx, pool, db.CreateAdminUserParams{
		Email:       *email,
		Password:    *password,
		DisplayName: *name,
	})
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "created admin user %s\n", userID)
	return nil
}

func databaseURLFromEnv(lookup envLookup) (string, error) {
	databaseURL := lookupString(lookup, "DATABASE_URL")
	if databaseURL != "" {
		return databaseURL, nil
	}
	if strings.EqualFold(lookupString(lookup, "APP_ENV"), "production") {
		return "", errors.New("DATABASE_URL is required")
	}
	return config.DefaultDatabaseURL, nil
}

func lookupString(lookup envLookup, key string) string {
	value, ok := lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}
