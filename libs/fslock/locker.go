package fslock

import (
	"errors"
	"os"
)

var ErrLocked = errors.New("fslock: directory is locked")

func Lock(path string) (*Locker, error) {
	l := New(path)
	err := l.Lock()
	if err != nil {
		return nil, err
	}

	return l, nil
}

// TODO Add windows support
type Locker struct {
	file *os.File
	path string
}

func New(path string) *Locker {
	return &Locker{path: path}
}

func (l *Locker) Lock() error {
	return l.lock()
}

func (l *Locker) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}

	return l.unlock()
}
