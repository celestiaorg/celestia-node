package fxutil

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestOverride(t *testing.T) {
	tt := struct {
		fx.In
		Buf *bytes.Buffer
		R   io.Reader
		S   string
	}{}

	ovrS := "override"
	prv := bytes.NewBuffer(nil)
	ovr := bytes.NewBuffer([]byte("xyz"))

	var prvR io.Reader = prv
	var ovrR io.Reader = ovr

	fopt, err := ParseOptions(
		Supply("supplied"),
		Provide(func() *bytes.Buffer {
			return prv
		}),
		Provide(func() io.Reader {
			return prvR
		}),
		OverrideSupply(&ovrS),
		OverrideSupply(ovr),
		OverrideSupply(&ovrR),
	)
	require.NoError(t, err)

	fxtest.New(t, fopt, fx.Populate(&tt), fx.NopLogger)
	assert.Equal(t, ovr, tt.Buf)
	assert.Equal(t, ovr, tt.R)
	assert.Equal(t, ovrS, tt.S)
}
