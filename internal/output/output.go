package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

func IsValidFormat(s string) bool {
	switch strings.ToLower(s) {
	case string(FormatTable), string(FormatJSON), string(FormatYAML):
		return true
	default:
		return false
	}
}

func ParseFormat(s string) Format {
	switch strings.ToLower(s) {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	default:
		return FormatTable
	}
}

func Print(out io.Writer, format Format, data any, tableFunc func(w io.Writer)) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("encoding JSON: %w", err)
		}
	case FormatYAML:
		enc := yaml.NewEncoder(out)
		enc.SetIndent(2)
		if err := enc.Encode(data); err != nil {
			return fmt.Errorf("encoding YAML: %w", err)
		}
	default:
		tableFunc(out)
	}
	return nil
}

func NewTabWriter(out io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
}

func PrintRow(w *tabwriter.Writer, fields ...any) {
	strs := make([]string, len(fields))
	for i, f := range fields {
		strs[i] = fmt.Sprintf("%v", f)
	}
	fmt.Fprintln(w, strings.Join(strs, "\t"))
}

// Pstr safely dereferences a pointer for display. Returns "<none>" for nil.
func Pstr[T any](p *T) string {
	if p == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", *p)
}
