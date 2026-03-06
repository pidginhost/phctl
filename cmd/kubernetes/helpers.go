package kubernetes

import (
	"fmt"

	"github.com/pidginhost/phctl/internal/cmdutil"
)

var (
	parseInt32   = cmdutil.ParseInt32
	outputFormat = cmdutil.OutputFormat
	force        = cmdutil.Force
)

func pstr[T any](p *T) string {
	if p == nil {
		return "<none>"
	}
	return fmt.Sprintf("%v", *p)
}
