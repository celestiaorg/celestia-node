package fslock

import (
	"errors"
	"os"
)

// ErrLocked is signaled when someone tries to lock an already locked file.
var ErrLocked = errors.New("fslock: directory is locked")

// Lock creates a new Locker under the given 'path'
// and immediately does Lock on it.
func Lock(path string) (*Locker, error) {
	l := New(path)
	err := l.Lock()
	if err != nil {
		return nil, err
	}

	return l, nil
}

// Locker is a simple utility meant to create lock files.
// This is to prevent multiple processes from managing the same working directory by purpose or
// accident. NOTE: Windows is not supported.
type Locker struct {
	file *os.File
	path string
}

// New creates a new Locker with a File pointing to the given 'path'.
func New(path string) *Locker {
	return &Locker{path: path}
}

// Lock locks the file.
// Subsequent calls will error with ErrLocked on any Locker instance looking to the same path.
func (l *Locker) Lock() error {
	return l.lock()
}

// Unlock frees up the lock.
func (l *Locker) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}

	return l.unlock()
}
