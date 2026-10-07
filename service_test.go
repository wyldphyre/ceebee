package main

import (
	"archive/zip"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"ceebee/settings"
)

// newTestService returns a ReaderService with in-memory settings.
func newTestService() *ReaderService {
	return &ReaderService{settings: settings.Load("")}
}

// writeBook writes a CBZ with the given number of small PNG pages.
func writeBook(t *testing.T, pages int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.cbz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for i := range pages {
		w, _ := zw.Create(fmt.Sprintf("p%d.png", i))
		if err := png.Encode(w, image.NewGray(image.Rect(0, 0, 20, 30))); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func TestPageMiddleware(t *testing.T) {
	s := newTestService()
	info, err := s.OpenPath(writeBook(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	handler := s.pageMiddleware(next)

	cases := []struct {
		path        string
		status      int
		contentType string
	}{
		{"/book/" + info.BookID + "/page/1", http.StatusOK, "image/png"},
		{"/book/" + info.BookID + "/page/2", http.StatusNotFound, ""},
		{"/book/" + info.BookID + "/page/-1", http.StatusNotFound, ""},
		{"/book/" + info.BookID + "/page/x", http.StatusNotFound, ""},
		{"/book/old-book/page/0", http.StatusNotFound, ""},
		{"/index.html", http.StatusTeapot, ""}, // not a page: passed on
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != c.status {
			t.Errorf("%s: status %d, want %d", c.path, rec.Code, c.status)
		}
		if c.contentType != "" && rec.Header().Get("Content-Type") != c.contentType {
			t.Errorf("%s: Content-Type %q", c.path, rec.Header().Get("Content-Type"))
		}
	}

	// Opening another book makes the first one's URLs stale.
	if _, err := s.OpenPath(writeBook(t, 1)); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/book/"+info.BookID+"/page/0", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("previous book's page: status %d, want 404", rec.Code)
	}
}

func TestOpenPathRecent(t *testing.T) {
	s := newTestService()
	changed := 0
	s.onRecentChanged = func() { changed++ }

	path := writeBook(t, 1)
	if _, err := s.OpenPath(path); err != nil {
		t.Fatal(err)
	}
	if got := s.settings.Recent(); !slices.Equal(got, []string{path}) {
		t.Fatalf("Recent = %v", got)
	}

	// A book that has since been deleted is dropped from the list.
	os.Remove(path)
	if _, err := s.OpenPath(path); err == nil {
		t.Fatal("opening a deleted book should fail")
	}
	if got := s.settings.Recent(); len(got) != 0 || changed != 1 {
		t.Errorf("after opening a deleted book: Recent = %v, changed %d times", got, changed)
	}

	// A book that exists but can't be read stays in the list.
	bad := filepath.Join(t.TempDir(), "bad.cbz")
	os.WriteFile(bad, []byte("not a zip"), 0o644)
	s.settings.AddRecent(bad)
	if _, err := s.OpenPath(bad); err == nil {
		t.Fatal("opening a bad book should fail")
	}
	if got := s.settings.Recent(); !slices.Equal(got, []string{bad}) {
		t.Errorf("after opening a bad book: Recent = %v", got)
	}
}

func TestOpenPathStartPage(t *testing.T) {
	s := newTestService()
	path := writeBook(t, 3)
	s.settings.SetPosition(path, 2)
	info, err := s.OpenPath(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.StartPage != 2 {
		t.Errorf("StartPage = %d, want 2", info.StartPage)
	}

	// A saved position past the end, say after the book was edited, is ignored.
	s.settings.SetPosition(path, 9)
	if info, _ = s.OpenPath(path); info.StartPage != 0 {
		t.Errorf("StartPage = %d, want 0", info.StartPage)
	}
}

func TestStartupPath(t *testing.T) {
	s := newTestService()
	// A file the OS opens before the frontend is ready is held for it.
	s.openFromOS("/comics/a.cbz")
	if got := s.StartupPath(); got != "/comics/a.cbz" {
		t.Errorf("StartupPath = %q", got)
	}
	if s.pendingPath != "" || !s.frontendReady {
		t.Errorf("after StartupPath: pending %q, ready %v", s.pendingPath, s.frontendReady)
	}
}

func TestReopenKeepsPage(t *testing.T) {
	s := newTestService()
	dir := t.TempDir()
	path := filepath.Join(dir, "book.cbz")
	f, _ := os.Create(path)
	zw := zip.NewWriter(f)
	for _, name := range []string{"01.png", "02.png", "03.png", "zz_cover.png"} {
		w, _ := zw.Create(name)
		png.Encode(w, image.NewGray(image.Rect(0, 0, 20, 30)))
	}
	zw.Close()
	f.Close()

	if _, err := s.OpenPath(path); err != nil {
		t.Fatal(err)
	}
	// Reading 02.png, then turning on cover detection, which moves the cover
	// to the front and 02.png from index 1 to 2.
	s.settings.SetDetectCovers(true)
	info, err := s.Reopen(1)
	if err != nil {
		t.Fatal(err)
	}
	if info.StartPage != 2 {
		t.Errorf("StartPage = %d, want 2", info.StartPage)
	}
	if s.book.PageName(0) != "zz_cover.png" {
		t.Errorf("first page = %q", s.book.PageName(0))
	}
}
