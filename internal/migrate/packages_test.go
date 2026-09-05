package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const clockTest = `package example
import("testing";"time")
func TestClock(t *testing.T){time.Sleep(time.Second)}
`

func put(t *testing.T, file, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestPackagePatternsRespectTagsAndNestedModules(t *testing.T) {
	file := fixture(t, "1.25", clockTest)
	root := filepath.Dir(file)
	put(t, filepath.Join(root, "child", "child_test.go"), clockTest)
	put(t, filepath.Join(root, "tagged_test.go"), "//go:build scouttag\n\n"+strings.ReplaceAll(clockTest, "TestClock", "TestTagged"))
	put(t, filepath.Join(root, "nested", "go.mod"), "module nested\ngo 1.25\n")
	put(t, filepath.Join(root, "nested", "nested_test.go"), clockTest)
	for _, tc := range []struct {
		name     string
		patterns []string
		tags     string
		files    int
	}{
		{"default", nil, "", 1},
		{"recursive", []string{"./..."}, "", 2},
		{"dedup", []string{".", "./...", file}, "", 2},
		{"tagged", []string{"./..."}, "scouttag", 3},
		{"explicit_file", []string{file}, "", 1},
		{"relative_file", []string{"sample_test.go"}, "", 1},
		{"explicit_nested_module", []string{filepath.Join(root, "nested")}, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ScanPackages(root, tc.patterns, tc.tags)
			if err != nil || r.Files != tc.files || len(r.Candidates) != tc.files {
				t.Fatalf("%#v %v", r, err)
			}
		})
	}
	r, err := ScanPackages(root, []string{root + "/..."}, "")
	if err != nil || r.Files != 2 {
		t.Fatalf("%#v %v", r, err)
	}
}

func TestPackageErrors(t *testing.T) {
	file := fixture(t, "1.25", clockTest)
	root := filepath.Dir(file)
	if err := os.Mkdir(filepath.Join(root, "empty"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := ScanPackages(root, []string{"./empty/..."}, ""); err == nil {
		t.Fatal("empty package pattern accepted")
	}
	for _, patterns := range [][]string{{"-mod=mod"}, {"main.go"}, {"missing_test.go"}, {"./missing"}, {"./nothing/..."}} {
		if _, err := ScanPackages(root, patterns, ""); err == nil {
			t.Errorf("accepted %v", patterns)
		}
	}
	put(t, filepath.Join(root, "broken_test.go"), "not Go")
	if _, err := ScanPackages(root, []string{"broken_test.go"}, ""); err == nil {
		t.Fatal("malformed file accepted")
	}
	if err := os.Remove(filepath.Join(root, "broken_test.go")); err != nil {
		t.Fatal(err)
	}
	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(root, "link_test.go")
		if err := os.Symlink(file, link); err != nil {
			t.Skip(err)
		}
		if _, err := ScanPackages(root, []string{link}, ""); err == nil {
			t.Fatal("symlink accepted")
		}
	})
	t.Setenv("GOTOOLCHAIN", "invalid-value")
	if _, err := ScanPackages(root, nil, ""); err == nil {
		t.Fatal("failed go command ignored")
	}
}

func TestPrepareAndApplyPreserveSourceAndPermissions(t *testing.T) {
	file := fixture(t, "1.25", clockTest)
	root := filepath.Dir(file)
	changes, skipped, err := Prepare(root, nil, "", "TestClock")
	if err != nil || len(changes) != 1 || len(skipped) != 0 {
		t.Fatalf("%#v %#v %v", changes, skipped, err)
	}
	before, _ := os.ReadFile(file)
	if string(before) != clockTest {
		t.Fatal("Prepare wrote source")
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	originalStat, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(changes); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(file)
	if !strings.Contains(string(after), "synctest.Test(") {
		t.Fatal("no applied bubble")
	}
	st, _ := os.Stat(file)
	if st.Mode().Perm() != originalStat.Mode().Perm() {
		t.Fatalf("mode %v", st.Mode())
	}
	if err := Apply(changes); err == nil {
		t.Fatal("stale source not rejected")
	}
	if _, _, err := Prepare(root, nil, "", "["); err == nil {
		t.Fatal("invalid regexp accepted")
	}
	if _, _, err := Prepare(root, []string{"./missing"}, "", ""); err == nil {
		t.Fatal("invalid packages accepted")
	}
	changes, skipped, err = Prepare(root, nil, "", "")
	if err != nil || len(changes) != 0 || len(skipped) != 1 {
		t.Fatalf("second pass %#v %#v %v", changes, skipped, err)
	}
}

func TestPrepareSelectionAndBoundaries(t *testing.T) {
	file := fixture(t, "1.25", clockTest)
	root := filepath.Dir(file)
	put(t, filepath.Join(root, "child", "child_test.go"), clockTest)
	changes, _, err := Prepare(root, []string{"./..."}, "", "TestClock")
	if err != nil || len(changes) != 2 {
		t.Fatalf("%#v %v", changes, err)
	}
	for _, c := range changes {
		diff, err := c.Diff(root)
		if err != nil || !strings.Contains(diff, "synctest.Test(") {
			t.Fatalf("%s %v", diff, err)
		}
	}
	changes, _, err = Prepare(root, nil, "", "DoesNotExist")
	if err != nil || len(changes) != 0 {
		t.Fatalf("%#v %v", changes, err)
	}
	other := fixture(t, "1.25", clockTest)
	if _, _, err := Prepare(root, []string{other}, "", ""); err == nil {
		t.Fatal("outside root accepted")
	}
	if _, err := (Change{File: other}).Diff(root); err == nil {
		t.Fatal("outside diff accepted")
	}
	put(t, file, strings.ReplaceAll(clockTest, `"testing";"time"`, `"testing";"time";"testing/synctest"`))
	if _, _, err := Prepare(root, nil, "", ""); err == nil {
		t.Fatal("unsupported rewrite accepted")
	}
}

func TestApplyPreflightsAllFiles(t *testing.T) {
	file := fixture(t, "1.25", clockTest)
	change, err := rewrite(file, []string{"TestClock"})
	if err != nil {
		t.Fatal(err)
	}
	missing := Change{File: filepath.Join(t.TempDir(), "missing_test.go")}
	if err := Apply([]Change{*change, missing}); err == nil {
		t.Fatal("missing source accepted")
	}
	data, _ := os.ReadFile(file)
	if string(data) != clockTest {
		t.Fatal("earlier file changed before preflight completed")
	}
	if err := Apply([]Change{{File: filepath.Dir(file)}}); err == nil {
		t.Fatal("directory accepted")
	}
}
