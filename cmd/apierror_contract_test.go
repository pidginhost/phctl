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

	// A non-SDK Execute method is not an API error source.
	commandErr := rootCmd.Execute()
	if commandErr != nil {
		return fmt.Errorf("executing command: %w", commandErr)
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

func TestAPIErrorContractTracksLexicalBindings(t *testing.T) {
	const src = `package fixture

func nestedBlock() error {
	_, _, err := api.Get().Execute()
	{
		f, err := os.Open(name)
		if err != nil {
			return fmt.Errorf("opening: %w", err)
		}
		_ = f
	}
	return fmt.Errorf("getting after block: %w", err)
}

func ifInit() error {
	_, _, err := api.Get().Execute()
	if f, err := os.Open(name); err != nil {
		return fmt.Errorf("opening: %w", err)
	} else {
		_ = f
	}
	return fmt.Errorf("getting after if: %w", err)
}

func switchInit() error {
	_, _, err := api.Get().Execute()
	switch f, err := os.Open(name); {
	case err != nil:
		return fmt.Errorf("opening: %w", err)
	default:
		_ = f
	}
	return fmt.Errorf("getting after switch: %w", err)
}

func apiIfInit() error {
	if _, _, err := api.Get().Execute(); err != nil {
		return fmt.Errorf("getting in if: %w", err)
	}
	return nil
}

func apiSwitchInit() error {
	switch _, _, err := api.Get().Execute(); {
	case err != nil:
		return fmt.Errorf("getting in switch: %w", err)
	}
	return nil
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	bindings, violations := analyzeAPIErrors(file, fset, "fixture.go")
	if bindings != 5 {
		t.Fatalf("API error bindings = %d, want 5", bindings)
	}
	if len(violations) != 5 {
		t.Fatalf("violations = %#v, want one for each API error", violations)
	}
}

func TestAPIErrorContractTracksAssignmentsAndBranches(t *testing.T) {
	const src = `package fixture

func reassigned() error {
	_, _, err := api.Get().Execute()
	err = fmt.Errorf("getting after assignment: %w", err)
	return err
}

func aliased() error {
	_, _, err := api.Get().Execute()
	apiErr := err
	return fmt.Errorf("getting through alias: %w", apiErr)
}

func conditional(fromAPI bool) error {
	var err error
	if fromAPI {
		_, _, err = api.Get().Execute()
	} else {
		_, err = os.Open(name)
	}
	return fmt.Errorf("conditional failure: %w", err)
}

func multipleRHS() error {
	value, err := getValue(), client.RawDelete(ctx, path)
	_ = value
	return fmt.Errorf("deleting with another result: %w", err)
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
	if len(violations) != 4 {
		t.Fatalf("violations = %#v, want assignment, alias, API branch, and multiple-RHS violations", violations)
	}
}

func TestAPIErrorContractRecognizesEveryRawHelper(t *testing.T) {
	const src = `package fixture

func rawGet() error {
	err := client.RawGet(ctx, path, dst)
	if err != nil {
		return fmt.Errorf("getting: %w", err)
	}
	return nil
}

func rawPost() error {
	err := client.RawPost(ctx, path, body, dst)
	if err != nil {
		return fmt.Errorf("posting: %w", err)
	}
	return nil
}

func rawDelete() error {
	err := client.RawDelete(ctx, path)
	if err != nil {
		return fmt.Errorf("deleting: %w", err)
	}
	return nil
}

func rawFetchAll() error {
	_, err := client.RawFetchAll[Thing](ctx, path)
	if err != nil {
		return fmt.Errorf("fetching: %w", err)
	}
	return nil
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fixture.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	bindings, violations := analyzeAPIErrors(file, fset, "fixture.go")
	if bindings != 4 || len(violations) != 4 {
		t.Fatalf("bindings = %d, violations = %#v; want one violation per raw helper", bindings, violations)
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
	analyzer := apiErrorAnalyzer{fset: fset, path: path}
	analyzer.analyzeBlock(body, make(apiErrorState))
	return analyzer.sites, analyzer.violations
}

// apiErrorState is keyed by the declaration an identifier resolves to, not by
// its spelling. An `err` declared in an if-init, switch-init, or nested block
// must not overwrite the state of an outer `err` with the same name.
type apiErrorState map[any]bool

type apiErrorAnalyzer struct {
	fset       *token.FileSet
	path       string
	sites      int
	violations []string
}

func cloneAPIErrorState(state apiErrorState) apiErrorState {
	cloned := make(apiErrorState, len(state))
	for binding := range state {
		cloned[binding] = true
	}
	return cloned
}

// mergeAPIErrorStates keeps a binding tainted if an API error can reach the
// join through any branch. This is deliberately a may-analysis: missing a
// genuinely lossy path is worse than asking a future author to make an
// ambiguous rebind explicit.
func mergeAPIErrorStates(states ...apiErrorState) apiErrorState {
	merged := make(apiErrorState)
	for _, state := range states {
		for binding := range state {
			merged[binding] = true
		}
	}
	return merged
}

func (a *apiErrorAnalyzer) analyzeBlock(body *ast.BlockStmt, state apiErrorState) (apiErrorState, bool) {
	if body == nil {
		return state, true
	}
	return a.analyzeStmtList(body.List, state)
}

func (a *apiErrorAnalyzer) analyzeStmtList(stmts []ast.Stmt, state apiErrorState) (apiErrorState, bool) {
	for _, stmt := range stmts {
		var fallsThrough bool
		state, fallsThrough = a.analyzeStmt(stmt, state)
		if !fallsThrough {
			return state, false
		}
	}
	return state, true
}

func (a *apiErrorAnalyzer) analyzeStmt(stmt ast.Stmt, state apiErrorState) (apiErrorState, bool) {
	switch node := stmt.(type) {
	case *ast.AssignStmt:
		a.assign(node.Lhs, node.Rhs, state)
	case *ast.DeclStmt:
		decl, ok := node.Decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.VAR {
			return state, true
		}
		for _, spec := range decl.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			lhs := make([]ast.Expr, len(value.Names))
			for i, name := range value.Names {
				lhs[i] = name
			}
			a.assign(lhs, value.Values, state)
		}
	case *ast.ExprStmt:
		a.inspectExpr(node.X, state)
	case *ast.ReturnStmt:
		a.inspectExprs(node.Results, state)
		return state, false
	case *ast.BlockStmt:
		return a.analyzeBlock(node, state)
	case *ast.IfStmt:
		if node.Init != nil {
			state, _ = a.analyzeStmt(node.Init, state)
		}
		a.inspectExpr(node.Cond, state)
		base := cloneAPIErrorState(state)
		thenState, thenFalls := a.analyzeBlock(node.Body, cloneAPIErrorState(base))
		elseState, elseFalls := base, true
		if node.Else != nil {
			elseState, elseFalls = a.analyzeStmt(node.Else, cloneAPIErrorState(base))
		}
		return mergeFallingAPIErrorStates(thenState, thenFalls, elseState, elseFalls)
	case *ast.SwitchStmt:
		if node.Init != nil {
			state, _ = a.analyzeStmt(node.Init, state)
		}
		a.inspectExpr(node.Tag, state)
		return a.analyzeCaseClauses(node.Body, state)
	case *ast.TypeSwitchStmt:
		if node.Init != nil {
			state, _ = a.analyzeStmt(node.Init, state)
		}
		if node.Assign != nil {
			state, _ = a.analyzeStmt(node.Assign, state)
		}
		return a.analyzeCaseClauses(node.Body, state)
	case *ast.ForStmt:
		if node.Init != nil {
			state, _ = a.analyzeStmt(node.Init, state)
		}
		a.inspectExpr(node.Cond, state)
		bodyState, bodyFalls := a.analyzeBlock(node.Body, cloneAPIErrorState(state))
		if bodyFalls && node.Post != nil {
			bodyState, _ = a.analyzeStmt(node.Post, bodyState)
		}
		// A loop may execute zero times, so both entry and body states reach
		// the following statement for this conservative analysis.
		return mergeAPIErrorStates(state, bodyState), true
	case *ast.RangeStmt:
		a.inspectExpr(node.X, state)
		bodyState := cloneAPIErrorState(state)
		for _, expr := range []ast.Expr{node.Key, node.Value} {
			if ident, ok := expr.(*ast.Ident); ok && ident.Obj != nil {
				delete(bodyState, ident.Obj)
			}
		}
		bodyState, _ = a.analyzeBlock(node.Body, bodyState)
		return mergeAPIErrorStates(state, bodyState), true
	case *ast.SelectStmt:
		return a.analyzeCommClauses(node.Body, state)
	case *ast.GoStmt:
		a.inspectExpr(node.Call, state)
	case *ast.DeferStmt:
		a.inspectExpr(node.Call, state)
	case *ast.SendStmt:
		a.inspectExpr(node.Chan, state)
		a.inspectExpr(node.Value, state)
	case *ast.IncDecStmt:
		a.inspectExpr(node.X, state)
	case *ast.LabeledStmt:
		return a.analyzeStmt(node.Stmt, state)
	}
	return state, true
}

func mergeFallingAPIErrorStates(first apiErrorState, firstFalls bool, second apiErrorState, secondFalls bool) (apiErrorState, bool) {
	switch {
	case firstFalls && secondFalls:
		return mergeAPIErrorStates(first, second), true
	case firstFalls:
		return first, true
	case secondFalls:
		return second, true
	default:
		return make(apiErrorState), false
	}
}

func (a *apiErrorAnalyzer) analyzeCaseClauses(body *ast.BlockStmt, state apiErrorState) (apiErrorState, bool) {
	var exits []apiErrorState
	hasDefault := false
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			continue
		}
		if len(clause.List) == 0 {
			hasDefault = true
		}
		a.inspectExprs(clause.List, state)
		caseState, falls := a.analyzeStmtList(clause.Body, cloneAPIErrorState(state))
		if falls {
			exits = append(exits, caseState)
		}
	}
	if !hasDefault {
		exits = append(exits, state)
	}
	if len(exits) == 0 {
		return make(apiErrorState), false
	}
	return mergeAPIErrorStates(exits...), true
}

func (a *apiErrorAnalyzer) analyzeCommClauses(body *ast.BlockStmt, state apiErrorState) (apiErrorState, bool) {
	var exits []apiErrorState
	hasDefault := false
	for _, stmt := range body.List {
		clause, ok := stmt.(*ast.CommClause)
		if !ok {
			continue
		}
		caseState := cloneAPIErrorState(state)
		if clause.Comm == nil {
			hasDefault = true
		} else {
			caseState, _ = a.analyzeStmt(clause.Comm, caseState)
		}
		caseState, falls := a.analyzeStmtList(clause.Body, caseState)
		if falls {
			exits = append(exits, caseState)
		}
	}
	if !hasDefault {
		exits = append(exits, state)
	}
	if len(exits) == 0 {
		return make(apiErrorState), false
	}
	return mergeAPIErrorStates(exits...), true
}

func (a *apiErrorAnalyzer) assign(lhs, rhs []ast.Expr, state apiErrorState) {
	a.inspectExprs(rhs, state)
	origins := make([]bool, len(lhs))
	if len(rhs) == 1 {
		if call, ok := rhs[0].(*ast.CallExpr); ok && returnsAPIError(call, len(lhs)) {
			if len(lhs) > 0 {
				origins[len(lhs)-1] = true
				a.countAPIBinding(lhs[len(lhs)-1])
			}
		} else if len(lhs) == 1 {
			origins[0] = exprIsAPIError(rhs[0], state)
		}
	} else if len(rhs) == len(lhs) {
		for i := range rhs {
			if call, ok := rhs[i].(*ast.CallExpr); ok && returnsAPIError(call, 1) {
				origins[i] = true
				a.countAPIBinding(lhs[i])
			} else {
				origins[i] = exprIsAPIError(rhs[i], state)
			}
		}
	}
	for i, expr := range lhs {
		ident, ok := expr.(*ast.Ident)
		if !ok || ident.Name == "_" || ident.Obj == nil {
			continue
		}
		if origins[i] {
			state[ident.Obj] = true
		} else {
			delete(state, ident.Obj)
		}
	}
}

func (a *apiErrorAnalyzer) countAPIBinding(expr ast.Expr) {
	if ident, ok := expr.(*ast.Ident); ok && ident.Name != "_" && ident.Obj != nil {
		a.sites++
	}
}

func exprIsAPIError(expr ast.Expr, state apiErrorState) bool {
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Obj != nil && state[node.Obj]
	case *ast.ParenExpr:
		return exprIsAPIError(node.X, state)
	default:
		return false
	}
}

func (a *apiErrorAnalyzer) inspectExprs(exprs []ast.Expr, state apiErrorState) {
	for _, expr := range exprs {
		a.inspectExpr(expr, state)
	}
}

func (a *apiErrorAnalyzer) inspectExpr(expr ast.Expr, state apiErrorState) {
	if expr == nil {
		return
	}
	ast.Inspect(expr, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		recv, name, ok := calledSelector(call.Fun)
		if !ok || recv == nil || recv.Name != "fmt" || (name != "Errorf" && name != "Error") {
			return true
		}
		seen := make(map[any]bool)
		for _, arg := range call.Args {
			ast.Inspect(arg, func(n ast.Node) bool {
				if _, ok := n.(*ast.FuncLit); ok {
					return false
				}
				ident, ok := n.(*ast.Ident)
				if !ok || ident.Obj == nil || !state[ident.Obj] || seen[ident.Obj] {
					return true
				}
				seen[ident.Obj] = true
				a.violations = append(a.violations, fmt.Sprintf(
					"%s:%d wraps API error %q with fmt.Errorf; use cmdutil.APIError so the response body survives",
					a.path, a.fset.Position(call.Pos()).Line, ident.Name))
				return true
			})
		}
		return true
	})
}

// returnsAPIError reports whether call produces an error that carries an API
// response body: a generated SDK request (.Execute()), one of the raw HTTP
// helpers, or cmdutil.FetchAll, which propagates the error from the page
// fetcher it drives.
func returnsAPIError(call *ast.CallExpr, results int) bool {
	recv, name, ok := calledSelector(call.Fun)
	if !ok {
		return false
	}
	if name == "Execute" {
		// Generated SDK Execute methods return an HTTP response plus an
		// error, and usually a decoded payload as well. Requiring that shape
		// avoids treating cobra.Command.Execute (error only) as an API call.
		return results >= 2
	}
	if recv == nil {
		return false
	}
	switch recv.Name {
	case "client":
		return name == "RawFetchAll" || name == "RawGet" || name == "RawPost" || name == "RawDelete"
	case "cmdutil":
		return name == "FetchAll"
	}
	return false
}
