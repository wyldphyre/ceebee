package settings

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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
	if err := s.SetPosition("/comics/a.cbz", Position{Page: 12, Name: "p12.png"}); err != nil {
		t.Fatal(err)
	}

	s = Load(path)
	if got := s.View(); got != view {
		t.Errorf("View = %+v, want %+v", got, view)
	}
	if p, ok := s.Position("/comics/a.cbz"); !ok || p.Page != 12 || p.Name != "p12.png" {
		t.Errorf("Position = %+v, %v", p, ok)
	}
	if _, ok := s.Position("/comics/other.cbz"); ok {
		t.Error("unknown book should have no position")
	}
}

func TestRememberPositionOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Load(path)
	s.SetPosition("/a.cbz", Position{Page: 5})
	if err := s.SetRememberPosition(false); err != nil {
		t.Fatal(err)
	}
	s.SetPosition("/b.cbz", Position{Page: 7})

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
		s.SetPosition(fmt.Sprintf("/book%d.cbz", i), Position{Page: i})
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

	if err := s.RemoveRecent("/book9.cbz"); err != nil {
		t.Fatal(err)
	}
	want = append(want[:3], want[4:]...)
	if got := Load(path).Recent(); !reflect.DeepEqual(got, want) {
		t.Errorf("after RemoveRecent: %v, want %v", got, want)
	}

	if err := s.ClearRecent(); err != nil {
		t.Fatal(err)
	}
	if got := Load(path).Recent(); len(got) != 0 {
		t.Errorf("after ClearRecent: %v", got)
	}
}

func TestDetectSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	s := Load(path)
	if s.DetectCovers() || s.DetectCredits() {
		t.Error("cover and credit detection should default to off")
	}
	if err := s.SetDetectCovers(true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDetectCredits(true); err != nil {
		t.Fatal(err)
	}
	s = Load(path)
	if !s.DetectCovers() || !s.DetectCredits() {
		t.Error("cover and credit detection should be saved")
	}
}

func TestUnfinished(t *testing.T) {
	s := Load("")
	s.SetPosition("/old.cbz", Position{Page: 4, Name: "p4.png", PageCount: 20})
	s.SetPosition("/finished.cbz", Position{PageCount: 20})
	s.SetPosition("/new.cbz", Position{Page: 9, Name: "p9.png"})
	// Make the order certain whatever the clock's resolution.
	p := s.data.Positions["/old.cbz"]
	p.Time = p.Time.Add(-time.Minute)
	s.data.Positions["/old.cbz"] = p

	var paths []string
	for _, b := range s.Unfinished() {
		paths = append(paths, b.Path)
	}
	if want := []string{"/new.cbz", "/old.cbz"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("Unfinished = %q, want %q", paths, want)
	}
	if b := s.Unfinished()[1]; b.Page != 4 || b.PageCount != 20 {
		t.Errorf("old.cbz = page %d of %d, want 4 of 20", b.Page, b.PageCount)
	}

	s.SetRememberPosition(false)
	if got := s.Unfinished(); len(got) != 0 {
		t.Errorf("Unfinished with positions off = %v, want none", got)
	}
}

func TestForgetPosition(t *testing.T) {
	s := Load("")
	s.SetPosition("/a.cbz", Position{Page: 3})
	if err := s.ForgetPosition("/a.cbz"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Position("/a.cbz"); ok {
		t.Error("position should be forgotten")
	}
	if err := s.ForgetPosition("/never.cbz"); err != nil {
		t.Errorf("forgetting an unknown book: %v", err)
	}
}

func TestFindMoved(t *testing.T) {
	s := Load("")
	s.SetPosition("/comics/Book.cbz", Position{Page: 3, Size: 100})
	s.SetPosition("/old/Moved.cbz", Position{Page: 5, Size: 200})
	s.SetPosition("/comics/Renamed.cbz", Position{Page: 7, Size: 300})
	s.SetPosition("/comics/Unsized.cbz", Position{Page: 2})
	missing := func(string) bool { return false }
	present := func(string) bool { return true }

	cases := []struct {
		name   string
		path   string
		size   int64
		exists func(string) bool
		want   string
	}{
		{"case change, old path still found", "/comics/Book.CBZ", 100, present, "/comics/Book.cbz"},
		{"case change, different size", "/comics/Book.CBZ", 999, missing, ""},
		{"moved folder, same name", "/new/moved.CBZ", 200, missing, "/old/Moved.cbz"},
		{"moved folder, old file still there", "/new/Moved.cbz", 200, present, ""},
		{"renamed in same folder", "/comics/New Name.cbz", 300, missing, "/comics/Renamed.cbz"},
		{"different name and folder", "/other/Other.cbz", 300, missing, ""},
		{"no size saved, case change", "/comics/Unsized.CBZ", 50, missing, "/comics/Unsized.cbz"},
		{"no size saved, moved", "/new/Unsized.cbz", 50, missing, ""},
	}
	for _, c := range cases {
		got, ok := s.FindMoved(c.path, c.size, c.exists)
		if got != c.want || ok != (c.want != "") {
			t.Errorf("%s: FindMoved = %q, %v; want %q", c.name, got, ok, c.want)
		}
	}

	if err := s.MovePosition("/old/Moved.cbz", "/new/Moved.cbz"); err != nil {
		t.Fatal(err)
	}
	if p, ok := s.Position("/new/Moved.cbz"); !ok || p.Page != 5 {
		t.Errorf("moved position = %+v, %v; want page 5", p, ok)
	}
	if _, ok := s.Position("/old/Moved.cbz"); ok {
		t.Error("old position should be gone")
	}
}
