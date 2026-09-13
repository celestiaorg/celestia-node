package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiClientCloser(t *testing.T) {
	called := 0
	closer1 := func() { called++ }
	closer2 := func() { called++ }

	var mc multiClientCloser
	mc.register(closer1)
	mc.register(closer2)

	require.Len(t, mc.closers, 2)
	mc.closeAll()
	require.Equal(t, 2, called)
	require.Nil(t, mc.closers)

	// Repeated calls to closeAll should be safe and no-op
	require.NotPanics(t, func() {
		mc.closeAll()
	})
	require.Equal(t, 2, called)
}

func TestClientCloseCallsClosers(t *testing.T) {
	called := false
	c := &Client{}
	c.closer.register(func() { called = true })

	c.Close()
	require.True(t, called)
}
