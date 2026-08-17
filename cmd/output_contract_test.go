package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// The -o/--output contract
//
// Rule: a command that reports a resource returned by the API must render it
// through internal/output (output.Print / output.Result), never through
// cmd.Print*. Anything written with cmd.Print* is a plain sentence, so under
// `-o json` the caller gets prose where it asked for a document — the flag
// silently does nothing.
//
// Commands whose only output is an acknowledgement of a side effect ("Server 42
// deleted."), an interactive prompt, or progress narration are exempt: there is
// no payload to encode.
//
// This guard catches the common regression shape: printing fields off a decoded
// API response with cmd.Print*. It keys off the response-variable names this
// codebase uses by convention, so it is a tripwire rather than a proof — a
// response bound to an unusual name slips past it.
var apiResponseIdents = map[string]bool{
	"resp":   true,
	"result": true,
	"key":    true,
}

func TestCommandsReturningDataDoNotBypassOutputFlag(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve cmd dir: %v", err)
	}

	var violations []string
	fset := token.NewFileSet()

	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if !isCmdPrintCall(call.Fun) {
				return true
			}
			for _, arg := range call.Args {
				field, ok := apiResponseFieldRoot(arg)
				if !ok {
					continue
				}
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				violations = append(violations, fmt.Sprintf(
					"%s:%d prints %s via cmd.Print*; route it through output.Result so -o json works",
					filepath.ToSlash(rel), fset.Position(call.Pos()).Line, field))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd tree: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("commands returning API data must not print via cmd.Print*:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

// isCmdPrintCall reports whether fn is a cmd.Print / cmd.Printf / cmd.Println
// selector. Stderr variants are deliberately excluded: progress and diagnostics
// belong on stderr regardless of -o.
func isCmdPrintCall(fn ast.Expr) bool {
	sel, ok := fn.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	recv, ok := sel.X.(*ast.Ident)
	if !ok || recv.Name != "cmd" {
		return false
	}
	switch sel.Sel.Name {
	case "Print", "Printf", "Println":
		return true
	default:
		return false
	}
}

// apiResponseFieldRoot reports whether expr reads a field off a variable that
// this codebase binds an API response to, e.g. resp.Id or key.Fingerprint.
func apiResponseFieldRoot(expr ast.Expr) (string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok || !apiResponseIdents[ident.Name] {
		return "", false
	}
	return ident.Name + "." + sel.Sel.Name, true
}
