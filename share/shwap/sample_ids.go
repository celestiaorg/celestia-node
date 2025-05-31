package shwap

import (
	"context"
	"errors"
	"fmt"
	"io"
)

type SampleIDs []SampleID

func (s SampleIDs) Len() int {
	return len(s)
}

func (s SampleIDs) WriteTo(writer io.Writer) (int64, error) {
	var n int64
	for _, sampleID := range s {
		nn, err := sampleID.WriteTo(writer)
		n += nn
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (s *SampleIDs) ReadFrom(reader io.Reader) (int64, error) {
	var ids []SampleID
	var n int64
	for {
		var id SampleID
		nn, err := id.ReadFrom(reader)
		n += nn
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return n, err
		}

		ids = append(ids, id)
	}

	// all sampleIDs have been read
	*s = ids
	return n, nil
}

func (s *SampleIDs) Name() string {
	return "samples_v0"
}

func (s SampleIDs) Height() uint64 {
	if len(s) == 0 {
		return 0
	}
	return s[0].Height()
}

func (s SampleIDs) Validate() error {
	for _, sample := range s {
		if err := sample.Validate(); err != nil {
			return fmt.Errorf("failed to validate sample %v: %w", sample, err)
		}
	}
	return nil
}

func (s SampleIDs) ContainerDataReader(ctx context.Context, acc Accessor) (io.Reader, error) {
	samples := make([]Sample, len(s))
	for i, sampleID := range s {
		sample, err := acc.Sample(ctx, SampleCoords{Row: sampleID.RowIndex, Col: sampleID.ShareIndex})
		if err != nil {
			// short-circuit even if at least one of the request failed.
			return nil, err
		}
		samples[i] = sample
	}
	return NewSamplesReader(samples...), nil
}
