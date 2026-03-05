package output

import (
	"testing"
)

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input string
		want  Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"yaml", FormatYAML},
		{"YAML", FormatYAML},
		{"table", FormatTable},
		{"TABLE", FormatTable},
		{"", FormatTable},
		{"unknown", FormatTable},
	}

	for _, tt := range tests {
		got := ParseFormat(tt.input)
		if got != tt.want {
			t.Errorf("ParseFormat(%q) = %q, want %q", tt.input, got, tt.want)
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
}
