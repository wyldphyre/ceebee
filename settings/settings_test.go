package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDefaults(t *testing.T) {
	s := Load(filepath.Join(t.TempDir(), "missing", "settings.json"))
	if got := s.View(); got != (View{TwoPage: true, ScaleMode: "window", ShowProgress: true}) {
		t.Errorf("View = %+v", got)
	}
	if !s.RememberPosition() {
		t.Error("RememberPosition should default to on")
	}
}

func TestCorruptFileGivesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte("{not json"), 0o644)
	if got := Load(path).View(); !got.TwoPage || got.ScaleMode != "window" {
		t.Errorf("View = %+v", got)
	}
}

func TestPartialFileKeepsOtherDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"view":{"twoPage":false,"scaleMode":"width","showProgress":true}}`), 0o644)
	s := Load(path)
	if got := s.View(); got.TwoPage || got.ScaleMode != "width" {
		t.Errorf("View = %+v", got)
	}
	if !s.RememberPosition() {
		t.Error("RememberPosition should keep its default")
	}
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "CeeBee", "settings.json")
	s := Load(path)
	view := View{TwoPage: false, ScaleMode: "original", ShowProgress: false}
	if err := s.SetView(view); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPosition("/comics/a.cbz", 12); err != nil {
		t.Fatal(err)
	}

	s = Load(path)
	if got := s.View(); got != view {
		t.Errorf("View = %+v, want %+v", got, view)
	}
	if page, ok := s.Position("/comics/a.cbz"); !ok || page != 12 {
		t.Errorf("Position = %d, %v", page, ok)
	}
	if _, ok := s.Position("/comics/other.cbz"); ok {
		t.Error("unknown book should have no position")
	}
}

func TestRememberPositionOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Load(path)
	s.SetPosition("/a.cbz", 5)
	if err := s.SetRememberPosition(false); err != nil {
		t.Fatal(err)
	}
	s.SetPosition("/b.cbz", 7)

	s = Load(path)
	if s.RememberPosition() {
		t.Error("RememberPosition should be off")
	}
	s.SetRememberPosition(true)
	if _, ok := s.Position("/a.cbz"); ok {
		t.Error("turning it off should forget saved positions")
	}
	if _, ok := s.Position("/b.cbz"); ok {
		t.Error("positions should not be saved while off")
	}
}

func TestPrune(t *testing.T) {
	s := Load(filepath.Join(t.TempDir(), "settings.json"))
	for i := 0; i <= maxPositions; i++ {
		s.SetPosition(fmt.Sprintf("/book%d.cbz", i), i)
	}
	if n := len(s.data.Positions); n != maxPositions {
		t.Errorf("kept %d positions, want %d", n, maxPositions)
	}
	if _, ok := s.Position("/book0.cbz"); ok {
		t.Error("the oldest position should be dropped")
	}
	if _, ok := s.Position(fmt.Sprintf("/book%d.cbz", maxPositions)); !ok {
		t.Error("the newest position should be kept")
	}
}

func TestRecent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Load(path)
	for i := 0; i < maxRecent+2; i++ {
		s.AddRecent(fmt.Sprintf("/book%d.cbz", i))
	}
	s.AddRecent("/book5.cbz") // reopening moves it to the top

	got := Load(path).Recent()
	want := []string{"/book5.cbz", "/book11.cbz", "/book10.cbz", "/book9.cbz", "/book8.cbz",
		"/book7.cbz", "/book6.cbz", "/book4.cbz", "/book3.cbz", "/book2.cbz"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Recent = %v, want %v", got, want)
	}

	if err := s.ClearRecent(); err != nil {
		t.Fatal(err)
	}
	if got := Load(path).Recent(); len(got) != 0 {
		t.Errorf("after ClearRecent: %v", got)
	}
}
