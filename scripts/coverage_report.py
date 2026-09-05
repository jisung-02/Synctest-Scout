#!/usr/bin/env python3
"""Generate a statement-weighted report and fail below explicit thresholds."""
import argparse
from collections import defaultdict
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess

ROOT = Path(__file__).resolve().parents[1]
BLOCK = re.compile(r"(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)")


def read_profile(text):
    lines = text.splitlines()
    if not lines or lines[0] not in ("mode: atomic", "mode: count", "mode: set"):
        raise ValueError("missing or unsupported coverage mode")
    blocks = {}
    for line in lines[1:]:
        match = BLOCK.fullmatch(line)
        if not match:
            raise ValueError(f"invalid coverage record: {line!r}")
        file, *numbers = match.groups()
        start_line, start_col, end_line, end_col, statements, count = map(int, numbers)
        if min(start_line, start_col, end_line, end_col) < 1 or (end_line, end_col) < (start_line, start_col):
            raise ValueError("invalid source coordinates")
        key = (file, start_line, start_col, end_line, end_col)
        if key in blocks and blocks[key][0] != statements:
            raise ValueError("conflicting statement count for one block")
        blocks[key] = (statements, count > 0 or blocks.get(key, (0, False))[1])
    packages = defaultdict(lambda: {"covered": 0, "statements": 0})
    for key, (statements, hit) in blocks.items():
        package = key[0].rsplit("/", 1)[0]
        packages[package]["statements"] += statements
        packages[package]["covered"] += statements if hit else 0
    total = {k: sum(p[k] for p in packages.values()) for k in ("covered", "statements")}
    if total["statements"] == 0:
        raise ValueError("empty coverage cannot satisfy a gate")
    for p in [total, *packages.values()]:
        p["percent"] = 100 * p["covered"] / p["statements"] if p["statements"] else 100
    return {"total": total, "packages": dict(sorted(packages.items()))}


def gate(report, minimum, package_minimum):
    if not 0 <= minimum <= 100 or not 0 <= package_minimum <= 100:
        raise ValueError("thresholds must be in [0, 100]")
    failures = []
    if report["total"]["percent"] < minimum:
        failures.append(f"total {report['total']['percent']:.2f}% < {minimum}%")
    for name, value in report["packages"].items():
        if value["percent"] < package_minimum:
            failures.append(f"{name}: {value['percent']:.2f}% < {package_minimum}%")
    return failures


def fingerprint():
    digest = hashlib.sha256()
    files = [ROOT / "go.mod", *ROOT.glob("cmd/**/*.go"), *ROOT.glob("internal/**/*.go")]
    for path in sorted(files):
        digest.update(path.relative_to(ROOT).as_posix().encode() + b"\0" + path.read_bytes())
    return digest.hexdigest()


def write_report(report, output, minimum, package_minimum):
    output.mkdir(parents=True, exist_ok=True)
    report["generated_at"] = datetime.now(timezone.utc).isoformat()
    report["go_source_sha256"] = fingerprint()
    report["go_version"] = subprocess.check_output(["go", "version"], text=True).strip()
    report["thresholds"] = {"total": minimum, "each_package": package_minimum}
    report["failures"] = gate(report, minimum, package_minimum)
    total = report["total"]
    lines = ["# Synctest Scout coverage", "", "[English](summary.md) | [한국어](summary.ko.md)", "", f"Generated: {report['generated_at']}", "",
             f"`{report['go_version']}`", "",
             f"**{total['percent']:.2f}%** ({total['covered']}/{total['statements']} statements).",
             f"Gate: total ≥ {minimum}%; every package ≥ {package_minimum}%.", "",
             "| Package | Covered | Statements | Coverage |", "|---|---:|---:|---:|"]
    for name, p in report["packages"].items():
        lines.append(f"| `{name}` | {p['covered']} | {p['statements']} | {p['percent']:.2f}% |")
    lines += ["", "Result: " + ("FAIL — " + "; ".join(report["failures"]) if report["failures"] else "PASS"), "",
              "This is statement coverage, not proof of semantic safety or branch coverage.",
              "No Go source package is excluded. The one-line process exit and some OS-error paths may be uncovered.",
              "The README badge is this committed snapshot; every CI run produces a fresh Job Summary and coverage artifact.", "",
              f"Go source fingerprint: `{report['go_source_sha256']}`.", ""]
    (output / "summary.md").write_text("\n".join(lines), encoding="utf-8")
    korean = ["# Synctest Scout 커버리지", "", "[English](summary.md) | [한국어](summary.ko.md)", "",
              f"생성 시각: {report['generated_at']}", "", f"`{report['go_version']}`", "",
              f"**{total['percent']:.2f}%** ({total['covered']}/{total['statements']} 문장).",
              f"통과 기준: 전체 ≥ {minimum}%; 각 패키지 ≥ {package_minimum}%.", "",
              "| 패키지 | 실행한 문장 | 전체 문장 | 커버리지 |", "|---|---:|---:|---:|"]
    for name, value in report["packages"].items():
        korean.append(f"| `{name}` | {value['covered']} | {value['statements']} | {value['percent']:.2f}% |")
    korean += ["", "결과: " + ("FAIL — " + "; ".join(report["failures"]) if report["failures"] else "PASS"), "",
               "문장 커버리지이며 분기 커버리지나 전환 의미의 안전성을 입증하지 않습니다.",
               "제외한 Go 패키지는 없습니다. 프로세스 종료 한 줄과 일부 OS 오류 경로는 미측정일 수 있습니다.",
               "README 배지는 커밋된 스냅샷이며 CI 실행마다 새 Job Summary와 아티팩트를 생성합니다.", "",
               f"Go 소스 지문: `{report['go_source_sha256']}`.", ""]
    (output / "summary.ko.md").write_text("\n".join(korean), encoding="utf-8")
    (output / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
    color = "#15803d" if not report["failures"] else "#b91c1c"
    label = f"{total['percent']:.1f}%"
    (output / "coverage.svg").write_text(
        f'<svg xmlns="http://www.w3.org/2000/svg" width="154" height="22" role="img" aria-label="coverage snapshot: {label}">'
        '<title>Coverage snapshot</title><rect width="154" height="22" rx="4" fill="#334155"/>'
        f'<path d="M96 0h54q4 0 4 4v14q0 4-4 4H96z" fill="{color}"/>'
        '<g fill="white" font-family="Verdana,sans-serif" font-size="11" text-anchor="middle">'
        f'<text x="48" y="15">coverage</text><text x="125" y="15">{label}</text></g></svg>\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("profile", type=Path)
    parser.add_argument("--output", type=Path, default=ROOT / "coverage")
    parser.add_argument("--minimum", type=float, default=95)
    parser.add_argument("--package-minimum", type=float, default=90)
    parser.add_argument("--snapshot", action="store_true", help="refresh the committed README report and badge")
    args = parser.parse_args()
    report = read_profile(args.profile.read_text())
    write_report(report, args.output, args.minimum, args.package_minimum)
    if args.snapshot:
        (ROOT / "docs").mkdir(exist_ok=True)
        for suffix in (".md", ".ko.md"):
            content = (args.output / ("summary" + suffix)).read_text(encoding="utf-8")
            content = content.replace("](summary", "](coverage")
            (ROOT / "docs" / ("coverage" + suffix)).write_text(content, encoding="utf-8")
        shutil.copyfile(args.output / "coverage.svg", ROOT / "docs" / "coverage.svg")
    print((args.output / "summary.md").read_text(encoding="utf-8"))
    return bool(report["failures"])


if __name__ == "__main__":
    raise SystemExit(main())
