package output

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"Json", FormatJSON},
		{"yaml", FormatYAML},
		{"YAML", FormatYAML},
		{"Yaml", FormatYAML},
		{"table", FormatTable},
		{"TABLE", FormatTable},
		{"", FormatTable},
		{"unknown", FormatTable},
		{"csv", FormatTable},
	}

	for _, tt := range tests {
		got := ParseFormat(tt.input)
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestIsValidFormat(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"table", true},
		{"TABLE", true},
		{"json", true},
		{"yaml", true},
		{"csv", false},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := IsValidFormat(tt.input); got != tt.want {
			t.Errorf("IsValidFormat(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPstr(t *testing.T) {
	s := "hello"
	if got := Pstr(&s); got != "hello" {
		t.Errorf("Pstr(&%q) = %q, want %q", s, got, "hello")
	}
	if got := Pstr[string](nil); got != "<none>" {
		t.Errorf("Pstr(nil) = %q, want %q", got, "<none>")
	}
	n := 42
	if got := Pstr(&n); got != "42" {
		t.Errorf("Pstr(&42) = %q, want %q", got, "42")
	}
	b := true
	if got := Pstr(&b); got != "true" {
		t.Errorf("Pstr(&true) = %q, want %q", got, "true")
	}
	if got := Pstr[int](nil); got != "<none>" {
		t.Errorf("Pstr[int](nil) = %q, want %q", got, "<none>")
	}
}

func TestNewTabWriter(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTabWriter(&buf)
	if tw == nil {
		t.Fatal("NewTabWriter returned nil")
	}
	PrintRow(tw, "A", "B")
	PrintRow(tw, "hello", "world")
	tw.Flush()

	out := buf.String()
	if !strings.Contains(out, "A") || !strings.Contains(out, "B") {
		t.Errorf("expected header, got: %q", out)
	}
	if !strings.Contains(out, "hello") || !strings.Contains(out, "world") {
		t.Errorf("expected data, got: %q", out)
	}
}

func TestPrintRow(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTabWriter(&buf)
	PrintRow(tw, "id", 42, true)
	tw.Flush()

	out := buf.String()
	for _, want := range []string{"id", "42", "true"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got: %q", want, out)
		}
	}
}

func TestPrintRowTabSeparated(t *testing.T) {
	var buf bytes.Buffer
	tw := NewTabWriter(&buf)
	PrintRow(tw, "col1", "col2", "col3")
	tw.Flush()

	out := buf.String()
	// After tab writer flushes, columns should be separated by spaces
	if !strings.Contains(out, "col1") || !strings.Contains(out, "col3") {
		t.Errorf("expected all columns, got: %q", out)
	}
}

// captureStdout runs fn while capturing os.Stdout output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("r.Close: %v", err)
	}
	return buf.String()
}

func TestPrintJSON(t *testing.T) {
	data := map[string]string{"key": "value"}
	out := captureStdout(t, func() {
		if err := Print(FormatJSON, data, nil); err != nil {
			t.Fatalf("Print JSON error: %v", err)
		}
	})

	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %q", err, out)
	}
	if got["key"] != "value" {
		t.Errorf("key = %q, want %q", got["key"], "value")
	}
}

func TestPrintYAML(t *testing.T) {
	data := map[string]string{"name": "test"}
	out := captureStdout(t, func() {
		if err := Print(FormatYAML, data, nil); err != nil {
			t.Fatalf("Print YAML error: %v", err)
		}
	})

	var got map[string]string
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not valid YAML: %v\noutput: %q", err, out)
	}
	if got["name"] != "test" {
		t.Errorf("name = %q, want %q", got["name"], "test")
	}
}

func TestPrintTable(t *testing.T) {
	called := false
	out := captureStdout(t, func() {
		if err := Print(FormatTable, nil, func(w io.Writer) {
			called = true
			_, _ = w.Write([]byte("table output\n"))
		}); err != nil {
			t.Fatalf("Print table error: %v", err)
		}
	})

	if !called {
		t.Error("table function was not called")
	}
	if !strings.Contains(out, "table output") {
		t.Errorf("expected table output, got: %q", out)
	}
}

func TestPrintJSONSlice(t *testing.T) {
	data := []string{"a", "b", "c"}
	out := captureStdout(t, func() {
		if err := Print(FormatJSON, data, nil); err != nil {
			t.Fatalf("Print JSON error: %v", err)
		}
	})

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d items, want 3", len(got))
	}
}

func TestPrintJSONIndented(t *testing.T) {
	data := map[string]int{"x": 1}
	out := captureStdout(t, func() {
		_ = Print(FormatJSON, data, nil)
	})

	// Should be indented with 2 spaces
	if !strings.Contains(out, "  ") {
		t.Errorf("JSON output should be indented, got: %q", out)
	}
}
