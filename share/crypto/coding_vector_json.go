package crypto

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type codingVectorJSON struct {
	K     uint16   `json:"k"`
	Seed  string   `json:"seed,omitempty"`  // base64
	Coeff []string `json:"coeff,omitempty"` // base64 list (optional)
}

func (v CodingVector) MarshalJSON() ([]byte, error) {
	out := codingVectorJSON{K: v.K}

	zero := true
	for _, b := range v.Seed {
		if b != 0 {
			zero = false
			break
		}
	}
	if !zero {
		out.Seed = base64.StdEncoding.EncodeToString(v.Seed[:])
	}

	if len(v.Coeffs) > 0 {
		out.Coeff = make([]string, 0, len(v.Coeffs))
		for _, c := range v.Coeffs {
			out.Coeff = append(out.Coeff, base64.StdEncoding.EncodeToString(c.Bytes))
		}
	}

	return json.Marshal(out)
}

func (v *CodingVector) UnmarshalJSON(b []byte) error {
	var in codingVectorJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return err
	}
	v.K = in.K

	if in.Seed != "" {
		seedBytes, err := base64.StdEncoding.DecodeString(in.Seed)
		if err != nil {
			return fmt.Errorf("coding vector seed: %w", err)
		}
		if len(seedBytes) != 32 {
			return fmt.Errorf("coding vector seed: expected 32 bytes, got %d", len(seedBytes))
		}
		copy(v.Seed[:], seedBytes)
	}

	if len(in.Coeff) > 0 {
		v.Coeffs = make([]FieldElement, 0, len(in.Coeff))
		for _, s := range in.Coeff {
			cb, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("coding vector coeff: %w", err)
			}
			v.Coeffs = append(v.Coeffs, FieldElement{Bytes: cb})
		}
	}

	return nil
}

