package migrate

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Arbitrary source must never panic or mutate the project, even when it parses
// but cannot type-check. Fuzzing does not execute the supplied Go source.
func FuzzScanSource(f *testing.F) {
	for _, seed := range []string{
		"", "package broken", "package x\nfunc TestA(",
		`package x;import("testing";"time");func TestA(t *testing.T){time.Sleep(0)}`,
		`package x;import("testing";"time");func TestA(t *testing.T){t.Run("child",func(t *testing.T){time.Sleep(0)})}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, code string) {
		if len(code) > 8192 {
			t.Skip()
		}
		file := fixture(t, "1.25", code)
		report, err := Scan(filepath.Dir(file))
		if err == nil {
			for _, c := range report.Candidates {
				// Errors are expected for unsupported or ambiguous structures.
				if _, err := Patch(file, []string{c.Test}); err == nil && c.Status != "review" {
					t.Fatal("manual candidate produced patch")
				}
			}
		}
		after, err := os.ReadFile(file)
		if err != nil || string(after) != code {
			t.Fatalf("scan/patch mutated input: %v", err)
		}
	})
}

// Exercise diff round trips, arbitrary string literals, import aliases,
// parallel tests, and Sleep placement. The resulting code must parse and a
// second migration must be rejected instead of nesting another bubble.
func FuzzPatchRoundTrip(f *testing.F) {
	f.Add("sample", false, false, uint8(0))
	f.Add("\uD55C\uAE00\n\"quoted\"", true, true, uint8(1))
	f.Add("", false, true, uint8(2))
	f.Fuzz(func(t *testing.T, value string, parallel, alias bool, shape uint8) {
		if len(value) > 1024 {
			t.Skip()
		}
		clock, imports := "time", `"testing";"time"`
		if alias {
			clock = "clock"
			imports = `"testing";clock "time"`
		}
		sleep := clock + ".Sleep(0)"
		bodies := []string{sleep, "if true {" + sleep + "}", "switch 1 {case 1:" + sleep + "}", "select {default:" + sleep + "}"}
		body := "_ = " + strconv.Quote(value) + ";" + bodies[int(shape)%len(bodies)]
		if parallel {
			body = "t.Parallel();" + body
		}
		code := "package example\nimport(" + imports + ")\nfunc TestA(t *testing.T){" + body + "}\n"
		file := fixture(t, "1.25", code)
		diff, err := Patch(file, []string{"TestA"})
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "git", "apply", "-")
		cmd.Dir = filepath.Dir(file)
		cmd.Stdin = strings.NewReader(diff)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("apply: %v %s", err, out)
		}
		after, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parser.ParseFile(token.NewFileSet(), file, after, parser.AllErrors); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(after), "synctest.Test(") {
			t.Fatal("patch not applied")
		}
		if _, err := Patch(file, []string{"TestA"}); err == nil {
			t.Fatal("nested migration accepted")
		}
	})
}
