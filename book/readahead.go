package book

import "sync"

// readAheadPages is how many pages after the one requested are read ahead
// of time from a book read as its pages are shown (CBZ and non-solid CBR),
// so turning pages quickly doesn't wait on a slow disk or network. Other
// books' pages are already in memory.
const readAheadPages = 8

// maxCachedPages is how many pages the read-ahead cache holds: those read
// ahead, and a few already shown, for going back.
const maxCachedPages = readAheadPages + 4

// pageCache holds pages read ahead of time, keyed by index.
type pageCache struct {
	mu      sync.Mutex
	pages   map[int]cachedPage
	reading map[int]chan struct{} // pages being read, closed when done
	last    int                   // the page last requested
	running bool                  // whether a read-ahead goroutine is running
	closed  bool

	firstRead chan struct{} // closed once a page has been read
	firstOnce sync.Once
}

type cachedPage struct {
	data []byte
	err  error
}

func newPageCache() *pageCache {
	return &pageCache{pages: map[int]cachedPage{}, reading: map[int]chan struct{}{}, firstRead: make(chan struct{})}
}

// readPage returns a page, from the cache if it has been read ahead, and
// reads ahead the pages after it.
func (b *Book) readPage(index int) ([]byte, error) {
	c := b.cache
	defer c.firstOnce.Do(func() { close(c.firstRead) })
	c.mu.Lock()
	c.last = index
	for {
		if p, ok := c.pages[index]; ok {
			c.mu.Unlock()
			b.readAhead()
			return p.data, p.err
		}
		// Wait for a read already under way rather than reading it twice.
		done, busy := c.reading[index]
		if !busy {
			break
		}
		c.mu.Unlock()
		<-done
		c.mu.Lock()
	}
	done := make(chan struct{})
	c.reading[index] = done
	c.mu.Unlock()

	data, err := b.read(b.pages[index])
	c.store(index, data, err, done)
	b.readAhead()
	return data, err
}

// store caches a page that has been read, and tells anyone waiting for it.
func (c *pageCache) store(index int, data []byte, err error, done chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.reading, index)
	close(done)
	if c.closed {
		return
	}
	c.pages[index] = cachedPage{data, err}
	// Drop the pages furthest from the one last requested.
	for len(c.pages) > maxCachedPages {
		far, farDistance := -1, -1
		for i := range c.pages {
			if d := abs(i - c.last); d > farDistance {
				far, farDistance = i, d
			}
		}
		delete(c.pages, far)
	}
}

// readAhead starts reading the pages after the one last requested, in order,
// unless that is already happening. One goroutine reads them, so reads stay
// in order, which suits disks and networks.
func (b *Book) readAhead() {
	c := b.cache
	c.mu.Lock()
	if c.running || c.closed {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()
	go func() {
		for {
			c.mu.Lock()
			index := c.nextToRead(len(b.pages))
			if index < 0 || c.closed {
				c.running = false
				c.mu.Unlock()
				return
			}
			done := make(chan struct{})
			c.reading[index] = done
			c.mu.Unlock()

			data, err := b.read(b.pages[index])
			c.store(index, data, err, done)
		}
	}()
}

// nextToRead returns the first page within readAheadPages after the one last
// requested that isn't cached or being read, or -1. The caller holds c.mu.
func (c *pageCache) nextToRead(pageCount int) int {
	for i := c.last + 1; i <= c.last+readAheadPages && i < pageCount; i++ {
		_, cached := c.pages[i]
		_, busy := c.reading[i]
		if !cached && !busy {
			return i
		}
	}
	return -1
}

// close stops reading ahead and empties the cache.
func (c *pageCache) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.pages = map[int]cachedPage{}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
