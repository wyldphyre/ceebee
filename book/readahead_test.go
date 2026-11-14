package book

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// writePagesZip writes a CBZ of n pages whose contents are "page 0" and so on.
func writePagesZip(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pages.cbz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for i := range n {
		w, _ := zw.Create(fmt.Sprintf("p%02d.png", i))
		fmt.Fprintf(w, "page %d", i)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	return path
}

// cachedIndexes waits for reading ahead to finish and returns the cached pages.
func cachedIndexes(t *testing.T, b *Book) map[int]bool {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		b.cache.mu.Lock()
		running := b.cache.running || len(b.cache.reading) > 0
		cached := map[int]bool{}
		for i := range b.cache.pages {
			cached[i] = true
		}
		b.cache.mu.Unlock()
		if !running {
			return cached
		}
		if time.Now().After(deadline) {
			t.Fatal("reading ahead didn't finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestReadAhead(t *testing.T) {
	b, err := Open(writePagesZip(t, 20), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	if data, _, err := b.Page(0); err != nil || string(data) != "page 0" {
		t.Fatalf("Page(0) = %q, %v", data, err)
	}
	cached := cachedIndexes(t, b)
	for i := 0; i <= readAheadPages; i++ {
		if !cached[i] {
			t.Errorf("page %d isn't cached after reading page 0", i)
		}
	}
	if cached[readAheadPages+1] {
		t.Errorf("page %d was read further ahead than %d pages", readAheadPages+1, readAheadPages)
	}
	if data, _, _ := b.Page(3); string(data) != "page 3" {
		t.Errorf("cached Page(3) = %q", data)
	}

	// Jumping ahead reads ahead from there and drops the furthest pages.
	b.Page(15)
	cached = cachedIndexes(t, b)
	if len(cached) > maxCachedPages {
		t.Errorf("%d pages cached, want at most %d", len(cached), maxCachedPages)
	}
	for i := 15; i < 20; i++ {
		if !cached[i] {
			t.Errorf("page %d isn't cached after reading page 15", i)
		}
	}
	if cached[0] {
		t.Error("page 0 should have been dropped")
	}
}

func TestReadAheadConcurrent(t *testing.T) {
	b, err := Open(writePagesZip(t, 30), Options{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for k := range 30 {
				i := (g*7 + k*3) % 30
				if data, _, err := b.Page(i); err != nil || string(data) != fmt.Sprintf("page %d", i) {
					t.Errorf("Page(%d) = %q, %v", i, data, err)
				}
			}
		})
	}
	wg.Wait()
	// Closing while reading ahead stops it without failing.
	b.Page(0)
	b.Close()
	cachedIndexes(t, b)
}
