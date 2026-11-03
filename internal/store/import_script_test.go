package store

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestImportScript(t *testing.T) {
	if _, err := exec.LookPath("sqlite3"); err != nil {
		t.Skip("sqlite3 nicht im PATH")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "import_params.sql"))
	if err != nil {
		t.Fatal(err)
	}
	scriptSQL, err := os.ReadFile(script)
	if err != nil {
		t.Fatalf("Skript lesen: %v", err)
	}

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "params"), 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"a.json":      `{"params":{"nr_of_figures":1}}`,
		"b.json":      `{"params":{"nr_of_figures":2}}`,
		"kaputt.json": `{`,
		"notes.txt":   `kein JSON-Import`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, "params", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	dbPath := filepath.Join(dir, "go-02.db")
	s, err := Open(ctx, dbPath) // legt das Schema an
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	runScript := func() {
		t.Helper()
		cmd := exec.Command("sqlite3", dbPath)
		cmd.Dir = dir
		cmd.Stdin = bytes.NewReader(scriptSQL)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("sqlite3: %v\n%s", err, out)
		}
	}

	for run := 1; run <= 2; run++ {
		runScript()

		s, err := Open(ctx, dbPath)
		if err != nil {
			t.Fatal(err)
		}
		variants, err := s.ListVariants(ctx)
		_ = s.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(variants) != 2 {
			t.Fatalf("Lauf %d: %d Varianten, want 2", run, len(variants))
		}
		got := map[string]string{}
		for _, v := range variants {
			if v.SourceFile == nil {
				t.Fatalf("Variante %q ohne SourceFile", v.Name)
			}
			got[v.Name] = *v.SourceFile
		}
		if got["a"] != "params/a.json" || got["b"] != "params/b.json" {
			t.Errorf("Lauf %d: Varianten = %v", run, got)
		}
	}
}
