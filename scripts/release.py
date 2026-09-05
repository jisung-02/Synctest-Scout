#!/usr/bin/env python3
"""Build versioned CLI archives and SHA-256 checksums; never publish."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parents[1]
NAME = "synctest-scout"
TARGETS = [(goos, arch) for goos in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]
VERSION = re.compile(r"v(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?")


def validate_version(version):
    if not VERSION.fullmatch(version):
        raise ValueError("version must be vMAJOR.MINOR.PATCH, optionally with a prerelease suffix")
    if "-" in version and any(x.isdigit() and len(x)>1 and x.startswith("0") for x in version.split("-",1)[1].split(".")):
        raise ValueError("numeric prerelease identifiers must not have leading zeroes")


def archive(path, files, windows):
    # Fixed timestamps and ownership make packaging independent of the host.
    if windows:
        with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
            for name, (data, mode) in sorted(files.items()):
                info = zipfile.ZipInfo(name, (1980, 1, 1, 0, 0, 0))
                info.create_system = 3
                info.external_attr = (0o100000 | mode) << 16
                info.compress_type = zipfile.ZIP_DEFLATED
                zf.writestr(info, data)
    else:
        with path.open("wb") as raw, gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as gz, tarfile.open(fileobj=gz, mode="w") as tf:
            for name, (data, mode) in sorted(files.items()):
                info = tarfile.TarInfo(name)
                info.size, info.mode, info.mtime = len(data), mode, 0
                tf.addfile(info, io.BytesIO(data))


def metadata(command, fallback):
    p = subprocess.run(command, cwd=ROOT, text=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    return p.stdout.strip() if p.returncode == 0 else fallback


def build(version, output, targets=TARGETS):
    validate_version(version)
    if output.exists() and any(output.iterdir()):
        raise ValueError("output directory must be empty; choose a fresh directory")
    output.mkdir(parents=True, exist_ok=True)
    commit = metadata(["git","rev-parse","HEAD"], "unknown")
    date = metadata(["git","log","-1","--format=%cI"], "unknown")
    manifest = {"name":NAME, "version":version, "commit":commit, "date":date, "go":metadata(["go","version"],"unknown"), "artifacts":[]}
    with tempfile.TemporaryDirectory(prefix=NAME+"-build-") as work:
        for goos, arch in targets:
            exe = NAME + (".exe" if goos == "windows" else "")
            dest = Path(work) / exe
            env = dict(os.environ, GOOS=goos, GOARCH=arch, CGO_ENABLED="0")
            subprocess.run(["go","build","-trimpath","-buildvcs=false","-ldflags",f"-s -w -X main.version={version} -X main.commit={commit} -X main.date={date}","-o",str(dest),"./cmd/synctest-scout"], cwd=ROOT, env=env, check=True)
            extension = "zip" if goos == "windows" else "tar.gz"
            path = output / f"{NAME}_{version[1:]}_{goos}_{arch}.{extension}"
            files = {exe: (dest.read_bytes(), 0o755)}
            # Preserve relative documentation links in downloaded archives.
            for pattern in ("*.md", "docs/*.md", "docs/*.svg", "research/*.md", "research/*.json", "research/patches/*", "research/logs/*", "research/licenses/*"):
                for document in ROOT.glob(pattern):
                    files[document.relative_to(ROOT).as_posix()] = (document.read_bytes(), 0o644)
            files["LICENSE"] = ((ROOT / "LICENSE").read_bytes(), 0o644)
            archive(path, files, goos == "windows")
            manifest["artifacts"].append({"file":path.name,"os":goos,"arch":arch,"sha256":hashlib.sha256(path.read_bytes()).hexdigest(),"bytes":path.stat().st_size})
            print(path.name, flush=True)
    (output/"manifest.json").write_text(json.dumps(manifest,indent=2)+"\n")
    checked = [*(output/a["file"] for a in manifest["artifacts"]), output/"manifest.json"]
    (output/"checksums.txt").write_text("".join(f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n" for p in sorted(checked)))
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    build(args.version, args.output.resolve())


if __name__ == "__main__":
    main()
