package blob

import "encoding/json"

// Batch is wrapper over the slice of Blobs + fee params.
type Batch struct {
	Blobs    []*Blob `json:"blobs"`
	Fee      int64   `json:"fee"`
	GasLimit uint64  `json:"gas_limit"`
}

// Option is a function that allows to configure the Submit request.
type Option func(*Batch)

// WithFee is an option that allows to configure the fee to submit a batch of the blobs.
func WithFee(fee int64) Option {
	return func(batch *Batch) {
		batch.Fee = fee
	}
}

// WithGasLimit is an option that allows to configure the gas limit to submit a batch of the blobs.
func WithGasLimit(limit uint64) Option {
	return func(batch *Batch) {
		batch.GasLimit = limit
	}
}

func NewBatch(blobs []*Blob, options ...Option) *Batch {
	batch := &Batch{
		Blobs: blobs,
		Fee:   -1,
	}

	for _, opt := range options {
		opt(batch)
	}
	return batch
}

func (b *Batch) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(b.Blobs)
	if err != nil {
		return nil, err
	}
	return json.Marshal(&struct {
		Blobs    json.RawMessage `json:"blobs"`
		Fee      int64           `json:"fee"`
		GasLimit uint64          `json:"gas_limit"`
	}{
		Blobs:    data,
		Fee:      b.Fee,
		GasLimit: b.GasLimit,
	})
}

func (b *Batch) UnmarshalJSON(data []byte) error {
	j := &struct {
		Blobs    json.RawMessage `json:"blobs"`
		Fee      int64           `json:"fee"`
		GasLimit uint64          `json:"gas_limit"`
	}{}
	err := json.Unmarshal(data, j)
	if err != nil {
		return err
	}

	err = json.Unmarshal(j.Blobs, &b.Blobs)
	if err != nil {
		return err
	}
	b.Fee = j.Fee
	b.GasLimit = j.GasLimit
	return nil
}
