// Package settings stores the reader's preferences and reading positions in
// a JSON file in the user's config directory.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// maxRecent is how many recently opened books File › Open Recent lists.
const maxRecent = 10

// maxPositions limits how many books' reading positions are kept; the least
// recently read are dropped first.
const maxPositions = 500

// View holds the toolbar settings that are restored at startup.
type View struct {
	TwoPage      bool   `json:"twoPage"`
	ScaleMode    string `json:"scaleMode"` // "window", "width" or "original"
	ShowProgress bool   `json:"showProgress"`
}

// Position is where reading stopped in a book.
type Position struct {
	Page int `json:"page"`
	// Name is the page's entry name in the archive, which finds the page
	// again if the pages' order changes. Empty for the first page.
	Name string    `json:"name,omitempty"`
	Time time.Time `json:"time"`
}

type file struct {
	View             View                `json:"view"`
	RememberPosition bool                `json:"rememberPosition"`
	DetectCovers     bool                `json:"detectCovers"`  // see book.Options
	DetectCredits    bool                `json:"detectCredits"` // see book.Options
	Positions        map[string]Position `json:"positions"`     // keyed by book path
	Recent           []string            `json:"recent"`        // book paths, newest first
}

// Store is the settings file, loaded into memory. It is safe for concurrent use.
type Store struct {
	mu   sync.Mutex
	path string
	data file
}

// DefaultPath returns the settings file location: CeeBee/settings.json in the
// OS's per-user config directory.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "CeeBee", "settings.json"), nil
}

// Load reads the settings file at path. A missing or unreadable file gives
// the defaults; any fields it lacks also keep their defaults. An empty path
// keeps settings in memory only.
func Load(path string) *Store {
	s := &Store{
		path: path,
		data: file{
			View:             View{TwoPage: true, ScaleMode: "window", ShowProgress: true},
			RememberPosition: true,
		},
	}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &s.data)
	}
	if s.data.Positions == nil {
		s.data.Positions = map[string]Position{}
	}
	return s
}

// View returns the saved toolbar settings.
func (s *Store) View() View {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.View
}

// SetView saves the toolbar settings.
func (s *Store) SetView(v View) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.View = v
	return s.save()
}

// RememberPosition reports whether reading positions are being saved.
func (s *Store) RememberPosition() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.RememberPosition
}

// SetRememberPosition turns saving reading positions on or off. Turning it
// off forgets all saved positions.
func (s *Store) SetRememberPosition(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.RememberPosition = on
	if !on {
		s.data.Positions = map[string]Position{}
	}
	return s.save()
}

// DetectCovers reports whether pages named as covers are moved to the front.
func (s *Store) DetectCovers() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DetectCovers
}

// SetDetectCovers turns detecting cover pages on or off.
func (s *Store) SetDetectCovers(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DetectCovers = on
	return s.save()
}

// DetectCredits reports whether pages named as credits are moved to the end.
func (s *Store) DetectCredits() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.DetectCredits
}

// SetDetectCredits turns detecting credit pages on or off.
func (s *Store) SetDetectCredits(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DetectCredits = on
	return s.save()
}

// Position returns the saved position in a book, if positions are remembered
// and one was saved.
func (s *Store) Position(book string) (Position, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.data.RememberPosition {
		return Position{}, false
	}
	p, ok := s.data.Positions[book]
	return p, ok
}

// SetPosition saves the page reached in a book, by index and entry name. It
// does nothing when positions are not being remembered.
func (s *Store) SetPosition(book string, page int, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.data.RememberPosition {
		return nil
	}
	s.data.Positions[book] = Position{Page: page, Name: name, Time: time.Now()}
	if len(s.data.Positions) > maxPositions {
		s.prune()
	}
	return s.save()
}

// Recent returns the recently opened books, newest first.
func (s *Store) Recent() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.data.Recent...)
}

// AddRecent records a book as the most recently opened.
func (s *Store) AddRecent(book string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := []string{book}
	for _, b := range s.data.Recent {
		if b != book && len(recent) < maxRecent {
			recent = append(recent, b)
		}
	}
	s.data.Recent = recent
	return s.save()
}

// RemoveRecent drops a book from the recently opened list.
func (s *Store) RemoveRecent(book string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	recent := s.data.Recent[:0]
	for _, b := range s.data.Recent {
		if b != book {
			recent = append(recent, b)
		}
	}
	s.data.Recent = recent
	return s.save()
}

// ClearRecent empties the recently opened list.
func (s *Store) ClearRecent() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.Recent = nil
	return s.save()
}

// prune drops the least recently read positions beyond maxPositions.
func (s *Store) prune() {
	books := make([]string, 0, len(s.data.Positions))
	for book := range s.data.Positions {
		books = append(books, book)
	}
	sort.Slice(books, func(i, j int) bool {
		return s.data.Positions[books[i]].Time.After(s.data.Positions[books[j]].Time)
	})
	for _, book := range books[maxPositions:] {
		delete(s.data.Positions, book)
	}
}

// save writes the file atomically: to a temporary file, then renamed over
// the old one, so a crash mid-write can't leave it corrupt.
func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
