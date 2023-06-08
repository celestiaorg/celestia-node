package state

import (
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddressMarshalling(t *testing.T) {
	accAddrString := "celestia1377k5an3f94v6wyaceu0cf4nq6gk2jtpc46g7h"
	valAddrString := "celestiavaloper1q3v5cugc8cdpud87u4zwy0a74uxkk6u4q4gx4p"
	accAddr, err := sdk.AccAddressFromBech32(accAddrString)
	require.NoError(t, err)
	valAddr, err := sdk.ValAddressFromBech32(valAddrString)
	require.NoError(t, err)

	accAddrBytes, err := accAddr.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, []byte("\""+accAddrString+"\""), accAddrBytes)

	valAddrBytes, err := valAddr.MarshalJSON()
	assert.NoError(t, err)
	assert.Equal(t, []byte("\""+valAddrString+"\""), valAddrBytes)

	var accAddrUnmarshalled Address
	err = accAddrUnmarshalled.UnmarshalJSON(accAddrBytes)
	assert.NoError(t, err)
	assert.Equal(t, accAddr, accAddrUnmarshalled.Address)

	var valAddrUnmarshalled Address
	err = valAddrUnmarshalled.UnmarshalJSON(valAddrBytes)
	assert.NoError(t, err)
	assert.Equal(t, valAddr, valAddrUnmarshalled.Address)
}
