# go-02

Lokale Web-App: JSON-Parameter -> SVG-Bilder, Ablage in SQLite.
Details: `README.md`, `start-server.md`, `docs/superpowers/`.

## Befehle

- Start:  `go run ./cmd/go-02`
- Flags:  `-db` (Default `go-02.db`), `-addr` (`127.0.0.1:8080`)
- Build:  `go build ./...`
- Test:   `go test ./...`
- Einzel: `go test ./internal/web -run TestName`
- Vet:    `go vet ./...`
- Format: `gofmt -l .` (Ausgabe muss leer sein)
- Lint:   `golangci-lint run` (Config: `.golangci.yml`)
- Ohne cgo: `CGO_ENABLED=0 go build ./...` muss laufen
- Vor Abschluss: Build, Vet, gofmt, Lint, Test grün.

`golangci-lint` muss lokal installiert sein (derzeit nicht der Fall).
Keine CI, kein Makefile.

## Struktur

    cmd/go-02/           main: Flags, Verdrahtung, Shutdown
    internal/generator/  Parameter -> SVG (kein HTTP, keine DB)
    internal/store/      SQLite-Zugriff (modernc.org/sqlite)
    internal/web/        Handler, Templates (go:embed), CSS
    scripts/             import_params.sql (Import per sqlite3)
    docs/superpowers/    Specs und Pläne

- `web` nutzt `generator` und `store`, nie umgekehrt.
- `web` definiert sein eigenes kleines `Store`-Interface.
- Erweitern: Anordnung in `layoutFor`, Figur in `drawFigure`,
  Regel in `evalRule` (je ein neuer `case`).

## Konventionen

- Fehler mit Kontext: `fmt.Errorf("...: %w", err)`.
- Fehlertexte, Log-Meldungen, Bezeichner: englisch.
- Kommentare, Doc-Kommentare, Nutzertexte (UI, HTTP): deutsch.
- Feldfehler nennen das Feld: `field %q: want ..., got ...`.
- Sentinel-Fehler: `store.ErrNotFound`, Prüfung per `errors.Is`.
- Zufall nur über `generator.Source`, nie global.
- Server rendert HTML (`html/template`), kein JavaScript.
- Tests: table-driven mit `t.Run`, Datei `*_test.go` daneben.

## Tests

- Deterministisch: fester Seed, DB nur in `t.TempDir()`.
- Web-Tests mit `httptest`, keine echten Netzwerkaufrufe.
- `import_script_test.go` braucht `sqlite3` im `PATH`,
  sonst wird der Test übersprungen (kein Fehler).

## Stolpersteine

- Kein generierter Code. Abhängigkeit: nur `modernc.org/sqlite`.
- Keine Umgebungsvariablen; Konfiguration nur über Flags.
- Eine DB-Verbindung (`SetMaxOpenConns(1)`) gegen `SQLITE_BUSY`.
- `?` oder `#` im DB-Pfad bricht die Verbindung.
- Default-DB `go-02.db` liegt im Arbeitsverzeichnis; `.gitignore`
  ignoriert `*.db` nicht. Keine DB committen.
- Parameterformat und Grenzen: `start-server.md`.
- `.superpowers/` ist ignoriert (Brainstorming-Arbeitsdateien).
- Gespeicherte SVGs sind fremder Inhalt. CSP, `nosniff` und
  `<img>` mit `data:`-URI nicht abschwächen.
- `.golangci.yml` nicht lockern; `//nolint` nur mit Begründung.

## Arbeitsweise

- Neue Funktion: Brainstorming -> Spec (`docs/superpowers/specs/`)
  -> Plan (`docs/superpowers/plans/`) -> TDD, erst ein Test.
- Commit-Nachrichten: Imperativ, kurz und konkret, ohne Präfix.
- Ein Thema je Commit.
- Neue Abhängigkeit oder Umbau: erst kurz begründen.
- Kein `git push --force`; `go.sum` nicht von Hand ändern.
- Keine Änderung an `.env`, Secrets oder CI ohne Rückfrage.
