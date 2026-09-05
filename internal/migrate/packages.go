package migrate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ScanPackages follows go list package/build-tag selection. No selected Go
// tests are executed; go list may access the module cache and module proxies.
func ScanPackages(root string, patterns []string, tags string) (Report, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	files := map[string]bool{}
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "-") {
			return Report{}, fmt.Errorf("unexpected package option %q; flags must precede packages", pattern)
		}
		dir, arg := root, pattern
		path := pattern
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if strings.HasSuffix(pattern, ".go") {
			if !strings.HasSuffix(pattern, "_test.go") {
				return Report{}, fmt.Errorf("%s is not a test file", pattern)
			}
			files[filepath.Clean(path)] = true
			continue
		}
		prefix := path
		if filepath.Base(path) == "..." {
			prefix = filepath.Dir(path)
		}
		if st, e := os.Stat(prefix); e == nil && st.IsDir() {
			dir = prefix
			arg = "."
			if filepath.Base(path) == "..." {
				arg = "./..."
			}
		}
		args := []string{"list", "-e", "-json", "-mod=readonly"}
		if tags != "" {
			args = append(args, "-tags", tags)
		}
		args = append(args, "--", arg)
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		var diagnostic bytes.Buffer
		cmd.Stderr = &diagnostic
		data, e := cmd.Output()
		if e != nil {
			return Report{}, fmt.Errorf("go list %s: %w: %s", pattern, e, strings.TrimSpace(diagnostic.String()))
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		count := 0
		for {
			var pkg struct {
				Dir                       string
				TestGoFiles, XTestGoFiles []string
				Error                     *struct{ Err string }
			}
			e := decoder.Decode(&pkg)
			if e == io.EOF {
				break
			}
			if e != nil {
				return Report{}, fmt.Errorf("go list JSON: %w", e)
			}
			if pkg.Error != nil {
				return Report{}, fmt.Errorf("go list %s: %s", pattern, pkg.Error.Err)
			}
			count++
			for _, file := range append(pkg.TestGoFiles, pkg.XTestGoFiles...) {
				files[filepath.Join(pkg.Dir, file)] = true
			}
		}
		if count == 0 {
			return Report{}, fmt.Errorf("pattern %q matched no packages", pattern)
		}
	}
	ordered := make([]string, 0, len(files))
	for file := range files {
		ordered = append(ordered, file)
	}
	sort.Strings(ordered)
	report := Report{Root: root, Candidates: []Candidate{}}
	for _, file := range ordered {
		st, err := os.Lstat(file)
		if err != nil {
			return Report{}, err
		}
		if !st.Mode().IsRegular() {
			return Report{}, fmt.Errorf("not a regular test file: %s", file)
		}
		s, err := parse(file)
		if err != nil {
			return Report{}, err
		}
		report.Files++
		for _, test := range s.tests() {
			c := s.analyze(test, moduleVersion(filepath.Dir(file)))
			if len(c.Signals) > 0 {
				report.Candidates = append(report.Candidates, c)
			}
		}
	}
	return report, nil
}

type Change struct {
	File          string
	Before, After []byte
}

// Prepare resolves and rewrites every selected file before any writes occur.
func Prepare(root string, patterns []string, tags, run string) ([]Change, []Candidate, error) {
	filter, err := regexp.Compile(run)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid -run: %w", err)
	}
	report, err := ScanPackages(root, patterns, tags)
	if err != nil {
		return nil, nil, err
	}
	groups := map[string][]string{}
	var skipped []Candidate
	for _, c := range report.Candidates {
		if !filter.MatchString(c.Test) {
			continue
		}
		if c.Status != "review" {
			skipped = append(skipped, c)
			continue
		}
		groups[c.File] = append(groups[c.File], c.Test)
	}
	files := make([]string, 0, len(groups))
	for file := range groups {
		files = append(files, file)
	}
	sort.Strings(files)
	var changes []Change
	for _, file := range files {
		if _, err := relativeFile(report.Root, file); err != nil {
			return nil, nil, err
		}
		change, err := rewrite(file, groups[file])
		if err != nil {
			return nil, nil, err
		}
		changes = append(changes, *change)
	}
	return changes, skipped, nil
}

func relativeFile(root, file string) (string, error) {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("refusing changes outside %s; use -C to select the project root", root)
	}
	return filepath.ToSlash(rel), nil
}

func (c Change) Diff(root string) (string, error) {
	rel, err := relativeFile(root, c.File)
	if err != nil {
		return "", err
	}
	return unified(rel, string(c.Before), string(c.After))
}

// Apply uses an atomic rename for each file and preserves its permissions.
// It rejects stale source before staging anything. It is not a multi-file
// filesystem transaction: a rename failure can leave earlier files applied.
func Apply(changes []Change) error {
	for _, c := range changes {
		st, err := os.Lstat(c.File)
		if err != nil {
			return err
		}
		if !st.Mode().IsRegular() {
			return fmt.Errorf("not a regular file: %s", c.File)
		}
		current, err := os.ReadFile(c.File)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, c.Before) {
			return fmt.Errorf("source changed since scan: %s", c.File)
		}
	}
	for _, c := range changes {
		st, err := os.Stat(c.File)
		if err != nil {
			return err
		}
		f, err := os.CreateTemp(filepath.Dir(c.File), ".synctest-scout-*")
		if err != nil {
			return err
		}
		name := f.Name()
		defer os.Remove(name)
		if err = f.Chmod(st.Mode().Perm()); err != nil {
			f.Close()
			return err
		}
		if _, err = f.Write(c.After); err != nil {
			f.Close()
			return err
		}
		if err = f.Close(); err != nil {
			return err
		}
		if err = os.Rename(name, c.File); err != nil {
			return fmt.Errorf("replace %s: %w", c.File, err)
		}
	}
	return nil
}
