package confirm

import (
	"bytes"
	"strings"
	"testing"
)

func TestActionYes(t *testing.T) {
	var out bytes.Buffer

	if !Action(strings.NewReader("y\n"), &out, "Continue?") {
		t.Error("expected Action to return true for 'y'")
	}
}

func TestActionYesFull(t *testing.T) {
	var out bytes.Buffer

	if !Action(strings.NewReader("yes\n"), &out, "Continue?") {
		t.Error("expected Action to return true for 'yes'")
	}
}

func TestActionNo(t *testing.T) {
	var out bytes.Buffer

	if Action(strings.NewReader("n\n"), &out, "Continue?") {
		t.Error("expected Action to return false for 'n'")
	}
}

func TestActionEmpty(t *testing.T) {
	var out bytes.Buffer

	if Action(strings.NewReader("\n"), &out, "Continue?") {
		t.Error("expected Action to return false for empty input")
	}
}
