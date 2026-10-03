package book

import (
	"encoding/xml"
	"strconv"
	"strings"
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
	if err := xml.Unmarshal(data, &ci); err != nil {
		return nil, err
	}
	return &ci, nil
}

// IsRTL reports whether the book reads right to left. A nil ComicInfo is LTR.
func (ci *ComicInfo) IsRTL() bool {
	return ci != nil && strings.TrimSpace(ci.Manga) == "YesAndRightToLeft"
}

// Cover returns the first valid FrontCover page index, or 0.
func (ci *ComicInfo) Cover(pageCount int) int {
	if ci == nil {
		return 0
	}
	for _, p := range ci.Pages {
		if strings.TrimSpace(p.Type) != "FrontCover" {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(p.Image))
		if err == nil && n >= 0 && n < pageCount {
			return n
		}
	}
	return 0
}

// TitleOr builds the display title, falling back to fallback.
func (ci *ComicInfo) TitleOr(fallback string) string {
	if ci == nil {
		return fallback
	}
	series := strings.TrimSpace(ci.Series)
	title := strings.TrimSpace(ci.Title)
	if series != "" {
		s := series + " #" + strings.TrimSpace(ci.Number)
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
