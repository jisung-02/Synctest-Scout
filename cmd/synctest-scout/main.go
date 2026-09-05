package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jisung-02/Synctest-Scout/internal/migrate"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = `usage: synctest-scout [-C dir] <command> [arguments]

Commands:
    scan       list timing-related tests in Go packages
    fix        apply synctest rewrites; -diff previews without writing
    version    print version information

Use "synctest-scout help <command>" for command flags.
Package arguments follow go list: default is ., use ./... for all packages.
fix -diff exits 1 when changes exist, like go fix -diff.
Review diffs before applying: this tool does not prove semantic safety.`

var errDiff = errors.New("changes available")

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

func execute(args []string, out, errOut io.Writer) int {
	if err := run(args, out, errOut); err != nil && !errors.Is(err, flag.ErrHelp) {
		if errors.Is(err, errDiff) {
			return 1
		}
		fmt.Fprintln(errOut, "error:", err)
		return 1
	}
	return 0
}

func run(args []string, out, errOut io.Writer) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(args) > 0 && (args[0] == "-C" || strings.HasPrefix(args[0], "-C=")) {
		var dir string
		if args[0] == "-C" {
			if len(args) < 2 {
				return fmt.Errorf("-C requires a directory")
			}
			dir = args[1]
			args = args[2:]
		} else {
			dir = strings.TrimPrefix(args[0], "-C=")
			args = args[1:]
		}
		if dir == "" {
			return fmt.Errorf("-C requires a directory")
		}
		root, err = filepath.Abs(dir)
		if err != nil {
			return err
		}
		st, err := os.Stat(root)
		if err != nil {
			return err
		}
		if !st.IsDir() {
			return fmt.Errorf("-C requires a directory")
		}
	}
	return runAt(root, args, out, errOut)
}

func runAt(root string, args []string, out, errOut io.Writer) error {
	if len(args) == 0 {
		return errors.New(usage)
	}
	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			switch args[1] {
			case "scan", "fix", "patch":
				return runAt(root, []string{args[1], "-h"}, out, out)
			default:
				return fmt.Errorf("unknown help topic %q", args[1])
			}
		}
		_, err := fmt.Fprintln(out, usage)
		return err
	case "version", "--version":
		if len(args) != 1 {
			return fmt.Errorf("version accepts no arguments")
		}
		_, err := fmt.Fprintf(out, "synctest-scout %s (commit %s, built %s)\n", version, commit, date)
		return err
	case "scan":
		fs := flag.NewFlagSet("scan", flag.ContinueOnError)
		fs.SetOutput(errOut)
		asJSON := fs.Bool("json", false, "emit a JSON report")
		tags := fs.String("tags", "", "comma-separated build tags (as in go list)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		report, err := migrate.ScanPackages(root, fs.Args(), *tags)
		if err != nil {
			return err
		}
		if *asJSON {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(report)
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "%d timing candidates in %d test files; Go >= 1.25 required for patches.\n", len(report.Candidates), report.Files)
		fmt.Fprintln(&buf, "Heuristic source scan, not a safety proof. Review callees, shared state, and time semantics.")
		for _, c := range report.Candidates {
			fmt.Fprintf(&buf, "\n%s:%d  %s [%s]\n  timing: %s\n", c.File, c.Line, c.Test, c.Status, strings.Join(c.Signals, ", "))
			for _, reason := range c.Reasons {
				fmt.Fprintln(&buf, "  -", reason)
			}
			for _, note := range c.Notes {
				fmt.Fprintln(&buf, "  -", note)
			}
		}
		_, err = io.WriteString(out, buf.String())
		return err
	case "fix":
		fs := flag.NewFlagSet("fix", flag.ContinueOnError)
		fs.SetOutput(errOut)
		diff := fs.Bool("diff", false, "print unified diffs instead of writing; exit 1 if changes exist")
		dry := fs.Bool("n", false, "print files that would change without writing")
		run := fs.String("run", "", "regular expression selecting full test paths, e.g. TestDo/context")
		tags := fs.String("tags", "", "comma-separated build tags (as in go list)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *diff && *dry {
			return fmt.Errorf("-diff and -n cannot be combined")
		}
		changes, skipped, err := migrate.Prepare(root, fs.Args(), *tags, *run)
		if err != nil {
			return err
		}
		for _, c := range skipped {
			fmt.Fprintf(errOut, "%s:%d: skipping %s: %s\n", c.File, c.Line, c.Test, strings.Join(c.Reasons, "; "))
		}
		var text strings.Builder
		for _, c := range changes {
			if *diff {
				d, err := c.Diff(root)
				if err != nil {
					return err
				}
				text.WriteString(d)
			} else {
				rel, err := filepath.Rel(root, c.File)
				if err != nil {
					return err
				}
				fmt.Fprintln(&text, filepath.ToSlash(rel))
			}
		}
		if !*diff && !*dry {
			if err := migrate.Apply(changes); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, text.String()); err != nil {
			return err
		}
		if *diff && len(changes) > 0 {
			return errDiff
		}
		return nil
	case "patch":
		fs := flag.NewFlagSet("patch", flag.ContinueOnError)
		fs.SetOutput(errOut)
		file := fs.String("file", "", "test source file")
		tests := fs.String("test", "", "exact test paths, comma-separated; supports TestName/subtest")
		reviewed := fs.Bool("reviewed", false, "acknowledge manual review of callees, state isolation and virtual-time semantics")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *file == "" || *tests == "" || fs.NArg() != 0 {
			return fmt.Errorf("patch requires -file and -test")
		}
		if !*reviewed {
			return fmt.Errorf("source scan cannot prove isolation; review the selected tests and callees, then use -reviewed")
		}
		if !filepath.IsAbs(*file) {
			*file = filepath.Join(root, *file)
		}
		diff, err := migrate.Patch(*file, strings.Split(*tests, ","))
		if err != nil {
			return err
		}
		_, err = io.WriteString(out, diff)
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}
