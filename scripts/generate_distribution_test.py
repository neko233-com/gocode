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
