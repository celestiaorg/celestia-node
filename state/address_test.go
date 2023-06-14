package state

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressMarshalling(t *testing.T) {
	testCases := []struct {
		name           string
		addressString  string
		addressFromStr func(string) (interface{}, error)
		marshalJSON    func(interface{}) ([]byte, error)
		unmarshalJSON  func([]byte) (interface{}, error)
	}{
		{
			name:           "Account Address",
			addressString:  "celestia1377k5an3f94v6wyaceu0cf4nq6gk2jtpc46g7h",
			addressFromStr: func(s string) (interface{}, error) { return sdk.AccAddressFromBech32(s) },
			marshalJSON:    func(addr interface{}) ([]byte, error) { return addr.(sdk.AccAddress).MarshalJSON() },
			unmarshalJSON: func(b []byte) (interface{}, error) {
				var addr sdk.AccAddress
				err := addr.UnmarshalJSON(b)
				return addr, err
			},
		},
		{
			name:           "Validator Address",
			addressString:  "celestiavaloper1q3v5cugc8cdpud87u4zwy0a74uxkk6u4q4gx4p",
			addressFromStr: func(s string) (interface{}, error) { return sdk.ValAddressFromBech32(s) },
			marshalJSON:    func(addr interface{}) ([]byte, error) { return addr.(sdk.ValAddress).MarshalJSON() },
			unmarshalJSON: func(b []byte) (interface{}, error) {
				var addr sdk.ValAddress
				err := addr.UnmarshalJSON(b)
				return addr, err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := tc.addressFromStr(tc.addressString)
			require.NoError(t, err)

			addrBytes, err := tc.marshalJSON(addr)
			assert.NoError(t, err)
			assert.Equal(t, []byte("\""+tc.addressString+"\""), addrBytes)

			addrUnmarshalled, err := tc.unmarshalJSON(addrBytes)
			assert.NoError(t, err)
			assert.Equal(t, addr, addrUnmarshalled)
		})
	}
}
