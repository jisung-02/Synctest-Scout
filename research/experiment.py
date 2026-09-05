#!/usr/bin/env python3
"""Reproduce the pinned-repository scan and before/after experiment.

Requires Go >= 1.25, Git, and Python >= 3.12. Runs downloaded Go test code.
Pristine checkouts are retained in .research/repos; all experimental edits are
made in temporary copies. No commits, issues, or pull requests are published.
"""

import io
import json
import os
from pathlib import Path
import platform
import statistics
import subprocess
import tarfile
import tempfile
import time


ROOT = Path(__file__).resolve().parents[1]
WORK = ROOT / ".research"
OUT = ROOT / "research"
REPOS = {
    "backoff": ("https://github.com/cenkalti/backoff.git", "ffcfd8ab39e2910a1180ba0b7a02a52f0485adc9"),
    "retry-go": ("https://github.com/avast/retry-go.git", "5bccbfa9340dfe6609f4ecfce30c971e2756c796"),
    "go-retry": ("https://github.com/sethvargo/go-retry.git", "f6b3e1a9f1c599bf6fd42d01811a62fc4b9b7502"),
}
PATCHES = {
    "go-retry-backoff.patch": ("backoff_test.go", "TestWithMaxDuration"),
    "go-retry-context.patch": ("retry_test.go", "TestDo/context_canceled,TestDo/deadline_exceeded"),
}
COUNT = 20
TRIALS = 3


def run(args, cwd=ROOT, check=True, input=None, timeout=120):
    proc = subprocess.run(list(map(str, args)), cwd=cwd, text=True, input=input,
                          stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
    if check and proc.returncode:
        raise RuntimeError(f"{args}: exit {proc.returncode}\n{proc.stdout}")
    return proc


def snapshot(repo, sha, dest):
    dest.mkdir()
    archive = subprocess.check_output(["git", "archive", sha], cwd=repo)
    with tarfile.open(fileobj=io.BytesIO(archive)) as tf:
        tf.extractall(dest, filter="data")
    # Without an independent git root, git apply can silently skip file paths
    # when these snapshots live below the tool's own repository root.
    run(["git", "init", "--quiet"], cwd=dest)


def main():
    for path in (WORK / "repos", OUT / "scans", OUT / "patches", OUT / "logs", ROOT / "bin"):
        path.mkdir(parents=True, exist_ok=True)
    binary = ROOT / "bin" / "synctest-scout"
    run(["go", "build", "-o", binary, "./cmd/synctest-scout"])
    result = {
        "go_version": run(["go", "version"]).stdout.strip(),
        "platform": platform.platform(),
        "utc_time": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "method": "Precompiled non-race test binaries; wall time includes process startup, excludes compilation; 20 repetitions per invocation, 3 trials per case, interleaved before/after.",
        "repositories": {},
        "measurements": {},
    }
    with tempfile.TemporaryDirectory(prefix="experiment-", dir=WORK) as tmp:
        tmp = Path(tmp)
        for name, (url, sha) in REPOS.items():
            print(f"Scan {name} at {sha[:12]}", flush=True)
            repo = WORK / "repos" / name
            if not repo.exists():
                run(["git", "clone", "--no-checkout", url, repo])
            if run(["git", "cat-file", "-e", sha], cwd=repo, check=False).returncode:
                run(["git", "fetch", "origin", sha], cwd=repo)
            dest = tmp / name
            snapshot(repo, sha, dest)
            scan = json.loads(run([binary, "scan", "-json", str(dest) + "/..."]).stdout)
            scan["root"] = name
            for candidate in scan["candidates"]:
                candidate["file"] = str(Path(candidate["file"]).relative_to(dest))
            (OUT / "scans" / f"{name}.json").write_text(json.dumps(scan, indent=2) + "\n")
            result["repositories"][name] = {"url": url, "commit": sha, "test_files": scan["test_files"], "candidates": len(scan["candidates"])}

        before = tmp / "go-retry"
        after = tmp / "go-retry-after"
        snapshot(WORK / "repos" / "go-retry", REPOS["go-retry"][1], after)
        for name, (file, tests) in PATCHES.items():
            patch = run([binary, "patch", "-file", before / file, "-test", tests, "-reviewed"]).stdout
            (OUT / "patches" / name).write_text(patch)
            run(["git", "apply", "--check", "-"], cwd=after, input=patch)
            run(["git", "apply", "-"], cwd=after, input=patch)
            rewritten = (after / file).read_text()
            if rewritten == (before / file).read_text() or rewritten.count("synctest.Test(") != len(tests.split(",")):
                raise RuntimeError(f"Patch was not applied to every selected test: {file}")

        bins = {}
        for label, cwd in (("before", before), ("after", after)):
            print(f"Compile {label}; run full suite with race detector, count=10", flush=True)
            bins[label] = tmp / f"{label}.test"
            run(["go", "test", "-c", "-o", bins[label], "."], cwd=cwd)
            p = run(["go", "test", "-race", "-count=10", "-timeout=60s", "./..."], cwd=cwd, check=False)
            (OUT / "logs" / f"{label}-race.txt").write_text(p.stdout)
            result[f"{label}_full_suite_race"] = {"exit_code": p.returncode, "count": 10}
            if p.returncode:
                raise RuntimeError(f"Full suite failed: {label}\n{p.stdout}")

        cases = {
            "TestWithMaxDuration": "^TestWithMaxDuration$",
            "TestDo/context_canceled": "^TestDo$/^context_canceled$",
            "TestDo/deadline_exceeded": "^TestDo$/^deadline_exceeded$",
        }
        for name, pattern in cases.items():
            print(f"Measure {name}: {TRIALS} x {COUNT} before and after", flush=True)
            samples = {"before": [], "after": []}
            for trial in range(TRIALS):
                for label, cwd in (("before", before), ("after", after)):
                    start = time.perf_counter()
                    p = run([bins[label], "-test.run="+pattern, "-test.count="+str(COUNT), "-test.timeout=30s", "-test.v"], cwd=cwd)
                    samples[label].append(time.perf_counter() - start)
                    log = name.replace("/", "-") + f"-{label}-{trial}.txt"
                    (OUT / "logs" / log).write_text(p.stdout)
                    # Count only leaf passes; a passing parent alone is insufficient.
                    passes = sum(line.strip().startswith("--- PASS: "+name+" (") for line in p.stdout.splitlines())
                    if passes != COUNT:
                        raise RuntimeError(f"Expected {COUNT} passes for {name}, got {passes}")
            medians = {key: statistics.median(values) for key, values in samples.items()}
            result["measurements"][name] = {
                "count_per_trial": COUNT, "trials": TRIALS,
                "seconds": samples, "median_batch_seconds": medians,
                "wall_time_ratio": medians["before"] / medians["after"],
            }

        # Deliberately break expiry exactly at the deadline; restore in finally.
        print("Mutation check: expiry <= 0 changed to < 0", flush=True)
        code = after / "backoff.go"
        original = code.read_text()
        assert original.count("if diff <= 0 {") == 1
        try:
            code.write_text(original.replace("if diff <= 0 {", "if diff < 0 {"))
            p = run(["go", "test", "-run=^TestWithMaxDuration$", "-count=1", "-timeout=10s", "."], cwd=after, check=False)
            (OUT / "logs" / "mutation.txt").write_text(p.stdout)
            detected = p.returncode != 0 and "should stop" in p.stdout
            result["mutation"] = {"change": "WithMaxDuration expiry <= 0 changed to < 0", "detected": detected, "exit_code": p.returncode}
            if not detected:
                raise RuntimeError("Converted test did not detect the expected mutation\n"+p.stdout)
        finally:
            code.write_text(original)

    (OUT / "results.json").write_text(json.dumps(result, indent=2) + "\n")
    print("Complete: research/results.json", flush=True)


if __name__ == "__main__":
    main()
