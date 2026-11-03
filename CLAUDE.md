# Projekt: go-01

Go-Projekt, spec-driven entwickelt mit dem Superpowers-Workflow (brainstorming → Spec → Plan → TDD-Umsetzung).

## Stack

- Sprache: Go (Version siehe `go.mod`), Modul `github.com/djw4u/go-01`
- Abhängigkeiten über Go Modules; `go.mod` und `go.sum` sind eingecheckt
- Tests: `go test` (Standardbibliothek, ggf. table-driven Tests)

## Befehle

- Build: `go build ./...`
- Tests: `go test ./...`
- Einzelnes Paket / einzelner Test: `go test ./pfad/zum/paket -run TestName`
- Race-Detector: `go test -race ./...`
- Vet: `go vet ./...`
- Format: `gofmt -w .` (Prüfung: `gofmt -l .` muss leer sein)
- Lint: `golangci-lint run` (falls installiert und konfiguriert)
- Abhängigkeiten laden: `go mod download`
- Abhängigkeit hinzufügen: `go get <paket>`, danach `go mod tidy`
- Einmalig ein Tool ausführen: `go run <paket>@<version>`

## Projektstruktur

```
cmd/<name>/   main-Pakete (dünn, nur Verdrahtung)
internal/     Anwendungscode, nicht importierbar von außen
docs/         Specs und Pläne (Superpowers-Workflow)
```

Die Struktur wird mit dem Projekt wachsen; bei Abweichungen diese Datei anpassen.

## Code-Konventionen

- Idiomatisches Go: Effective Go und Code Review Comments als Maßstab
- Code ist `gofmt`-formatiert
- Fehler nicht verschlucken: behandeln oder mit Kontext weiterreichen (`fmt.Errorf("...: %w", err)`); keine `panic` im Normalbetrieb
- `context.Context` als erster Parameter bei blockierenden oder abbrechbaren Operationen
- Kleine Interfaces, dort definiert, wo sie genutzt werden; Strukturen zurückgeben, Interfaces akzeptieren
- Keine toten Code- oder Auskommentier-Reste, keine globalen veränderlichen Zustände
- Exportierte Bezeichner haben Doc-Kommentare
- Neue Abhängigkeiten nur, wenn nötig, und vorher kurz begründen; Standardbibliothek bevorzugen

## Tests

- Neue Funktionalität und Bugfixes bekommen einen Test
- Bei einem Bugfix zuerst einen fehlschlagenden Test schreiben, dann beheben (TDD)
- Tests müssen deterministisch sein (keine echten Netzwerkaufrufe, keine Abhängigkeit von Uhrzeit oder Reihenfolge); Zeit und I/O über Interfaces injizieren
- Test-Dateien als `*_test.go` neben dem Code

## Arbeitsweise für den Agenten

- Vor dem Abschluss grün haben: `go build ./...`, `go vet ./...`, `gofmt -l .` (leer), `go test ./...`
- Änderungen klein halten, ein Thema pro Branch und Commit
- Commit-Nachrichten im Imperativ, kurz und konkret (z. B. `Fix: Timeout im Retry-Handler`)
- Specs und Pläne liegen unter `docs/`; Implementierung folgt dem freigegebenen Plan
- Vor größeren Änderungen (neue Abhängigkeit, Umbau der Struktur, Änderung der Public API) erst einen kurzen Plan vorlegen
- Bei Unklarheit nachfragen statt raten
- Falls ein Paket nicht baut oder läuft (z. B. cgo, plattformspezifischer Code), nicht stillschweigend ausweichen, sondern das Problem melden

## Tabus

- Keine Änderungen an `.env`, Secrets oder CI-Konfiguration ohne Rückfrage
- Keine Secrets in Code, Logs oder Commits
- Kein `git push --force`, kein `rm -rf` außerhalb von Build-Ordnern
- `go.sum` nicht von Hand editieren
- Nicht auf `main` committen, immer auf einem Feature-Branch arbeiten
