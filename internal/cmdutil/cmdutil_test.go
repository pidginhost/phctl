package cmdutil

import (
	"testing"
)

func TestParseInt32(t *testing.T) {
	tests := []struct {
		input   string
		want    int32
		wantErr bool
	}{
		{"1", 1, false},
		{"0", 0, false},
		{"2147483647", 2147483647, false},
		{"-1", -1, false},
		{"abc", 0, true},
		{"", 0, true},
		{"99999999999", 0, true},
	}

	for _, tt := range tests {
		got, err := ParseInt32(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseInt32(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseInt32(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
