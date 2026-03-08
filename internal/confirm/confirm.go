package confirm

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func Action(in io.Reader, out io.Writer, msg string) bool {
	fmt.Fprintf(out, "%s [y/N]: ", msg)
	reader := bufio.NewReader(in)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
