package share

// TODO(@walldiss): refactor this into proper package once we have a better idea of what it should look like
func RowRangeForNamespace(root *Root, namespace Namespace) (from, to int) {
	for i, row := range root.RowRoots {
		if !namespace.IsOutsideRange(row, row) {
			if from == 0 {
				from = i
			}
			to = i
		}
	}
	return from, to
}
