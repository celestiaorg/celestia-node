package canary

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

func rawLogFrame(stream byte, payload []byte) []byte {
	h := make([]byte, 8, 8+len(payload))
	h[0] = stream
	binary.BigEndian.PutUint32(h[4:], uint32(len(payload)))
	return append(h, payload...)
}

func TestNativeLogFrameReservedBytesRejected(t *testing.T) {
	frame := rawLogFrame(2, nil)
	frame[1] = 1
	require.Error(t, copyNativeStderr(io.Discard, bytes.NewReader(frame)))
}

func TestNativeMuxCompleteFragmentedAndTruncatedFrames(t *testing.T) {
	for _, stream := range []byte{1, 2} {
		frame := rawLogFrame(stream, []byte("payload"))
		var dst bytes.Buffer
		require.NoError(t, copyNativeStderr(&dst, iotest.OneByteReader(bytes.NewReader(frame))))
		if stream == 2 {
			require.Equal(t, "payload", dst.String())
		} else {
			require.Empty(t, dst.String())
		}
		for n := 1; n < len(frame); n++ {
			require.Error(
				t,
				copyNativeStderr(io.Discard, bytes.NewReader(frame[:n])),
				"stream %d truncation %d",
				stream,
				n,
			)
		}
	}
	require.NoError(t, copyNativeStderr(io.Discard, bytes.NewReader(nil)))
	for _, stream := range []byte{0, 3, 4, 255} {
		require.Error(t, copyNativeStderr(io.Discard, bytes.NewReader(rawLogFrame(stream, nil))))
	}
}

func TestNativeLogRecordRequiresTerminatingNewline(t *testing.T) {
	c := collectorFor(t, localProfile())
	require.NoError(t, c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: time.Now().UTC()}))
	raw, err := json.Marshal(
		map[string]any{
			"ts":     time.Now().UTC().Format(time.RFC3339Nano),
			"level":  "info",
			"logger": "p2p",
			"msg":    "synthetic complete JSON without native record delimiter",
		},
	)
	require.NoError(t, err)
	reader, writer := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	capture := captureLogs(ctx, reader, c, 1, nil, 0)
	capture.expected.Store(true)
	_, err = writer.Write(rawLogFrame(2, raw))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	capture.drain(ctx)
	require.Equal(t, true, c.Diagnostics()["evidence_gap"], "unterminated native stderr record accepted")
}

// TestCapturedNativeMuxIntegrity replays a Docker log capture given in
// CFN_NATIVE_MUX_PATH and checks its framing and record boundaries.
func TestCapturedNativeMuxIntegrity(t *testing.T) {
	path := os.Getenv("CFN_NATIVE_MUX_PATH")
	if path == "" {
		t.Skip("set CFN_NATIVE_MUX_PATH and CFN_NATIVE_MUX_SHA256 for real native capture replay")
	}
	expected := os.Getenv("CFN_NATIVE_MUX_SHA256")
	require.Len(t, expected, 64)
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, (64<<20)+1))
	require.NoError(t, err)
	require.NotEmpty(t, raw)
	require.LessOrEqual(t, len(raw), 64<<20)
	require.Equal(t, expected, fmt.Sprintf("%x", sha256.Sum256(raw)))
	var stderr bytes.Buffer
	require.NoError(t, copyNativeStderr(&stderr, bytes.NewReader(raw)))
	scanner := bufio.NewScanner(&stderr)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	scanner.Split(nativeLogLines)
	records := 0
	for scanner.Scan() {
		records++
	}
	require.NoError(t, scanner.Err())
	require.Positive(t, records)
	require.Error(t, copyNativeStderr(io.Discard, bytes.NewReader(raw[:len(raw)-1])))
	t.Logf(
		"verified native capture bytes=%d stderr_records=%d sha256=%s; truncated clone rejected",
		len(raw),
		records,
		expected,
	)
}

func TestExpectedLogEOFRejectsPartialMuxHeader(t *testing.T) {
	c := collectorFor(t, localProfile())
	require.NoError(t, c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: time.Now().UTC()}))
	reader, writer := io.Pipe()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	capture := captureLogs(ctx, reader, c, 1, nil, 0)
	capture.expected.Store(true)
	_, err := writer.Write([]byte{2, 0, 0, 0, 0, 0, 0})
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	capture.drain(ctx)
	require.Equal(
		t,
		true,
		c.Diagnostics()["evidence_gap"],
		"partial native mux header was silently accepted after expected shutdown",
	)
}

// scriptedNode answers Inspect from a script; the last state repeats.
type scriptedNode struct {
	mu     sync.Mutex
	states []model.ProcessInfo
	calls  int
}

func (n *scriptedNode) Inspect(context.Context) (model.ProcessInfo, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	state := n.states[min(n.calls, len(n.states)-1)]
	n.calls++
	return state, nil
}

func TestCaptureTellsANodeExitFromALostStream(t *testing.T) {
	running := model.ProcessInfo{ContainerID: "container", Running: true}
	exited := model.ProcessInfo{ContainerID: "container", FinishedAt: time.Now().UTC(), ExitCode: 2}
	record, err := json.Marshal(map[string]any{
		"ts": time.Now().UTC().Format(time.RFC3339Nano), "level": "info", "logger": "p2p", "msg": "alive",
	})
	require.NoError(t, err)
	for _, tc := range []struct {
		name    string
		states  []model.ProcessInfo
		tail    []byte // bytes the node wrote after its last complete record
		exited  bool
		gapWhen string
	}{
		{"node exited", []model.ProcessInfo{exited}, nil, true, ""},
		{"node exited mid-record", []model.ProcessInfo{exited}, []byte(`{"logger":"das","msg":"sam`), true, ""},
		{"exit reported late", []model.ProcessInfo{running, running, exited}, nil, true, ""},
		{"node still running", []model.ProcessInfo{running}, nil, false, "epoch 1: unexpected log stream termination"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := collectorFor(t, localProfile())
			require.NoError(t, c.BeginEpoch(telemetry.Epoch{Number: 1, StartedAt: time.Now().UTC().Add(-time.Second)}))
			reader, writer := io.Pipe()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			capture := captureLogs(ctx, reader, c, 1, &scriptedNode{states: tc.states}, 200*time.Millisecond)
			_, err := writer.Write(rawLogFrame(2, append(record, '\n')))
			require.NoError(t, err)
			if tc.tail != nil {
				_, err = writer.Write(rawLogFrame(2, tc.tail))
				require.NoError(t, err)
			}
			require.NoError(t, writer.Close())
			select {
			case <-capture.done:
			case <-ctx.Done():
				t.Fatal("capture did not finish")
			}
			info, ok := capture.exited()
			require.Equal(t, tc.exited, ok)
			d := c.Diagnostics()
			require.Equal(t, tc.gapWhen != "", d["evidence_gap"])
			require.Equal(t, tc.gapWhen, d["gap_reason"])
			if tc.exited {
				require.Equal(t, 2, info.ExitCode)
			}
		})
	}
}
