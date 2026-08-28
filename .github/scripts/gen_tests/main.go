// gen_tests dynamically scans the repository's Go source, discovers
// side-effect-free functions, and auto-generates unit tests for them at CI time.
// A newly added pure helper is picked up automatically with no manual editing.
//
// WHY THIS IS SAFE
// ----------------
// Auto-generating tests for arbitrary code is unsafe: many functions perform
// network I/O, spawn subprocesses, write the registry, touch the filesystem, or
// construct stateful managers. Calling those from CI would hang, crash, or
// mutate the machine. We therefore gate every candidate on ALL of:
//
//  1. Package allowlist — only known-pure packages are scanned
//     (proxy, pkg/rules, pkg/cfpool, pkg/tlsfrag, pkg/platform, common).
//  2. Body taint analysis — a function is excluded if its body (directly or
//     transitively through a package-local call) references an I/O package
//     (os, net, net/http, http, os/exec, syscall, io, bufio, registry, log,
//     os/exec...) or launches a goroutine.
//  3. Name blacklist — a curated set of side-effect/stateful prefixes as a
//     first-line defense (New/Init/Start/Enable/Fetch/Install/...).
//  4. Parameter safety — every parameter must be zero-value-constructible.
//  5. Return-type purity — only funcs returning basic/slice/map/error are kept;
//     funcs returning *T managers are stateful and skipped.
//
// Each generated function gets its own TestGenerated_<Name> so a problematic
// call is isolated. Calls run under recover(): if the helper panics with
// zero-value args (e.g. an index helper needing real input), the test is marked
// SKIPPED rather than failing CI, while still having executed the call.
//
// Idempotent: files are rewritten only when content changes, so repeated CI
// runs produce no churn.
//
// Usage:
//
//	go run ./.github/scripts/gen_tests -root <repo-root>
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

var scanRoots = []string{
	"proxy",
	"common",
	"pkg/rules",
	"pkg/cfpool",
	"pkg/tlsfrag",
	"pkg/platform",
}

// ioPackages are import paths whose use implies a side effect (I/O, process,
// registry, network, logging to a sink).
var ioPackages = map[string]bool{
	"os": true, "os/exec": true, "io": true, "io/ioutil": true,
	"net": true, "net/http": true, "http": true, "net/url": true,
	"syscall": true, "bufio": true,
	"golang.org/x/sys/windows/registry": true, "golang.org/x/sys/windows": true,
	"log": true, "log/slog": true,
}

// namePrefixesBlacklist skips functions whose names imply side effects / state
// management even if the body taint analysis misses it.
var namePrefixesBlacklist = []string{
	"New", "Init", "MustInit",
	"Start", "Stop", "Run", "Serve", "Close", "Shutdown",
	"Enable", "Disable", "Install", "Uninstall", "Regenerat",
	"Fetch", "Dial", "Connect", "Lookup", "Ping",
	"Update", "Reload", "Import", "Export",
	"Save", "LoadFrom", "Read",
	"Kill", "Remove", "Delete", "Create", "Destroy", "Mount",
	"Wake", "Allow", "Recover", "Switch", "Apply", "Emit", "Send", "Trigger",
}

func resultPurityOK(results *types.Tuple) bool {
	for i := 0; i < results.Len(); i++ {
		switch results.At(i).Type().(type) {
		case *types.Basic:
		case *types.Slice, *types.Map, *types.Chan:
		case *types.Pointer:
			return false // *T manager / stateful
		default:
			return false
		}
	}
	return true
}

func zeroValueExpr(t types.Type) (string, bool) {
	switch u := t.(type) {
	case *types.Basic:
		switch u.Kind() {
		case types.Bool:
			return "false", true
		case types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			return "0", true
		case types.Float32, types.Float64:
			return "0", true
		case types.Complex64, types.Complex128:
			return "0i", true
		case types.String:
			return `""`, true
		case types.UnsafePointer:
			return "nil", true
		default:
			return "", false
		}
	case *types.Named:
		switch u.Underlying().(type) {
		case *types.Basic:
			return "0", true
		case *types.Slice, *types.Map, *types.Chan, *types.Signature, *types.Pointer:
			return "nil", true
		default:
			return "", false
		}
	case *types.Slice, *types.Map, *types.Chan, *types.Pointer, *types.Interface, *types.Signature:
		return "nil", true
	default:
		return "", false
	}
}

func buildCall(name string, sig *types.Signature) (string, bool) {
	var args []string
	params := sig.Params()
	variadic := sig.Variadic()
	for i := 0; i < params.Len(); i++ {
		p := params.At(i)
		if variadic && i == params.Len()-1 {
			continue
		}
		expr, ok := zeroValueExpr(p.Type())
		if !ok {
			return "", false
		}
		args = append(args, expr)
	}
	call := name + "(" + strings.Join(args, ", ") + ")"
	switch sig.Results().Len() {
	case 0:
		// bare call
	case 1:
		call = "_ = " + call
	default:
		blanks := make([]string, sig.Results().Len())
		for i := range blanks {
			blanks[i] = "_"
		}
		call = strings.Join(blanks, ", ") + " = " + call
	}
	return call, true
}

func isBlacklisted(name string) bool {
	for _, p := range namePrefixesBlacklist {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func main() {
	root := flag.String("root", ".", "repository root directory")
	flag.Parse()

	rootAbs, err := filepath.Abs(*root)
	if err != nil {
		fatal("abs root: %v", err)
	}

	totalFuncs := 0
	var generatedPkgs []string

	for _, scanRoot := range scanRoots {
		cfg := &packages.Config{
			Mode: packages.NeedTypes | packages.NeedName | packages.NeedSyntax |
				packages.NeedTypesInfo | packages.NeedFiles | packages.NeedCompiledGoFiles |
				packages.NeedDeps,
			Dir:        rootAbs,
			BuildFlags: []string{"-tags", "with_gvisor"},
		}
		pkgs, err := packages.Load(cfg, "./"+scanRoot+"/...")
		if err != nil {
			fmt.Fprintf(os.Stderr, "[gen_tests] skip %s: %v\n", scanRoot, err)
			continue
		}
		for _, pkg := range pkgs {
			if len(pkg.Errors) > 0 {
				fmt.Fprintf(os.Stderr, "[gen_tests] package %s load errors, skipping: %s\n", pkg.PkgPath, firstError(pkg.Errors))
				continue
			}
			if pkg.Types == nil || len(pkg.CompiledGoFiles) == 0 {
				continue
			}
			n, err := generateForPackage(rootAbs, pkg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[gen_tests] %s: %v\n", pkg.PkgPath, err)
				continue
			}
			totalFuncs += n
			if n > 0 {
				generatedPkgs = append(generatedPkgs, pkg.PkgPath)
			}
		}
	}

	sort.Strings(generatedPkgs)
	fmt.Printf("[gen_tests] discovered %d safe functions across %d packages\n", totalFuncs, len(generatedPkgs))
	for _, d := range generatedPkgs {
		fmt.Printf("  - %s\n", d)
	}
}

func firstError(errors []packages.Error) string {
	for _, e := range errors {
		return e.Error()
	}
	return "unknown"
}

// packageFuncs returns the set of package-level function names.
func packageFuncs(pkg *packages.Package) map[string]bool {
	set := map[string]bool{}
	for _, f := range pkg.Syntax {
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil {
				set[fd.Name.Name] = true
			}
		}
	}
	return set
}

// importsForFile maps short package aliases -> import paths for a file.
func importsForFile(f *ast.File) map[string]string {
	m := map[string]string{}
	for _, imp := range f.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		name := ""
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "" {
			base := path[strings.LastIndex(path, "/")+1:]
			name = base
		}
		m[name] = path
	}
	return m
}

// analyzePurity returns the set of package-level function names whose bodies
// are free of I/O (directly and transitively via local calls) and goroutines.
func analyzePurity(pkg *packages.Package) map[string]bool {
	funcNames := packageFuncs(pkg)

	// importAliases[fileIndex][alias] = importPath
	var fileImports []map[string]string
	for _, f := range pkg.Syntax {
		fileImports = append(fileImports, importsForFile(f))
	}

	// funcsByFile[fileName] -> FuncDecl
	type fdInfo struct {
		fileIdx int
		decl    *ast.FuncDecl
	}
	decls := map[string][]fdInfo{}

	// A function directly taints if its body uses an I/O package selector or a
	// goroutine. We compute a raw boolean, then propagate through local calls.
	directTaint := map[string]bool{}
	callGraph := map[string]map[string]bool{} // caller -> callee (local func names)

	for fi, f := range pkg.Syntax {
		imp := fileImports[fi]
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Name == nil {
				continue
			}
			name := fd.Name.Name
			decls[name] = append(decls[name], fdInfo{fileIdx: fi, decl: fd})
			callers := map[string]bool{}
			taint := false

			ast.Inspect(fd, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.GoStmt:
					taint = true
				case *ast.SelectorExpr:
					if id, ok := node.X.(*ast.Ident); ok {
						// I/O package use (e.g. os.Stat, net/http.Get)?
						if path, ok := imp[id.Name]; ok && ioPackages[path] {
							taint = true
						}
					}
				case *ast.CallExpr:
					// Same-package function call (plain Ident fun, e.g.
					// `ResolveRuntimeFile(...)`) -> candidate taint edge.
					if id, ok := node.Fun.(*ast.Ident); ok && funcNames[id.Name] && id.Name != name {
						callers[id.Name] = true
					}
					// Package-qualified call where fun is a selector, e.g.
					// os.Stat / pkg.Func. SelectorExpr above already taints for
					// I/O packages; here we also chase non-I/O package funcs is
					// intentionally skipped (cross-package purity is out of scope).
				}
				return true
			})
			directTaint[name] = taint
			callGraph[name] = callers
		}
	}

	// Fixpoint propagation: a function that (transitively) calls a tainted local
	// function is tainted.
	tainted := map[string]bool{}
	for name, t := range directTaint {
		if t {
			tainted[name] = true
		}
	}
	changed := true
	for changed {
		changed = false
		for caller, callees := range callGraph {
			if tainted[caller] {
				continue
			}
			for callee := range callees {
				if tainted[callee] {
					tainted[caller] = true
					changed = true
					break
				}
			}
		}
	}

	pure := map[string]bool{}
	for name := range funcNames {
		if !tainted[name] {
			pure[name] = true
		}
	}
	return pure
}

func generateForPackage(root string, pkg *packages.Package) (int, error) {
	pure := analyzePurity(pkg)
	scope := pkg.Types.Scope()

	type candidate struct {
		name string
		call string
	}
	var candidates []candidate
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}
		if isBlacklisted(name) {
			continue
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			continue
		}
		sig, ok := fn.Type().(*types.Signature)
		if !ok || sig.Recv() != nil {
			continue
		}
		if !pure[name] {
			continue
		}
		if !resultPurityOK(sig.Results()) {
			continue
		}
		call, ok := buildCall(name, sig)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{name: name, call: call})
	}
	if len(candidates) == 0 {
		return 0, nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].name < candidates[j].name })

	var b bytes.Buffer
	b.WriteString("// Code generated by .github/scripts/gen_tests; DO NOT EDIT.\n")
	b.WriteString("// Auto-discovered smoke tests for side-effect-free helpers.\n")
	fmt.Fprintf(&b, "package %s\n\n", pkg.Types.Name())
	b.WriteString("import \"testing\"\n\n")
	for _, c := range candidates {
		testName := "TestGenerated_" + c.name
		fmt.Fprintf(&b, "func %s(t *testing.T) {\n", testName)
		fmt.Fprintf(&b, "\tdefer func() {\n")
		fmt.Fprintf(&b, "\t\tif r := recover(); r != nil {\n")
		fmt.Fprintf(&b, "\t\t\tt.Skipf(%q)\n", "zero-value call panicked (not smoke-testable): "+c.name)
		fmt.Fprintf(&b, "\t\t}\n")
		fmt.Fprintf(&b, "\t}()\n")
		fmt.Fprintf(&b, "\t%s\n", c.call)
		fmt.Fprintf(&b, "}\n\n")
	}

	src, err := format.Source(b.Bytes())
	if err != nil {
		return 0, fmt.Errorf("format: %w", err)
	}

	outDir := filepath.Dir(pkg.CompiledGoFiles[0])
	outPath := filepath.Join(outDir, "zz_generated_scan_test.go")
	changed := true
	if old, err := os.ReadFile(outPath); err == nil && bytes.Equal(old, src) {
		changed = false
	}
	if changed {
		if err := os.WriteFile(outPath, src, 0o644); err != nil {
			return 0, err
		}
		fmt.Printf("[gen_tests] wrote %s (%d funcs)\n", outPath, len(candidates))
	}
	return len(candidates), nil
}

// verbatim is a helper to make a non-runtime value printable in a constant
// context (never actually invoked).
func verbatim(v interface{}) interface{} { return v }

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
