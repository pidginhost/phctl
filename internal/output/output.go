package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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

func Print(format Format, data any, tableFunc func(w io.Writer)) {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(data)
	case FormatYAML:
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		_ = enc.Encode(data)
	default:
		tableFunc(os.Stdout)
	}
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

// Pstr safely dereferences a pointer for display. Returns "" for nil.
func Pstr[T any](p *T) string {
	if p == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", *p)
}
