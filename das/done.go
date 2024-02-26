package das

import (
	"context"
	"fmt"
)

type done struct {
	name     string
	finished chan struct{}
}

func newDone(name string) done {
	return done{
		name:     name,
		finished: make(chan struct{}),
	}
}

func (d *done) indicateDone() {
	close(d.finished)
}

func (d *done) wait(ctx context.Context) error {
	select {
	case <-d.finished:
	case <-ctx.Done():
		return fmt.Errorf("%v stuck: %w", d.name, ctx.Err())
	}
	return nil
}
