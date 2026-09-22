package telemetry

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
)

var errNotJSON = errors.New("not a JSON object record")

// The records the adapter reads: go-log/v2 JSON core, celestia-node
// share/light and das loggers.
const (
	loggerLight = "share/light"
	loggerDAS   = "das"

	msgSessionStart = "starting sampling session"
	msgSampled      = "sampled header"
	msgSampleFailed = "failed to sample header"

	levelDebug = "debug"
	levelInfo  = "info"
	levelError = "error"
)

// decodeValue rejects duplicate keys at every nesting level. UseNumber keeps
// uint64 heights exact instead of routing them through float64.
func decodeValue(d *json.Decoder, depth int) (any, error) {
	if depth > 32 {
		return nil, fmt.Errorf("JSON nesting limit")
	}
	tok, err := d.Token()
	if err != nil {
		return nil, err
	}
	switch tok {
	case json.Delim('{'):
		m := map[string]any{}
		for d.More() {
			k, err := d.Token()
			if err != nil {
				return nil, err
			}
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("object key")
			}
			if _, ok = m[key]; ok {
				return nil, fmt.Errorf("duplicate key")
			}
			v, err := decodeValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			m[key] = v
		}
		_, err = d.Token()
		return m, err
	case json.Delim('['):
		var a []any
		for d.More() {
			v, err := decodeValue(d, depth+1)
			if err != nil {
				return nil, err
			}
			a = append(a, v)
		}
		_, err = d.Token()
		return a, err
	default:
		return tok, nil
	}
}

func hex32(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, b := range []byte(s) {
		if (b < '0' || b > '9') && (b < 'A' || b > 'F') {
			return false
		}
	}
	return true
}

func positive(v any) (uint64, bool) {
	n, ok := v.(json.Number)
	if !ok {
		return 0, false
	}
	s := string(n)
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return 0, false
		}
	}
	i, err := strconv.ParseUint(s, 10, 64)
	return i, err == nil && i > 0
}

// parseLog decodes one go-log JSON record. Relevant records are the
// light-availability session start and the DAS worker completion/failure for
// a header. Records from other loggers are ignored; lines that are not JSON
// objects return errNotJSON so the caller can count them without failing.
//
// Record shapes (celestia-node, go-log/v2 JSON core, millisecond timestamps):
//
//	START:   logger=share/light level=debug msg="starting sampling session" root
//	SUCCESS: logger=das msg="sampled header" type height hash "EDS square width" "data root" "finished (s)"
//	         (level info for recent jobs, debug for catchup/retry)
//	FAILURE: logger=das level=error msg="failed to sample header" ... "square width" err
func parseLog(raw []byte) (logRecord, bool, error) {
	var r logRecord
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return r, false, errNotJSON
	}
	d := json.NewDecoder(bytes.NewReader(trimmed))
	d.UseNumber()
	v, err := decodeValue(d, 0)
	if err != nil {
		return r, false, fmt.Errorf("malformed JSON record: %w", err)
	}
	if _, err = d.Token(); !errors.Is(err, io.EOF) {
		return r, false, fmt.Errorf("trailing JSON")
	}
	m, ok := v.(map[string]any)
	if !ok {
		return r, false, errNotJSON
	}
	text := func(k string) string { s, _ := m[k].(string); return s }
	if v, exists := m["logger"]; exists {
		if _, ok := v.(string); !ok {
			return r, false, fmt.Errorf("logger type")
		}
	}
	logger, msg := text("logger"), text("msg")
	if logger == loggerLight || logger == loggerDAS {
		if _, ok := m["msg"].(string); !ok {
			return r, false, fmt.Errorf("msg type")
		}
	}
	switch {
	case logger == loggerLight && msg == msgSessionStart:
		r.kind = kindStart
		r.root = text("root")
		if text("level") != levelDebug {
			return r, false, fmt.Errorf("start level")
		}
	case logger == loggerDAS && (msg == msgSampled || msg == msgSampleFailed):
		r.kind = kindSuccess
		key := "EDS square width"
		level := levelDebug
		if text("type") == model.JobRecent {
			level = levelInfo
		}
		if msg == msgSampleFailed {
			r.kind = kindFailure
			key = "square width"
			level = levelError
			if text("err") == "" {
				return r, false, fmt.Errorf("failure err")
			}
		}
		if text("level") != level {
			return r, false, fmt.Errorf("completion level")
		}
		r.jobType = text("type")
		switch r.jobType {
		case model.JobRecent, model.JobCatchup, model.JobRetry:
		default:
			return r, false, fmt.Errorf("job type")
		}
		var ok bool
		r.height, ok = positive(m["height"])
		if !ok {
			return r, false, fmt.Errorf("height")
		}
		w, ok := positive(m[key])
		if !ok || w > math.MaxInt32 {
			return r, false, fmt.Errorf("width")
		}
		r.width = int(w)
		r.root = text("data root")
		r.hash = text("hash")
		if !hex32(r.hash) {
			return r, false, fmt.Errorf("hash")
		}
		n, ok := m["finished (s)"].(json.Number)
		if !ok {
			return r, false, fmt.Errorf("duration")
		}
		r.duration, err = n.Float64()
		if err != nil || r.duration < 0 || math.IsInf(r.duration, 0) || math.IsNaN(r.duration) {
			return r, false, fmt.Errorf("duration")
		}
	default:
		return r, false, nil
	}
	if !hex32(r.root) {
		return r, false, fmt.Errorf("root")
	}
	if text("caller") == "" {
		return r, false, fmt.Errorf("caller")
	}
	r.ts, err = time.Parse(time.RFC3339Nano, text("ts"))
	if err != nil || r.ts.IsZero() {
		return r, false, fmt.Errorf("timestamp")
	}
	return r, true, nil
}
