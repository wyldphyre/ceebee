package book

import (
	"bytes"
	"encoding/xml"
	"io"
	"strconv"
	"strings"

	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
)

// ComicInfo holds the ComicInfo.xml fields the reader uses.
type ComicInfo struct {
	Series string `xml:"Series"`
	Number string `xml:"Number"`
	Title  string `xml:"Title"`
	Manga  string `xml:"Manga"`
	Pages  []struct {
		Image string `xml:"Image,attr"`
		Type  string `xml:"Type,attr"`
	} `xml:"Pages>Page"`
}

// ParseComicInfo parses ComicInfo.xml.
func ParseComicInfo(data []byte) (*ComicInfo, error) {
	var ci ComicInfo
	if err := newDecoder(data).Decode(&ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

// newDecoder returns an XML decoder that also reads ComicInfo.xml files that
// aren't UTF-8. UTF-16 files, which start with a byte order mark, are
// converted to UTF-8 first; other encodings named in the XML declaration,
// such as windows-1252, are converted as they are read.
func newDecoder(data []byte) *xml.Decoder {
	utf16 := bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff})
	if utf16 {
		decoder := unicode.UTF16(unicode.BigEndian, unicode.UseBOM).NewDecoder()
		if converted, err := decoder.Bytes(data); err == nil {
			data = converted
		}
	}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) {
		if utf16 && strings.HasPrefix(strings.ToLower(label), "utf-16") {
			return input, nil // already converted
		}
		enc, err := htmlindex.Get(label)
		if err != nil {
			return nil, err
		}
		return enc.NewDecoder().Reader(input), nil
	}
	return d
}

// Field is one ComicInfo.xml element, such as Writer or Summary.
type Field struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ParseMetadata returns the non-empty top-level ComicInfo.xml elements that
// hold plain text, in file order. Elements with children, such as Pages, are
// skipped.
func ParseMetadata(data []byte) ([]Field, error) {
	d := newDecoder(data)
	var fields []Field
	var text strings.Builder
	name, depth, hasChildren := "", 0, false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return fields, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 2 {
				name, hasChildren = t.Name.Local, false
				text.Reset()
			} else if depth > 2 {
				hasChildren = true
			}
		case xml.CharData:
			if depth == 2 {
				text.Write(t)
			}
		case xml.EndElement:
			if depth == 2 && !hasChildren {
				if value := strings.TrimSpace(text.String()); value != "" {
					fields = append(fields, Field{Name: name, Value: value})
				}
			}
			depth--
		}
	}
}

// IsRTL reports whether the book reads right to left. A nil ComicInfo is LTR.
func (ci *ComicInfo) IsRTL() bool {
	return ci != nil && strings.TrimSpace(ci.Manga) == "YesAndRightToLeft"
}

// Cover returns the first valid FrontCover page index, or 0.
func (ci *ComicInfo) Cover(pageCount int) int {
	index, _ := ci.FrontCover(pageCount)
	return index
}

// FrontCover returns the first valid FrontCover page index, and whether there
// is one.
func (ci *ComicInfo) FrontCover(pageCount int) (int, bool) {
	if ci == nil {
		return 0, false
	}
	for _, p := range ci.Pages {
		if strings.TrimSpace(p.Type) != "FrontCover" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(p.Image))
		if err == nil && n >= 0 && n < pageCount {
			return n, true
		}
	}
	return 0, false
}

// TitleOr builds the display title, "{Series} #{Number} – {Title}", leaving
// out the parts that are missing, and falling back to fallback.
func (ci *ComicInfo) TitleOr(fallback string) string {
	if ci == nil {
		return fallback
	}
	series := strings.TrimSpace(ci.Series)
	title := strings.TrimSpace(ci.Title)
	if series != "" {
		s := series
		if number := strings.TrimSpace(ci.Number); number != "" {
			s += " #" + number
		}
		if title != "" {
			s += " – " + title
		}
		return s
	}
	if title != "" {
		return title
	}
	return fallback
}
