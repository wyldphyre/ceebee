// Package book opens comic book archives and exposes their pages.
package book

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// Format is an archive container format.
type Format int

const (
	FormatUnknown Format = iota
	FormatZip
	FormatRar
	Format7z
	FormatTar
)

// ErrUnsupported is returned when a file is not a readable comic archive.
var ErrUnsupported = errors.New("unsupported or unreadable file")

// ErrNoPages is returned when an archive contains no images.
var ErrNoPages = errors.New("archive contains no images")

var imageTypes = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".bmp":  "image/bmp",
}

// Book is an open comic archive.
type Book struct {
	Title      string
	CoverIndex int
	RTL        bool
	// Alone marks the pages shown on their own in two-page mode, besides the
	// cover: wide pages, and the covers found by Options.DetectCovers.
	Alone    []bool
	Metadata []Field // ComicInfo.xml fields; nil if there is none

	pages []string // entry names, sorted

	// CBZ: pages are read on demand from the open zip.
	zr       *zip.ReadCloser
	zipFiles map[string]*zip.File

	// CBR, CB7, CBT: page bytes are read into memory when the book opens.
	data map[string][]byte
}

// DetectFormat identifies the archive format from its magic bytes, falling
// back to the file extension.
func DetectFormat(header []byte, name string) Format {
	switch {
	case bytes.HasPrefix(header, []byte("PK\x03\x04")), bytes.HasPrefix(header, []byte("PK\x05\x06")):
		return FormatZip
	case bytes.HasPrefix(header, []byte("Rar!\x1a\x07")):
		return FormatRar
	case bytes.HasPrefix(header, []byte("7z\xbc\xaf\x27\x1c")):
		return Format7z
	case len(header) >= 262 && string(header[257:262]) == "ustar":
		return FormatTar
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".cbz":
		return FormatZip
	case ".cbr":
		return FormatRar
	case ".cb7":
		return Format7z
	case ".cbt":
		return FormatTar
	}
	return FormatUnknown
}

// Options change the order of a book's pages.
type Options struct {
	// DetectCovers moves pages whose file names contain "cover" to the front,
	// each shown on its own, when ComicInfo.xml doesn't say which page is the
	// cover.
	DetectCovers bool
	// DetectCredits moves pages whose file names contain "credits" to the end.
	DetectCredits bool
}

// Open opens the comic archive at filename.
func Open(filename string, opts Options) (*Book, error) {
	header, err := readHeader(filename)
	if err != nil {
		// Both are wrapped, so callers can tell a missing file with
		// errors.Is(err, fs.ErrNotExist).
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}

	b := &Book{}
	var names []string
	switch DetectFormat(header, filename) {
	case FormatZip:
		names, err = b.openZip(filename)
	case FormatRar:
		names, err = b.readAll(filename, walkRar)
	case Format7z:
		names, err = b.readAll(filename, walk7z)
	case FormatTar:
		names, err = b.readAll(filename, walkTar)
	default:
		return nil, ErrUnsupported
	}
	if err != nil {
		b.Close()
		return nil, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}

	b.pages = PageNames(names)
	if len(b.pages) == 0 {
		b.Close()
		return nil, ErrNoPages
	}

	var info *ComicInfo
	if name := findComicInfo(names); name != "" {
		if data, err := b.read(name); err == nil {
			info, _ = ParseComicInfo(data)
			b.Metadata, _ = ParseMetadata(data)
		}
	}
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	b.Title = info.TitleOr(base)
	b.RTL = info.IsRTL()

	// ComicInfo.xml numbers pages in their natural order, so note the cover
	// it names before any pages move.
	coverIndex, namedCover := info.FrontCover(len(b.pages))
	coverName := b.pages[coverIndex]
	var covers []string
	if opts.DetectCovers && !namedCover {
		b.pages, covers = movePages(b.pages, "cover", true)
	}
	if opts.DetectCredits {
		b.pages, _ = movePages(b.pages, "credits", false)
	}
	if namedCover {
		b.CoverIndex = b.PageIndex(coverName)
	}

	sizes := make([]image.Point, len(b.pages))
	for i, name := range b.pages {
		sizes[i] = b.pageSize(name)
	}
	b.Alone = WideFlags(sizes)
	for _, name := range covers {
		b.Alone[b.PageIndex(name)] = true
	}
	return b, nil
}

// movePages moves the pages whose file names contain word, ignoring case, to
// the front or the end, keeping their order. It returns the new order and the
// pages it moved.
func movePages(pages []string, word string, toFront bool) (ordered, moved []string) {
	var rest []string
	for _, name := range pages {
		base := path.Base(strings.ReplaceAll(name, "\\", "/"))
		if strings.Contains(strings.ToLower(base), word) {
			moved = append(moved, name)
		} else {
			rest = append(rest, name)
		}
	}
	if toFront {
		return append(moved, rest...), moved
	}
	return append(rest, moved...), moved
}

func readHeader(filename string) ([]byte, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	header := make([]byte, 512)
	n, err := io.ReadFull(f, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return header[:n], nil
}

func (b *Book) openZip(filename string) ([]string, error) {
	zr, err := zip.OpenReader(filename)
	if err != nil {
		return nil, err
	}
	b.zr = zr
	b.zipFiles = make(map[string]*zip.File)
	var names []string
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		b.zipFiles[f.Name] = f
		names = append(names, f.Name)
	}
	return names, nil
}

// walkFunc calls visit for every regular file in an archive, in archive order.
type walkFunc func(filename string, visit func(name string, r io.Reader) error) error

// readAll reads every page and ComicInfo.xml into memory.
func (b *Book) readAll(filename string, walk walkFunc) ([]string, error) {
	b.data = make(map[string][]byte)
	var names []string
	err := walk(filename, func(name string, r io.Reader) error {
		names = append(names, name)
		if !isPage(name) && !isComicInfo(name) {
			return nil
		}
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		b.data[name] = data
		return nil
	})
	return names, err
}

func walkRar(filename string, visit func(string, io.Reader) error) error {
	r, err := rardecode.OpenReader(filename)
	if err != nil {
		return err
	}
	defer r.Close()
	for {
		h, err := r.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.IsDir {
			continue
		}
		if err := visit(h.Name, r); err != nil {
			return err
		}
	}
}

func walk7z(filename string, visit func(string, io.Reader) error) error {
	r, err := sevenzip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		err = visit(f.Name, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func walkTar(filename string, visit func(string, io.Reader) error) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	tr := tar.NewReader(f)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag != tar.TypeReg {
			continue
		}
		if err := visit(h.Name, tr); err != nil {
			return err
		}
	}
}

func (b *Book) open(name string) (io.ReadCloser, error) {
	if b.zr != nil {
		f, ok := b.zipFiles[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return f.Open()
	}
	data, ok := b.data[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func (b *Book) read(name string) ([]byte, error) {
	rc, err := b.open(name)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}

// PageCount returns the number of pages.
func (b *Book) PageCount() int { return len(b.pages) }

// PageName returns the archive entry name of the page at index, or "".
func (b *Book) PageName(index int) string {
	if index < 0 || index >= len(b.pages) {
		return ""
	}
	return b.pages[index]
}

// PageIndex returns the index of the page with the given entry name, or -1.
func (b *Book) PageIndex(name string) int {
	return slices.Index(b.pages, name)
}

// Page returns the bytes and content type of the page at index.
func (b *Book) Page(index int) ([]byte, string, error) {
	if index < 0 || index >= len(b.pages) {
		return nil, "", os.ErrNotExist
	}
	name := b.pages[index]
	data, err := b.read(name)
	if err != nil {
		return nil, "", err
	}
	return data, imageTypes[strings.ToLower(path.Ext(name))], nil
}

// Close releases the archive.
func (b *Book) Close() error {
	if b.zr != nil {
		return b.zr.Close()
	}
	return nil
}

// PageNames filters archive entry names down to page images and sorts them
// naturally by full path.
func PageNames(names []string) []string {
	var pages []string
	for _, name := range names {
		if isPage(name) {
			pages = append(pages, name)
		}
	}
	sort.SliceStable(pages, func(i, j int) bool { return NaturalLess(pages[i], pages[j]) })
	return pages
}

func isPage(name string) bool {
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.HasSuffix(name, "/") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if part == "__MACOSX" {
			return false
		}
	}
	base := path.Base(name)
	if strings.HasPrefix(base, ".") {
		return false
	}
	_, ok := imageTypes[strings.ToLower(path.Ext(base))]
	return ok
}

func isComicInfo(name string) bool {
	return strings.EqualFold(path.Base(strings.ReplaceAll(name, "\\", "/")), "ComicInfo.xml")
}

// findComicInfo returns the ComicInfo.xml entry, preferring the archive root.
func findComicInfo(names []string) string {
	found := ""
	for _, name := range names {
		if !isComicInfo(name) {
			continue
		}
		if !strings.ContainsAny(name, "/\\") {
			return name
		}
		if found == "" {
			found = name
		}
	}
	return found
}

// NaturalLess compares strings case-insensitively, treating runs of digits as
// numbers, so "page2" sorts before "page10".
func NaturalLess(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	for a != "" && b != "" {
		if isDigit(a[0]) && isDigit(b[0]) {
			na, ra := splitDigits(a)
			nb, rb := splitDigits(b)
			ta, tb := strings.TrimLeft(na, "0"), strings.TrimLeft(nb, "0")
			if len(ta) != len(tb) {
				return len(ta) < len(tb)
			}
			if ta != tb {
				return ta < tb
			}
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			a, b = ra, rb
			continue
		}
		if a[0] != b[0] {
			return a[0] < b[0]
		}
		a, b = a[1:], b[1:]
	}
	return len(a) < len(b)
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func splitDigits(s string) (digits, rest string) {
	i := 0
	for i < len(s) && isDigit(s[i]) {
		i++
	}
	return s[:i], s[i:]
}
