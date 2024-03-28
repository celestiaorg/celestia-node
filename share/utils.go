package share

// TODO(@walldiss): refactor this into proper package once we have a better idea of what it should look like
func RowRangeForNamespace(root *Root, namespace Namespace) (from, to int) {
	from = -1
	for i, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			if from == -1 {
				from = i
			}
			to = i + 1
		}
	}
	if to == 0 {
		return 0, 0
	}
	return from, to
}
