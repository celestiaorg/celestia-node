package test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"golang.org/x/crypto/sha3"

	"github.com/celestiaorg/celestia-node/libs/header"
)

type Raw struct {
	ChainID      string
	PreviousHash header.Hash

	Height int64
	Time   time.Time
}

type DummyHeader struct {
	Raw

	hash header.Hash
}

func (d *DummyHeader) New() header.Header {
	return new(DummyHeader)
}

func (d *DummyHeader) IsZero() bool {
	return d == nil
}

func (d *DummyHeader) ChainID() string {
	return d.Raw.ChainID
}

func (d *DummyHeader) Hash() header.Hash {
	if len(d.hash) == 0 {
		if err := d.rehash(); err != nil {
			panic(err)
		}
	}
	return d.hash
}

func (d *DummyHeader) rehash() error {
	b, err := d.MarshalBinary()
	if err != nil {
		return err
	}
	hash := sha3.Sum512(b)
	d.hash = hash[:]
	return nil
}

func (d *DummyHeader) Height() int64 {
	return d.Raw.Height
}

func (d *DummyHeader) LastHeader() header.Hash {
	return d.Raw.PreviousHash
}

func (d *DummyHeader) Time() time.Time {
	return d.Raw.Time
}

func (d *DummyHeader) IsRecent(blockTime time.Duration) bool {
	return time.Since(d.Time()) <= blockTime
}

func (d *DummyHeader) IsExpired(period time.Duration) bool {
	expirationTime := d.Time().Add(period)
	return expirationTime.Before(time.Now())
}

func (d *DummyHeader) VerifyAdjacent(other header.Header) error {
	if other.Height() != d.Height()+1 {
		return fmt.Errorf("invalid Height, expected: %d, got: %d", d.Height()+1, other.Height())
	}

	if err := d.verify(other); err != nil {
		return err
	}

	return nil
}

func (d *DummyHeader) VerifyNonAdjacent(other header.Header) error {
	return d.verify(other)
}

func (d *DummyHeader) verify(header header.Header) error {
	// wee1
	epsilon := 10 * time.Second
	if header.Time().After(time.Now().Add(epsilon)) {
		return errors.New("header Time too far in the future")
	}

	if header.Height() <= d.Height() {
		return errors.New("expected new header Height to be larger than old header Time")
	}

	if header.Time().Before(d.Time()) {
		return errors.New("expected new header Time to be after old header Time")
	}

	return nil
}

func (d *DummyHeader) Validate() error {
	return nil
}

func (d *DummyHeader) MarshalBinary() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(d.Raw)
	return buf.Bytes(), err
}

func (d *DummyHeader) UnmarshalBinary(data []byte) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	err := dec.Decode(&d.Raw)
	if err != nil {
		return err
	}
	err = d.rehash()
	if err != nil {
		return err
	}

	return nil
}

// RandBytes returns slice of n-bytes, or nil in case of error
func RandBytes(n int) []byte {
	buf := make([]byte, n)

	c, err := rand.Read(buf)
	if err != nil || c != n {
		return nil
	}

	return buf
}

func randInt63() int64 {
	var buf [8]byte

	_, err := rand.Read(buf[:])
	if err != nil {
		return math.MaxInt64
	}

	return int64(binary.BigEndian.Uint64(buf[:]) & math.MaxInt64)
}

func RandDummyHeader(t *testing.T) *DummyHeader {
	t.Helper()
	dh, err := randDummyHeader()
	if err != nil {
		t.Fatal(err)
	}
	return dh
}

func randDummyHeader() (*DummyHeader, error) {
	dh := &DummyHeader{
		Raw{
			PreviousHash: RandBytes(32),
			Height:       randInt63(),
			Time:         time.Now().UTC(),
		},
		nil,
	}
	err := dh.rehash()
	return dh, err
}

func mustRandDummyHeader() *DummyHeader {
	dh, err := randDummyHeader()
	if err != nil {
		panic(err)
	}
	return dh
}

// Suite provides everything you need to test chain of Headers.
// If not, please don't hesitate to extend it for your case.
type Suite struct {
	t *testing.T

	head *DummyHeader
}

type Generator[H header.Header] interface {
	GetRandomHeader() H
}

// NewTestSuite setups a new test suite.
func NewTestSuite(t *testing.T) *Suite {
	return &Suite{
		t: t,
	}
}

func (s *Suite) genesis() *DummyHeader {
	return &DummyHeader{
		hash: nil,
		Raw: Raw{
			PreviousHash: nil,
			Height:       1,
			Time:         time.Now().Add(-10 * time.Second).UTC(),
		},
	}
}

func (s *Suite) Head() *DummyHeader {
	if s.head == nil {
		s.head = s.genesis()
	}
	return s.head
}

func (s *Suite) GenDummyHeaders(num int) []*DummyHeader {
	headers := make([]*DummyHeader, num)
	for i := range headers {
		headers[i] = s.GetRandomHeader()
	}
	return headers
}

func (s *Suite) GetRandomHeader() *DummyHeader {
	if s.head == nil {
		s.head = s.genesis()
		return s.head
	}

	dh := mustRandDummyHeader()
	dh.Raw.Height = s.head.Height() + 1
	dh.Raw.PreviousHash = s.head.Hash()
	_ = dh.rehash()
	s.head = dh
	return s.head
}

type DummySubscriber struct {
	Headers []*DummyHeader
}

func (mhs *DummySubscriber) AddValidator(func(context.Context, *DummyHeader) pubsub.ValidationResult) error {
	return nil
}

func (mhs *DummySubscriber) Subscribe() (header.Subscription[*DummyHeader], error) {
	return mhs, nil
}

func (mhs *DummySubscriber) NextHeader(ctx context.Context) (*DummyHeader, error) {
	defer func() {
		if len(mhs.Headers) > 1 {
			// pop the already-returned header
			cp := mhs.Headers
			mhs.Headers = cp[1:]
		} else {
			mhs.Headers = make([]*DummyHeader, 0)
		}
	}()
	if len(mhs.Headers) == 0 {
		return nil, context.Canceled
	}
	return mhs.Headers[0], nil
}

func (mhs *DummySubscriber) Stop(context.Context) error { return nil }
func (mhs *DummySubscriber) Cancel()                    {}
