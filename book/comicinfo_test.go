package book

import (
	"reflect"
	"testing"

	"golang.org/x/text/encoding/unicode"
)

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
		{"<ComicInfo><Series>Saga</Series></ComicInfo>", "Saga"},
		{"<ComicInfo><Series>Saga</Series><Title>Part</Title></ComicInfo>", "Saga – Part"},
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

func TestParseMetadata(t *testing.T) {
	fields, err := ParseMetadata([]byte(`<?xml version="1.0"?>
<ComicInfo>
  <Title>Okitsushima</Title>
  <Series>Umi no Misaki</Series>
  <Number></Number>
  <Summary>
    Line one.
    Line two.
  </Summary>
  <Pages><Page Image="0" Type="FrontCover"/></Pages>
  <Manga>YesAndRightToLeft</Manga>
</ComicInfo>`))
	if err != nil {
		t.Fatal(err)
	}
	want := []Field{
		{"Title", "Okitsushima"},
		{"Series", "Umi no Misaki"},
		{"Summary", "Line one.\n    Line two."},
		{"Manga", "YesAndRightToLeft"},
	}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("got %q, want %q", fields, want)
	}

	if _, err := ParseMetadata([]byte("<ComicInfo><Title>")); err == nil {
		t.Error("expected parse error")
	}
}

func TestOtherEncodings(t *testing.T) {
	// "Café" in windows-1252, where é is the single byte 0xe9.
	latin1 := []byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?>" +
		"<ComicInfo><Title>Caf\xe9</Title><Manga>YesAndRightToLeft</Manga></ComicInfo>")

	utf8 := "<?xml version=\"1.0\" encoding=\"utf-16\"?>" +
		"<ComicInfo><Title>Café</Title><Manga>YesAndRightToLeft</Manga></ComicInfo>"
	encode := func(e unicode.Endianness) []byte {
		data, err := unicode.UTF16(e, unicode.UseBOM).NewEncoder().Bytes([]byte(utf8))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}

	cases := map[string][]byte{
		"windows-1252":         latin1,
		"utf-16 little-endian": encode(unicode.LittleEndian),
		"utf-16 big-endian":    encode(unicode.BigEndian),
		"utf-8 with BOM":       []byte("\xef\xbb\xbf<ComicInfo><Title>Café</Title><Manga>YesAndRightToLeft</Manga></ComicInfo>"),
	}
	for name, data := range cases {
		ci, err := ParseComicInfo(data)
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if ci.Title != "Café" || !ci.IsRTL() {
			t.Errorf("%s: title %q, rtl %v", name, ci.Title, ci.IsRTL())
		}
		fields, err := ParseMetadata(data)
		if err != nil || len(fields) != 2 || fields[0].Value != "Café" {
			t.Errorf("%s: metadata %q, %v", name, fields, err)
		}
	}
}
