package fslock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocker(t *testing.T) {
	path := filepath.Join(os.TempDir(), ".lock")
	defer os.Remove(path)

	locker := New(path)
	locker2 := New(path)

	err := locker.Lock()
	if err != nil {
		t.Fatal(err)
	}

	err = locker2.Lock()
	if err != ErrLocked {
		t.Fatal("No locking")
	}

	err = locker.Unlock()
	if err != nil {
		t.Fatal(err)
	}

	err = locker2.Lock()
	if err != nil {
		t.Fatal(err)
	}

	err = locker2.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}
