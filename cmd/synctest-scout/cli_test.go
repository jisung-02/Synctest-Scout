package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jisung-02/Synctest-Scout/internal/migrate"
)

func project(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range map[string]string{
		"go.mod": "module fixture\n\ngo 1.25\n",
		"sample_test.go": `package fixture
import ("testing"; "time"; "net/http")
func TestClock(t *testing.T) { time.Sleep(time.Second) }
func TestNetwork(t *testing.T) { time.Sleep(time.Second); http.Get("https://example.invalid") }
`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestCLICommandsAndExitCodes(t *testing.T) {
	dir := project(t)
	file := filepath.Join(dir, "sample_test.go")
	cases := []struct {
		name               string
		args               []string
		code               int
		output, diagnostic string
	}{
		{"help", []string{"--help"}, 0, "usage:", ""},
		{"version", []string{"version"}, 0, "synctest-scout dev", ""},
		{"empty", nil, 1, "", "usage:"},
		{"unknown", []string{"oops"}, 1, "", "unknown command"},
		{"version_args", []string{"version", "oops"}, 1, "", "no arguments"},
		{"scan_help", []string{"scan", "-h"}, 0, "", "Usage of scan:"},
		{"scan_flag", []string{"scan", "-bad"}, 1, "", "flag provided but not defined"},
		{"scan_args", []string{"scan", dir, dir}, 0, "2 timing candidates", ""},
		{"scan_missing", []string{"scan", filepath.Join(dir, "missing")}, 1, "", "error:"},
		{"scan_text", []string{"scan", dir}, 0, "2 timing candidates", ""},
		{"patch_help", []string{"patch", "-h"}, 0, "", "Usage of patch:"},
		{"patch_flag", []string{"patch", "-bad"}, 1, "", "flag provided but not defined"},
		{"patch_args", []string{"patch"}, 1, "", "requires -file and -test"},
		{"patch_extra", []string{"patch", "-file", file, "-test", "TestClock", "oops"}, 1, "", "requires -file and -test"},
		{"patch_unreviewed", []string{"patch", "-file", file, "-test", "TestClock"}, 1, "", "-reviewed"},
		{"patch_missing", []string{"patch", "-file", file, "-test", "TestMissing", "-reviewed"}, 1, "", "not all test paths found"},
		{"patch", []string{"patch", "-file", file, "-test", "TestClock", "-reviewed"}, 0, "synctest.Test", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			if got := execute(tc.args, &out, &diagnostic); got != tc.code {
				t.Fatalf("exit %d, want %d; %s", got, tc.code, diagnostic.String())
			}
			if tc.output != "" && !strings.Contains(out.String(), tc.output) {
				t.Fatalf("stdout %q missing %q", out.String(), tc.output)
			}
			if tc.diagnostic != "" && !strings.Contains(diagnostic.String(), tc.diagnostic) {
				t.Fatalf("stderr %q missing %q", diagnostic.String(), tc.diagnostic)
			}
			if tc.code != 0 && out.Len() != 0 {
				t.Fatalf("failed command emitted partial output: %s", out.String())
			}
		})
	}
}

func TestScanJSONAndDefaultDirectory(t *testing.T) {
	dir := project(t)
	t.Chdir(dir)
	var out bytes.Buffer
	if err := run([]string{"scan", "-json"}, &out, io.Discard); err != nil {
		t.Fatal(err)
	}
	var report migrate.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Candidates) != 2 || report.Candidates[0].Test != "TestClock" || report.Candidates[1].Status != "manual" {
		t.Fatalf("unexpected report: %#v", report)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestCLIPropagatesOutputFailure(t *testing.T) {
	dir := project(t)
	for _, args := range [][]string{
		{"help"}, {"version"}, {"scan", dir}, {"scan", "-json", dir},
		{"patch", "-file", filepath.Join(dir, "sample_test.go"), "-test", "TestClock", "-reviewed"},
	} {
		if err := run(args, failingWriter{}, io.Discard); !errors.Is(err, io.ErrClosedPipe) {
			t.Errorf("%v: %v", args, err)
		}
	}
}

func TestBuiltCLI(t *testing.T) {
	if testing.Short() {
		t.Skip("subprocess build")
	}
	binary := filepath.Join(t.TempDir(), "synctest-scout.exe")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", binary, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	for _, tc := range []struct {
		args    []string
		success bool
		want    string
	}{
		{[]string{"version"}, true, "synctest-scout dev"},
		{[]string{"invalid"}, false, "unknown command"},
	} {
		out, err := exec.CommandContext(t.Context(), binary, tc.args...).CombinedOutput()
		if (err == nil) != tc.success || !strings.Contains(string(out), tc.want) {
			t.Fatalf("%v: %v %s", tc.args, err, out)
		}
	}
}
