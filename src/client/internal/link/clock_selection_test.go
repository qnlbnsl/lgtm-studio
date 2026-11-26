package link

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

func TestOpenElapsedClockIncludesSuspend(t *testing.T) {
	// Comparing elapsed values cannot distinguish the two clocks on a machine
	// that has never suspended. Read the argument at the production call site.
	for _, tc := range []struct{ platform, clock string }{
		{"linux", "CLOCK_BOOTTIME"},
		{"darwin", "CLOCK_MONOTONIC_RAW"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			file, err := parser.ParseFile(token.NewFileSet(), "clock_"+tc.platform+".go", nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatalf("could not read the platform clock helper: %v", err)
			}
			var clocks []string
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Name.Name != "elapsedClock" {
					continue
				}
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok || !unixSelector(call.Fun, "ClockGettime") {
						return true
					}
					clock := "<unrecognized>"
					if len(call.Args) == 2 {
						if arg, ok := call.Args[0].(*ast.SelectorExpr); ok && unixSelector(arg, arg.Sel.Name) {
							clock = arg.Sel.Name
						}
					}
					clocks = append(clocks, clock)
					return true
				})
			}
			if !slices.Equal(clocks, []string{tc.clock}) {
				t.Fatalf("elapsedClock requests %v, want only unix.%s", clocks, tc.clock)
			}
		})
	}
}

func unixSelector(expr ast.Expr, member string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != member {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "unix"
}
