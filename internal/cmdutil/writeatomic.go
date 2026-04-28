package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// renameFunc is a seam so tests can simulate a failing rename.
var renameFunc = os.Rename

// WriteAtomic writes data to path via a temp file in the same directory
// followed by an os.Rename, so observers either see the previous file or
// the new file — never a half-written intermediate state. The temp file
// is cleaned up on any failure.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() { _ = os.Remove(tmpName) }

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("setting temp file permissions: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return fmt.Errorf("flushing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := renameFunc(tmpName, path); err != nil {
		cleanup()
		return fmt.Errorf("renaming into place: %w", err)
	}
	return nil
}
