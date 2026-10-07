package book

// BuildSpreads groups pages into two-page spreads. The cover and the pages
// marked in alone (such as wide pages) are always on their own; pages before
// and after the cover are paired in order, with any leftover page alone.
// alone may be nil.
func BuildSpreads(pageCount, coverIndex int, alone []bool) [][]int {
	if coverIndex < 0 || coverIndex >= pageCount {
		coverIndex = 0
	}
	isAlone := func(i int) bool { return i < len(alone) && alone[i] }
	spreads := [][]int{}
	pair := func(from, to int) {
		for i := from; i < to; {
			if i+1 < to && !isAlone(i) && !isAlone(i+1) {
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
