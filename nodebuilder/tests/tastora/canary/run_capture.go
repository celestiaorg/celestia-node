package canary

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"sync/atomic"
	"time"

	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/model"
	"github.com/celestiaorg/celestia-node/nodebuilder/tests/tastora/canary/telemetry"
)

// logCapture reads the attach stream opened before Start, once the epoch is bound
// to the actual start time. There is no fallback to ContainerLogs, whose replay
// can be truncated.
type logCapture struct {
	stream io.ReadCloser
	done   chan struct{}
	// expected marks a stream end the engine anticipates (graceful stop); cut
	// marks an attachment the engine itself closed while the node still ran.
	expected, cut atomic.Bool
	bytes         atomic.Int64
	c             *telemetry.Collector
	epoch         int
	// exit holds the process state when the node's output ended because the
	// node exited on its own.
	exit atomic.Pointer[model.ProcessInfo]
}

// nodeProcess reports the state of the node process whose output is captured.
type nodeProcess interface {
	Inspect(context.Context) (model.ProcessInfo, error)
}

func captureLogs(
	ctx context.Context,
	stream io.ReadCloser,
	c *telemetry.Collector,
	epoch int,
	node nodeProcess,
	settle time.Duration,
) *logCapture {
	capture := &logCapture{stream: stream, done: make(chan struct{}), c: c, epoch: epoch}
	go func() {
		defer close(capture.done)
		p, w := io.Pipe()
		parsed := make(chan struct{})
		var scanErr error
		go func() {
			defer close(parsed)
			scanner := bufio.NewScanner(p)
			scanner.Split(nativeLogLines)
			scanner.Buffer(make([]byte, 4096), 1<<20)
			for scanner.Scan() {
				capture.bytes.Add(int64(len(scanner.Bytes())))
				// The collector keeps an incompatibility or gap sticky; keep draining the stream.
				_ = c.LogRecord(epoch, bytes.Clone(scanner.Bytes()))
			}
			scanErr = scanner.Err()
			p.Close()
		}()
		err := copyNativeStderr(w, stream)
		if capture.cut.Load() {
			err = nil // The engine closed the attachment; the transport error is its own.
		}
		w.CloseWithError(err)
		<-parsed
		// The node's output ended although the engine did not stop it. If the
		// node exited on its own, every record it wrote was delivered: that is
		// a node failure for the waiting phase to report, not lost evidence.
		exited := false
		if !capture.expected.Load() && ctx.Err() == nil && err == nil {
			if info, ok := awaitExit(ctx, node, settle); ok {
				capture.exit.Store(&info)
				exited = true
			}
		}
		// A fragment cut by the engine's own close or by the node's own exit is
		// not lost evidence: every complete record before it was delivered.
		if scanErr != nil && ((!capture.cut.Load() && !exited) || !errors.Is(scanErr, io.ErrUnexpectedEOF)) {
			c.MarkGap(epoch, "native stderr record overflow or truncation")
		}
		switch {
		case exited:
		case !capture.expected.Load() && ctx.Err() == nil:
			c.MarkGap(epoch, "unexpected log stream termination")
		case err != nil && ctx.Err() == nil:
			c.MarkGap(epoch, "log transport error")
		}
	}()
	return capture
}

// awaitExit reports whether the node process has exited, giving Docker up to
// settle to publish the exit after the node's output ended.
func awaitExit(ctx context.Context, node nodeProcess, settle time.Duration) (model.ProcessInfo, bool) {
	if node == nil {
		return model.ProcessInfo{}, false
	}
	deadline := time.Now().Add(settle)
	for {
		info, err := node.Inspect(ctx)
		if err == nil && !info.Running && !info.FinishedAt.IsZero() {
			return info, true
		}
		if !time.Now().Before(deadline) || pause(ctx, settle/20) != nil {
			return model.ProcessInfo{}, false
		}
	}
}

// exited returns the process state when the node's output ended because the
// node exited on its own.
func (c *logCapture) exited() (model.ProcessInfo, bool) {
	if c == nil {
		return model.ProcessInfo{}, false
	}
	if info := c.exit.Load(); info != nil {
		return *info, true
	}
	return model.ProcessInfo{}, false
}

// The node's stderr records end with a newline; valid JSON alone does not prove
// that the last record arrived whole.
func nativeLogLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) > 0 && bytes.IndexByte(data, '\n') < 0 {
		return 0, nil, io.ErrUnexpectedEOF
	}
	return bufio.ScanLines(data, atEOF)
}

// copyNativeStderr requires complete Docker mux frames, including on expected
// shutdown. Moby StdCopy treats a partial final header/payload as clean EOF.
// CopyN streams payloads without allocating a buffer from an untrusted length.
func copyNativeStderr(stderr io.Writer, source io.Reader) error {
	var h [8]byte
	for {
		n, err := io.ReadFull(source, h[:])
		if errors.Is(err, io.EOF) && n == 0 {
			return nil
		}
		if err != nil {
			return err
		}
		if h[1] != 0 || h[2] != 0 || h[3] != 0 {
			return errors.New("invalid native log frame header")
		}
		var destination io.Writer
		switch h[0] {
		case 1:
			destination = io.Discard
		case 2:
			destination = stderr
		default:
			return errors.New("unexpected native log stream")
		}
		size := int64(binary.BigEndian.Uint32(h[4:]))
		if _, err = io.CopyN(destination, source, size); err != nil {
			return err
		}
	}
}

func (c *logCapture) drain(ctx context.Context) {
	if c == nil {
		return
	}
	select {
	case <-c.done:
	case <-ctx.Done():
		c.c.MarkGap(c.epoch, "log drain deadline")
		c.stream.Close()
		<-c.done
	}
	c.stream.Close()
}

func (c *logCapture) close() {
	if c == nil {
		return
	}
	c.cut.Store(true)
	c.expected.Store(true)
	c.stream.Close()
	<-c.done
}
