package confirm

import (
	"os"
	"testing"
)

func pipeStdin(t *testing.T, input string) (cleanup func()) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("writing to pipe: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing pipe writer: %v", err)
	}

	old := os.Stdin
	os.Stdin = r
	return func() { os.Stdin = old }
}

func TestActionYes(t *testing.T) {
	cleanup := pipeStdin(t, "y\n")
	defer cleanup()

	if !Action("Continue?") {
		t.Error("expected Action to return true for 'y'")
	}
}

func TestActionYesFull(t *testing.T) {
	cleanup := pipeStdin(t, "yes\n")
	defer cleanup()

	if !Action("Continue?") {
		t.Error("expected Action to return true for 'yes'")
	}
}

func TestActionNo(t *testing.T) {
	cleanup := pipeStdin(t, "n\n")
	defer cleanup()

	if Action("Continue?") {
		t.Error("expected Action to return false for 'n'")
	}
}

func TestActionEmpty(t *testing.T) {
	cleanup := pipeStdin(t, "\n")
	defer cleanup()

	if Action("Continue?") {
		t.Error("expected Action to return false for empty input")
	}
}
