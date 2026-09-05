import io
from pathlib import Path
import tarfile
import tempfile
import unittest
import zipfile

from coverage_report import gate, read_profile, write_report
from release import archive, validate_version


class CoverageTests(unittest.TestCase):
    def test_weighted_and_duplicate_blocks(self):
        report = read_profile("mode: atomic\np/a.go:1.1,2.2 9 0\np/a.go:1.1,2.2 9 3\np/b.go:1.1,2.2 1 0\n")
        self.assertEqual(report["total"], {"statements":10,"covered":9,"percent":90.0})
        self.assertFalse(gate(report,90,90))
        self.assertTrue(gate(report,90.01,90))

    def test_package_gate_is_independent(self):
        report = read_profile("mode: count\nlarge/a.go:1.1,2.2 999 1\nsmall/a.go:1.1,2.2 1 0\n")
        self.assertEqual(len(gate(report,95,90)),1)
        self.assertIn("small",gate(report,95,90)[0])

    def test_bad_profiles_fail_closed(self):
        for profile in ["", "mode: bad", "mode: atomic", "mode: set\ngarbage", "mode: set\na.go:0.1,2.2 1 1", "mode: set\na.go:3.1,2.2 1 1", "mode: set\na.go:1.1,2.2 1 1\na.go:1.1,2.2 2 1"]:
            with self.subTest(profile=profile), self.assertRaises(ValueError): read_profile(profile)

    def test_bilingual_reports_share_measurements(self):
        report = read_profile("mode: set\np/a.go:1.1,2.2 1 1")
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            write_report(report, output, 95, 90)
            for filename in ("summary.md", "summary.ko.md"):
                content = (output / filename).read_text(encoding="utf-8")
                for expected in ("100.00%", "PASS", "[English](summary.md)", "[한국어](summary.ko.md)"):
                    self.assertIn(expected, content)

    def test_invalid_threshold(self):
        report=read_profile("mode: set\np/a.go:1.1,2.2 1 1")
        with self.assertRaises(ValueError): gate(report,101,90)


class ReleaseTests(unittest.TestCase):
    def test_tag_validation(self):
        for v in ["v0.1.0","v1.0.0-rc.1","v0.0.0-dev"]: validate_version(v)
        for v in ["1.0.0","v01.0.0","../escape","v1.0.0;echo bad","v1.0.0-01","v1.2.3\n"]:
            with self.subTest(v=v), self.assertRaises(ValueError): validate_version(v)

    def test_archive_contents_and_reproducibility(self):
        files={"synctest-scout":(b"binary",0o755),"README.md":(b"docs",0o644)}
        with tempfile.TemporaryDirectory() as d:
            for windows in (True,False):
                a,b=Path(d)/"first",Path(d)/"second"
                archive(a,files,windows);archive(b,files,windows)
                self.assertEqual(a.read_bytes(),b.read_bytes())
                if windows:
                    with zipfile.ZipFile(a) as zf:
                        self.assertEqual(zf.read("synctest-scout"),b"binary")
                        self.assertEqual(zf.getinfo("synctest-scout").external_attr>>16 & 0o777,0o755)
                else:
                    with tarfile.open(a) as tf:
                        self.assertEqual(tf.extractfile("synctest-scout").read(),b"binary")
                        self.assertEqual(tf.getmember("synctest-scout").mode,0o755)
                        self.assertEqual(tf.getnames(),sorted(files))


if __name__ == "__main__": unittest.main()
