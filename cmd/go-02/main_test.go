package main

import (
	"context"
	"net"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

// freeAddr liefert eine aktuell freie Loopback-Adresse.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return addr
}

func TestRun_StartsAndStopsOnCancel(t *testing.T) {
	addr := freeAddr(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-addr", addr, "-db", filepath.Join(t.TempDir(), "test.db")})
	}()

	deadline := time.After(5 * time.Second)
	for ready := false; !ready; {
		select {
		case err := <-done:
			t.Fatalf("run endete vorzeitig: %v", err)
		case <-deadline:
			t.Fatal("Server wurde nicht erreichbar")
		default:
		}
		resp, err := http.Get("http://" + addr + "/werkstatt")
		if err == nil {
			ready = resp.StatusCode == http.StatusOK
			_ = resp.Body.Close()
		}
		if !ready {
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run nach Abbruch: %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("run endete nach Abbruch nicht")
	}
}

func TestNewServer_SetsAllTimeouts(t *testing.T) {
	srv := newServer(http.NotFoundHandler())
	timeouts := map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	}
	for name, d := range timeouts {
		if d <= 0 {
			t.Errorf("%s = %v, want > 0", name, d)
		}
	}
}

func TestRun_InvalidFlagFails(t *testing.T) {
	if err := run(context.Background(), []string{"-gibt-es-nicht"}); err == nil {
		t.Fatal("run mit ungültigem Flag: want error, got nil")
	}
}
