# SVG-Bildgenerator Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lokale Web-App, die aus JSON-Parametern SVG-Bilder mit geometrischen Figuren erzeugt, Varianten und Bilder in SQLite speichert und gespeicherte Bilder im eigenen Fenster zeigt.

**Architecture:** Server-gerenderte Seiten (`html/template`, `go:embed`). `internal/generator` (Parameter → SVG) ist unabhängig von HTTP und Datenbank, `internal/store` kapselt SQLite (`modernc.org/sqlite`), `internal/web` verdrahtet beides in Handlern, `cmd/go-02` startet den Server.

**Tech Stack:** Go (Version laut `go.mod`), Standardbibliothek, `modernc.org/sqlite`, HTML/CSS ohne Frontend-Bibliotheken.

**Spec:** `docs/superpowers/specs/2026-11-01-bildgenerator-design.md`

## Global Constraints

- Standardbibliothek bevorzugt; einzige neue Abhängigkeit ist `modernc.org/sqlite` (reines Go, kein cgo). `CGO_ENABLED=0 go build ./...` muss laufen.
- Frontend nur HTML und CSS über `html/template` mit `go:embed`; keine Frontend-Bibliotheken in dieser Version (JSON-Editor = `<textarea>`, kein `htmx`).
- `context.Context` als erster Parameter bei blockierenden Operationen; Fehler mit `fmt.Errorf("...: %w", err)` weiterreichen; keine globalen veränderlichen Zustände; exportierte Bezeichner mit Doc-Kommentar.
- Tests deterministisch: Zufall nur über `generator.Source` (geseedetes `rand.New(rand.NewPCG(1, 2))` aus `math/rand/v2`), keine Netzwerkaufrufe, DB nur in `t.TempDir()`.
- Ungültige Parameter führen immer zu einem Fehler mit Kontext, nie zu einem stillen oder leeren Bild.
- Pro Task: eigener Branch `feature/<kurzname>` von aktuellem `main` (nach Merge des Vorgängers), ein Commit, PR. Nie auf `main` committen. Abschluss jedes Tasks: `go build ./...`, `go vet ./...`, `gofmt -l .` (leer), `go test ./...` grün.

## Entscheidungen des Plans (vom Spec offengelassen)

- SVG-Fläche `viewBox="0 0 100 100"`; Figuren dürfen über den Rand ragen (Wurzel-`<svg>` schneidet ab).
- `x`/`y` sind der **Mittelpunkt** einer Figur. `transform` ist ein Drehwinkel in Grad um diesen Mittelpunkt (`rotate(a x y)`).
- Figurenarten: `rect`, `square`, `circle`, `line`, `polygon`, `triangle`. `width` ist bei allen Pflicht; `height` nur bei `rect`. `square`: Seite = `width`. `circle`: Radius = `width/2`. `line`: horizontal von `x-width/2` bis `x+width/2`, danach gedreht. `polygon`: regelmäßiges n-Eck mit Umkreisradius `width/2`, erster Punkt oben, `sides` (Regel oder fester Wert, Standard 5, mindestens 3). `triangle` = `polygon` mit `sides` 3.
- Optionale Attribute werden nur ausgegeben, wenn das Feld vorhanden ist, in dieser Reihenfolge: `fill`, `opacity`, `stroke`, `stroke-width`, `stroke-opacity`, `stroke-linecap`, `transform`. `line` bekommt kein `fill`.
- Zahlen im SVG: höchstens 3 Nachkommastellen, nachgestellte Nullen entfallen (`50`, `12.5`).
- `palette` ist reine Metadaten und beeinflusst das Zeichnen nicht. `colorid` entfällt komplett (wird nicht gelesen, auch nicht in Beispielen).
- `GRID`: feste Felder `rows`, `columns` (ganze Zahlen ≥ 1), Zellmittelpunkte zeilenweise über die Fläche 0..100; Anzahl = `rows*columns`, `nr_of_figures`, `x`, `y` werden ignoriert. `PDS`: feste Felder `nr_of_figures` und `min_distance` (> 0), Punkte in 0..100 per Dart-Throwing mit höchstens `100*nr_of_figures` Versuchen; findet es weniger Punkte, wird ohne Fehler die kleinere Zahl geliefert. `RANDOM`: `x`, `y` je Figur aus Regeln, Anzahl `nr_of_figures` (≥ 1).
- Web: Vorschau als `<img src="data:image/svg+xml;base64,…">`; der Bildinhalt reist als verstecktes Formularfeld `svg` zum Speichern (Server prüft: beginnt mit `<svg `, höchstens 1 MiB). Fehler in der Oberfläche als Meldung über der Seite mit Status 422.
- Standardwerte: Datenbankdatei `go-02.db`, Adresse `127.0.0.1:8080`, Flags `-db` und `-addr`.

## Review Focus

- Ungültiges JSON im Editor: Meldung statt Absturz, nichts wird gespeichert (Task 6).
- Leere Auswahlliste `["rc",[[]]]` und `["ri",[10,5]]`: Fehler statt Panic (Task 2).
- PDS mit unmöglicher Dichte (`min_distance` größer als die Fläche): endet schnell, liefert 1 Figur (Task 4).
- Gespeicherte SVGs sind fremder Inhalt: Auslieferung mit CSP und `nosniff`, Vorschau nur als `<img>` (Task 6).
- Unbekannte oder nicht numerische Bild-/Varianten-IDs: 404 (Task 6).

---

### Task 1: Store (SQLite)

**Files:**
- Modify: `go.mod`, `go.sum` (nur über `go get modernc.org/sqlite` und `go mod tidy`)
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Produces:
```go
type Variant struct {
    ID         int64
    Name       string
    Params     string // komplette JSON-Datei inkl. äußerem "params"
    SourceFile *string
    ParentID   *int64
    CreatedAt  time.Time
}
type Image struct {
    ID, VariantID int64
    SVG           string
    CreatedAt     time.Time
}
var ErrNotFound = errors.New("store: not found")
func Open(ctx context.Context, path string) (*Store, error) // legt Schema aus dem Spec per IF NOT EXISTS an, Fremdschlüssel aktiv
func (s *Store) Close() error
func (s *Store) ListVariants(ctx context.Context) ([]Variant, error)                     // nach id aufsteigend
func (s *Store) GetVariant(ctx context.Context, id int64) (Variant, error)              // ErrNotFound
func (s *Store) CreateVariant(ctx context.Context, name, params string, parentID int64) (int64, error) // parentID 0 = NULL
func (s *Store) SaveImage(ctx context.Context, variantID int64, svg string) (int64, error)
func (s *Store) ListImages(ctx context.Context, variantID int64) ([]Image, error)       // 0 = alle; neueste (höchste id) zuerst
func (s *Store) GetImage(ctx context.Context, id int64) (Image, error)                  // ErrNotFound
```

- [ ] **Step 1: Failing tests** in `store_test.go` (je Test eigene DB in `t.TempDir()`): `TestOpen_IsIdempotent` (zweimal öffnen ohne Fehler); `TestCreateVariant_RoundTrip` (Name, Params, `ParentID` gesetzt bzw. `nil` bei 0, `SourceFile` `nil`); `TestCreateVariant_RejectsInvalidJSON` (Fehler wegen `CHECK json_valid`); `TestSaveImage_UnknownVariantFails` (Fremdschlüssel); `TestGetVariant_NotFound` und `TestGetImage_NotFound` (`errors.Is(err, ErrNotFound)`); `TestListImages_FilterAndOrder` (zwei Varianten, Filter liefert nur deren Bilder, Reihenfolge neueste zuerst, `0` liefert alle).
- [ ] **Step 2:** `go test ./internal/store/` → FAIL (Paket fehlt).
- [ ] **Step 3:** `go get modernc.org/sqlite && go mod tidy`; `store.go` mit obigen Signaturen implementieren. Treiber per `import _ "modernc.org/sqlite"`, Name `"sqlite"`, DSN `file:<pfad>?_pragma=foreign_keys(1)`. `created_at` wird als Text `2006-01-02 15:04:05` (UTC) gelesen.
- [ ] **Step 4:** `go test ./internal/store/` und `CGO_ENABLED=0 go build ./...` → PASS.
- [ ] **Step 5:** Branch `feature/store`, Commit `Feature: SQLite-Store für Varianten und Bilder`, PR.

### Task 2: Generator – Parameter und Regeln

**Files:**
- Create: `internal/generator/params.go`, `internal/generator/rules.go`
- Test: `internal/generator/params_test.go`, `internal/generator/rules_test.go`

**Interfaces:**
- Produces:
```go
// Source ist die injizierte Zufallsquelle; *rand.Rand aus math/rand/v2 erfüllt sie.
type Source interface {
    IntN(n int) int
    Float64() float64
}
type Params struct{ /* unexportiert */ }
func Parse(raw []byte) (Params, error)                                  // erwartet {"params":{...}}
func (p Params) Has(name string) bool
func (p Params) Value(name string, src Source) (any, error)             // wertet Regel aus oder liefert festen Wert
func (p Params) FloatValue(name string, src Source) (float64, error)
func (p Params) StringValue(name string, src Source) (string, error)
func (p Params) FixedInt(name string) (int, error)                      // nur feste Werte, ganze Zahl
func (p Params) FixedFloat(name string) (float64, error)                // nur feste Werte
```
- Regel erkannt: JSON-Array der Länge 2 mit String an Stelle 0 und Array an Stelle 1 (`palette` ist deshalb nie eine Regel). `"ri"` mit `[min,max]` → Ganzzahl, beide Grenzen inklusive. `"rc"` mit `[[a,b,…]]` → ein Element der ersten Liste. Alles andere ist ein fester Wert.

- [ ] **Step 1: Failing tests:** `TestParse_Errors` (ungültiges JSON, fehlender Schlüssel `params`, `params` kein Objekt – jeweils Fehler); `TestValue_FixedPassesThrough` (`"structure":"RANDOM"`, `nr_of_figures` 32); `TestRI_WithinInclusiveBounds` (`["ri",[-20,120]]`, 1000 Ziehungen, alle in [-20,120]); `TestRI_SingleValue` (`[5,5]` → 5); `TestRI_MinGreaterMaxFails` (`[10,5]` → Fehler); `TestRC_PicksOnlyListed` (`[[0,180]]` → nur 0 oder 180, beide kommen vor); `TestRC_EmptyListFails` (`[[]]` → Fehler); `TestUnknownRuleFails` (`["xx",[1]]` → Fehler enthält `xx`); `TestMissingFieldFails` (Fehler enthält den Feldnamen); `TestSameSeedSameValues`; `TestPaletteIsNotARule` (5 Strings bleiben fester Wert).
- [ ] **Step 2:** `go test ./internal/generator/` → FAIL.
- [ ] **Step 3:** `Parse` mit `json.Unmarshal` in `map[string]any` implementieren (Zahlen als `float64`); Regelauswertung in `rules.go`.
- [ ] **Step 4:** `go test ./internal/generator/` → PASS.
- [ ] **Step 5:** Branch `feature/generator-regeln`, Commit `Feature: Parameter-Parsing und Zufallsregeln`, PR.

### Task 3: Generator – Figuren

**Files:**
- Create: `internal/generator/figure.go`
- Test: `internal/generator/figure_test.go`

**Interfaces:**
- Consumes: `Params`, `Source` aus Task 2.
- Produces: `func drawFigure(kind string, x, y float64, p Params, src Source) (string, error)` – liefert ein SVG-Element nach den Plan-Entscheidungen oben; unbekannte `kind` → Fehler mit Namen; fehlendes `width` (bzw. `height` bei `rect`) → Fehler mit Feldnamen.

- [ ] **Step 1: Failing tests** (alle mit festen Werten, exakte Strings): `TestDrawFigure_Rect` → `<rect x="30" y="40" width="40" height="20" fill="#4353D3" transform="rotate(180 50 50)"/>` bei Mittelpunkt (50,50), width 40, height 20, fill `#4353D3`, transform 180; `TestDrawFigure_Square` (width 10 bei (50,50) → `<rect x="45" y="45" width="10" height="10"/>`); `TestDrawFigure_Circle` (width 10 → `<circle cx="50" cy="50" r="5"/>`); `TestDrawFigure_Line` (width 20, stroke `#000`, stroke-width 2 → `<line x1="40" y1="50" x2="60" y2="50" stroke="#000" stroke-width="2"/>`, kein `fill`); `TestDrawFigure_Triangle` (width 20 → `<polygon points="50,40 58.66,55 41.34,55"/>`); `TestDrawFigure_PolygonSides` (`sides` 4, width 20 → `points="50,40 60,50 50,60 40,50"`); `TestDrawFigure_AttributeOrder` (alle optionalen Felder gesetzt → Reihenfolge `fill, opacity, stroke, stroke-width, stroke-opacity, stroke-linecap, transform`); `TestDrawFigure_Errors` (unbekannte Art, fehlendes `width`, `rect` ohne `height`, `sides` < 3).
- [ ] **Step 2:** `go test ./internal/generator/ -run DrawFigure` → FAIL.
- [ ] **Step 3:** `drawFigure` per `switch kind` implementieren; Zahlenformatierung als kleine Hilfsfunktion `num(float64) string` (3 Nachkommastellen, getrimmt). Regeln je Figur über `FloatValue`/`StringValue` auswerten.
- [ ] **Step 4:** `go test ./internal/generator/` → PASS.
- [ ] **Step 5:** Branch `feature/generator-figuren`, Commit `Feature: SVG-Figuren im Generator`, PR.

### Task 4: Generator – Anordnungen und Generate

**Files:**
- Create: `internal/generator/layout.go`, `internal/generator/generator.go`
- Test: `internal/generator/layout_test.go`, `internal/generator/generator_test.go`

**Interfaces:**
- Consumes: `Params`, `Source`, `drawFigure`.
- Produces:
```go
// Generate erzeugt aus den Parametern ein SVG (viewBox 0 0 100 100).
func Generate(p Params, src Source) (string, error)
type point struct{ X, Y float64 }
type layout func(p Params, src Source) ([]point, error)
func layoutFor(structure string) (layout, bool) // "RANDOM", "GRID", "PDS" per switch
```
- Ausgabe: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` + Elemente (je Figur: Art per `StringValue("figures", src)`) + `</svg>`. Neue Anordnung = ein weiterer `case` in `layoutFor`.

- [ ] **Step 1: Failing tests:** `TestGenerate_Random_CountAndKinds` (Beispiel aus dem Spec mit `nr_of_figures` 32 → genau 32 Figur-Elemente, nur Arten aus `figures`; zählen per `regexp`); `TestGenerate_SameSeedSameSVG` und `TestGenerate_DifferentSeedDiffers`; `TestGenerate_Grid` (rows 2, columns 2, feste Art `circle` → `cx="25" cy="25"`, `"75" "25"`, `"25" "75"`, `"75" "75"` in dieser Reihenfolge); `TestGenerate_PDS_MinDistance` (n 20, `min_distance` 10 → ≤ 20 Figuren, alle Paare Abstand ≥ 10, Mittelpunkte aus `cx`/`cy` gelesen); `TestGenerate_PDS_ImpossibleDensityTerminates` (`min_distance` 200, n 10 → genau 1 Figur, kein Fehler); `TestGenerate_Errors` (fehlendes `structure`, unbekannte Anordnung nennt den Namen, `nr_of_figures` 0, GRID ohne `rows`, PDS mit `min_distance` ≤ 0, ungültige Figurenart).
- [ ] **Step 2:** `go test ./internal/generator/` → FAIL.
- [ ] **Step 3:** Anordnungen und `Generate` implementieren; Zahlenfelder `rows`, `columns`, `nr_of_figures` über `FixedInt`, `min_distance` über `FixedFloat`.
- [ ] **Step 4:** `go test ./internal/generator/` → PASS.
- [ ] **Step 5:** Branch `feature/generator-anordnungen`, Commit `Feature: Anordnungen RANDOM, GRID, PDS und Generate`, PR.

### Task 5: Import-Skript

**Files:**
- Create: `scripts/import_params.sql` (Statement aus dem Spec, Kopfkommentar: App muss einmal gestartet sein, damit das Schema existiert; Aufruf `sqlite3 go-02.db < scripts/import_params.sql` im Verzeichnis, das `params/` enthält)
- Test: `internal/store/import_script_test.go`

**Interfaces:**
- Consumes: `store.Open`, `ListVariants`.

- [ ] **Step 1: Failing test** `TestImportScript`: überspringt per `t.Skip`, wenn `sqlite3` nicht im `PATH`. Legt in `t.TempDir()` einen Ordner `params/` mit `a.json` (`{"params":{"nr_of_figures":1}}`), `b.json` (gültig) und `kaputt.json` (`{`) sowie `notes.txt` an, erzeugt die DB über `store.Open` (dann schließen), führt das Skript über `exec.Command("sqlite3", db)` mit dem Skript als stdin aus (Arbeitsverzeichnis = Temp-Ordner). Erwartung: genau 2 Varianten, Namen `a` und `b`, `SourceFile` `params/a.json`; zweiter Lauf ändert die Anzahl nicht.
- [ ] **Step 2:** `go test ./internal/store/ -run ImportScript` → FAIL (Skript fehlt).
- [ ] **Step 3:** Skript anlegen.
- [ ] **Step 4:** `go test ./internal/store/` → PASS (oder SKIP ohne `sqlite3`, dann im Bericht nennen).
- [ ] **Step 5:** Branch `feature/import-skript`, Commit `Feature: SQL-Skript zum Import der Parameter`, PR.

### Task 6: Web-Schicht

**Files:**
- Create: `internal/web/web.go`, `internal/web/templates/{layout,werkstatt,galerie}.html`, `internal/web/static/style.css`
- Test: `internal/web/web_test.go`

**Interfaces:**
- Consumes: `generator.Parse`, `generator.Generate`, `generator.Source`, `store.Variant`, `store.Image`, `store.ErrNotFound`.
- Produces:
```go
// Store ist die vom Web-Paket genutzte Teilmenge von *store.Store.
type Store interface {
    ListVariants(ctx context.Context) ([]store.Variant, error)
    GetVariant(ctx context.Context, id int64) (store.Variant, error)
    CreateVariant(ctx context.Context, name, params string, parentID int64) (int64, error)
    SaveImage(ctx context.Context, variantID int64, svg string) (int64, error)
    ListImages(ctx context.Context, variantID int64) ([]store.Image, error)
    GetImage(ctx context.Context, id int64) (store.Image, error)
}
func New(s Store, newSource func() generator.Source) (http.Handler, error) // Templates aus go:embed
```
- Routen (Go-1.22-`ServeMux`-Muster): `GET /` → 303 `/werkstatt`; `GET /werkstatt?variant=ID`; `POST /werkstatt/generate` (Felder `variant_id`, `params`); `POST /werkstatt/save-image` (`variant_id`, `svg`) → 303 `/galerie?variant=ID`; `POST /werkstatt/save-variant` (`variant_id` als Ausgangsvariante, `name`, `params`) → 303 `/werkstatt?variant=NEU`; `GET /galerie?variant=ID` (Raster aus `<img src="/bilder/{id}.svg">`, jeweils in `<a target="_blank">`); `GET /bilder/{id}.svg`; `GET /static/style.css`.
- Bild-Antwort: `Content-Type: image/svg+xml`, `Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'`, `X-Content-Type-Options: nosniff`.

- [ ] **Step 1: Failing tests** (`httptest`, echter Store in `t.TempDir()`, `newSource` liefert `rand.New(rand.NewPCG(1, 2))`): `TestWerkstatt_ListsVariantsAndShowsParams` (Parameter HTML-escaped im `<textarea>`); `TestGenerate_Valid` (200, Body enthält `data:image/svg+xml;base64,` und das versteckte Feld `svg`; nichts in `images`); `TestGenerate_InvalidJSON` (422, Fehlermeldung im Body, kein `data:image`); `TestSaveImage` (303 auf `/galerie?variant=ID`, genau ein Bild gespeichert); `TestSaveImage_RejectsNonSVGAndUnknownVariant` (`svg` ohne Präfix `<svg ` → 422; unbekannte `variant_id` → 404); `TestSaveVariant` (303, neue Variante mit `ParentID` und `SourceFile == nil`); `TestSaveVariant_RejectsEmptyNameAndInvalidParams` (422, nichts angelegt); `TestGalerie_FilterByVariant`; `TestBild_Headers` (Typ, CSP, `nosniff`, Body = gespeichertes SVG); `TestBild_NotFound` (unbekannte und nicht numerische ID → 404); `TestRootRedirectsAndStaticCSS`.
- [ ] **Step 2:** `go test ./internal/web/` → FAIL.
- [ ] **Step 3:** Handler und Templates implementieren; Formulare als einfache POST-Formulare, gemeinsame Navigation (Werkstatt | Galerie) im Layout; Body-Limit 1 MiB per `http.MaxBytesReader`.
- [ ] **Step 4:** `go test ./internal/web/` → PASS.
- [ ] **Step 5:** Branch `feature/web`, Commit `Feature: Web-Oberfläche mit Werkstatt und Galerie`, PR.

### Task 7: Programmstart und README

**Files:**
- Create: `cmd/go-02/main.go`
- Modify: `README.md`
- Test: `cmd/go-02/main_test.go`

**Interfaces:**
- Consumes: `store.Open`, `web.New`, `generator.Source`.
- Produces: `func run(ctx context.Context, args []string) error` – parst `-db` und `-addr`, öffnet den Store, startet den Server, fährt bei Abbruch des `ctx` per `Shutdown` geordnet herunter und gibt dann `nil` zurück. `main` ruft `run` mit `signal.NotifyContext(os.Interrupt)`. `newSource` liefert pro Aufruf `rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))`.

- [ ] **Step 1: Failing tests:** `TestRun_StartsAndStopsOnCancel` (`-addr 127.0.0.1:0`, `-db` in `t.TempDir()`, `ctx` nach kurzem Warten abbrechen → `run` gibt `nil` zurück); `TestRun_InvalidFlagFails`.
- [ ] **Step 2:** `go test ./cmd/go-02/` → FAIL.
- [ ] **Step 3:** `main.go` implementieren; README um Start (`go run ./cmd/go-02`), Flags und Import (`sqlite3 go-02.db < scripts/import_params.sql`) ergänzen.
- [ ] **Step 4:** Alle Prüfungen: `go build ./...`, `go vet ./...`, `gofmt -l .` (leer), `go test ./...`, `CGO_ENABLED=0 go build ./...`. Manuell: App starten, `curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/werkstatt` → `200`.
- [ ] **Step 5:** Branch `feature/main`, Commit `Feature: Programmstart und README`, PR.
