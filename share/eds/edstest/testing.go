package edstest

import (
	"crypto/rand"
	"testing"

	coretypes "github.com/cometbft/cometbft/types"
	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-app/v6/app"
	"github.com/celestiaorg/celestia-app/v6/app/encoding"
	"github.com/celestiaorg/celestia-app/v6/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/v6/pkg/da"
	"github.com/celestiaorg/celestia-app/v6/pkg/user"
	"github.com/celestiaorg/celestia-app/v6/pkg/wrapper"
	"github.com/celestiaorg/celestia-app/v6/test/util/blobfactory"
	"github.com/celestiaorg/celestia-app/v6/test/util/testfactory"
	blobtypes "github.com/celestiaorg/celestia-app/v6/x/blob/types"
	libshare "github.com/celestiaorg/go-square/v3/share"
	"github.com/celestiaorg/nmt"
	"github.com/celestiaorg/rsmt2d"

	"github.com/celestiaorg/celestia-node/share"
)

const (
	accountName = "test-account"
	testChainID = "private"
)

func RandByzantineEDS(t testing.TB, odsSize int, options ...nmt.Option) *rsmt2d.ExtendedDataSquare {
	eds := RandEDS(t, odsSize)
	shares := eds.Flattened()
	copy(shares[0][libshare.NamespaceSize:], shares[1][libshare.NamespaceSize:]) // corrupting eds
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
	shares, err := libshare.RandShares(odsSize * odsSize)
	require.NoError(t, err)
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
		share.DefaultRSMT2DCodec(),
		wrapper.NewConstructor(uint64(odsSize)),
	)
	require.NoError(t, err, "failure to recompute the extended data square")
	return eds
}

// RandEDSWithTailPadding generates EDS of given ODS size filled with randomized and tail padding shares.
func RandEDSWithTailPadding(t testing.TB, odsSize, padding int) *rsmt2d.ExtendedDataSquare {
	shares, err := libshare.RandShares(odsSize * odsSize)
	require.NoError(t, err)
	for i := len(shares) - padding; i < len(shares); i++ {
		paddingShare := libshare.TailPaddingShare()
		shares[i] = paddingShare
	}

	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
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
	namespace libshare.Namespace,
	namespacedAmount, odsSize int,
) (*rsmt2d.ExtendedDataSquare, *share.AxisRoots) {
	shares, err := libshare.RandSharesWithNamespace(namespace, namespacedAmount, odsSize*odsSize)
	require.NoError(t, err)
	eds, err := rsmt2d.ComputeExtendedDataSquare(
		libshare.ToBytes(shares),
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
	[]*blobtypes.MsgPayForBlobs,
	[]*libshare.Blob,
	[]libshare.Namespace,
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

	txs := make(coretypes.Txs, 0) //nolint:prealloc
	txs = append(txs, coreTxs...)

	eds, err := da.ConstructEDS(txs.ToSliceOfBytes(), appconsts.Version, -1)
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
) ([]libshare.Namespace, []*blobtypes.MsgPayForBlobs, []*libshare.Blob, []coretypes.Tx) {
	nss := make([]libshare.Namespace, 0, numberOfTransactions)
	msgs := make([]*blobtypes.MsgPayForBlobs, 0, numberOfTransactions)
	blobs := make([]*libshare.Blob, 0, numberOfTransactions)
	coreTxs := make([]coretypes.Tx, 0, numberOfTransactions)
	config := encoding.MakeConfig(app.ModuleEncodingRegisters...)
	keyring := testfactory.TestKeyring(config.Codec, accountName)
	account := user.NewAccount(accountName, 0, 0)
	signer, err := user.NewSigner(keyring, config.TxConfig, testChainID, account)
	require.NoError(t, err)

	for i := range numberOfTransactions {
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
) (libshare.Namespace, *blobtypes.MsgPayForBlobs, *libshare.Blob, coretypes.Tx) {
	ns := libshare.RandomBlobNamespace()
	account := signer.Account(accountName)
	msg, b := blobfactory.RandMsgPayForBlobsWithNamespaceAndSigner(account.Address().String(), ns, size)
	blob, err := libshare.NewBlob(ns, b.Data(), b.ShareVersion(), b.Signer())
	require.NoError(t, err)
	cTx, _, err := signer.CreatePayForBlobs(accountName, []*libshare.Blob{blob})
	require.NoError(t, err)
	return ns, msg, b, cTx
}
