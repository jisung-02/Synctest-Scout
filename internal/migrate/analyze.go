// Package migrate implements a deliberately syntax-only migration assistant.
// A candidate is an invitation to review, never a claim of semantic safety.
package migrate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/version"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

type Candidate struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Test    string   `json:"test"`
	Status  string   `json:"status"`
	Signals []string `json:"signals"`
	Reasons []string `json:"reasons,omitempty"`
	Notes   []string `json:"notes"`
}

type Report struct {
	Root       string      `json:"root"`
	Files      int         `json:"test_files"`
	Candidates []Candidate `json:"candidates"`
}

type source struct {
	name    string
	data    []byte
	fset    *token.FileSet
	file    *ast.File
	imports map[string]string
}

type testBody struct {
	name     string
	param    *ast.Ident
	typeExpr ast.Expr
	body     *ast.BlockStmt
}

func parse(name string) (*source, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	s := &source{name: name, data: data, fset: token.NewFileSet(), imports: map[string]string{}}
	s.file, err = parser.ParseFile(s.fset, name, data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	for _, imp := range s.file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		alias := filepath.Base(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		s.imports[alias] = path
	}
	return s, nil
}

// parser's object resolution distinguishes local shadowing from package names.
func (s *source) imported(expr ast.Expr) (string, string) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Obj != nil {
		return "", ""
	}
	return s.imports[x.Name], sel.Sel.Name
}

func (s *source) parameter(ft *ast.FuncType) (*ast.Ident, ast.Expr) {
	if ft.Params == nil || len(ft.Params.List) != 1 || (ft.Results != nil && len(ft.Results.List) != 0) {
		return nil, nil
	}
	f := ft.Params.List[0]
	if len(f.Names) != 1 || f.Names[0].Name == "_" {
		return nil, nil
	}
	ptr, ok := f.Type.(*ast.StarExpr)
	if !ok {
		return nil, nil
	}
	pkg, name := s.imported(ptr.X)
	if pkg != "testing" || name != "T" {
		return nil, nil
	}
	return f.Names[0], f.Type
}

func method(call *ast.CallExpr, param *ast.Ident) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok || x.Obj != param.Obj || x.Name != param.Name {
		return ""
	}
	return sel.Sel.Name
}

func (s *source) tests() []testBody {
	var out []testBody
	var collect func(testBody)
	collect = func(t testBody) {
		out = append(out, t)
		ast.Inspect(t.body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || method(call, t.param) != "Run" {
				return true
			}
			if len(call.Args) != 2 {
				return false
			}
			label, ok := call.Args[0].(*ast.BasicLit)
			if !ok || label.Kind != token.STRING {
				return false
			}
			lit, ok := call.Args[1].(*ast.FuncLit)
			if !ok {
				return false
			}
			param, typ := s.parameter(lit.Type)
			if param == nil {
				return false
			}
			name, _ := strconv.Unquote(label.Value)
			collect(testBody{name: t.name + "/" + name, param: param, typeExpr: typ, body: lit.Body})
			return false
		})
	}
	for _, decl := range s.file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		suffix := []rune(strings.TrimPrefix(fn.Name.Name, "Test"))
		if len(suffix) > 0 && unicode.IsLower(suffix[0]) {
			continue
		}
		param, typ := s.parameter(fn.Type)
		if param != nil {
			collect(testBody{name: fn.Name.Name, param: param, typeExpr: typ, body: fn.Body})
		}
	}
	return out
}

func leadingParallel(t testBody) *ast.ExprStmt {
	if len(t.body.List) == 0 {
		return nil
	}
	expr, ok := t.body.List[0].(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := expr.X.(*ast.CallExpr)
	if ok && method(call, t.param) == "Parallel" {
		return expr
	}
	return nil
}

func (s *source) analyze(t testBody, version string) Candidate {
	c := Candidate{File: s.name, Line: s.fset.Position(t.body.Pos()).Line, Test: t.name, Status: "review", Notes: []string{"Review transitive calls, package/global state, external goroutines, and wall-clock assumptions."}}
	signals, blockers := map[string]bool{}, map[string]bool{}
	parallel := leadingParallel(t)
	if parallel != nil {
		c.Notes = append(c.Notes, "Leading t.Parallel() stays outside the bubble.")
	}
	if !atLeast125(version) {
		blockers["Module Go version is "+version+"; patch requires an explicit Go >= 1.25 support decision."] = true
	}
	if strings.Contains(string(s.data), "Code generated") {
		blockers["Generated file; edit the generator instead."] = true
	}
	if _, ok := s.imports["."]; ok {
		blockers["Dot imports are not supported by this syntax-only analyzer."] = true
	}
	ast.Inspect(t.body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		m := method(call, t.param)
		if m == "Run" {
			blockers["Contains subtests; select a literal leaf subtest instead."] = true
			return false
		}
		if m == "Parallel" && (parallel == nil || parallel.X != call) {
			blockers["Non-leading t.Parallel() needs manual restructuring."] = true
		}
		if m == "Deadline" {
			blockers["t.Deadline() is a wall-clock boundary; review manually."] = true
		}
		pkg, name := s.imported(call.Fun)
		if pkg == "testing/synctest" {
			blockers["Already uses synctest; nesting bubbles is not supported."] = true
		}
		if pkg == "time" {
			switch name {
			case "Sleep", "After", "AfterFunc", "NewTimer", "NewTicker", "Tick":
				signals["time."+name] = true
			case "Now", "Since", "Until":
				c.Notes = append(c.Notes, "Clock measurements become virtual; confirm this matches the assertion's purpose.")
			}
		}
		if pkg == "context" && (name == "WithTimeout" || name == "WithDeadline" || name == "WithTimeoutCause" || name == "WithDeadlineCause") {
			signals["context."+name] = true
		}
		if strings.HasPrefix(pkg, "github.com/stretchr/testify/") && strings.HasPrefix(name, "Eventually") {
			signals["testify."+name] = true
			blockers["Eventually polling needs manual conversion and synchronization review."] = true
		}
		if pkg == "net" || strings.HasPrefix(pkg, "net/") || pkg == "os" || strings.HasPrefix(pkg, "os/") || pkg == "syscall" || pkg == "database/sql" || pkg == "runtime" {
			blockers["Possible external boundary: "+pkg+"."+name+"; review manually."] = true
		}
		return true
	})
	for signal := range signals {
		c.Signals = append(c.Signals, signal)
	}
	for reason := range blockers {
		c.Reasons = append(c.Reasons, reason)
	}
	sort.Strings(c.Signals)
	sort.Strings(c.Reasons)
	if len(c.Reasons) != 0 {
		c.Status = "manual"
	}
	c.Notes = unique(c.Notes)
	return c
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

func moduleVersion(dir string) string {
	for {
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[0] == "go" {
					return fields[1]
				}
			}
			return "unknown"
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "unknown"
		}
		dir = parent
	}
}

func atLeast125(v string) bool {
	return version.IsValid("go"+v) && version.Compare("go"+v, "go1.25") >= 0
}

func Scan(root string) (Report, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	report := Report{Root: abs, Candidates: []Candidate{}}
	err = filepath.WalkDir(abs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path != abs && (strings.HasPrefix(d.Name(), ".") || d.Name() == "vendor" || d.Name() == "testdata" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") || !d.Type().IsRegular() {
			return nil
		}
		s, err := parse(path)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		report.Files++
		version := moduleVersion(filepath.Dir(path))
		for _, test := range s.tests() {
			c := s.analyze(test, version)
			if len(c.Signals) > 0 {
				report.Candidates = append(report.Candidates, c)
			}
		}
		return nil
	})
	return report, err
}
