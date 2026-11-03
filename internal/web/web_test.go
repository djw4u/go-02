package web

import (
	"context"
	"fmt"
	"html"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/djw4u/go-02/internal/generator"
	"github.com/djw4u/go-02/internal/store"
)

const gridParams = `{"params":{"structure":"GRID","rows":2,"columns":2,"figures":"circle","width":10}}`

type env struct {
	t       *testing.T
	store   *store.Store
	handler http.Handler
}

func newEnv(t *testing.T) *env {
	t.Helper()
	s, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	h, err := New(s, func() generator.Source { return rand.New(rand.NewPCG(1, 2)) })
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	return &env{t: t, store: s, handler: h}
}

func (e *env) variant(name, params string) int64 {
	e.t.Helper()
	id, err := e.store.CreateVariant(context.Background(), name, params, 0)
	if err != nil {
		e.t.Fatalf("CreateVariant: %v", err)
	}
	return id
}

func (e *env) get(target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *env) post(target string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.handler.ServeHTTP(rec, req)
	return rec
}

func (e *env) images(variantID int64) []store.Image {
	e.t.Helper()
	imgs, err := e.store.ListImages(context.Background(), variantID)
	if err != nil {
		e.t.Fatalf("ListImages: %v", err)
	}
	return imgs
}

func wantStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body:\n%s", rec.Code, want, rec.Body.String())
	}
}

func TestWerkstatt_ListsVariantsAndShowsParams(t *testing.T) {
	e := newEnv(t)
	id := e.variant("Rasterbild", gridParams)
	e.variant("Zweite", gridParams)

	rec := e.get(fmt.Sprintf("/werkstatt?variant=%d", id))

	wantStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	for _, want := range []string{"Rasterbild", "Zweite", "&#34;structure&#34;"} {
		if !strings.Contains(body, want) {
			t.Errorf("Body enthält %q nicht", want)
		}
	}
}

func TestWerkstatt_NotFound(t *testing.T) {
	e := newEnv(t)
	for _, target := range []string{"/werkstatt?variant=999", "/werkstatt?variant=abc"} {
		wantStatus(t, e.get(target), http.StatusNotFound)
	}
}

func TestWerkstatt_WithoutVariantsShowsHint(t *testing.T) {
	e := newEnv(t)
	rec := e.get("/werkstatt")
	wantStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), "import_params.sql") {
		t.Error("Hinweis auf den Import fehlt")
	}
}

func TestGenerate_Valid(t *testing.T) {
	e := newEnv(t)
	id := e.variant("v", gridParams)

	rec := e.post("/werkstatt/generate", url.Values{
		"variant_id": {fmt.Sprint(id)},
		"params":     {gridParams},
	})

	wantStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	// html/template maskiert "+" im Attribut als &#43;; der Browser entschlüsselt es.
	if !strings.Contains(html.UnescapeString(body), "data:image/svg+xml;base64,") {
		t.Error("Vorschau als data-URI fehlt")
	}
	if !strings.Contains(body, `name="svg"`) {
		t.Error("verstecktes Feld svg fehlt")
	}
	if n := len(e.images(0)); n != 0 {
		t.Errorf("Generieren hat %d Bilder gespeichert, want 0", n)
	}
}

func TestGenerate_InvalidJSON(t *testing.T) {
	e := newEnv(t)
	id := e.variant("v", gridParams)

	rec := e.post("/werkstatt/generate", url.Values{
		"variant_id": {fmt.Sprint(id)},
		"params":     {"{"},
	})

	wantStatus(t, rec, http.StatusUnprocessableEntity)
	body := rec.Body.String()
	if strings.Contains(body, "data:image") {
		t.Error("Vorschau trotz Fehler angezeigt")
	}
	if !strings.Contains(body, `class="error"`) {
		t.Error("Fehlermeldung fehlt")
	}
	if n := len(e.images(0)); n != 0 {
		t.Errorf("%d Bilder gespeichert, want 0", n)
	}
}

func TestGenerate_InvalidParameters(t *testing.T) {
	e := newEnv(t)
	id := e.variant("v", gridParams)

	rec := e.post("/werkstatt/generate", url.Values{
		"variant_id": {fmt.Sprint(id)},
		"params":     {`{"params":{"structure":"SPIRAL"}}`},
	})

	wantStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), "SPIRAL") {
		t.Error("Fehlermeldung nennt die Ursache nicht")
	}
}

func TestSaveImage(t *testing.T) {
	e := newEnv(t)
	id := e.variant("v", gridParams)
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"></svg>`

	rec := e.post("/werkstatt/save-image", url.Values{
		"variant_id": {fmt.Sprint(id)},
		"svg":        {svg},
	})

	wantStatus(t, rec, http.StatusSeeOther)
	if got, want := rec.Header().Get("Location"), fmt.Sprintf("/galerie?variant=%d", id); got != want {
		t.Errorf("Location = %q, want %q", got, want)
	}
	imgs := e.images(id)
	if len(imgs) != 1 || imgs[0].SVG != svg {
		t.Errorf("Bilder = %+v", imgs)
	}
}

func TestSaveImage_RejectsNonSVGAndUnknownVariant(t *testing.T) {
	e := newEnv(t)
	id := e.variant("v", gridParams)

	rec := e.post("/werkstatt/save-image", url.Values{
		"variant_id": {fmt.Sprint(id)},
		"svg":        {"<html>kein svg</html>"},
	})
	wantStatus(t, rec, http.StatusUnprocessableEntity)

	rec = e.post("/werkstatt/save-image", url.Values{
		"variant_id": {"999"},
		"svg":        {"<svg ></svg>"},
	})
	wantStatus(t, rec, http.StatusNotFound)

	if n := len(e.images(0)); n != 0 {
		t.Errorf("%d Bilder gespeichert, want 0", n)
	}
}

func TestSaveVariant(t *testing.T) {
	e := newEnv(t)
	parent := e.variant("Basis", gridParams)
	changed := strings.Replace(gridParams, `"rows":2`, `"rows":3`, 1)

	rec := e.post("/werkstatt/save-variant", url.Values{
		"variant_id": {fmt.Sprint(parent)},
		"name":       {"Basis (neu)"},
		"params":     {changed},
	})

	wantStatus(t, rec, http.StatusSeeOther)
	all, err := e.store.ListVariants(context.Background())
	if err != nil || len(all) != 2 {
		t.Fatalf("Varianten = %v, %v; want 2", all, err)
	}
	created := all[1]
	if want := fmt.Sprintf("/werkstatt?variant=%d", created.ID); rec.Header().Get("Location") != want {
		t.Errorf("Location = %q, want %q", rec.Header().Get("Location"), want)
	}
	if created.Name != "Basis (neu)" || created.Params != changed {
		t.Errorf("created = %+v", created)
	}
	if created.ParentID == nil || *created.ParentID != parent || created.SourceFile != nil {
		t.Errorf("ParentID/SourceFile = %v/%v", created.ParentID, created.SourceFile)
	}
}

func TestSaveVariant_RejectsEmptyNameAndInvalidParams(t *testing.T) {
	e := newEnv(t)
	parent := e.variant("Basis", gridParams)

	for name, form := range map[string]url.Values{
		"leerer Name":       {"variant_id": {fmt.Sprint(parent)}, "name": {"  "}, "params": {gridParams}},
		"ungültige Params":  {"variant_id": {fmt.Sprint(parent)}, "name": {"x"}, "params": {"{"}},
		"Params ohne Block": {"variant_id": {fmt.Sprint(parent)}, "name": {"x"}, "params": {`{"a":1}`}},
	} {
		t.Run(name, func(t *testing.T) {
			wantStatus(t, e.post("/werkstatt/save-variant", form), http.StatusUnprocessableEntity)
		})
	}
	all, _ := e.store.ListVariants(context.Background())
	if len(all) != 1 {
		t.Errorf("%d Varianten, want 1", len(all))
	}
}

func TestGalerie_FilterByVariant(t *testing.T) {
	e := newEnv(t)
	v1 := e.variant("eins", gridParams)
	v2 := e.variant("zwei", gridParams)
	i1, _ := e.store.SaveImage(context.Background(), v1, "<svg >1</svg>")
	i2, _ := e.store.SaveImage(context.Background(), v2, "<svg >2</svg>")

	all := e.get("/galerie").Body.String()
	for _, id := range []int64{i1, i2} {
		if !strings.Contains(all, fmt.Sprintf("/bilder/%d.svg", id)) {
			t.Errorf("Galerie ohne Filter enthält Bild %d nicht", id)
		}
	}

	rec := e.get(fmt.Sprintf("/galerie?variant=%d", v1))
	wantStatus(t, rec, http.StatusOK)
	body := rec.Body.String()
	if !strings.Contains(body, fmt.Sprintf("/bilder/%d.svg", i1)) {
		t.Error("gefiltertes Bild fehlt")
	}
	if strings.Contains(body, fmt.Sprintf("/bilder/%d.svg", i2)) {
		t.Error("Bild einer anderen Variante ist sichtbar")
	}
	if !strings.Contains(body, `target="_blank"`) {
		t.Error("Link öffnet nicht im eigenen Fenster")
	}
}

func TestBild_Headers(t *testing.T) {
	e := newEnv(t)
	v := e.variant("v", gridParams)
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	id, err := e.store.SaveImage(context.Background(), v, svg)
	if err != nil {
		t.Fatal(err)
	}

	rec := e.get(fmt.Sprintf("/bilder/%d.svg", id))

	wantStatus(t, rec, http.StatusOK)
	h := rec.Header()
	if got := h.Get("Content-Type"); got != "image/svg+xml" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := h.Get("Content-Security-Policy"); got != "default-src 'none'; style-src 'unsafe-inline'" {
		t.Errorf("Content-Security-Policy = %q", got)
	}
	if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q", got)
	}
	if rec.Body.String() != svg {
		t.Errorf("Body = %q, want gespeichertes SVG", rec.Body.String())
	}
}

func TestBild_NotFound(t *testing.T) {
	e := newEnv(t)
	for _, target := range []string{"/bilder/999.svg", "/bilder/abc.svg", "/bilder/1"} {
		wantStatus(t, e.get(target), http.StatusNotFound)
	}
}

func TestRootRedirectsAndStaticCSS(t *testing.T) {
	e := newEnv(t)

	rec := e.get("/")
	wantStatus(t, rec, http.StatusSeeOther)
	if got := rec.Header().Get("Location"); got != "/werkstatt" {
		t.Errorf("Location = %q", got)
	}

	wantStatus(t, e.get("/static/style.css"), http.StatusOK)
	wantStatus(t, e.get("/gibt-es-nicht"), http.StatusNotFound)
}
