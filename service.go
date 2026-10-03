package main

import (
	"crypto/rand"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"ceebee/book"
	"ceebee/settings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// BookInfo describes the open book to the frontend.
type BookInfo struct {
	BookID     string       `json:"bookId"`
	Title      string       `json:"title"`
	PageCount  int          `json:"pageCount"`
	CoverIndex int          `json:"coverIndex"`
	RTL        bool         `json:"rtl"`
	Spreads    [][]int      `json:"spreads"`   // two-page mode spreads
	Metadata   []book.Field `json:"metadata"`  // ComicInfo.xml fields, if any
	StartPage  int          `json:"startPage"` // page to open at: the saved position, or 0
}

// ReaderService holds the open book and serves its pages.
type ReaderService struct {
	app      *application.App
	settings *settings.Store

	mu       sync.RWMutex
	book     *book.Book
	bookID   string
	bookPath string // absolute path, the key for its saved reading position

	// A file the OS asked us to open before the frontend was ready for it.
	pendingPath   string
	frontendReady bool
}

// OpenDialog shows the native file dialog and opens the chosen book. It
// returns nil if the dialog is cancelled.
func (s *ReaderService) OpenDialog() (*BookInfo, error) {
	path, err := s.app.Dialog.OpenFile().
		SetTitle("Open Comic").
		AddFilter("Comic Books", "*.cbz;*.cbr;*.cb7;*.cbt").
		PromptForSingleSelection()
	if err != nil || path == "" {
		return nil, err
	}
	return s.OpenPath(path)
}

// OpenPath opens the book at path, replacing the current one.
func (s *ReaderService) OpenPath(path string) (*BookInfo, error) {
	b, err := book.Open(path)
	if err != nil {
		return nil, err
	}
	id := rand.Text()
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	start, ok := s.settings.Position(path)
	if !ok || start < 0 || start >= b.PageCount() {
		start = 0
	}

	s.mu.Lock()
	old := s.book
	s.book, s.bookID, s.bookPath = b, id, path
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}

	return &BookInfo{
		BookID:     id,
		Title:      b.Title,
		PageCount:  b.PageCount(),
		CoverIndex: b.CoverIndex,
		RTL:        b.RTL,
		Spreads:    book.BuildSpreads(b.PageCount(), b.CoverIndex, b.Wide),
		Metadata:   b.Metadata,
		StartPage:  start,
	}, nil
}

// ViewSettings returns the saved toolbar settings.
func (s *ReaderService) ViewSettings() settings.View {
	return s.settings.View()
}

// SaveViewSettings saves the toolbar settings.
func (s *ReaderService) SaveViewSettings(v settings.View) {
	if err := s.settings.SetView(v); err != nil {
		log.Printf("saving settings: %v", err)
	}
}

// SavePosition records the page reached in the open book, if bookID is still
// the open book.
func (s *ReaderService) SavePosition(bookID string, page int) {
	s.mu.RLock()
	path, current := s.bookPath, bookID == s.bookID
	s.mu.RUnlock()
	if !current {
		return
	}
	if err := s.settings.SetPosition(path, page); err != nil {
		log.Printf("saving reading position: %v", err)
	}
}

// Version returns the app version.
func (s *ReaderService) Version() string {
	return version
}

// StartupPath returns the file to open when the frontend starts: one the OS
// asked to open while it was loading, or else one passed on the command line.
func (s *ReaderService) StartupPath() string {
	s.mu.Lock()
	s.frontendReady = true
	path := s.pendingPath
	s.pendingPath = ""
	s.mu.Unlock()
	if path != "" {
		return path
	}
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return ""
}

// openFromOS opens a file the OS asked us to open, such as one double-clicked
// in Finder. Before the frontend has started, it is held for StartupPath.
func (s *ReaderService) openFromOS(path string) {
	s.mu.Lock()
	ready := s.frontendReady
	if !ready {
		s.pendingPath = path
	}
	s.mu.Unlock()
	if ready {
		s.app.Event.Emit("open-file", path)
	}
}

// pageMiddleware serves /book/{bookID}/page/{index} from the open book.
func (s *ReaderService) pageMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, "/book/")
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		id, indexStr, ok := strings.Cut(rest, "/page/")
		index, err := strconv.Atoi(indexStr)
		if !ok || err != nil {
			http.NotFound(w, r)
			return
		}

		s.mu.RLock()
		defer s.mu.RUnlock()
		if s.book == nil || id != s.bookID {
			http.NotFound(w, r)
			return
		}
		data, ctype, err := s.book.Page(index)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		w.Header().Set("Content-Type", ctype)
		w.Write(data)
	})
}
