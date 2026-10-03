package main

import (
	"crypto/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"ceebee/book"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// BookInfo describes the open book to the frontend.
type BookInfo struct {
	BookID     string       `json:"bookId"`
	Title      string       `json:"title"`
	PageCount  int          `json:"pageCount"`
	CoverIndex int          `json:"coverIndex"`
	RTL        bool         `json:"rtl"`
	Spreads    [][]int      `json:"spreads"`  // two-page mode spreads
	Metadata   []book.Field `json:"metadata"` // ComicInfo.xml fields, if any
}

// ReaderService holds the open book and serves its pages.
type ReaderService struct {
	app *application.App

	mu     sync.RWMutex
	book   *book.Book
	bookID string

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

	s.mu.Lock()
	old := s.book
	s.book, s.bookID = b, id
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
	}, nil
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
