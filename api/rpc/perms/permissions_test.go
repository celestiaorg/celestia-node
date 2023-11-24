package perms

import (
	"errors"
	"reflect"
	"testing"

	"github.com/filecoin-project/go-jsonrpc/auth"
)

func TestFromString(t *testing.T) {
	tests := []struct {
		input       string
		expected    Permission
		expectedErr error
	}{
		{
			"default", Default, nil,
		},
		{
			"read", Read, nil,
		},
		{
			"r", Read, nil,
		},
		{
			"readwrite", ReadWrite, nil,
		},
		{
			"rw", ReadWrite, nil,
		},
		{
			"admin", Admin, nil,
		},
		{
			"invalid", Default, ErrInvalidPermissions,
		},
	}

	for _, test := range tests {
		result, err := FromString(test.input)
		if result != test.expected {
			t.Errorf(
				"FromSting(%s) returned unexpected permission. Expected: %v, Got: %v",
				test.input,
				test.expected,
				result,
			)
		}

		if err != nil && errors.Is(errors.Unwrap(err), test.expectedErr) {
			t.Errorf(
				"FromSting(%s) returned unexpected error. Expected: %v, Got: %v",
				test.input,
				test.expectedErr,
				err,
			)
		}
	}
}

func TestWith(t *testing.T) {
	tests := []struct {
		input    Permission
		expected []auth.Permission
	}{
		{
			Default, permissions[Default],
		},
		{
			Read, permissions[Read],
		},
		{
			ReadWrite, permissions[ReadWrite],
		},
		{
			Admin, permissions[Admin],
		},
	}

	for _, test := range tests {
		result := With(test.input)
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf("With(%v) returned unexpected result. Expected: %v, Got: %v", test.input, test.expected, result)
		}
	}
}
func TestPermissions(t *testing.T) {
	tests := []struct {
		permission Permission
		expected   []auth.Permission
	}{
		{
			Default, []auth.Permission{"read"},
		},
		{
			Read, []auth.Permission{"read"},
		},
		{
			ReadWrite, []auth.Permission{"read", "write"},
		},
		{
			Admin, []auth.Permission{"read", "write", "admin"},
		},
	}

	for _, test := range tests {
		result := permissions[test.permission]
		if !reflect.DeepEqual(result, test.expected) {
			t.Errorf(
				"Permissions[%v] returned unexpected result. Expected: %v, Got: %v",
				test.permission,
				test.expected,
				result,
			)
		}
	}
}
