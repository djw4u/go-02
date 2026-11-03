// Command go-02 startet die lokale Web-App zum Erzeugen von SVG-Bildern.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/djw4u/go-02/internal/generator"
	"github.com/djw4u/go-02/internal/store"
	"github.com/djw4u/go-02/internal/web"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "go-02:", err)
		os.Exit(1)
	}
}

// run startet den Server und kehrt nach einem Abbruch von ctx mit nil zurück,
// sobald der Server geordnet heruntergefahren ist.
func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("go-02", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dbPath := fs.String("db", "go-02.db", "Pfad der SQLite-Datenbank")
	addr := fs.String("addr", "127.0.0.1:8080", "Adresse des HTTP-Servers")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	st, err := store.Open(ctx, *dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = st.Close() }()

	handler, err := web.New(st, newSource)
	if err != nil {
		return err
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", *addr, err)
	}
	srv := newServer(handler)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	slog.Info("server listening", "addr", "http://"+ln.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

// newServer baut den HTTP-Server mit Zeitlimits für alle Phasen einer
// Verbindung.
func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// newSource liefert pro Aufruf eine frisch gesäte Zufallsquelle.
func newSource() generator.Source {
	return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
}
