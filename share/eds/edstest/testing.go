package edstest

import (
	"testing"

	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/app"
	"github.com/celestiaorg/celestia-app/app/encoding"
	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/da"
	"github.com/celestiaorg/celestia-app/pkg/namespace"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	"github.com/celestiaorg/celestia-app/pkg/square"
	"github.com/celestiaorg/celestia-app/pkg/wrapper"
	"github.com/celestiaorg/celestia-app/test/util/blobfactory"
	"github.com/celestiaorg/celestia-app/test/util/testfactory"
	"github.com/celestiaorg/celestia-app/x/blob/types"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

func RandByzantineEDS(t *testing.T, size int, options ...nmt.Option) *rsmt2d.ExtendedDataSquare {
	eds := RandEDS(t, size)
	shares := eds.Flattened()
	copy(share.GetData(shares[0]), share.GetData(shares[1])) // corrupting eds
	eds, err := rsmt2d.ImportExtendedDataSquare(shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(size),
			options...))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDS generates EDS filled with the random data with the given size for original square. It
// uses require.TestingT to be able to take both a *testing.T and a *testing.B.
func RandEDS(t require.TestingT, size int) *rsmt2d.ExtendedDataSquare {
	shares := sharetest.RandShares(t, size*size)
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(size)))
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

func RandEDSWithNamespace(
	t require.TestingT,
	namespace share.Namespace,
	size int,
) (*rsmt2d.ExtendedDataSquare, *share.Root) {
	shares := sharetest.RandSharesWithNamespace(t, namespace, size*size)
	eds, err := rsmt2d.ComputeExtendedDataSquare(shares, share.DefaultRSMT2DCodec(), wrapper.NewConstructor(uint64(size)))
	require.NoError(t, err, "failure to recompute the extended data square")
	dah, err := share.NewRoot(eds)
	require.NoError(t, err)
	return eds, dah
}

// GenerateTestBlock generates a set of test blocks with a specific blob size and number of
// transactions
func GenerateTestBlock(
	t *testing.T,
	blobSize, numberOfTransactions int,
) (
	[]*types.MsgPayForBlobs,
	[]*types.Blob,
	[]namespace.Namespace,
	*rsmt2d.ExtendedDataSquare,
	coretypes.Txs,
	*da.DataAvailabilityHeader,
	[]byte,
) {
	nss, msgs, blobs, coreTxs := createTestBlobTransactions(
		t,
		numberOfTransactions,
		blobSize,
	)

	txs := make(coretypes.Txs, 0)
	txs = append(txs, coreTxs...)
	dataSquare, err := square.Construct(
		txs.ToSliceOfBytes(),
		appconsts.LatestVersion,
		appconsts.SquareSizeUpperBound(appconsts.LatestVersion),
	)
	require.NoError(t, err)

	// erasure the data square which we use to create the data root.
	eds, err := da.ExtendShares(shares.ToBytes(dataSquare))
	require.NoError(t, err)

	// create the new data root by creating the data availability header (merkle
	// roots of each row and col of the erasure data).
	dah, err := da.NewDataAvailabilityHeader(eds)
	require.NoError(t, err)
	dataRoot := dah.Hash()

	return msgs, blobs, nss, eds, coreTxs, &dah, dataRoot
}

// createTestBlobTransactions generates a set of transactions that can be added to a blob.
// The number of transactions dictates the number of PFBs that will be returned.
// The size refers to the size of the data contained in the PFBs in bytes.
func createTestBlobTransactions(
	t *testing.T,
	numberOfTransactions, size int,
) ([]namespace.Namespace, []*types.MsgPayForBlobs, []*types.Blob, []coretypes.Tx) {
	acc := "blobstream-api-tests"
	kr := testfactory.GenerateKeyring(acc)
	signer := types.NewKeyringSigner(kr, acc, "test")

	nss := make([]namespace.Namespace, 0)
	msgs := make([]*types.MsgPayForBlobs, 0)
	blobs := make([]*types.Blob, 0)
	coreTxs := make([]coretypes.Tx, 0)
	for i := 0; i < numberOfTransactions; i++ {
		ns, msg, blob, coreTx := createTestBlobTransaction(t, signer, size+i*1000)
		nss = append(nss, ns)
		msgs = append(msgs, msg)
		blobs = append(blobs, blob)
		coreTxs = append(coreTxs, coreTx)
	}

	return nss, msgs, blobs, coreTxs
}

// createTestBlobTransaction creates a test blob transaction using a specific signer and a specific
// PFB size. The size is in bytes.
func createTestBlobTransaction(
	t *testing.T,
	signer *types.KeyringSigner,
	size int,
) (namespace.Namespace, *types.MsgPayForBlobs, *types.Blob, coretypes.Tx) {
	addr, err := signer.GetSignerInfo().GetAddress()
	require.NoError(t, err)

	ns := namespace.RandomBlobNamespace()
	msg, blob := blobfactory.RandMsgPayForBlobsWithNamespaceAndSigner(addr.String(), ns, size)
	require.NoError(t, err)

	builder := signer.NewTxBuilder()
	stx, err := signer.BuildSignedTx(builder, msg)
	require.NoError(t, err)
	rawTx, err := encoding.MakeConfig(app.ModuleEncodingRegisters...).TxConfig.TxEncoder()(stx)
	require.NoError(t, err)
	cTx, err := coretypes.MarshalBlobTx(rawTx, blob)
	require.NoError(t, err)
	return ns, msg, blob, cTx
}
