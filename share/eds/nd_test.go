package eds

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/share/eds/edstest"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func TestNamespaceData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	const odsSize = 8
	sharesAmount := odsSize * odsSize
	namespace := sharetest.RandV0Namespace()
	for amount := 1; amount < sharesAmount; amount++ {
		eds, root := edstest.RandEDSWithNamespace(t, namespace, amount, odsSize)
		rsmt2d := &Rsmt2D{ExtendedDataSquare: eds}
		nd, err := NamespaceData(ctx, rsmt2d, namespace)
		require.NoError(t, err)
		require.True(t, len(nd) > 0)
		require.Len(t, nd.Flatten(), amount)

		err = nd.Validate(root, namespace)
		require.NoError(t, err)
	}
}
