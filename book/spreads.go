package book

// BuildSpreads groups pages into two-page spreads. The cover and any wide
// pages are always alone; pages before and after the cover are paired in
// order, with any leftover page alone. wide may be nil.
func BuildSpreads(pageCount, coverIndex int, wide []bool) [][]int {
	if coverIndex < 0 || coverIndex >= pageCount {
		coverIndex = 0
	}
	isWide := func(i int) bool { return i < len(wide) && wide[i] }
	spreads := [][]int{}
	pair := func(from, to int) {
		for i := from; i < to; {
			if i+1 < to && !isWide(i) && !isWide(i+1) {
				spreads = append(spreads, []int{i, i + 1})
				i += 2
			} else {
				spreads = append(spreads, []int{i})
				i++
			}
		}
	}
	pair(0, coverIndex)
	if pageCount > 0 {
		spreads = append(spreads, []int{coverIndex})
	}
	pair(coverIndex+1, pageCount)
	return spreads
}
