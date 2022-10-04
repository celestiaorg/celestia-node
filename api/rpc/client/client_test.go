package client

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBalance(t *testing.T) {
	t.Skip()
	client, closer, err := NewClient(context.Background(), "http://localhost:26658")
	defer closer()
	require.NoError(t, err)
	balance, err := client.State.Balance(context.Background())
	require.NoError(t, err)
	fmt.Println(balance)
}
