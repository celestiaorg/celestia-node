package file

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

// Due to bug on macOS we need to specify additional linker flags:
// 	go test -v -run=^$ -fuzz=Fuzz_writeHeader -ldflags=-extldflags=-Wl,-ld_classic .

func Fuzz_readHeader(f *testing.F) {
	f.Add([]byte(nil))
	f.Add([]byte{0})
	f.Add([]byte{1, 2, 3, 4})
	f.Add([]byte{100: 123})
	f.Add(bytes.Repeat([]byte{1}, 10))

	f.Fuzz(func(t *testing.T, b []byte) {
		r := bytes.NewReader(b)

		_, err := readHeader(r)
		if err != nil {
			return
		}
	})
}

func Fuzz_writeHeader(f *testing.F) {
	f.Add(uint8(0), uint8(0), uint16(0), uint16(0), []byte{31: 0})
	f.Add(uint8(1), uint8(1), uint16(1), uint16(1), []byte{31: 0})
	f.Add(uint8(1), uint8(1), uint16(1), uint16(1), []byte{100: 1})

	f.Fuzz(func(t *testing.T, ver, typ uint8, shs, sqs uint16, b []byte) {
		// we expect hash to be 32 bytes, crop or extend to 32 bytes.
		diff := len(b) - 32
		if diff > 0 {
			b = b[:32]
		} else {
			pad := bytes.Repeat([]byte{0}, -diff)
			b = append(b, pad...)
		}

		testHdr := headerV0{
			fileVersion: fileVersion(ver),
			fileType:    fileType(typ),
			shareSize:   shs,
			squareSize:  sqs,
			datahash:    b,
		}

		w := bytes.NewBuffer(nil)
		err := writeHeader(w, &testHdr)
		if err != nil {
			return
		}

		hdr, err := readHeader(w)
		if err != nil {
			return
		}

		require.Equal(t, hdr.fileType, testHdr.fileType)
		require.Equal(t, hdr.fileType, testHdr.fileType)
		require.Equal(t, hdr.shareSize, testHdr.shareSize)
		require.Equal(t, hdr.squareSize, testHdr.squareSize)
		require.Equal(t, hdr.datahash, testHdr.datahash)
	})
}
