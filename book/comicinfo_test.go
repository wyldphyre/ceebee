package book

import "testing"

func TestMangaValues(t *testing.T) {
	for manga, want := range map[string]bool{
		"YesAndRightToLeft": true,
		"Yes":               false,
		"No":                false,
		"Unknown":           false,
		"":                  false,
	} {
		ci, err := ParseComicInfo([]byte("<ComicInfo><Manga>" + manga + "</Manga></ComicInfo>"))
		if err != nil {
			t.Fatal(err)
		}
		if got := ci.IsRTL(); got != want {
			t.Errorf("Manga %q: IsRTL = %v, want %v", manga, got, want)
		}
	}
}

func TestFrontCover(t *testing.T) {
	ci, err := ParseComicInfo([]byte(`<ComicInfo><Pages>
		<Page Image="0" Type="Story"/>
		<Page Image="3" Type="FrontCover"/>
		<Page Image="5" Type="FrontCover"/>
	</Pages></ComicInfo>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ci.Cover(10); got != 3 {
		t.Errorf("Cover = %d, want 3", got)
	}
}

func TestFrontCoverOutOfRange(t *testing.T) {
	ci, err := ParseComicInfo([]byte(`<ComicInfo><Pages>
		<Page Image="12" Type="FrontCover"/>
		<Page Image="x" Type="FrontCover"/>
		<Page Image="-1" Type="FrontCover"/>
		<Page Image="4" Type="FrontCover"/>
	</Pages></ComicInfo>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := ci.Cover(5); got != 4 {
		t.Errorf("Cover = %d, want 4 (first valid)", got)
	}
	if got := ci.Cover(3); got != 0 {
		t.Errorf("Cover = %d, want 0 (none valid)", got)
	}
}

func TestMissingOrMalformed(t *testing.T) {
	if _, err := ParseComicInfo([]byte("<ComicInfo><Manga>")); err == nil {
		t.Error("expected parse error")
	}
	var ci *ComicInfo // missing or failed to parse
	if ci.IsRTL() || ci.Cover(5) != 0 || ci.TitleOr("file") != "file" {
		t.Error("nil ComicInfo should give defaults")
	}
}

func TestTitle(t *testing.T) {
	cases := []struct{ xml, want string }{
		{"<ComicInfo><Series>Saga</Series><Number>3</Number><Title>Part</Title></ComicInfo>", "Saga #3 – Part"},
		{"<ComicInfo><Series>Saga</Series><Number>3</Number></ComicInfo>", "Saga #3"},
		{"<ComicInfo><Title>Alone</Title></ComicInfo>", "Alone"},
		{"<ComicInfo></ComicInfo>", "file"},
	}
	for _, c := range cases {
		ci, err := ParseComicInfo([]byte(c.xml))
		if err != nil {
			t.Fatal(err)
		}
		if got := ci.TitleOr("file"); got != c.want {
			t.Errorf("%s: got %q, want %q", c.xml, got, c.want)
		}
	}
}
