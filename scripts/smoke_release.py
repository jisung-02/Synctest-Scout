#!/usr/bin/env python3
"""Verify all checksums and execute the archive for the current host."""
import argparse
import hashlib
import json
from pathlib import Path
import platform
import subprocess
import tarfile
import tempfile
import zipfile


def verify(directory):
    checks = {}
    for line in (directory / "checksums.txt").read_text().splitlines():
        digest, name = line.split("  ", 1)
        if Path(name).name != name or name in checks:
            raise ValueError("invalid or duplicate checksum path")
        if hashlib.sha256((directory/name).read_bytes()).hexdigest() != digest:
            raise ValueError(f"checksum mismatch: {name}")
        checks[name] = digest
    manifest = json.loads((directory/"manifest.json").read_text())
    expected = {a["file"] for a in manifest["artifacts"]} | {"manifest.json"}
    if set(checks) != expected or len(manifest["artifacts"]) != 6:
        raise ValueError("incomplete artifact set")
    if {(a["os"],a["arch"]) for a in manifest["artifacts"]} != {(o,a) for o in ("linux","darwin","windows") for a in ("amd64","arm64")}:
        raise ValueError("incomplete platform matrix")
    for a in manifest["artifacts"]:
        if a["sha256"] != checks[a["file"]] or a["bytes"] != (directory/a["file"]).stat().st_size:
            raise ValueError("manifest checksum or size mismatch")
    goos = {"Darwin":"darwin","Linux":"linux","Windows":"windows"}[platform.system()]
    arch = "arm64" if platform.machine().lower() in ("arm64","aarch64") else "amd64"
    artifact = next(a for a in manifest["artifacts"] if (a["os"],a["arch"]) == (goos,arch))
    exe = "synctest-scout" + (".exe" if goos == "windows" else "")
    path = directory / artifact["file"]
    if goos == "windows":
        with zipfile.ZipFile(path) as zf: data = zf.read(exe)
    else:
        with tarfile.open(path) as tf: data = tf.extractfile(exe).read()
    with tempfile.TemporaryDirectory(prefix="scout-smoke-") as work:
        binary = Path(work) / exe
        binary.write_bytes(data); binary.chmod(0o755)
        out = subprocess.check_output([str(binary),"version"],text=True)
        if f"synctest-scout {manifest['version']} " not in out:
            raise ValueError("embedded version mismatch")
        subprocess.run([str(binary),"help"],check=True,stdout=subprocess.DEVNULL)
    print(f"Verified 6 archives, checksums, manifest, and {goos}/{arch} CLI startup.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("directory",type=Path)
    verify(parser.parse_args().directory)
