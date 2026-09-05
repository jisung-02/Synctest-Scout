#!/usr/bin/env python3
"""Portable local equivalents of the GitHub Actions quality checks."""
import argparse
from pathlib import Path
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]


def run(args, **kwargs):
    print("+ " + " ".join(map(str, args)), flush=True)
    subprocess.run(list(map(str, args)), cwd=ROOT, check=True, **kwargs)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("task", choices=["lint", "test", "coverage", "stress", "fuzz"])
    parser.add_argument("--snapshot", action="store_true")
    parser.add_argument("--fuzztime", default="60s")
    args = parser.parse_args()
    if args.task == "lint":
        files = sorted([*ROOT.glob("cmd/**/*.go"), *ROOT.glob("internal/**/*.go")])
        result = subprocess.check_output(["gofmt", "-l", *map(str, files)], cwd=ROOT, text=True)
        if result:
            sys.exit("gofmt required:\n" + result)
        run(["go", "vet", "./..."])
        run([sys.executable, "-m", "unittest", "discover", "-s", "scripts", "-p", "test_*.py", "-v"])
    elif args.task == "test":
        run(["go", "test", "-race", "-shuffle=on", "-count=3", "-timeout=5m", "./..."])
    elif args.task == "stress":
        run(["go", "test", "-short", "-race", "-shuffle=on", "-count=50", "-cpu=1,2,4", "-timeout=10m", "-run=Test(ScanClassifies|PatchRefusal|AllTiming|PatchStatement|ModuleVersion|Unified)", "./internal/migrate"])
    elif args.task == "fuzz":
        for target in ["FuzzScanSource", "FuzzPatchRoundTrip"]:
            run(["go", "test", "./internal/migrate", "-run=^$", "-fuzz=^"+target+"$", "-fuzztime="+args.fuzztime, "-parallel=2", "-timeout=10m"])
    else:
        output = ROOT / "coverage"
        output.mkdir(exist_ok=True)
        profile = output / "coverage.out"
        run(["go", "test", "-race", "-covermode=atomic", "-coverpkg=./...", "-coverprofile="+str(profile), "-shuffle=on", "-count=1", "-timeout=5m", "./..."])
        with (output / "functions.txt").open("w") as stream:
            run(["go", "tool", "cover", "-func="+str(profile)], stdout=stream)
        run(["go", "tool", "cover", "-html="+str(profile), "-o", output / "index.html"])
        run([sys.executable, "scripts/coverage_report.py", profile, *( ["--snapshot"] if args.snapshot else [])])


if __name__ == "__main__":
    main()
