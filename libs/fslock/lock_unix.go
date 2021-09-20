//go:build darwin || freebsd || linux

package fslock

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
)

func (l *Locker) lock() (err error) {
	l.file, err = os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return fmt.Errorf("fslock: error opening file: %v", err)
	}

	_, err = l.file.WriteString(strconv.Itoa(os.Getpid()))
	if err != nil {
		return fmt.Errorf("fslock: error writing process id: %v", err)
	}

	err = syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil && err.Error() == "resource temporarily unavailable" {
		return ErrLocked
	}
	if err != nil {
		return fmt.Errorf("fslock: flocking error: %v", err)
	}

	return nil
}

func (l *Locker) unlock() error {
	err := syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN|syscall.LOCK_NB)
	if err != nil {
		return fmt.Errorf("fslock: unflocking error: %v", err)
	}

	file := l.file
	l.file = nil
	err = file.Close()
	if err != nil {
		return fmt.Errorf("fslock: while closing file: %v", err)
	}

	return os.Remove(l.path)
}
