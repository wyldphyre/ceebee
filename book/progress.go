package book

import (
	"io/fs"
	"os"
	"sync"
)

// progress reports how far through its file an archive has been read.
type progress struct {
	mu     sync.Mutex
	done   int64
	total  int64
	report func(done, total int64)
}

// reached notes that the file has been read up to offset n. Archives that
// skip entries by seeking still move forward, so the furthest offset reached
// tracks how much of the file is done.
func (p *progress) reached(n int64) {
	if p == nil || p.report == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if n > p.done {
		p.done = n
		p.report(min(n, p.total), p.total)
	}
}

// trackedFile is an archive file that reports its reading progress. It
// doesn't embed *os.File, whose WriteTo would let io.Copy read past it.
type trackedFile struct {
	f   *os.File
	p   *progress
	pos int64
}

func openTracked(name string, p *progress) (*trackedFile, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return &trackedFile{f: f, p: p}, nil
}

func (t *trackedFile) Read(b []byte) (int, error) {
	n, err := t.f.Read(b)
	t.pos += int64(n)
	t.p.reached(t.pos)
	return n, err
}

func (t *trackedFile) ReadAt(b []byte, off int64) (int, error) {
	n, err := t.f.ReadAt(b, off)
	t.p.reached(off + int64(n))
	return n, err
}

func (t *trackedFile) Seek(offset int64, whence int) (int64, error) {
	pos, err := t.f.Seek(offset, whence)
	if err == nil {
		t.pos = pos
		t.p.reached(pos)
	}
	return pos, err
}

func (t *trackedFile) Stat() (fs.FileInfo, error) { return t.f.Stat() }
func (t *trackedFile) Close() error               { return t.f.Close() }

// trackedFS opens RAR volumes as tracked files. Progress is measured against
// the first volume, so a multi-volume archive shows complete early.
type trackedFS struct{ p *progress }

func (t trackedFS) Open(name string) (fs.File, error) { return openTracked(name, t.p) }
