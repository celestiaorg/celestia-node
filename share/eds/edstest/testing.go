package edstest

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	coretypes "github.com/tendermint/tendermint/types"

	"github.com/celestiaorg/celestia-app/v2/app"
	"github.com/celestiaorg/celestia-app/v2/app/encoding"
	"github.com/celestiaorg/celestia-app/v2/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v2/pkg/da"
	"github.com/celestiaorg/celestia-app/v2/pkg/user"
	"github.com/celestiaorg/celestia-app/v2/pkg/wrapper"
	"github.com/celestiaorg/celestia-app/v2/test/util/blobfactory"
	"github.com/celestiaorg/celestia-app/v2/test/util/testfactory"
	"github.com/celestiaorg/celestia-app/v2/x/blob/types"
	"github.com/celestiaorg/go-square/blob"
	"github.com/celestiaorg/go-square/namespace"
	appshares "github.com/celestiaorg/go-square/shares"
	"github.com/celestiaorg/go-square/square"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
	"github.com/celestiaorg/celestia-node/share/sharetest"
)

const (
	accountName = "test-account"
	testChainID = "private"
)

func RandByzantineEDS(t testing.TB, odsSize int, options ...nmt.Option) *rsmt2d.ExtendedDataSquare {
	eds := RandEDS(t, odsSize)
	shares := eds.Flattened()
	copy(share.GetData(shares[0]), share.GetData(shares[1])) // corrupting eds
	eds, err := rsmt2d.ImportExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize), options...),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDS generates EDS filled with the random data with the given size for original square.
func RandEDS(t testing.TB, odsSize int) *rsmt2d.ExtendedDataSquare {
	shares := sharetest.RandShares(t, odsSize*odsSize)
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDSWithTailPadding generates EDS of given ODS size filled with randomized and tail padding shares.
func RandEDSWithTailPadding(t testing.TB, odsSize, padding int) *rsmt2d.ExtendedDataSquare {
	shares := sharetest.RandShares(t, odsSize*odsSize)
	for i := len(shares) - padding; i < len(shares); i++ {
		paddingShare := appshares.TailPaddingShare()
		shares[i] = paddingShare.ToBytes()
	}

	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDSWithNamespace generates EDS with given square size. Returned EDS will have
// namespacedAmount of shares with the given namespace.
func RandEDSWithNamespace(
	t testing.TB,
	namespace share.Namespace,
	namespacedAmount, odsSize int,
) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	shares := sharetest.RandSharesWithNamespace(t, namespace, namespacedAmount, odsSize*odsSize)
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		shares,
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	roots, err := share.NewAxisRoots(eds)
	require.NoError(t, err)
	return eds, roots
}

// RandomAxisRoots generates random share.AxisRoots for the given eds size.
func RandomAxisRoots(t testing.TB, edsSize int) *share.AxisRoots {
	roots := make([][]byte, edsSize*2)
	for i := range roots {
		root := make([]byte, edsSize)
		_, err := rand.Read(root)
		require.NoError(t, err)
		roots[i] = root
	}

	rows := roots[:edsSize]
	cols := roots[edsSize:]
	return &share.AxisRoots{
		RowRoots:    rows,
		ColumnRoots: cols,
	}
}

// GenerateTestBlock generates a set of test blocks with a specific blob size and number of
// transactions
func GenerateTestBlock(
	t *testing.T,
	blobSize, numberOfTransactions int,
) (
	[]*types.MsgPayForBlobs,
	[]*blob.Blob,
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
		appconsts.SquareSizeUpperBound(appconsts.LatestVersion),
		appconsts.SubtreeRootThreshold(appconsts.LatestVersion),
	)
	require.NoError(t, err)

	// erasure the data square which we use to create the data root.
	eds, err := da.ExtendShares(appshares.ToBytes(dataSquare))
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
) ([]namespace.Namespace, []*types.MsgPayForBlobs, []*blob.Blob, []coretypes.Tx) {
	nss := make([]namespace.Namespace, 0)
	msgs := make([]*types.MsgPayForBlobs, 0)
	blobs := make([]*blob.Blob, 0)
	coreTxs := make([]coretypes.Tx, 0)
	config := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	keyring := testfactory.TestKeyring(config.Codec, accountName)
	account := user.NewAccount(accountName, 0, 0)
	signer, err := user.NewSigner(keyring, config.TxConfig, testChainID, appconsts.LatestVersion, account)
	require.NoError(t, err)

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
	signer *user.Signer,
	size int,
) (namespace.Namespace, *types.MsgPayForBlobs, *blob.Blob, coretypes.Tx) {
	ns := namespace.RandomBlobNamespace()
	account := signer.Account(accountName)
	msg, b := blobfactory.RandMsgPayForBlobsWithNamespaceAndSigner(account.Address().String(), ns, size)
	cTx := BuildCoreTx(t, signer, accountName, b)
	return ns, msg, b, cTx
}

// BuildCoreTx takes a signer, a message and a blob and creates a core transaction.
// The core transaction is the final form of a transaction that gets pushed
// into the square builder.
func BuildCoreTx(
	t *testing.T,
	signer *user.Signer,
	accountName string,
	b *blob.Blob,
) coretypes.Tx {
	cTx, _, err := signer.CreatePayForBlobs(accountName, []*blob.Blob{b})
	require.NoError(t, err)
	return cTx
}
