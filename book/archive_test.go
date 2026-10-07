package book

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestNaturalSort(t *testing.T) {
	names := []string{"page10.jpg", "Page2.jpg", "page1.jpg", "b/01.jpg", "a/10.jpg", "a/9.jpg", "page02.jpg"}
	sort.SliceStable(names, func(i, j int) bool { return NaturalLess(names[i], names[j]) })
	want := []string{"a/9.jpg", "a/10.jpg", "b/01.jpg", "page1.jpg", "Page2.jpg", "page02.jpg", "page10.jpg"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}

func TestPageNamesFiltering(t *testing.T) {
	names := []string{
		"ComicInfo.xml",
		"notes.txt",
		"chapter1/",
		"chapter1/p2.PNG",
		"chapter1/p1.jpg",
		".cover.jpg",
		"chapter1/._p1.jpg",
		"__MACOSX/chapter1/p1.jpg",
		"x.webp", "x.avif", "x.bmp", "x.gif", "x.jpeg",
	}
	got := PageNames(names)
	want := []string{"chapter1/p1.jpg", "chapter1/p2.PNG", "x.avif", "x.bmp", "x.gif", "x.jpeg", "x.webp"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestFindComicInfoPrefersRoot(t *testing.T) {
	if got := findComicInfo([]string{"sub/ComicInfo.xml", "comicinfo.XML"}); got != "comicinfo.XML" {
		t.Errorf("got %q", got)
	}
	if got := findComicInfo([]string{"a/ComicInfo.xml", "b/ComicInfo.xml"}); got != "a/ComicInfo.xml" {
		t.Errorf("got %q", got)
	}
}

var fixtureFiles = []struct{ name, data string }{
	{"p10.png", "ten"},
	{"p2.png", "two"},
	{"notes.txt", "not a page"},
	{"ComicInfo.xml", "<ComicInfo><Title>Fixture</Title><Manga>YesAndRightToLeft</Manga></ComicInfo>"},
}

func writeZip(t *testing.T, path string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range fixtureFiles {
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(e.data))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func writeTar(t *testing.T, path string) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(f)
	for _, e := range fixtureFiles {
		tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o644, Size: int64(len(e.data)), Typeflag: tar.TypeReg})
		tw.Write([]byte(e.data))
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func checkFixtureBook(t *testing.T, path string, wantFormat Format) {
	t.Helper()
	header, err := readHeader(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := DetectFormat(header, path); got != wantFormat {
		t.Errorf("DetectFormat = %v, want %v", got, wantFormat)
	}
	b, err := Open(path, Options{})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer b.Close()
	if b.PageCount() != 2 {
		t.Fatalf("PageCount = %d, want 2", b.PageCount())
	}
	for i, want := range []string{"two", "ten"} {
		data, ctype, err := b.Page(i)
		if err != nil || string(data) != want || ctype != "image/png" {
			t.Errorf("Page(%d) = %q, %q, %v; want %q, image/png", i, data, ctype, err, want)
		}
	}
	if _, _, err := b.Page(2); err == nil {
		t.Error("Page(2) should fail")
	}
	if b.Title != "Fixture" || !b.RTL || b.CoverIndex != 0 {
		t.Errorf("metadata = %q rtl=%v cover=%d", b.Title, b.RTL, b.CoverIndex)
	}
	if want := []Field{{"Title", "Fixture"}, {"Manga", "YesAndRightToLeft"}}; !reflect.DeepEqual(b.Metadata, want) {
		t.Errorf("Metadata = %q, want %q", b.Metadata, want)
	}
}

func TestFormats(t *testing.T) {
	dir := t.TempDir()

	cbz := filepath.Join(dir, "book.cbz")
	writeZip(t, cbz)
	t.Run("cbz", func(t *testing.T) { checkFixtureBook(t, cbz, FormatZip) })

	cbt := filepath.Join(dir, "book.cbt")
	writeTar(t, cbt)
	t.Run("cbt", func(t *testing.T) { checkFixtureBook(t, cbt, FormatTar) })

	t.Run("cbr", func(t *testing.T) { checkFixtureBook(t, "testdata/fixture.cbr", FormatRar) })
	t.Run("cb7", func(t *testing.T) { checkFixtureBook(t, "testdata/fixture.cb7", Format7z) })

	// A zip mislabelled as .cbr is detected by its content.
	mislabelled := filepath.Join(dir, "really-a-zip.cbr")
	writeZip(t, mislabelled)
	t.Run("mislabelled", func(t *testing.T) { checkFixtureBook(t, mislabelled, FormatZip) })
}

func TestDetectFormatFallsBackToExtension(t *testing.T) {
	for name, want := range map[string]Format{
		"a.CBZ": FormatZip, "a.cbr": FormatRar, "a.cb7": Format7z, "a.cbt": FormatTar, "a.pdf": FormatUnknown,
	} {
		if got := DetectFormat([]byte("junk"), name); got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestOpenErrors(t *testing.T) {
	dir := t.TempDir()

	junk := filepath.Join(dir, "junk.txt")
	os.WriteFile(junk, []byte("hello"), 0o644)
	if _, err := Open(junk, Options{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("junk: got %v, want ErrUnsupported", err)
	}

	corrupt := filepath.Join(dir, "corrupt.cbz")
	os.WriteFile(corrupt, []byte("PK\x03\x04garbage"), 0o644)
	if _, err := Open(corrupt, Options{}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("corrupt: got %v, want ErrUnsupported", err)
	}

	empty := filepath.Join(dir, "empty.cbz")
	f, _ := os.Create(empty)
	zw := zip.NewWriter(f)
	w, _ := zw.Create("readme.txt")
	w.Write([]byte("no images"))
	zw.Close()
	f.Close()
	if _, err := Open(empty, Options{}); !errors.Is(err, ErrNoPages) {
		t.Errorf("empty: got %v, want ErrNoPages", err)
	}
}
