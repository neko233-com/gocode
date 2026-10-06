"""Generate pinned, free installation channels from actual release artifacts."""
import argparse
import hashlib
import json
import pathlib
import re
import uuid


def digest(path):
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def generate(version, source, artifacts, output):
    if not re.fullmatch(r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)", version):
        raise ValueError("Invalid stable release version")
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("Release requires an immutable source commit")
    base = f"https://github.com/neko233-com/gocode/releases/download/v{version}/"
    names = [f"gocode-{version}-windows-amd64.msi", f"gocode-{version}-windows-amd64.zip"]
    names += [f"gocode-{version}-darwin-{arch}.{ext}" for arch in ("amd64", "arm64") for ext in ("zip", "tar.gz")]
    hashes = {name: digest(artifacts / name) for name in names}
    output.mkdir(parents=True, exist_ok=True)
    root = pathlib.Path(__file__).resolve().parent.parent
    replacements = {"@VERSION@": version, "@MSI_SHA256@": hashes[names[0]],
                    "@MAC_AMD_SHA256@": hashes[f"gocode-{version}-darwin-amd64.tar.gz"],
                    "@MAC_ARM_SHA256@": hashes[f"gocode-{version}-darwin-arm64.tar.gz"]}
    for template, target in (("install.ps1.in", "install.ps1"), ("install-macos.sh.in", "install-macos.sh")):
        text = (root / "distribution" / template).read_text(encoding="utf-8")
        for key, value in replacements.items():
            text = text.replace(key, value)
        (output / target).write_text(text, encoding="utf-8", newline="\n")
    (output / "SHA256SUMS").write_text("".join(f"{hashes[n]}  {n}\n" for n in names), encoding="utf-8")
    manifest_input = {"version": version, "source": source, "assets": [
        {"path": str((artifacts / f"gocode-{version}-{platform}-{arch}.zip").resolve()), "platform": f"{platform}/{arch}"}
        for platform, arch in (("windows", "amd64"), ("darwin", "amd64"), ("darwin", "arm64"))]}
    (output / "manifest-input.json").write_text(json.dumps(manifest_input, indent=2), encoding="utf-8")
    # Keep ProductCode in sync with the MSI packager's stable GUID byte algorithm.
    upgrade = "eed33be1-e465-4a97-a598-a29f430f9324"
    raw = hashlib.sha256(f"{upgrade}:product:{version}".encode()).digest()[:16]
    product = "{" + str(uuid.UUID(bytes_le=raw)).upper() + "}"
    winget = output / "winget"
    winget.mkdir(exist_ok=True)
    (winget / "neko233-com.gocode.yaml").write_text(f"""# yaml-language-server: $schema=https://aka.ms/winget-manifest.singleton.1.12.0.schema.json
PackageIdentifier: neko233-com.gocode
PackageVersion: {version}
PackageLocale: en-US
Publisher: neko233-com
PublisherUrl: https://github.com/neko233-com
PackageName: gocode
PackageUrl: https://github.com/neko233-com/gocode
License: MIT
LicenseUrl: https://github.com/neko233-com/gocode/blob/main/LICENSE
ShortDescription: Native Go desktop code editor powered by godesktop.
Moniker: gocode
Commands:
  - gocode
Installers:
  - Architecture: x64
    InstallerType: msi
    Scope: user
    InstallerUrl: {base}{names[0]}
    InstallerSha256: {hashes[names[0]].upper()}
    ProductCode: '{product}'
    InstallModes: [silent, silentWithProgress]
    UpgradeBehavior: install
ManifestType: singleton
ManifestVersion: 1.12.0
""", encoding="utf-8")
    bucket = output / "bucket"
    bucket.mkdir(exist_ok=True)
    scoop = {"version": version, "description": "Native Go desktop editor powered by godesktop", "homepage": "https://github.com/neko233-com/gocode", "license": "MIT",
             "architecture": {"64bit": {"url": base + names[1], "hash": hashes[names[1]]}},
             "bin": "gocode.exe", "shortcuts": [["gocode-launch.exe", "gocode"]],
             "checkver": {"github": "https://github.com/neko233-com/gocode"},
             "autoupdate": {"architecture": {"64bit": {"url": "https://github.com/neko233-com/gocode/releases/download/v$version/gocode-$version-windows-amd64.zip"}}}}
    (bucket / "gocode.json").write_text(json.dumps(scoop, indent=2), encoding="utf-8")
    casks = output / "Casks"
    casks.mkdir(exist_ok=True)
    (casks / "gocode.rb").write_text(f"""cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "{version}"
  sha256 arm: "{hashes[f'gocode-{version}-darwin-arm64.tar.gz']}",
         intel: "{hashes[f'gocode-{version}-darwin-amd64.tar.gz']}"
  url "https://github.com/neko233-com/gocode/releases/download/v#{{version}}/gocode-#{{version}}-darwin-#{{arch}}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{{appdir}}/gocode.app/Contents/MacOS/gocode"
end
""", encoding="utf-8")
    return hashes


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True)
    parser.add_argument("--source", required=True)
    parser.add_argument("--artifacts", type=pathlib.Path, required=True)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    generate(args.version, args.source, args.artifacts, args.output)
