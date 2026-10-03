package book

import (
	"reflect"
	"testing"
)

func TestBuildSpreads(t *testing.T) {
	cases := []struct {
		pages, cover int
		want         [][]int
	}{
		{7, 0, [][]int{{0}, {1, 2}, {3, 4}, {5, 6}}},
		{6, 0, [][]int{{0}, {1, 2}, {3, 4}, {5}}},
		{7, 2, [][]int{{0, 1}, {2}, {3, 4}, {5, 6}}},
		{5, 4, [][]int{{0, 1}, {2, 3}, {4}}},
		{6, 5, [][]int{{0, 1}, {2, 3}, {4}, {5}}},
		{1, 0, [][]int{{0}}},
		{2, 0, [][]int{{0}, {1}}},
		{2, 1, [][]int{{0}, {1}}},
	}
	for _, c := range cases {
		if got := BuildSpreads(c.pages, c.cover, nil); !reflect.DeepEqual(got, c.want) {
			t.Errorf("BuildSpreads(%d, %d) = %v, want %v", c.pages, c.cover, got, c.want)
		}
	}
}

func TestBuildSpreadsWidePages(t *testing.T) {
	cases := []struct {
		pages, cover int
		wide         []int
		want         [][]int
	}{
		// A wide page breaks pairing; the page before it is left alone.
		{8, 0, []int{3}, [][]int{{0}, {1, 2}, {3}, {4, 5}, {6, 7}}},
		{8, 0, []int{2}, [][]int{{0}, {1}, {2}, {3, 4}, {5, 6}, {7}}},
		{6, 0, []int{1, 2}, [][]int{{0}, {1}, {2}, {3, 4}, {5}}},
		{6, 0, []int{5}, [][]int{{0}, {1, 2}, {3, 4}, {5}}},
		// Wide pages before the cover, and a wide cover.
		{7, 3, []int{0}, [][]int{{0}, {1, 2}, {3}, {4, 5}, {6}}},
		{5, 0, []int{0}, [][]int{{0}, {1, 2}, {3, 4}}},
	}
	for _, c := range cases {
		wide := make([]bool, c.pages)
		for _, i := range c.wide {
			wide[i] = true
		}
		if got := BuildSpreads(c.pages, c.cover, wide); !reflect.DeepEqual(got, c.want) {
			t.Errorf("BuildSpreads(%d, %d, wide %v) = %v, want %v", c.pages, c.cover, c.wide, got, c.want)
		}
	}
}
