package book

import (
	"archive/zip"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeNamedBook writes a CBZ with a small portrait PNG for each page name,
// plus ComicInfo.xml if comicInfo isn't empty.
func writeNamedBook(t *testing.T, pages []string, comicInfo string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "book.cbz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, name := range pages {
		w, _ := zw.Create(name)
		if err := png.Encode(w, image.NewGray(image.Rect(0, 0, 20, 30))); err != nil {
			t.Fatal(err)
		}
	}
	if comicInfo != "" {
		w, _ := zw.Create("ComicInfo.xml")
		w.Write([]byte(comicInfo))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

func openOrder(t *testing.T, path string, opts Options) *Book {
	t.Helper()
	b, err := Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	return b
}

func TestPageOrderDefault(t *testing.T) {
	path := writeNamedBook(t, []string{"p1.jpg", "Credits.png", "p2.jpg", "cover.jpg"}, "")
	b := openOrder(t, path, Options{})
	if want := []string{"cover.jpg", "Credits.png", "p1.jpg", "p2.jpg"}; !reflect.DeepEqual(b.pages, want) {
		t.Errorf("pages = %v, want %v", b.pages, want)
	}
}

func TestDetectCovers(t *testing.T) {
	path := writeNamedBook(t, []string{"p1.png", "p2.png", "p3.png", "Cover.png", "z_variant_cover.png", "scans/discovery.png"}, "")
	b := openOrder(t, path, Options{DetectCovers: true})
	// Covers keep their name order: scans/… sorts before z_….
	want := []string{"Cover.png", "scans/discovery.png", "z_variant_cover.png", "p1.png", "p2.png", "p3.png"}
	if !reflect.DeepEqual(b.pages, want) {
		t.Errorf("pages = %v, want %v", b.pages, want)
	}
	if b.CoverIndex != 0 {
		t.Errorf("CoverIndex = %d, want 0", b.CoverIndex)
	}
	// Every cover is shown on its own.
	if wantAlone := []bool{true, true, true, false, false, false}; !reflect.DeepEqual(b.Alone, wantAlone) {
		t.Errorf("Alone = %v, want %v", b.Alone, wantAlone)
	}
	if got := BuildSpreads(b.PageCount(), b.CoverIndex, b.Alone); !reflect.DeepEqual(got, [][]int{{0}, {1}, {2}, {3, 4}, {5}}) {
		t.Errorf("spreads = %v", got)
	}
}

func TestDetectCoversKeepsComicInfoCover(t *testing.T) {
	// ComicInfo.xml names page 1 (p2.png) as the cover, so nothing moves.
	info := `<ComicInfo><Pages><Page Image="1" Type="FrontCover"/></Pages></ComicInfo>`
	path := writeNamedBook(t, []string{"cover.png", "p2.png", "p3.png"}, info)
	b := openOrder(t, path, Options{DetectCovers: true})
	if want := []string{"cover.png", "p2.png", "p3.png"}; !reflect.DeepEqual(b.pages, want) {
		t.Errorf("pages = %v, want %v", b.pages, want)
	}
	if b.CoverIndex != 1 {
		t.Errorf("CoverIndex = %d, want 1", b.CoverIndex)
	}
}

func TestDetectCredits(t *testing.T) {
	// In name order the credits pages come first, so ComicInfo.xml's page 3
	// is p3.png.
	info := `<ComicInfo><Pages><Page Image="3" Type="FrontCover"/></Pages></ComicInfo>`
	path := writeNamedBook(t, []string{"a_Credits.png", "p2.png", "p3.png", "p4.png", "credits_2.png"}, info)
	b := openOrder(t, path, Options{DetectCredits: true})
	want := []string{"p2.png", "p3.png", "p4.png", "a_Credits.png", "credits_2.png"}
	if !reflect.DeepEqual(b.pages, want) {
		t.Errorf("pages = %v, want %v", b.pages, want)
	}
	if b.PageName(b.CoverIndex) != "p3.png" {
		t.Errorf("cover = %q, want p3.png", b.PageName(b.CoverIndex))
	}
}

func TestDetectBoth(t *testing.T) {
	path := writeNamedBook(t, []string{"00_credits.png", "01.png", "02.png", "99_cover.png"}, "")
	b := openOrder(t, path, Options{DetectCovers: true, DetectCredits: true})
	if want := []string{"99_cover.png", "01.png", "02.png", "00_credits.png"}; !reflect.DeepEqual(b.pages, want) {
		t.Errorf("pages = %v, want %v", b.pages, want)
	}
	if b.CoverIndex != 0 {
		t.Errorf("CoverIndex = %d, want 0", b.CoverIndex)
	}
}

func TestDetectCreditsWithoutComicInfoCover(t *testing.T) {
	// With no cover named, the cover is the first page after reordering, not
	// the credits page that was first.
	path := writeNamedBook(t, []string{"00_credits.png", "01.png", "02.png"}, "")
	b := openOrder(t, path, Options{DetectCredits: true})
	if b.PageName(b.CoverIndex) != "01.png" {
		t.Errorf("cover = %q, want 01.png", b.PageName(b.CoverIndex))
	}
}
