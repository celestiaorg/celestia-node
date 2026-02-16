package shwap

import (
	"bytes"
	"context"
	"fmt"
	"io"

	libshare "github.com/celestiaorg/go-square/v3/share"
)

// BlobIDSize defines the total size of a BlobID in bytes, combining the
// size of a EdsID, the size of a Namespace and the commitment size
const BlobIDSize = EdsIDSize + libshare.NamespaceSize + commitmentSize

const commitmentSize = 32

var emptyCommitment = bytes.Repeat([]byte{0xFF}, commitmentSize)

type BlobID struct {
	EdsID
	DataNamespace libshare.Namespace
	Commitment    []byte
}

func NewBlobID(height uint64, ns libshare.Namespace, commitment []byte) (BlobID, error) {
	blobID := BlobID{
		EdsID:         EdsID{height: height},
		DataNamespace: ns,
		Commitment:    commitment,
	}
	return blobID, blobID.Validate()
}

// NewBlobsID constructs blobID with empty commitment which signals to collect all blobs from the requested namespace.
func NewBlobsID(height uint64, ns libshare.Namespace) (BlobID, error) {
	return NewBlobID(height, ns, emptyCommitment)
}

func (blbID *BlobID) Name() string {
	return blobName
}

func (blbID *BlobID) Validate() error {
	if err := blbID.EdsID.Validate(); err != nil {
		return fmt.Errorf("invalid EdsID: %w", err)
	}
	if err := blbID.DataNamespace.ValidateForBlob(); err != nil {
		return fmt.Errorf("not a blob namespace: %w", err)
	}
	if blbID.Commitment == nil || len(blbID.Commitment) != commitmentSize {
		return fmt.Errorf("invalid commitment size: expected:%d got:%d", commitmentSize, len(blbID.Commitment))
	}
	return nil
}

// BlobIDFromBinary deserializes a BlobID from its binary form. It returns
// an error if the binary data's length does not match the expected size.
func BlobIDFromBinary(data []byte) (*BlobID, error) {
	if len(data) != BlobIDSize {
		return nil,
			fmt.Errorf("invalid BlobID length: expected %d, got %d", BlobIDSize, len(data))
	}

	edsID, err := EdsIDFromBinary(data[:EdsIDSize])
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling EDSID: %w", err)
	}

	ns, err := libshare.NewNamespaceFromBytes(data[EdsIDSize : EdsIDSize+libshare.NamespaceSize])
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling namespace: %w", err)
	}

	blbid := &BlobID{
		EdsID:         edsID,
		DataNamespace: ns,
		Commitment:    data[EdsIDSize+libshare.NamespaceSize:],
	}
	return blbid, blbid.Validate()
}

// Equals checks equality of BlobID.
func (blbID *BlobID) Equals(other BlobID) bool {
	return blbID.EdsID.Equals(other.EdsID) && blbID.DataNamespace.Equals(other.DataNamespace) &&
		bytes.Equal(blbID.Commitment, other.Commitment)
}

// ReadFrom reads the binary form of BlobID from the provided reader.
func (blbID *BlobID) ReadFrom(r io.Reader) (int64, error) {
	data := make([]byte, BlobIDSize)
	n, err := io.ReadFull(r, data)
	if err != nil {
		return int64(n), err
	}
	if n != BlobIDSize {
		return int64(n), fmt.Errorf("BlobID: expected %d bytes, got %d", BlobIDSize, n)
	}
	id, err := BlobIDFromBinary(data)
	if err != nil {
		return int64(n), fmt.Errorf("BlobIDFromBinary: %w", err)
	}
	*blbID = *id
	return int64(n), nil
}

// MarshalBinary encodes BlobID into binary form.
// NOTE: Proto is avoided because
// * Its size is not deterministic which is required for IPLD.
// * No support for uint16
func (blbID *BlobID) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, BlobIDSize)
	data, err := blbID.appendBinary(data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// WriteTo writes the binary form of BlobID to the provided writer.
func (blbID *BlobID) WriteTo(w io.Writer) (int64, error) {
	data, err := blbID.MarshalBinary()
	if err != nil {
		return 0, err
	}
	n, err := w.Write(data)
	return int64(n), err
}

func (blbID *BlobID) appendBinary(data []byte) ([]byte, error) {
	data, err := blbID.AppendBinary(data)
	if err != nil {
		return nil, err
	}
	data = append(data, blbID.DataNamespace.Bytes()...)
	return append(data, blbID.Commitment...), nil
}

func (blbID *BlobID) ResponseReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	buf := &bytes.Buffer{}

	var commitments [][]byte
	if !bytes.Equal(blbID.Commitment, emptyCommitment) {
		commitments = append(commitments, blbID.Commitment)
	}

	blobs, err := acc.Blobs(ctx, blbID.DataNamespace, commitments...)
	if err != nil {
		return nil, fmt.Errorf("getting blobs from accessor: %w", err)
	}

	blobSlice := BlobSlice(blobs)
	_, err = blobSlice.WriteTo(buf)
	if err != nil {
		return nil, fmt.Errorf("writing blobs data: %w", err)
	}
	return buf, nil
}
