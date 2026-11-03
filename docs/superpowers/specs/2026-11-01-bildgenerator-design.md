# go-02: Parametergesteuerter SVG-Bildgenerator – Design

Stand: 2026-11-01 · Status: Entwurf zur Freigabe

## Zweck

go-02 ist ein Werkzeug, mit dem der Eigentümer SVG-Bilder erzeugt, die mit geometrischen Figuren (Quadrate, regelmässige Dreiecke, Rechtecke, Polygone, Kreise, Linien) gefüllt sind. Ein JSON-Parametersatz steuert die Erzeugung. Das Werkzeug läuft zunächst lokal als Web-App mit SQLite, wächst über die Zeit um Funktionen und soll im Endausbau als Web-App auch Dritten das Generieren erlauben.

## Ziele

- Bilder aus JSON-Parametern erzeugen; Parameter in der Web-App ändern und neu generieren.
- Parameter-Varianten und erzeugte SVGs in SQLite speichern; jedes Bild verweist auf die Variante, aus der es entstand.
- Gespeicherte SVGs in einem eigenen Browserfenster anzeigen.
- Gut erweiterbar: neue Anordnungen und Figurenarten ohne Umbau.

## Nicht-Ziele (erste Version)

- Keine Anmeldung, keine Nutzerverwaltung, kein Betrieb im Netz.
- Kein Import von JSON-Dateien in der App (Import läuft ausschließlich per SQL-Skript außerhalb der App).
- Keine Anordnungen außer RANDOM, GRID und PDS.

## Anti-Ziele

- Eine Web-Schicht, in der die Generierungslogik steckt. Der Generator muss ohne HTTP nutzbar sein.
- Stille Fehler: ungültige Parameter dürfen nicht zu einem leeren oder falschen Bild führen.

## Rahmenbedingungen

- Go (Version laut `go.mod`), Modul `github.com/djw4u/go-02`, Standardbibliothek bevorzugt.
- Frontend: HTML und CSS, ausgeliefert über `html/template` mit `go:embed`. Weitere Frontend-Bausteine (z. B. `htmx`, ein JSON-Editor) nur bei Bedarf und einzeln begründet.
- Abhängigkeit SQLite: `modernc.org/sqlite` (reines Go, kein cgo). Entscheidung des Entwurfs-Autors, vom Eigentümer bestätigt. Alternative `mattn/go-sqlite3` (cgo) wurde verworfen.
- Projektregeln aus `CLAUDE.md` gelten: TDD, deterministische Tests, `context.Context`, kleine Interfaces, keine globalen veränderlichen Zustände.

## Ansätze

- **A: Server rendert die Seiten (Go + `html/template`, optional `htmx`)** – ein Programm, wenig JavaScript, passt zu „HTML/CSS“. Nachteil: Nutzung durch Dritte oder per Programm braucht später eine zusätzliche JSON-Schnittstelle.
- **B: JSON-API plus statisches HTML/CSS/JS-Frontend** – näher am Endausbau, aber zwei Schichten und viel JavaScript von Anfang an.

**Gewählt: A**, weil jetzt ein lokales Werkzeug mit HTML/CSS gebraucht wird und die Trennung des Generators den Weg zu API und Dritten offen hält.

## Architektur

```
cmd/go-02/            main: Verdrahtung, Konfiguration, Start des HTTP-Servers
internal/generator/   Parameter -> SVG; ohne HTTP und ohne Datenbank nutzbar
internal/store/       SQLite-Zugriff (Varianten, Bilder)
internal/web/         HTTP-Handler, Templates, statische Dateien
scripts/              import_params.sql
```

Abhängigkeitsrichtung: `web` nutzt `generator` und `store`; `generator` kennt weder `web` noch `store`.

## Datenmodell (Entwurf, später anpassbar)

```sql
CREATE TABLE IF NOT EXISTS param_variants (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  params      TEXT NOT NULL CHECK (json_valid(params)),
  source_file TEXT UNIQUE,                      -- NULL bei manuell angelegten Varianten
  parent_id   INTEGER REFERENCES param_variants(id),
  created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS images (
  id          INTEGER PRIMARY KEY,
  variant_id  INTEGER NOT NULL REFERENCES param_variants(id),
  svg         TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

Das Schema wird beim Start der App angelegt, falls es fehlt (`IF NOT EXISTS`).

## Import (außerhalb der App)

Ein SQL-Skript `scripts/import_params.sql` liest alle `*.json`-Dateien eines Ordners (`fsdir()` des `sqlite3`-Programms) und legt für gültiges JSON je Datei eine Variante an:

```sql
INSERT OR IGNORE INTO param_variants (name, params, source_file)
SELECT replace(replace(name, 'params/', ''), '.json', ''), CAST(data AS TEXT), name
FROM fsdir('params')
WHERE name LIKE '%.json' AND json_valid(CAST(data AS TEXT));
```

Aufruf: `sqlite3 go-02.db < scripts/import_params.sql`. Ein erneuter Lauf überspringt bereits importierte Dateien (`source_file` ist eindeutig). Ungültiges JSON wird still übersprungen. Auf dem Entwicklungsrechner geprüft: `sqlite3` 3.54.0, `fsdir()` und `json_valid()` verfügbar. Die App selbst kennt weder JSON-Dateien noch den Import-Ordner.

## Parameterformat

Beispiel einer älteren Variante (Ausgangspunkt, nicht abschließend):

```json
{"params":{"nr_of_figures":32,
 "figures":["rc",[["rect","polygon","circle","line"]]],
 "x":["ri",[-20,120]],"y":["ri",[-20,120]],
 "fill":["rc",[["#4353D3","#32D761","#D5C734","#D532CE","#D83144"]]],
 "transform":["rc",[[0,180]]],"structure":"RANDOM",
 "opacity":["rc",[[0.6,0.4]]],"height":["rc",[[40,20]]],"width":["rc",[[50,10]]],
 "palette":["#4353D3","#32D761","#D5C734","#D532CE","#D83144"],
 "stroke":["rc",[["#4353D3","#32D761","#D5C734","#D532CE","#D83144"]]],
 "stroke-linecap":["rc",[["round"]]],"stroke-width":["rc",[[6,4]]],
 "stroke-opacity":["rc",[[0.8,0.6]]]}}
```

Lesart (Annahmen des Autors, bis zur Freigabe zu bestätigen):

- Regel-Felder sind Paare `[Regelname, Argumente]`. `"ri"`: zufällige Ganzzahl im Bereich `[min, max]`. `"rc"`: zufällige Auswahl aus der Liste, die das erste Argument ist (daher die doppelte Verschachtelung).
- Alle anderen Felder sind feste Werte (`nr_of_figures`, `structure`, `palette`).
- Pro Figur wird jedes Regel-Feld einzeln ausgewürfelt.
- `structure` wählt die Anordnung: `RANDOM`, `GRID`, `PDS` (Poisson Disc Sampling, zufällig mit Mindestabstand).
- `GRID` und `PDS` brauchen zusätzliche JSON-Felder (z. B. Zeilen/Spalten, Mindestabstand); deren Namen und Bedeutung legt der Plan fest.

## Generator

- Eingabe: Parameter (aus JSON) und eine injizierte Zufallsquelle. Ausgabe: SVG-Text.
- Zufallsquelle ist ein Interface bzw. ein übergebener Seed, damit Tests wiederholbare Ergebnisse liefern. In der App entsteht pro „Generieren“ ein neues, anderes Bild.
- Anordnungen (`RANDOM`, `GRID`, `PDS`) und Figurenarten sind austauschbar (registriert oder per Interface), neue kommen ohne Umbau des Kerns dazu.
- Ungültige Parameter (unbekannte Regel, unbekannte Anordnung, falsche Argumente, nicht lesbares JSON) führen zu einem Fehler mit Kontext, nie zu einem stillen Ergebnis.

## Verhalten der Web-App

- **Erzeugen:** „Generieren“ erzeugt ein Bild aus den aktuellen Parametern und zeigt es als Vorschau an. Das Bild ist dabei nicht gespeichert. Jedes Generieren liefert ein neues, anderes Bild.
- **Bild speichern:** Ein manueller Klick „SVG speichern“ legt das gezeigte Bild in `images` ab, verknüpft mit der gewählten Variante.
- **Variante speichern:** Geänderte Parameter bleiben ungespeichert, bis „Neue Variante“ gewählt wird. Die neue Variante bekommt `source_file = NULL` und `parent_id` auf die Ausgangsvariante.
- **Einzelbild-Fenster:** Gespeicherte Bilder sind unter eigener Adresse mit `Content-Type: image/svg+xml` abrufbar und öffnen sich im eigenen Browserfenster.

## Oberfläche (Layout B)

Zwei Seiten mit gemeinsamer Navigation:

- **Werkstatt:** Variantenauswahl, JSON-Editor, „Generieren“, Vorschau, „SVG speichern“, „Neue Variante“.
- **Galerie:** Raster der gespeicherten Bilder, Filter nach Variante, Klick öffnet das Bild im eigenen Fenster.

Umsetzung mit HTML und CSS; Frontend-Bausteine nur bei Bedarf.

## Entscheidungen des Autors (nicht im Gespräch bestätigt)

- Aufbau der Pakete wie oben (`cmd/go-02`, `internal/generator|store|web`).
- Tabellenspalte `parent_id` und `created_at`.
- Schema-Anlage beim App-Start.
- Die Spalte `params` speichert die komplette JSON-Datei inklusive des äußeren Schlüssels `"params"`; der Generator liest daraus den Inhalt von `params`.
- Beim Speichern einer neuen Variante gibt der Nutzer einen Namen ein; Vorschlag: `<Name der Ausgangsvariante> (neu)`.
- Löschen und Umbenennen von Varianten und Bildern gehört nicht zur ersten Version.
- Ein gespeichertes Bild bleibt unverändert; der verwendete Zufallswert wird nicht gespeichert, da das SVG selbst das Ergebnis ist.

## Dem Plan überlassen

- Genaue Feldnamen und Bedeutung der zusätzlichen Felder für `GRID` und `PDS`.
- Bedeutung von `palette` (nur Metadaten oder wirksam für die Farbwahl). `colorid` entfällt.
- Größe und `viewBox` des erzeugten SVGs (Beispielwerte `x`/`y` von -20 bis 120 deuten auf eine 100×100-Fläche hin).
- Wie Figuren je Art aus den Feldern gezeichnet werden (z. B. Polygonpunkte, Linienendpunkte).
- Fehlerdarstellung in der Oberfläche, Name und Ort der Datenbankdatei, Port und Konfiguration.
- Auswahl und Begründung von Frontend-Bausteinen (JSON-Editor, `htmx`).
