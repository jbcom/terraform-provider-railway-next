// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateAlwaysPersistsStateAfterRemoteObjectExists guards the single most
// damaging class of bug a Terraform provider can have.
//
// A Create() that calls the API to make a remote object and then returns early
// — without resp.State.Set — makes Terraform discard the plan and record
// nothing. The object exists in Railway; Terraform does not know about it. Every
// subsequent apply then fails with "a service named X already exists", and there
// is no recovery through the provider at all: the operator has to find and
// delete the object by hand.
//
// That is worse than a failed create. A failed create leaves nothing behind.
//
// This was not hypothetical. Every resource in this package had it — service,
// bucket, postgres, volume and service_domain — and it made the provider
// unusable against a project that already contained anything, because the first
// partial failure poisoned the namespace permanently.
//
// WHY A STATIC CHECK RATHER THAN FIVE UNIT TESTS. Testing each Create() against
// a faked API would prove those five paths behave today and say nothing about
// the sixth resource somebody adds next month. The bug is structural — an early
// return in the wrong place — so the test reads the structure. Adding a resource
// with the same mistake fails this test on the first run.
func TestCreateAlwaysPersistsStateAfterRemoteObjectExists(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("globbing resources: %v", err)
	}

	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}

		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, source, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			fn, ok := node.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "Create" || fn.Body == nil {
				return true
			}

			// WHERE THE REMOTE OBJECT STARTS EXISTING.
			//
			// Approximated by the first assignment to a field named ID or
			// ServiceID on the plan — every resource here does that immediately
			// after the create call returns, because that is where the API hands
			// back the identifier. Before that point an early return is correct;
			// after it, the object is real.
			creation := creationPoint(fn.Body)
			if creation < 0 {
				return true
			}

			for _, offender := range bareReturnsWithoutPersist(fn.Body, creation) {
				t.Errorf(
					"%s:%d: Create returns after the remote object exists without "+
						"calling resp.State.Set.\n"+
						"\tThe object is created in Railway and absent from Terraform state, so "+
						"every later apply fails with \"already exists\" and the only fix is "+
						"deleting it by hand.\n"+
						"\tPersist the partial state before returning — the apply should still "+
						"fail and still report why, but recoverably.",
					path, fset.Position(offender).Line,
				)
			}

			return true
		})
	}
}

// creationPoint returns the statement index at which the remote object should be
// considered to exist, or -1 when the function never records an identifier.
func creationPoint(body *ast.BlockStmt) int {
	for i, stmt := range body.List {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok {
			continue
		}
		for _, lhs := range assign.Lhs {
			selector, ok := lhs.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			if selector.Sel.Name == "ID" || selector.Sel.Name == "ServiceID" {
				return i
			}
		}
	}
	return -1
}

// bareReturnsWithoutPersist finds `return` statements after the creation point
// that no statement on the path to them has persisted state.
//
// **EVERY RETURN IS CHECKED AGAINST ITS OWN PATH**, which took three attempts
// to get right and is worth recording:
//
//   - Requiring the write to IMMEDIATELY PRECEDE the return flagged
//     `postgres.go`, which correctly writes state and then returns behind a
//     guard. A test that demands worse code to stay green is worse than no test.
//   - Tracking "has anything persisted yet" in source order passed code with the
//     bug REINTRODUCED: the flag latched on the first `saveState()` and every
//     later unguarded return inherited it. Verified by putting the bug back —
//     the test stayed green, which is the failure mode that matters.
//
// So each return is judged by what precedes it in its OWN enclosing blocks: the
// statements before it in its block, plus those before its block in each parent.
// A `saveState()` helper counts, which is why the match is on a call whose name
// mentions state rather than one exact expression.
func bareReturnsWithoutPersist(body *ast.BlockStmt, creation int) []token.Pos {
	var offenders []token.Pos

	var walk func(stmts []ast.Stmt, persistedBefore bool)
	walk = func(stmts []ast.Stmt, persistedBefore bool) {
		persisted := persistedBefore

		for _, stmt := range stmts {
			switch node := stmt.(type) {
			case *ast.ReturnStmt:
				if len(node.Results) == 0 && !persisted {
					offenders = append(offenders, node.Return)
				}

			case *ast.IfStmt:
				// The branch inherits what ran before the `if`, and nothing a
				// branch does leaks back out to the statements after it.
				walk(node.Body.List, persisted)
				if node.Else != nil {
					if block, ok := node.Else.(*ast.BlockStmt); ok {
						walk(block.List, persisted)
					} else if elseIf, ok := node.Else.(*ast.IfStmt); ok {
						walk([]ast.Stmt{elseIf}, persisted)
					}
				}

			case *ast.BlockStmt:
				walk(node.List, persisted)

			case *ast.AssignStmt:
				// **DECLARING A HELPER IS NOT CALLING IT**, and conflating the
				// two is why the first three versions of this test passed on
				// code with the bug reintroduced.
				//
				// `saveState := func() { resp.Diagnostics.Append(resp.State.Set(...)) }`
				// contains a state write in its BODY. Treating that as "state
				// has been persisted" latched the flag before any return, so
				// every unguarded return after it inherited a write that had
				// not happened. Verified by putting the bug back: the test
				// stayed green, which is the only failure mode that matters in
				// a regression test.
				if !declaresFunctionLiteral(node) && persistsState(stmt) {
					persisted = true
				}

			default:
				if persistsState(stmt) {
					persisted = true
				}
			}
		}
	}

	walk(body.List[creation:], false)

	return offenders
}

// declaresFunctionLiteral reports whether an assignment merely BINDS a closure,
// as `saveState := func() { ... }` does. The body has not run yet, so anything
// inside it must not count as having happened.
func declaresFunctionLiteral(assign *ast.AssignStmt) bool {
	for _, rhs := range assign.Rhs {
		if _, ok := rhs.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

// persistsState reports whether a statement writes the resource's state.
//
// Matches both the direct form — `resp.Diagnostics.Append(resp.State.Set(...))`
// — and a local helper such as `saveState()`, since several resources use one to
// avoid repeating the call on every failure path.
func persistsState(stmt ast.Stmt) bool {
	found := false

	ast.Inspect(stmt, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			if fn.Sel.Name == "Set" {
				if inner, ok := fn.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "State" {
					found = true
				}
			}
		case *ast.Ident:
			if strings.Contains(strings.ToLower(fn.Name), "state") {
				found = true
			}
		}

		return !found
	})

	return found
}
