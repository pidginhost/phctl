package cmd

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
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
func TestCommandsReturningDataDoNotBypassOutputFlag(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve cmd dir: %v", err)
	}

	var violations []string
	responseBindings := 0
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

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		bindings, found := analyzeCommandOutput(file, fset, filepath.ToSlash(rel))
		responseBindings += bindings
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd tree: %v", err)
	}

	// Keep the guard non-vacuous if SDK call shapes or the analyzer change.
	if responseBindings == 0 {
		t.Fatal("output contract guard found no API response bindings")
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("commands returning API data must not print via cmd.Print*:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func TestOutputContractTracksResponseVariablesByOrigin(t *testing.T) {
	const src = `package fixture

func run(cmd *cobra.Command) {
	r, _, err := api.List().Execute()
	cmd.Printf("ID: %d", r.ID)

	out, err := client.RawFetchAll[Thing](ctx, path)
	cmd.Println(out[0].Name)

	var created Thing
	if err := client.RawPost(ctx, path, body, &created, http.StatusCreated); err != nil {}
	cmd.Print(created.ID)
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	bindings, violations := analyzeCommandOutput(file, fset, "fixture.go")
	if bindings != 3 {
		t.Fatalf("response bindings = %d, want 3", bindings)
	}
	if len(violations) != 3 {
		t.Fatalf("violations = %#v, want one each for r, out, and created", violations)
	}
}

// analyzeCommandOutput ties response variables to the API calls that produce
// them instead of relying on a fixed list of names such as resp or result.
func analyzeCommandOutput(file *ast.File, fset *token.FileSet, path string) (int, []string) {
	responseBindings := 0
	var violations []string

	ast.Inspect(file, func(n ast.Node) bool {
		var body *ast.BlockStmt
		switch node := n.(type) {
		case *ast.FuncDecl:
			body = node.Body
		case *ast.FuncLit:
			body = node.Body
		default:
			return true
		}
		bindings, found := analyzeCommandOutputScope(body, fset, path)
		responseBindings += bindings
		violations = append(violations, found...)
		return true
	})

	return responseBindings, violations
}

func analyzeCommandOutputScope(body *ast.BlockStmt, fset *token.FileSet, path string) (int, []string) {
	responses := make(map[string]bool)
	addResponse := func(expr ast.Expr) {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name == "_" {
			return
		}
		responses[ident.Name] = true
	}

	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		switch node := n.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) >= 2 && len(node.Rhs) == 1 {
				if call, ok := node.Rhs[0].(*ast.CallExpr); ok && returnsAPIResponse(call) {
					addResponse(node.Lhs[0])
				}
			}
		case *ast.CallExpr:
			recv, name, ok := calledSelector(node.Fun)
			if !ok || recv == nil || recv.Name != "client" ||
				(name != "RawGet" && name != "RawPost") || len(node.Args) == 0 {
				return true
			}
			// RawPost accepts optional expected status codes after dst, so dst is
			// not necessarily the final argument.
			dstIndex := 2
			if name == "RawPost" {
				dstIndex = 3
			}
			if len(node.Args) <= dstIndex {
				return true
			}
			if ptr, ok := node.Args[dstIndex].(*ast.UnaryExpr); ok && ptr.Op == token.AND {
				addResponse(ptr.X)
			}
		}
		return true
	})

	var violations []string
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isCmdPrintCall(call.Fun) {
			return true
		}
		for _, arg := range call.Args {
			ast.Inspect(arg, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				field, ok := apiResponseField(sel, responses)
				if !ok {
					return true
				}
				violations = append(violations, fmt.Sprintf(
					"%s:%d prints %s via cmd.Print*; route it through output.Result so -o json works",
					path, fset.Position(call.Pos()).Line, field))
				return false
			})
		}
		return true
	})

	return len(responses), violations
}

func returnsAPIResponse(call *ast.CallExpr) bool {
	recv, name, ok := calledSelector(call.Fun)
	if !ok {
		return false
	}
	if name == "Execute" {
		return true
	}
	return recv != nil && recv.Name == "client" && name == "RawFetchAll"
}

func calledSelector(expr ast.Expr) (*ast.Ident, string, bool) {
	switch e := expr.(type) {
	case *ast.IndexExpr:
		return calledSelector(e.X)
	case *ast.IndexListExpr:
		return calledSelector(e.X)
	case *ast.SelectorExpr:
		recv, _ := e.X.(*ast.Ident)
		return recv, e.Sel.Name, true
	default:
		return nil, "", false
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

func apiResponseField(sel *ast.SelectorExpr, responses map[string]bool) (string, bool) {
	parts := []string{sel.Sel.Name}
	expr := sel.X
	for {
		switch e := expr.(type) {
		case *ast.SelectorExpr:
			parts = append(parts, e.Sel.Name)
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.ParenExpr:
			expr = e.X
		case *ast.StarExpr:
			expr = e.X
		case *ast.Ident:
			if !responses[e.Name] {
				return "", false
			}
			for left, right := 0, len(parts)-1; left < right; left, right = left+1, right-1 {
				parts[left], parts[right] = parts[right], parts[left]
			}
			return e.Name + "." + strings.Join(parts, "."), true
		default:
			return "", false
		}
	}
}
