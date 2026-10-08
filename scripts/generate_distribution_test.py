import json
import pathlib
import tempfile
import unittest
from importlib.util import module_from_spec, spec_from_file_location

spec = spec_from_file_location("distribution", pathlib.Path(__file__).with_name("generate-distribution.py"))
distribution = module_from_spec(spec)
spec.loader.exec_module(distribution)


class DistributionTest(unittest.TestCase):
    def test_actual_bytes_pin_every_channel_and_stable_msi_identity(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            for name in ["gocode-0.4.1-windows-amd64.msi", "gocode-0.4.1-windows-amd64.zip"] + [
                f"gocode-0.4.1-darwin-{arch}.{ext}" for arch in ("amd64", "arm64") for ext in ("zip", "tar.gz")
            ]:
                (artifacts / name).write_bytes(name.encode())
            output = root / "channels"
            hashes = distribution.generate("0.4.1", "a" * 40, artifacts, output)
            installer = (output / "install.ps1").read_text()
            self.assertIn(hashes["gocode-0.4.1-windows-amd64.msi"], installer)
            self.assertNotIn("@VERSION@", installer)
            winget = (output / "winget/neko233-com.gocode.yaml").read_text()
            # Same bytes_le GUID as the actual COM MSI upgrade acceptance package.
            self.assertIn("InstallerSha256: " + hashes["gocode-0.4.1-windows-amd64.msi"].upper(), winget)
            self.assertIn("gocode-0.4.1-windows-amd64.zip", (output / "bucket/gocode.json").read_text())
            self.assertIn(hashes["gocode-0.4.1-darwin-arm64.tar.gz"], (output / "Casks/gocode.rb").read_text())
            self.assertIn('depends_on macos: ">= :sequoia"', (output / "Casks/gocode.rb").read_text())

    def test_windows_only_does_not_relabel_or_advertise_old_macos(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            artifacts = root / "artifacts"
            artifacts.mkdir()
            for name in distribution.artifact_names("0.24.0", ["windows/amd64"]):
                (artifacts / name).write_bytes(("new Windows:" + name).encode())
            old_mac = artifacts / "gocode-0.23.0-darwin-arm64.tar.gz"
            old_mac.write_bytes(b"old genuine Mac bytes must not be republished")
            output = root / "channels"
            hashes = distribution.generate("0.24.0", "b" * 40, artifacts, output, ["windows/amd64"])
            self.assertEqual(set(hashes), {"gocode-0.24.0-windows-amd64.zip", "gocode-0.24.0-windows-amd64.msi"})
            self.assertFalse((output / "install-macos.sh").exists())
            self.assertFalse((output / "Casks").exists())
            self.assertEqual(old_mac.read_bytes(), b"old genuine Mac bytes must not be republished")
            manifest = json.loads((output / "manifest-input.json").read_text())
            self.assertEqual([asset["platform"] for asset in manifest["assets"]], ["windows/amd64"])
            self.assertNotIn("darwin", (output / "SHA256SUMS").read_text())
            metadata = json.loads((output / "release-platforms.json").read_text())
            self.assertEqual(metadata["platforms"], ["windows/amd64"])
            self.assertEqual(metadata["assets"], hashes)
            # Fixed output is idempotent, rather than adding per-run garbage.
            original = {p.relative_to(output): p.read_bytes() for p in output.rglob("*") if p.is_file()}
            distribution.generate("0.24.0", "b" * 40, artifacts, output, ["windows/amd64"])
            self.assertEqual(original, {p.relative_to(output): p.read_bytes() for p in output.rglob("*") if p.is_file()})

    def test_windows_only_refuses_to_modify_existing_mac_channels(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            (root / "install-macos.sh").write_bytes(b"verified prior Mac installer")
            with self.assertRaises(ValueError):
                distribution.generate("0.24.0", "b" * 40, root, root, ["windows/amd64"])
            self.assertEqual((root / "install-macos.sh").read_bytes(), b"verified prior Mac installer")

    def test_platform_subset_validation_before_output(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            output = root / "out"
            for selected in ([], ["linux/amd64"], ["windows/amd64", "windows/amd64"], ["darwin/amd64"], ["darwin/arm64"]):
                with self.subTest(platforms=selected), self.assertRaises(ValueError):
                    distribution.generate("0.24.0", "b" * 40, root, output, selected)
                self.assertFalse(output.exists())

    def test_rejects_untrusted_version_source_and_missing_platform(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            for version, source in (("../1", "a" * 40), ("01.0.0", "a" * 40), ("0.5.0", "main")):
                with self.assertRaises(ValueError):
                    distribution.generate(version, source, root, root / "out")
            with self.assertRaises(FileNotFoundError):
                distribution.generate("0.5.0", "a" * 40, root, root / "out")


if __name__ == "__main__":
    unittest.main()
