package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoStyleFixCommands(t *testing.T) {
	for _, mode := range []string{"diff", "dry", "write"} {
		t.Run(mode, func(t *testing.T) {
			root := project(t)
			file := filepath.Join(root, "sample_test.go")
			before, _ := os.ReadFile(file)
			args := []string{"-C", root, "fix", "-run", "^TestClock$"}
			if mode == "diff" {
				args = append(args, "-diff")
			}
			if mode == "dry" {
				args = append(args, "-n")
			}
			args = append(args, "./...")
			var out, diagnostic bytes.Buffer
			code := execute(args, &out, &diagnostic)
			want := 0
			if mode == "diff" {
				want = 1
			}
			if code != want || diagnostic.Len() != 0 {
				t.Fatalf("exit %d: %s", code, diagnostic.String())
			}
			after, _ := os.ReadFile(file)
			if mode == "write" {
				if bytes.Equal(before, after) || !strings.Contains(string(after), "synctest.Test(") {
					t.Fatal("no change written")
				}
			} else if !bytes.Equal(before, after) {
				t.Fatal("preview mutated source")
			}
			if mode == "diff" {
				if !strings.Contains(out.String(), "diff --git") {
					t.Fatal(out.String())
				}
			} else if strings.TrimSpace(out.String()) != "sample_test.go" {
				t.Fatal(out.String())
			}
		})
	}
}

func TestGoStyleHelpAndFailures(t *testing.T) {
	root := project(t)
	for _, args := range [][]string{
		{"help", "scan"}, {"help", "fix"}, {"-C=" + root, "scan", "-json", "-tags", "scouttag", "./..."},
		{"-C", root, "fix", "-n"}, {"-C", root, "fix", "-diff", "-run", "DoesNotExist"},
		{"-C", root, "patch", "-file", "sample_test.go", "-test", "TestClock", "-reviewed"},
	} {
		var out, diagnostic bytes.Buffer
		if code := execute(args, &out, &diagnostic); code != 0 {
			t.Fatalf("%v: %s", args, diagnostic.String())
		}
	}
	for _, args := range [][]string{
		{"-C"}, {"-C="}, {"-C", filepath.Join(root, "missing")}, {"-C", filepath.Join(root, "go.mod")},
		{"help", "unknown"}, {"fix", "-bad"}, {"fix", "-diff", "-n"},
		{"-C", root, "fix", "-run", "["}, {"-C", root, "fix", "missing_test.go"},
	} {
		if code := execute(args, io.Discard, io.Discard); code != 1 {
			t.Errorf("%v: exit %d", args, code)
		}
	}
	if err := run([]string{"-C", root, "fix", "-diff", "-run", "TestClock"}, failingWriter{}, io.Discard); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
}
