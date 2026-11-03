// Package store kapselt den SQLite-Zugriff auf Parameter-Varianten und
// erzeugte Bilder.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registriert den Treiber "sqlite"
)

// ErrNotFound wird zurückgegeben, wenn ein gesuchter Datensatz nicht existiert.
var ErrNotFound = errors.New("store: not found")

const timeLayout = "2006-01-02 15:04:05"

const schema = `
CREATE TABLE IF NOT EXISTS param_variants (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  params      TEXT NOT NULL CHECK (json_valid(params)),
  source_file TEXT UNIQUE,
  parent_id   INTEGER REFERENCES param_variants(id),
  created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS images (
  id          INTEGER PRIMARY KEY,
  variant_id  INTEGER NOT NULL REFERENCES param_variants(id),
  svg         TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

// Variant ist ein gespeicherter Parametersatz.
type Variant struct {
	ID         int64
	Name       string
	Params     string // komplette JSON-Datei inkl. äußerem "params"
	SourceFile *string
	ParentID   *int64
	CreatedAt  time.Time
}

// Image ist ein gespeichertes SVG, das aus einer Variante entstanden ist.
type Image struct {
	ID, VariantID int64
	SVG           string
	CreatedAt     time.Time
}

// Store ist der Zugriff auf die SQLite-Datenbank.
type Store struct {
	db *sql.DB
}

// Open öffnet die Datenbank unter path und legt das Schema an, falls es fehlt.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", path, err)
	}
	// Eine Verbindung serialisiert Schreibzugriffe und vermeidet SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close schließt die Datenbank.
func (s *Store) Close() error {
	return s.db.Close()
}

// ListVariants liefert alle Varianten nach id aufsteigend.
func (s *Store) ListVariants(ctx context.Context) ([]Variant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, params, source_file, parent_id, created_at FROM param_variants ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()

	variants := []Variant{}
	for rows.Next() {
		v, err := scanVariant(rows)
		if err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	return variants, nil
}

// GetVariant liefert die Variante mit der gegebenen id oder ErrNotFound.
func (s *Store) GetVariant(ctx context.Context, id int64) (Variant, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, params, source_file, parent_id, created_at FROM param_variants WHERE id = ?`, id)
	v, err := scanVariant(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Variant{}, ErrNotFound
	}
	return v, err
}

// CreateVariant legt eine neue Variante an. parentID 0 bedeutet: keine
// Ausgangsvariante.
func (s *Store) CreateVariant(ctx context.Context, name, params string, parentID int64) (int64, error) {
	parent := sql.NullInt64{Int64: parentID, Valid: parentID != 0}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO param_variants (name, params, parent_id) VALUES (?, ?, ?)`, name, params, parent)
	if err != nil {
		return 0, fmt.Errorf("create variant: %w", err)
	}
	return res.LastInsertId()
}

// SaveImage speichert ein SVG zur Variante variantID.
func (s *Store) SaveImage(ctx context.Context, variantID int64, svg string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO images (variant_id, svg) VALUES (?, ?)`, variantID, svg)
	if err != nil {
		return 0, fmt.Errorf("save image: %w", err)
	}
	return res.LastInsertId()
}

// ListImages liefert die Bilder einer Variante, neueste zuerst. variantID 0
// liefert alle Bilder.
func (s *Store) ListImages(ctx context.Context, variantID int64) ([]Image, error) {
	query := `SELECT id, variant_id, svg, created_at FROM images`
	args := []any{}
	if variantID != 0 {
		query += ` WHERE variant_id = ?`
		args = append(args, variantID)
	}
	query += ` ORDER BY id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	images := []Image{}
	for rows.Next() {
		img, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}
	return images, nil
}

// GetImage liefert das Bild mit der gegebenen id oder ErrNotFound.
func (s *Store) GetImage(ctx context.Context, id int64) (Image, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, variant_id, svg, created_at FROM images WHERE id = ?`, id)
	img, err := scanImage(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Image{}, ErrNotFound
	}
	return img, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanVariant(sc scanner) (Variant, error) {
	var (
		v       Variant
		source  sql.NullString
		parent  sql.NullInt64
		created string
	)
	if err := sc.Scan(&v.ID, &v.Name, &v.Params, &source, &parent, &created); err != nil {
		return Variant{}, fmt.Errorf("scan variant: %w", err)
	}
	if source.Valid {
		v.SourceFile = &source.String
	}
	if parent.Valid {
		v.ParentID = &parent.Int64
	}
	t, err := time.ParseInLocation(timeLayout, created, time.UTC)
	if err != nil {
		return Variant{}, fmt.Errorf("parse created_at %q: %w", created, err)
	}
	v.CreatedAt = t
	return v, nil
}

func scanImage(sc scanner) (Image, error) {
	var (
		img     Image
		created string
	)
	if err := sc.Scan(&img.ID, &img.VariantID, &img.SVG, &created); err != nil {
		return Image{}, fmt.Errorf("scan image: %w", err)
	}
	t, err := time.ParseInLocation(timeLayout, created, time.UTC)
	if err != nil {
		return Image{}, fmt.Errorf("parse created_at %q: %w", created, err)
	}
	img.CreatedAt = t
	return img, nil
}
