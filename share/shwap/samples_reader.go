package shwap

import (
	"errors"
	"io"
)

// SamplesReader is a wrapper over the samples that implements basic interfaces
// to stream the samples.
type SamplesReader struct {
	samples []Sample
	index   int    // index of the sample that will be sent next
	data    []byte // serialized sample
	offset  int64  // offset within the data
}

func NewSamplesReader(samples ...Sample) *SamplesReader {
	if len(samples) == 0 {
		samples = make([]Sample, 0)
	}
	return &SamplesReader{
		samples: samples,
	}
}

func (sr *SamplesReader) Samples() []Sample {
	return sr.samples
}

func (sr *SamplesReader) Len() int {
	return len(sr.samples)
}

// Read serializes samples into bytes one by one and put it inside the passed buffer `p`.
func (sr *SamplesReader) Read(p []byte) (int, error) {
	if sr.index >= len(sr.samples) {
		return 0, io.EOF
	}

	if sr.data == nil {
		sample := sr.samples[sr.index]
		serialized, err := sample.MarshalJSON()
		if err != nil {
			return 0, err
		}
		sr.data = append(sr.data, serialized...)
		sr.offset = 0
	}

	n := copy(p, sr.data[sr.offset:])
	sr.offset += int64(n)
	// move on to the next index if sample was fully written.
	if n >= len(sr.data) {
		sr.index++
		sr.data = nil
		sr.offset = 0
	}
	return n, nil
}

func (sr *SamplesReader) WriteTo(writer io.Writer) (int64, error) {
	var nn int64
	for _, sample := range sr.samples {
		n, err := sample.WriteTo(writer)
		if err != nil {
			return 0, err
		}
		nn += n
	}
	return nn, nil
}

func (sr *SamplesReader) ReadFrom(reader io.Reader) (int64, error) {
	var nn int64
	for {
		var sample Sample
		n, err := sample.ReadFrom(reader)
		nn += n
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nn, err
		}

		sr.samples = append(sr.samples, sample)
	}
	return nn, nil
}
