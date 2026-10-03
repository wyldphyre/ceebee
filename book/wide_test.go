package book

import (
	"archive/zip"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestWideFlags(t *testing.T) {
	portrait := image.Pt(1000, 1500)
	spread := image.Pt(2000, 1500)
	unknown := image.Point{}
	got := WideFlags([]image.Point{portrait, portrait, spread, unknown, image.Pt(1100, 1500), portrait})
	want := []bool{false, false, true, false, false, false}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	// A book of landscape pages has no wide pages.
	got = WideFlags([]image.Point{spread, spread, spread})
	if !reflect.DeepEqual(got, []bool{false, false, false}) {
		t.Errorf("all landscape: got %v", got)
	}

	if got := WideFlags([]image.Point{unknown, unknown}); !reflect.DeepEqual(got, []bool{false, false}) {
		t.Errorf("all unknown: got %v", got)
	}
}

func TestAvifSize(t *testing.T) {
	head := []byte("\x00\x00\x00\x14ispe\x00\x00\x00\x00")
	head = binary.BigEndian.AppendUint32(head, 1920)
	head = binary.BigEndian.AppendUint32(head, 1080)
	if got := avifSize(head); got != image.Pt(1920, 1080) {
		t.Errorf("got %v", got)
	}
	if got := avifSize([]byte("no box here")); got != (image.Point{}) {
		t.Errorf("got %v, want zero", got)
	}
}

func TestOpenDetectsWidePages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wide.cbz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	sizes := []image.Point{{20, 30}, {20, 30}, {40, 30}, {20, 30}, {20, 30}}
	for i, s := range sizes {
		w, _ := zw.Create(fmt.Sprintf("p%d.png", i))
		if err := png.Encode(w, image.NewGray(image.Rect(0, 0, s.X, s.Y))); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()
	f.Close()

	b, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if want := []bool{false, false, true, false, false}; !reflect.DeepEqual(b.Wide, want) {
		t.Errorf("Wide = %v, want %v", b.Wide, want)
	}
}
