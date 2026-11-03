package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) *Store {
	t.Helper()
	s, err := Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestOpen_IsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	for i := 0; i < 2; i++ {
		s, err := Open(context.Background(), path)
		if err != nil {
			t.Fatalf("Open #%d: %v", i+1, err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("Close #%d: %v", i+1, err)
		}
	}
}

func TestCreateVariant_RoundTrip(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	rootID, err := s.CreateVariant(ctx, "Basis", `{"params":{"a":1}}`, 0)
	if err != nil {
		t.Fatalf("CreateVariant root: %v", err)
	}
	childID, err := s.CreateVariant(ctx, "Basis (neu)", `{"params":{"a":2}}`, rootID)
	if err != nil {
		t.Fatalf("CreateVariant child: %v", err)
	}

	root, err := s.GetVariant(ctx, rootID)
	if err != nil {
		t.Fatalf("GetVariant root: %v", err)
	}
	if root.Name != "Basis" || root.Params != `{"params":{"a":1}}` {
		t.Errorf("root = %+v", root)
	}
	if root.ParentID != nil || root.SourceFile != nil {
		t.Errorf("root ParentID/SourceFile = %v/%v, want nil/nil", root.ParentID, root.SourceFile)
	}
	if root.CreatedAt.IsZero() {
		t.Error("root.CreatedAt is zero")
	}

	child, err := s.GetVariant(ctx, childID)
	if err != nil {
		t.Fatalf("GetVariant child: %v", err)
	}
	if child.ParentID == nil || *child.ParentID != rootID {
		t.Errorf("child.ParentID = %v, want %d", child.ParentID, rootID)
	}

	all, err := s.ListVariants(ctx)
	if err != nil {
		t.Fatalf("ListVariants: %v", err)
	}
	if len(all) != 2 || all[0].ID != rootID || all[1].ID != childID {
		t.Errorf("ListVariants = %+v, want root then child", all)
	}
}

func TestCreateVariant_RejectsInvalidJSON(t *testing.T) {
	s := openTemp(t)
	if _, err := s.CreateVariant(context.Background(), "kaputt", `{`, 0); err == nil {
		t.Fatal("CreateVariant with invalid JSON: want error, got nil")
	}
}

func TestSaveImage_UnknownVariantFails(t *testing.T) {
	s := openTemp(t)
	if _, err := s.SaveImage(context.Background(), 999, "<svg/>"); err == nil {
		t.Fatal("SaveImage for unknown variant: want error, got nil")
	}
}

func TestGetVariant_NotFound(t *testing.T) {
	s := openTemp(t)
	_, err := s.GetVariant(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetImage_NotFound(t *testing.T) {
	s := openTemp(t)
	_, err := s.GetImage(context.Background(), 42)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestListImages_FilterAndOrder(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)

	v1, err := s.CreateVariant(ctx, "v1", `{"params":{}}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	v2, err := s.CreateVariant(ctx, "v2", `{"params":{}}`, 0)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := s.SaveImage(ctx, v1, "<svg>a</svg>")
	b, _ := s.SaveImage(ctx, v2, "<svg>b</svg>")
	c, err := s.SaveImage(ctx, v1, "<svg>c</svg>")
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.ListImages(ctx, v1)
	if err != nil {
		t.Fatalf("ListImages(v1): %v", err)
	}
	if len(got) != 2 || got[0].ID != c || got[1].ID != a {
		t.Errorf("ListImages(v1) IDs = %v, want [%d %d]", ids(got), c, a)
	}
	if got[0].SVG != "<svg>c</svg>" || got[0].VariantID != v1 || got[0].CreatedAt.IsZero() {
		t.Errorf("image = %+v", got[0])
	}

	all, err := s.ListImages(ctx, 0)
	if err != nil {
		t.Fatalf("ListImages(0): %v", err)
	}
	if len(all) != 3 || all[0].ID != c || all[1].ID != b || all[2].ID != a {
		t.Errorf("ListImages(0) IDs = %v, want [%d %d %d]", ids(all), c, b, a)
	}

	one, err := s.GetImage(ctx, b)
	if err != nil {
		t.Fatalf("GetImage: %v", err)
	}
	if one.SVG != "<svg>b</svg>" || one.VariantID != v2 {
		t.Errorf("GetImage = %+v", one)
	}
}

func ids(imgs []Image) []int64 {
	out := make([]int64, len(imgs))
	for i, img := range imgs {
		out[i] = img.ID
	}
	return out
}
