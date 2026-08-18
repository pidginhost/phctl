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

// The API error contract
//
// Rule: an error coming back from the API must be wrapped with
// cmdutil.APIError, or propagated unchanged so an outer call can wrap it.
// Wrapping it with fmt.Errorf instead looks like it adds context but throws the
// response body away, and the body is the only part that says what was wrong:
//
//	attaching IPv4: 400 Bad Request                      <- fmt.Errorf
//	attaching IPv4: 400 Bad Request: ipv4=This field is required.   <- APIError
//
// The first reads like the server refused for no reason. Users cannot act on
// it, and neither can we when it lands in a bug report.
func TestAPIErrorsKeepTheResponseBody(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve cmd dir: %v", err)
	}

	var violations []string
	errorBindings := 0
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
		bindings, found := analyzeAPIErrors(file, fset, filepath.ToSlash(rel))
		errorBindings += bindings
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatalf("walk cmd tree: %v", err)
	}

	// Keep the guard non-vacuous if SDK call shapes or the analyzer change.
	if errorBindings == 0 {
		t.Fatal("API error guard found no API error bindings")
	}
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Fatalf("API errors must be wrapped with cmdutil.APIError:\n  %s",
			strings.Join(violations, "\n  "))
	}
}

func TestAPIErrorContractTracksErrorsByOrigin(t *testing.T) {
	const src = `package fixture

func run(cmd *cobra.Command) error {
	_, _, err := api.List().Execute()
	if err != nil {
		return fmt.Errorf("listing: %w", err)
	}

	things, err := client.RawFetchAll[Thing](ctx, path)
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}

	// Wrapped correctly: not a violation.
	one, _, err := api.Get().Execute()
	if err != nil {
		return cmdutil.APIError("getting", err)
	}

	// Propagated unchanged so an outer call can wrap it: not a violation.
	other, _, err := api.Get().Execute()
	if err != nil {
		return err
	}

	// Not an API error at all: not a violation.
	f, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("opening: %w", err)
	}
	_, _, _, _ = things, one, other, f
	return nil
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	bindings, violations := analyzeAPIErrors(file, fset, "fixture.go")
	if bindings != 4 {
		t.Fatalf("API error bindings = %d, want 4", bindings)
	}
	if len(violations) != 2 {
		t.Fatalf("violations = %#v, want one each for the Execute and RawFetchAll errors", violations)
	}
}

// analyzeAPIErrors ties error variables to the API calls that produce them,
// then reports the ones re-wrapped with fmt.Errorf.
func analyzeAPIErrors(file *ast.File, fset *token.FileSet, path string) (int, []string) {
	bindings := 0
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
		found, bad := analyzeAPIErrorScope(body, fset, path)
		bindings += found
		violations = append(violations, bad...)
		return true
	})

	return bindings, violations
}

func analyzeAPIErrorScope(body *ast.BlockStmt, fset *token.FileSet, path string) (int, []string) {
	apiErrors := make(map[string]bool)
	// Count binding sites, not distinct names: every scope reuses `err`, so
	// distinct names would report 1 no matter how many calls a command makes.
	sites := 0
	var violations []string

	// One ordered pass, so that rebinding is honoured. Commands routinely reuse
	// `err` for a file open or a strconv after an API call, and those errors
	// carry no response body -- wrapping them with fmt.Errorf is correct. A
	// two-pass analyzer would flag them.
	ast.Inspect(body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		switch node := n.(type) {
		case *ast.AssignStmt:
			if len(node.Rhs) != 1 || len(node.Lhs) == 0 {
				return true
			}
			// The error is conventionally the last result.
			ident, ok := node.Lhs[len(node.Lhs)-1].(*ast.Ident)
			if !ok || ident.Name == "_" {
				return true
			}
			call, isCall := node.Rhs[0].(*ast.CallExpr)
			if isCall && returnsAPIError(call) {
				apiErrors[ident.Name] = true
				sites++
			} else {
				delete(apiErrors, ident.Name)
			}
		case *ast.CallExpr:
			recv, name, ok := calledSelector(node.Fun)
			if !ok || recv == nil || recv.Name != "fmt" || (name != "Errorf" && name != "Error") {
				return true
			}
			for _, arg := range node.Args {
				ident, ok := arg.(*ast.Ident)
				if !ok || !apiErrors[ident.Name] {
					continue
				}
				violations = append(violations, fmt.Sprintf(
					"%s:%d wraps API error %q with fmt.Errorf; use cmdutil.APIError so the response body survives",
					path, fset.Position(node.Pos()).Line, ident.Name))
			}
		}
		return true
	})

	return sites, violations
}

// returnsAPIError reports whether call produces an error that carries an API
// response body: a generated SDK request (.Execute()), one of the raw HTTP
// helpers, or cmdutil.FetchAll, which propagates the error from the page
// fetcher it drives.
func returnsAPIError(call *ast.CallExpr) bool {
	recv, name, ok := calledSelector(call.Fun)
	if !ok {
		return false
	}
	if name == "Execute" {
		return true
	}
	if recv == nil {
		return false
	}
	switch recv.Name {
	case "client":
		return name == "RawFetchAll" || name == "RawGet" || name == "RawPost"
	case "cmdutil":
		return name == "FetchAll"
	}
	return false
}
