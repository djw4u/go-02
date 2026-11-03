# go-02

Lokale Web-App, die aus JSON-Parametern SVG-Bilder mit geometrischen Figuren erzeugt. Parameter-Varianten und gespeicherte Bilder liegen in SQLite. Entwickelt spec-driven mit Superpowers; Entwurf und Plan stehen unter `docs/superpowers/`.

## Start

```
go run ./cmd/go-02
```

Danach ist die App unter <http://127.0.0.1:8080> erreichbar.

| Flag    | Standard         | Bedeutung                    |
|---------|------------------|------------------------------|
| `-db`   | `go-02.db`       | Pfad der SQLite-Datenbank    |
| `-addr` | `127.0.0.1:8080` | Adresse des HTTP-Servers     |

Die Datenbank und ihr Schema legt die App beim ersten Start selbst an.

## Parameter importieren

Der Import läuft außerhalb der App per SQL. Die App muss einmal gestartet worden sein. Im Verzeichnis, das den Ordner `params/` mit den `*.json`-Dateien enthält:

```
sqlite3 go-02.db < scripts/import_params.sql
```

Bereits importierte Dateien werden übersprungen, ungültiges JSON ebenfalls.

## Bedienung

- **Werkstatt:** Variante wählen, JSON ändern, „Generieren“ zeigt eine Vorschau (noch nicht gespeichert). „SVG speichern“ legt das Bild ab, „Neue Variante“ speichert geänderte Parameter unter einem neuen Namen.
- **Galerie:** Alle gespeicherten Bilder, filterbar nach Variante. Ein Klick öffnet das SVG im eigenen Fenster.

## Entwicklung

```
go build ./...
go vet ./...
gofmt -l .
go test ./...
```
