-- Importiert Parameter-Varianten aus dem Ordner ./params in die Datenbank.
--
-- Voraussetzung: Die App wurde einmal gestartet, damit das Schema existiert.
-- Aufruf im Verzeichnis, das den Ordner params/ enthält:
--   sqlite3 go-02.db < scripts/import_params.sql
--
-- Jede *.json-Datei mit gültigem JSON wird eine Variante. Bereits importierte
-- Dateien (gleicher source_file) werden übersprungen, ungültiges JSON ebenso.
INSERT OR IGNORE INTO param_variants (name, params, source_file)
SELECT replace(replace(name, 'params/', ''), '.json', ''), CAST(data AS TEXT), name
FROM fsdir('params')
WHERE name LIKE '%.json' AND json_valid(CAST(data AS TEXT));
