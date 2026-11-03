# go-02: Server starten und weiterentwickeln

Lokale Web-App, die aus JSON-Parametern SVG-Bilder mit geometrischen Figuren erzeugt. Varianten und gespeicherte Bilder liegen in SQLite. Die App läuft auf <http://127.0.0.1:8080>.

## Starten

Datenbank und Parameterdateien liegen bewusst außerhalb des Projekts (`~/go-02-daten/`), damit nichts versehentlich ins Repo gerät. Die `.gitignore` ignoriert `*.db` nicht.

```bash
cd ~/code/go-02
go run ./cmd/go-02 -db ~/go-02-daten/go-02.db
```

Die Datenbank und ihr Schema legt die App beim ersten Start selbst an. Beenden mit `Ctrl+C` (geordnetes Herunterfahren).

Schnellerer Start ohne Übersetzen bei jedem Aufruf:

```bash
cd ~/code/go-02
go build -o ~/go-02-daten/go-02 ./cmd/go-02
~/go-02-daten/go-02 -db ~/go-02-daten/go-02.db
```

| Flag    | Standard         | Bedeutung                 |
|---------|------------------|---------------------------|
| `-db`   | `go-02.db`       | Pfad der SQLite-Datenbank |
| `-addr` | `127.0.0.1:8080` | Adresse des HTTP-Servers  |

Ist der Port belegt, mit `-addr 127.0.0.1:8081` einen anderen wählen.

## Parameter importieren

Der Import ist **keine App-Funktion**, sondern ein SQL-Skript. Die App muss einmal gestartet worden sein (Schema). Die JSON-Dateien liegen in `~/go-02-daten/params/`:

```bash
cd ~/go-02-daten
sqlite3 go-02.db < ~/code/go-02/scripts/import_params.sql
```

Bereits importierte Dateien (gleicher Pfad in `source_file`) und ungültiges JSON werden übersprungen. Danach die Seite neu laden.

Beispieldateien in `~/go-02-daten/params/`: `beispiel.json` (RANDOM, 32 Figuren), `raster.json` (GRID 6×6), `poisson.json` (PDS). `raster` und `poisson` habe ich selbst geschrieben, `beispiel` stammt aus einer älteren Variante.

## Bedienung

- **Werkstatt:** Variante wählen, JSON im Textfeld ändern. „Generieren“ zeigt eine Vorschau, jedes Mal ein anderes Bild, nichts wird gespeichert. „SVG speichern“ legt das gezeigte Bild ab. „Neue Variante“ speichert die geänderten Parameter unter dem Namen aus dem Feld daneben, die Ausgangsvariante bleibt unverändert.
- **Galerie:** Alle gespeicherten Bilder, filterbar nach Variante. Ein Klick öffnet das SVG im eigenen Fenster.

---

# Stand und Hintergrund

## Stand

Die erste Version ist fertig und in `main` (PRs #3 bis #6, Stand `f71ace1`). Entwurf: `docs/superpowers/specs/2026-11-01-bildgenerator-design.md` (die Statuszeile dort steht noch auf „Entwurf zur Freigabe“, sie ist freigegeben). Plan: `docs/superpowers/plans/2026-11-01-bildgenerator.md`. Entwurf und Plan enthalten die Begründungen hinter den Entscheidungen.

## Aufbau

```
cmd/go-02/            main: Flags, Verdrahtung, Server mit Timeouts, geordnetes Herunterfahren
internal/generator/   Parameter -> SVG (ohne HTTP, ohne Datenbank)
internal/store/       SQLite-Zugriff (modernc.org/sqlite, ohne cgo)
internal/web/         Handler, Templates (go:embed), static/style.css
scripts/              import_params.sql
docs/superpowers/     Entwurf und Plan
```

Abhängigkeitsrichtung: `web` nutzt `generator` und `store`; `generator` kennt weder `web` noch `store`. `web` definiert sein eigenes kleines `Store`-Interface.

## Parameterformat

Datei: `{"params": { ... }}`. Die Datenbank speichert die komplette Datei inklusive des äußeren `"params"`.

**Regeln** (ein Array aus Regelname und Argumenten):

- `["ri",[min,max]]`: zufällige Ganzzahl, Grenzen inklusive, Betrag höchstens 2^31.
- `["rc",[[a,b,c]]]`: zufälliges Element der Liste (die Liste steckt doppelt verschachtelt im Argument).
- Alles andere ist ein fester Wert. Ein Array wie `palette` ist keine Regel, weil es nicht die Form `[Text, Array]` hat.
- Jede Regel wird **pro Figur** neu gewürfelt, auch `figures`.

**Anordnung** (`structure`):

| Wert     | Felder                                                                                     |
|----------|--------------------------------------------------------------------------------------------|
| `RANDOM` | `nr_of_figures` (1 bis 10000), `x` und `y` (Regel oder fester Wert)                        |
| `GRID`   | `rows`, `columns` (feste Ganzzahlen, Produkt höchstens 10000); `nr_of_figures`, `x`, `y` werden ignoriert |
| `PDS`    | `nr_of_figures`, `min_distance` (fest, größer 0); Poisson Disc Sampling auf 0..100 per Dart-Throwing, höchstens 100 Versuche je Figur, passen nicht alle hinein, kommen ohne Fehler weniger heraus |

**Figuren** (`figures` liefert die Art): `rect`, `square`, `circle`, `line`, `polygon`, `triangle`.

- `x`/`y` sind der Mittelpunkt, die Fläche ist `viewBox="0 0 100 100"`, Figuren dürfen über den Rand ragen.
- `width` ist Pflicht. `rect` braucht zusätzlich `height`. `square`: Seite gleich `width`. `circle`: Radius gleich `width/2`. `line`: waagerecht von `x-width/2` bis `x+width/2`. `polygon`: regelmäßiges n-Eck mit Umkreisradius `width/2`, Feld `sides` (3 bis 1000, Standard 5). `triangle` ist ein `polygon` mit 3 Ecken.
- Optionale Attribute, nur wenn das Feld vorhanden ist, in dieser Reihenfolge: `fill` (nicht bei `line`), `opacity`, `stroke`, `stroke-width`, `stroke-opacity`, `stroke-linecap`, `transform`. `transform` ist ein Drehwinkel in Grad um den Mittelpunkt.
- `palette` ist reine Metadaten und ändert nichts. `colorid` gibt es nicht mehr.
- Zahlen im SVG haben höchstens 3 Nachkommastellen, Textwerte werden maskiert.
- Ungültige Parameter führen immer zu einem Fehler mit Meldung, nie zu einem stillen Ergebnis.

## Datenmodell

- `param_variants`: `id`, `name`, `params` (JSON, per `json_valid` geprüft), `source_file` (eindeutig, `NULL` bei in der App angelegten Varianten), `parent_id` (Ausgangsvariante), `created_at`.
- `images`: `id`, `variant_id`, `svg`, `created_at`. Jedes Bild verweist auf die Variante, aus der es entstand.
- Der Zufallswert wird nicht gespeichert: das SVG selbst ist das Ergebnis.

## Routen

`GET /` leitet auf `/werkstatt`. `GET /werkstatt?variant=ID`. `POST /werkstatt/generate`, `/werkstatt/save-image`, `/werkstatt/save-variant`. `GET /galerie?variant=ID`. `GET /bilder/{id}.svg` (mit `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'` und `nosniff`). `GET /static/style.css`.

## Wichtige Entscheidungen

- **Server rendert die Seiten** (`html/template`), kein JavaScript, keine Frontend-Bibliotheken. Ein JSON-Editor oder `htmx` käme nur bei Bedarf und einzeln begründet dazu.
- **`modernc.org/sqlite`** statt `mattn/go-sqlite3`: reines Go, kein C-Compiler nötig. `CGO_ENABLED=0 go build ./...` muss laufen.
- **Zufall nur über `generator.Source`** (ein Interface, `*rand.Rand` aus `math/rand/v2` erfüllt es). Tests nutzen einen festen Seed und sind deshalb wiederholbar. Die App nutzt pro Anfrage eine frisch gesäte Quelle.
- **Vorschau und Speichern:** Die Vorschau ist ein `<img>` mit `data:`-URI (Skripte laufen dort nicht). Beim Speichern reist das SVG als verstecktes Formularfeld mit. Der Server prüft nur, dass es mit `<svg ` beginnt und höchstens 1 MiB groß ist.
- **Obergrenzen** gegen Tippfehler: 10000 Figuren, 1000 Ecken, `ri`-Grenzen bis 2^31, Formulare bis 2 MiB, Server-Timeouts.
- **Eine Datenbankverbindung** (`SetMaxOpenConns(1)`), das vermeidet `SQLITE_BUSY` innerhalb der App.

## Bekannte Grenzen (zurückgestellt)

- Bei extremen Breiten wie `1e308` steht `+Inf` im SVG, ohne Fehler.
- SQLite ohne `busy_timeout` und WAL: Ein Import per `sqlite3` bei laufender App kann auf `SQLITE_BUSY` laufen. Ein `?` oder `#` im Datenbankpfad bricht die Verbindung.
- Die Galerie lädt die vollen SVGs, obwohl sie nur Metadaten braucht.
- Gespeicherte SVGs sind nicht geprüft und gelten als fremder Inhalt (CSP und `<img>` schützen). In der README fehlt dieser Hinweis.
- Kein `sandbox` in der CSP der Bilder, kein `Cache-Control` für statische Dateien.
- `Generate` bricht nicht ab, wenn die Verbindung getrennt wird.
- Bewusst nicht Teil der ersten Version: Anmeldung, mehrere Nutzer, Betrieb im Netz, Import über die Web-App, Löschen und Umbenennen von Varianten und Bildern, weitere Anordnungen.

---

# Mit Claude Code weiterentwickeln

## Sitzung starten

```bash
cd ~/code/go-02
claude
```

Claude liest dann `CLAUDE.md` im Projekt. Sie ist eigenständig; die Bun/TypeScript-Regeln aus `~/code/CLAUDE.md` gelten hier nicht.

## Arbeitsweise

Das Projekt wird spec-driven mit dem Superpowers-Workflow entwickelt:

1. Neue Funktion: sagen „Brainstorming für <Funktion>“. Claude fragt nach, legt Ansätze vor und schreibt einen Entwurf nach `docs/superpowers/specs/JJJJ-MM-TT-<thema>-design.md`.
2. Nach Deiner Freigabe entsteht der Plan in `docs/superpowers/plans/`.
3. Umsetzung nach Plan, testgetrieben (erst ein fehlschlagender Test). Entweder „native“ in der Sitzung oder mit einem Subagenten je Task; am Ende prüft ein frischer Reviewer den Branch.
4. PR nach `main`, Merge durch Dich.

Regeln aus `CLAUDE.md`, die Claude beachtet:

- Nie auf `main` committen, immer auf einem Feature-Branch, ein Thema pro Branch und Commit.
- Commit-Nachrichten im Imperativ, kurz und konkret, mit Präfix wie `Feature:`, `Fix:`, `Docs:`, `Chore:`.
- Vor dem Abschluss grün: `go build ./...`, `go vet ./...`, `gofmt -l .` (leer), `go test ./...`.
- Neue Abhängigkeiten nur nach kurzer Begründung. Vor größeren Änderungen (Struktur, Public API) erst ein Plan.
- Keine Änderungen an `.env`, Secrets oder CI ohne Rückfrage; kein `git push --force`.
- Tests sind deterministisch: Zufall und Zeit werden injiziert, keine echten Netzwerkaufrufe, Datenbanken nur in `t.TempDir()`.

Der Test für das Import-Skript braucht `sqlite3` im `PATH` und wird sonst übersprungen. `.superpowers/` (Mockups, Arbeitsdateien) ist in der `.gitignore`.

## Wo man ansetzt

- **Neue Anordnung:** ein weiterer `case` in `layoutFor` (`internal/generator/layout.go`).
- **Neue Figurenart:** ein weiterer `case` in `drawFigure` (`internal/generator/figure.go`).
- **Neue Regel:** ein weiterer `case` in `evalRule` (`internal/generator/rules.go`).
- **JSON-Schnittstelle für Dritte oder Programme:** Der Generator ist bereits unabhängig von HTTP. Es fehlt nur eine zusätzliche Handler-Schicht neben `internal/web`.
- **Mehrere Nutzer und Anmeldung:** Die Web-Schicht ist so geschnitten, dass sich das nachrüsten lässt. Das ist das Ziel im Endausbau, bewusst noch nicht gebaut.
- **Zurückgestellte Punkte** aus der Liste oben sind kleine, in sich abgeschlossene Aufgaben.
- **Offene Fragen:** Ob `palette` später die Farbwahl steuern soll, und wie weitere Anordnungen im JSON heißen und welche Felder sie brauchen.
