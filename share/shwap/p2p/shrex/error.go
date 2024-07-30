package shrex

import "errors"

// errorContains reports whether any error in err's tree matches any error in targets tree.
func errorContains(err, target error) bool {
	if errors.Is(err, target) || target == nil {
		return true
	}

	target = errors.Unwrap(target)
	if target == nil {
		return false
	}
	return errorContains(err, target)
}
