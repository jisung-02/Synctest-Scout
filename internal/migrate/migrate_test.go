package migrate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T, version, code string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo "+version+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "sample_test.go")
	if err := os.WriteFile(file, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestScanClassifiesBoundariesAndLiteralSubtests(t *testing.T) {
	file := fixture(t, "1.25", `package fixture
import ("testing"; clock "time"; "net/http"; "testing/synctest")
func TestCases(t *testing.T) {
 t.Run("plain", func(t *testing.T) { t.Parallel(); clock.Sleep(clock.Second) })
 t.Run("network", func(t *testing.T) { clock.Sleep(clock.Second); http.Get("http://localhost") })
 t.Run("existing", func(t *testing.T) { synctest.Test(t, func(t *testing.T) { clock.Sleep(clock.Second) }) })
 t.Run("parallel_later", func(t *testing.T) { clock.Sleep(clock.Second); t.Parallel() })
}
func TestShadowed(t *testing.T) { clock := struct{Sleep func(int)}{}; clock.Sleep(1) }
`)
	r, err := Scan(filepath.Dir(file))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Candidates) != 4 {
		t.Fatalf("got %#v", r.Candidates)
	}
	for i, c := range r.Candidates {
		want := "manual"
		if i == 0 {
			want = "review"
		}
		if c.Status != want {
			t.Errorf("%s: got %s, want %s", c.Test, c.Status, want)
		}
	}
}

func TestPatchRejectsUnsupportedInputs(t *testing.T) {
	cases := []struct{ name, version, imports, body, selection string }{
		{"old_go", "1.24", "", `time.Sleep(time.Second)`, "TestExample"},
		{"io", "1.25", `; "net/http"`, `time.Sleep(time.Second); http.Get("http://localhost")`, "TestExample"},
		{"parallel_later", "1.25", "", `time.Sleep(time.Second); t.Parallel()`, "TestExample"},
		{"subtests", "1.25", "", `time.Sleep(time.Second); t.Run("child", func(t *testing.T){})`, "TestExample"},
		{"missing", "1.25", "", `time.Sleep(time.Second)`, "TestMissing"},
		{"already_wrapped", "1.25", `; "testing/synctest"`, `synctest.Test(t, func(t *testing.T){ time.Sleep(time.Second) })`, "TestExample"},
		{"nested_bubble", "1.25", `; "testing/synctest"`, `synctest.Test(t, func(t *testing.T){ t.Run("child", func(t *testing.T){ time.Sleep(time.Second) }) })`, "TestExample/child"},
		{"duplicate_subtests", "1.25", "", `t.Run("child", func(t *testing.T){time.Sleep(time.Second)}); t.Run("child", func(t *testing.T){time.Sleep(time.Second)})`, "TestExample/child"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := fixture(t, tc.version, "package fixture\nimport (\"testing\"; \"time\""+tc.imports+")\nfunc TestExample(t *testing.T){"+tc.body+"}\n")
			if diff, err := Patch(file, []string{tc.selection}); err == nil {
				t.Fatalf("unexpected patch: %s", diff)
			}
		})
	}
}

func TestPatchAppliesAndRunsWithRaceDetector(t *testing.T) {
	if testing.Short() {
		t.Skip("compiled patch integration")
	}
	// This verifies imports, comments, alias collisions, literal subtests,
	// outer Parallel, fake time, and synchronization in the compiled result.
	file := fixture(t, "1.25", `//go:build !ignore

// Package fixture is a generated-patch integration fixture.
package fixture

// Keep this import comment.
import (check "testing"; clock "time")

func TestClock(t *check.T) {
 t.Parallel() // keep parallelism outside the bubble
 synctest := "local name collision"
 _ = synctest
 start := clock.Now()
 done := false
 go func() { clock.Sleep(clock.Second); done = true }()
 clock.Sleep(2*clock.Second) // keep this assertion synchronization
 if !done { t.Fatal("worker did not finish") }
 if clock.Since(start) != 2*clock.Second { t.Fatal("expected virtual time") }
}

func TestParent(t *check.T) {
 t.Parallel()
 t.Run("child", func(t *check.T) {
  t.Parallel()
  clock.Sleep(clock.Hour)
 })
}
`)
	before, _ := os.ReadFile(file)
	diff, err := Patch(file, []string{"TestClock", "TestParent/child"})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	if string(before) != string(after) {
		t.Fatal("Patch mutated source")
	}
	cmd := exec.Command("git", "apply", "--check", "-")
	cmd.Dir = filepath.Dir(file)
	cmd.Stdin = strings.NewReader(diff)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("apply check: %v\n%s\n%s", err, out, diff)
	}
	cmd = exec.Command("git", "apply", "-")
	cmd.Dir = filepath.Dir(file)
	cmd.Stdin = strings.NewReader(diff)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	cmd = exec.Command("go", "test", "-race", "-count=5", "-timeout=20s", "./...")
	cmd.Dir = filepath.Dir(file)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compiled patch: %v\n%s", err, out)
	}
}

func TestUnifiedHandlesMissingNewlineAndSpaces(t *testing.T) {
	dir := t.TempDir()
	name := "test file.go"
	before := "package example\n// before"
	after := "package example\n// after\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(before), 0600); err != nil {
		t.Fatal(err)
	}
	// Compare patch bytes independently of the host Git line-ending policy.
	cmd := exec.Command("git", "-c", "core.autocrlf=false", "apply", "-")
	cmd.Dir = dir
	diff, err := unified(name, before, after)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdin = strings.NewReader(diff)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("apply: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(filepath.Join(dir, name))
	if string(got) != after {
		t.Fatalf("got %q", got)
	}
}
