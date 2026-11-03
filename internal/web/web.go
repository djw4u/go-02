// Package web stellt die HTML-Oberfläche (Werkstatt und Galerie) und die
// Auslieferung gespeicherter SVG-Bilder bereit.
package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/djw4u/go-01/internal/generator"
	"github.com/djw4u/go-01/internal/store"
)

//go:embed templates/*.html static/style.css
var assets embed.FS

const (
	maxFormBytes = 2 << 20 // Obergrenze für einen Formular-Body
	maxSVGBytes  = 1 << 20 // Obergrenze für ein zu speicherndes SVG
)

// Store ist die vom Web-Paket genutzte Teilmenge von *store.Store.
type Store interface {
	ListVariants(ctx context.Context) ([]store.Variant, error)
	GetVariant(ctx context.Context, id int64) (store.Variant, error)
	CreateVariant(ctx context.Context, name, params string, parentID int64) (int64, error)
	SaveImage(ctx context.Context, variantID int64, svg string) (int64, error)
	ListImages(ctx context.Context, variantID int64) ([]store.Image, error)
	GetImage(ctx context.Context, id int64) (store.Image, error)
}

type server struct {
	store     Store
	newSource func() generator.Source
	pages     map[string]*template.Template
}

// page sind die Daten, die die Templates erhalten.
type page struct {
	Title    string
	Active   string
	Error    string
	Variants []store.Variant
	Selected *store.Variant
	Params   string
	Name     string
	Preview  template.URL
	SVG      string
	Images   []imageView
	FilterID int64
}

type imageView struct {
	ID          int64
	VariantName string
	Created     string
}

// New baut den HTTP-Handler der Anwendung. newSource liefert pro Aufruf eine
// frische Zufallsquelle für das Generieren.
func New(s Store, newSource func() generator.Source) (http.Handler, error) {
	srv := &server{store: s, newSource: newSource, pages: map[string]*template.Template{}}
	for _, name := range []string{"werkstatt", "galerie"} {
		t, err := template.ParseFS(assets, "templates/layout.html", "templates/"+name+".html")
		if err != nil {
			return nil, fmt.Errorf("parse template %q: %w", name, err)
		}
		srv.pages[name] = t
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/werkstatt", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /werkstatt", srv.werkstatt)
	mux.HandleFunc("POST /werkstatt/generate", srv.generate)
	mux.HandleFunc("POST /werkstatt/save-image", srv.saveImage)
	mux.HandleFunc("POST /werkstatt/save-variant", srv.saveVariant)
	mux.HandleFunc("GET /galerie", srv.galerie)
	mux.HandleFunc("GET /bilder/{file}", srv.bild)
	mux.Handle("GET /static/", http.FileServerFS(assets))
	return mux, nil
}

func (s *server) werkstatt(w http.ResponseWriter, r *http.Request) {
	data, err := s.workbench(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	if idStr := r.URL.Query().Get("variant"); idStr != "" {
		v, ok := s.variant(w, r, idStr)
		if !ok {
			return
		}
		data.selectVariant(v)
	} else if len(data.Variants) > 0 {
		data.selectVariant(data.Variants[0])
	}
	s.render(w, http.StatusOK, "werkstatt", data)
}

func (s *server) generate(w http.ResponseWriter, r *http.Request) {
	if !s.parseForm(w, r) {
		return
	}
	v, ok := s.variant(w, r, r.PostFormValue("variant_id"))
	if !ok {
		return
	}
	data, err := s.workbench(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	data.selectVariant(v)
	data.Params = r.PostFormValue("params")

	params, err := generator.Parse([]byte(data.Params))
	if err != nil {
		data.Error = err.Error()
		s.render(w, http.StatusUnprocessableEntity, "werkstatt", data)
		return
	}
	svg, err := generator.Generate(params, s.newSource())
	if err != nil {
		data.Error = err.Error()
		s.render(w, http.StatusUnprocessableEntity, "werkstatt", data)
		return
	}
	data.SVG = svg
	data.Preview = template.URL("data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg)))
	s.render(w, http.StatusOK, "werkstatt", data)
}

func (s *server) saveImage(w http.ResponseWriter, r *http.Request) {
	if !s.parseForm(w, r) {
		return
	}
	v, ok := s.variant(w, r, r.PostFormValue("variant_id"))
	if !ok {
		return
	}
	svg := r.PostFormValue("svg")
	if !strings.HasPrefix(svg, "<svg ") || len(svg) > maxSVGBytes {
		data, err := s.workbench(r.Context())
		if err != nil {
			s.serverError(w, err)
			return
		}
		data.selectVariant(v)
		data.Error = "Das Bild ist kein gültiges SVG oder zu groß und wurde nicht gespeichert."
		s.render(w, http.StatusUnprocessableEntity, "werkstatt", data)
		return
	}
	if _, err := s.store.SaveImage(r.Context(), v.ID, svg); err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/galerie?variant=%d", v.ID), http.StatusSeeOther)
}

func (s *server) saveVariant(w http.ResponseWriter, r *http.Request) {
	if !s.parseForm(w, r) {
		return
	}
	parent, ok := s.variant(w, r, r.PostFormValue("variant_id"))
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PostFormValue("name"))
	params := r.PostFormValue("params")

	fail := func(msg string) {
		data, err := s.workbench(r.Context())
		if err != nil {
			s.serverError(w, err)
			return
		}
		data.selectVariant(parent)
		data.Params, data.Name, data.Error = params, name, msg
		s.render(w, http.StatusUnprocessableEntity, "werkstatt", data)
	}
	if name == "" {
		fail("Bitte einen Namen für die neue Variante angeben.")
		return
	}
	if _, err := generator.Parse([]byte(params)); err != nil {
		fail(err.Error())
		return
	}
	id, err := s.store.CreateVariant(r.Context(), name, params, parent.ID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/werkstatt?variant=%d", id), http.StatusSeeOther)
}

func (s *server) galerie(w http.ResponseWriter, r *http.Request) {
	variants, err := s.store.ListVariants(r.Context())
	if err != nil {
		s.serverError(w, err)
		return
	}
	data := page{Title: "Galerie", Active: "galerie", Variants: variants}
	if idStr := r.URL.Query().Get("variant"); idStr != "" {
		v, ok := s.variant(w, r, idStr)
		if !ok {
			return
		}
		data.FilterID = v.ID
	}
	images, err := s.store.ListImages(r.Context(), data.FilterID)
	if err != nil {
		s.serverError(w, err)
		return
	}
	names := make(map[int64]string, len(variants))
	for _, v := range variants {
		names[v.ID] = v.Name
	}
	for _, img := range images {
		data.Images = append(data.Images, imageView{
			ID:          img.ID,
			VariantName: names[img.VariantID],
			Created:     img.CreatedAt.Format("2006-01-02 15:04"),
		})
	}
	s.render(w, http.StatusOK, "galerie", data)
}

// bild liefert ein gespeichertes SVG. Der Inhalt gilt als fremd: CSP und
// nosniff verhindern, dass darin enthaltene Skripte ausgeführt werden.
func (s *server) bild(w http.ResponseWriter, r *http.Request) {
	idStr, ok := strings.CutSuffix(r.PathValue("file"), ".svg")
	id, valid := parseID(idStr)
	if !ok || !valid {
		http.NotFound(w, r)
		return
	}
	img, err := s.store.GetImage(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, err)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "image/svg+xml")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'")
	h.Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write([]byte(img.SVG))
}

// workbench lädt die Daten, die jede Werkstatt-Ansicht braucht.
func (s *server) workbench(ctx context.Context) (page, error) {
	variants, err := s.store.ListVariants(ctx)
	if err != nil {
		return page{}, err
	}
	return page{Title: "Werkstatt", Active: "werkstatt", Variants: variants}, nil
}

func (p *page) selectVariant(v store.Variant) {
	p.Selected = &v
	p.Params = v.Params
	p.Name = v.Name + " (neu)"
}

// variant lädt die Variante zur Formular- oder Query-ID und antwortet bei
// ungültiger oder unbekannter ID selbst mit 404.
func (s *server) variant(w http.ResponseWriter, r *http.Request, idStr string) (store.Variant, bool) {
	id, ok := parseID(idStr)
	if !ok {
		http.NotFound(w, r)
		return store.Variant{}, false
	}
	v, err := s.store.GetVariant(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return store.Variant{}, false
	}
	if err != nil {
		s.serverError(w, err)
		return store.Variant{}, false
	}
	return v, true
}

func (s *server) parseForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxFormBytes)
	if err := r.ParseForm(); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Anfrage zu groß", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "Ungültige Formulardaten", http.StatusBadRequest)
		}
		return false
	}
	return true
}

// render führt das Template vollständig aus, bevor etwas geschrieben wird.
func (s *server) render(w http.ResponseWriter, status int, name string, data page) {
	var buf bytes.Buffer
	if err := s.pages[name].ExecuteTemplate(&buf, "layout", data); err != nil {
		s.serverError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
}

func (s *server) serverError(w http.ResponseWriter, err error) {
	slog.Error("request failed", "error", err)
	http.Error(w, "Interner Fehler", http.StatusInternalServerError)
}

func parseID(s string) (int64, bool) {
	id, err := strconv.ParseInt(s, 10, 64)
	return id, err == nil && id > 0
}
