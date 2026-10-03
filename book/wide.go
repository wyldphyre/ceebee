package book

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"image"
	"io"
	"path"
	"sort"
	"strings"

	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// wideFactor is how much wider than the median page (by aspect ratio) a page
// must be to count as wide.
const wideFactor = 1.5

// pageSize reads an image's dimensions from its header. It returns a zero
// size if they can't be determined.
func (b *Book) pageSize(name string) image.Point {
	rc, err := b.open(name)
	if err != nil {
		return image.Point{}
	}
	defer rc.Close()

	if strings.EqualFold(path.Ext(name), ".avif") {
		head, _ := io.ReadAll(io.LimitReader(rc, 64<<10))
		return avifSize(head)
	}
	cfg, _, err := image.DecodeConfig(bufio.NewReader(rc))
	if err != nil {
		return image.Point{}
	}
	return image.Point{X: cfg.Width, Y: cfg.Height}
}

// avifSize finds the image spatial extents ("ispe") property in an AVIF
// header: a 4-byte version/flags field followed by 32-bit width and height.
func avifSize(head []byte) image.Point {
	i := bytes.Index(head, []byte("ispe"))
	if i < 0 || i+16 > len(head) {
		return image.Point{}
	}
	return image.Point{
		X: int(binary.BigEndian.Uint32(head[i+8:])),
		Y: int(binary.BigEndian.Uint32(head[i+12:])),
	}
}

// WideFlags marks pages whose aspect ratio is at least wideFactor times the
// median aspect ratio of the book. Pages with an unknown size are not wide.
func WideFlags(sizes []image.Point) []bool {
	var aspects []float64
	for _, s := range sizes {
		if s.X > 0 && s.Y > 0 {
			aspects = append(aspects, float64(s.X)/float64(s.Y))
		}
	}
	wide := make([]bool, len(sizes))
	if len(aspects) == 0 {
		return wide
	}
	sort.Float64s(aspects)
	median := aspects[len(aspects)/2]
	for i, s := range sizes {
		if s.X > 0 && s.Y > 0 {
			wide[i] = float64(s.X)/float64(s.Y) >= median*wideFactor
		}
	}
	return wide
}
