package cmdutil

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestWriteAtomic_NewFile(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")

	if err := WriteAtomic(dst, []byte("hello"), 0600); err != nil {
		t.Fatalf("WriteAtomic error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(dst)
		if err != nil {
			t.Fatalf("Stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Errorf("perm = %o, want 0600", perm)
		}
	}
}

func TestWriteAtomic_ReplacesExisting(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")
	if err := os.WriteFile(dst, []byte("original"), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := WriteAtomic(dst, []byte("replaced"), 0600); err != nil {
		t.Fatalf("WriteAtomic error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("content = %q, want %q", got, "replaced")
	}
}

func TestWriteAtomic_NoLeftoverTempOnSuccess(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")
	if err := WriteAtomic(dst, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteAtomic error: %v", err)
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "out" {
		t.Errorf("expected single file 'out', got %v", entries)
	}
}

func TestWriteAtomic_PreservesOriginalOnRenameFailure(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")
	if err := os.WriteFile(dst, []byte("original"), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	old := renameFunc
	renameFunc = func(string, string) error { return errors.New("simulated rename failure") }
	t.Cleanup(func() { renameFunc = old })

	if err := WriteAtomic(dst, []byte("new"), 0600); err == nil {
		t.Fatal("expected error when rename fails")
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "original" {
		t.Errorf("dst was modified despite rename failure, got %q", got)
	}
}

func TestWriteAtomic_NoLeftoverTempOnRenameFailure(t *testing.T) {
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")
	if err := os.WriteFile(dst, []byte("original"), 0600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	old := renameFunc
	renameFunc = func(string, string) error { return errors.New("simulated rename failure") }
	t.Cleanup(func() { renameFunc = old })

	_ = WriteAtomic(dst, []byte("new"), 0600)

	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Errorf("expected only the original file to remain, got %v", entries)
	}
}

func TestWriteAtomic_TempInSameDirectory(t *testing.T) {
	// Same-directory temp ensures rename is on the same filesystem (atomic).
	tmp := t.TempDir()
	dst := filepath.Join(tmp, "out")

	old := renameFunc
	var capturedTmp string
	renameFunc = func(oldpath, newpath string) error {
		capturedTmp = oldpath
		return os.Rename(oldpath, newpath)
	}
	t.Cleanup(func() { renameFunc = old })

	if err := WriteAtomic(dst, []byte("x"), 0600); err != nil {
		t.Fatalf("WriteAtomic error: %v", err)
	}
	if filepath.Dir(capturedTmp) != tmp {
		t.Errorf("temp file %q is not in destination dir %q", capturedTmp, tmp)
	}
}
