package migrate

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type insertion struct {
	offset int
	text   string
}

// Patch emits a reviewable diff without modifying the input file or go.mod.
// The caller must review transitive code before using this syntactic rewrite.
func Patch(file string, names []string) (string, error) {
	change, err := rewrite(file, names)
	if err != nil {
		return "", err
	}
	return unified(filepath.Base(change.File), string(change.Before), string(change.After))
}

func rewrite(file string, names []string) (*Change, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("select at least one test")
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(abs, "_test.go") {
		return nil, fmt.Errorf("expected a _test.go file")
	}
	s, err := parse(abs)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, name := range names {
		if name == "" || wanted[name] {
			return nil, fmt.Errorf("empty or duplicate test selection %q", name)
		}
		wanted[name] = true
	}
	var selected []testBody
	for _, t := range s.tests() {
		if !wanted[t.name] {
			continue
		}
		for _, prev := range selected {
			if prev.name == t.name {
				return nil, fmt.Errorf("ambiguous test path %q", t.name)
			}
		}
		c := s.analyze(t, moduleVersion(filepath.Dir(abs)))
		if len(c.Signals) == 0 {
			return nil, fmt.Errorf("%s: no directly detected timing calls", t.name)
		}
		if len(c.Reasons) != 0 {
			return nil, fmt.Errorf("%s: %s", t.name, strings.Join(c.Reasons, "; "))
		}
		// A leaf subtest might already be inside a bubble defined by its parent.
		insideBubble := false
		ast.Inspect(s.file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				pkg, _ := s.imported(call.Fun)
				if pkg == "testing/synctest" && call.Pos() < t.body.Pos() && call.End() > t.body.End() {
					insideBubble = true
				}
			}
			return true
		})
		if insideBubble {
			return nil, fmt.Errorf("%s is already inside a synctest bubble", t.name)
		}
		selected = append(selected, t)
	}
	if len(selected) != len(wanted) {
		return nil, fmt.Errorf("not all test paths found; use exact paths from scan")
	}

	// Choose a fresh import name, including against identifiers in local scopes.
	used := map[string]bool{}
	ast.Inspect(s.file, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			used[id.Name] = true
		}
		return true
	})
	alias := "synctest"
	for i := 1; used[alias]; i++ {
		alias = "synctestMigrate" + strconv.Itoa(i)
	}
	for _, imp := range s.file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if path == "testing/synctest" {
			return nil, fmt.Errorf("file already imports testing/synctest; integrate manually to avoid alias or scope conflicts")
		}
	}
	pos := func(p token.Pos) int { return s.fset.Position(p).Offset }
	// A separate import declaration preserves import comments and build tags.
	importText := "import \"testing/synctest\"\n\n"
	if alias != "synctest" {
		importText = "import " + alias + " \"testing/synctest\"\n\n"
	}
	importPos := s.file.Decls[0].Pos()
	if decl, ok := s.file.Decls[0].(*ast.GenDecl); ok && decl.Doc != nil {
		importPos = decl.Doc.Pos()
	}
	if decl, ok := s.file.Decls[0].(*ast.FuncDecl); ok && decl.Doc != nil {
		importPos = decl.Doc.Pos()
	}
	edits := []insertion{{pos(importPos), importText}}
	for _, t := range selected {
		start := pos(t.body.Lbrace) + 1
		if parallel := leadingParallel(t); parallel != nil {
			start = pos(parallel.End())
		}
		typ := string(s.data[pos(t.typeExpr.Pos()):pos(t.typeExpr.End())])
		edits = append(edits, insertion{start, "\n" + alias + ".Test(" + t.param.Name + ", func(" + t.param.Name + " " + typ + ") {"})
		edits = append(edits, insertion{pos(t.body.Rbrace), "\n})\n"})
		// Only standalone Sleep statements are followed by Wait. Never add Wait
		// to deferred/async calls, or to a for/if/switch control expression.
		addWait := func(stmts []ast.Stmt) {
			for _, stmt := range stmts {
				expr, ok := stmt.(*ast.ExprStmt)
				if !ok {
					continue
				}
				call, ok := expr.X.(*ast.CallExpr)
				if !ok {
					continue
				}
				pkg, name := s.imported(call.Fun)
				if pkg == "time" && name == "Sleep" {
					edits = append(edits, insertion{pos(stmt.End()), "\n" + alias + ".Wait()"})
				}
			}
		}
		ast.Inspect(t.body, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.BlockStmt:
				addWait(n.List)
			case *ast.CaseClause:
				addWait(n.Body)
			case *ast.CommClause:
				addWait(n.Body)
			}
			return true
		})
	}
	sort.SliceStable(edits, func(i, j int) bool { return edits[i].offset > edits[j].offset })
	modified := string(s.data)
	for _, e := range edits {
		modified = modified[:e.offset] + e.text + modified[e.offset:]
	}
	formatted, err := format.Source([]byte(modified))
	if err != nil {
		return nil, fmt.Errorf("cannot format proposed patch: %w", err)
	}
	// Patch paths are relative to the input file's directory, so users can
	// apply with git apply --directory=<directory> from the repository root.
	return &Change{File: abs, Before: s.data, After: formatted}, nil
}

// Use Git's mature diff implementation so distant edits remain separate hunks.
func unified(name, before, after string) (string, error) {
	dir, err := os.MkdirTemp("", "synctest-scout-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	for _, side := range []string{"a", "b"} {
		if err := os.Mkdir(filepath.Join(dir, side), 0700); err != nil {
			return "", err
		}
	}
	for side, content := range map[string]string{"a": before, "b": after} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, side, name)), 0700); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(dir, side, name), []byte(content), 0600); err != nil {
			return "", err
		}
	}
	cmd := exec.Command("git", "diff", "--no-index", "--no-ext-diff", "--no-textconv", "--no-color", "--src-prefix=", "--dst-prefix=", "--", "a/"+name, "b/"+name)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 1 {
			return "", fmt.Errorf("git diff: %w", err)
		}
	}
	return string(output), nil
}
