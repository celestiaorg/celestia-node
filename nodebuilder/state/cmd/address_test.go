package cmd

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/state"
)

func TestParseAddressKind(t *testing.T) {
	accAddr := state.AccAddress(bytes.Repeat([]byte{1}, 20)).String()
	valAddr := state.ValAddress(bytes.Repeat([]byte{2}, 20)).String()

	t.Run("account address", func(t *testing.T) {
		got, err := parseAccAddressFromString(accAddr)
		require.NoError(t, err)
		require.Equal(t, accAddr, got.String())

		_, err = parseValAddressFromString(accAddr)
		require.ErrorContains(t, err, "is not a validator address")
	})

	t.Run("validator address", func(t *testing.T) {
		got, err := parseValAddressFromString(valAddr)
		require.NoError(t, err)
		require.Equal(t, valAddr, got.String())

		_, err = parseAccAddressFromString(valAddr)
		require.ErrorContains(t, err, "is not an account address")
	})

	t.Run("invalid address", func(t *testing.T) {
		_, err := parseAccAddressFromString("nonsense")
		require.Error(t, err)

		_, err = parseValAddressFromString("nonsense")
		require.Error(t, err)
	})
}
