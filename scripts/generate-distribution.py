"""Generate pinned, free installation channels from actual release artifacts."""
import argparse
import hashlib
import json
import pathlib
import re
import stat
import uuid

SUPPORTED_PLATFORMS = ("windows/amd64", "darwin/amd64", "darwin/arm64")


def release_platforms(platforms=None):
    selected = SUPPORTED_PLATFORMS if platforms is None else tuple(platforms)
    if not selected or len(selected) != len(set(selected)) or any(p not in SUPPORTED_PLATFORMS for p in selected):
        raise ValueError("Invalid or duplicate release platforms")
    # The existing macOS install template/cask describes both native CPUs.
    # Never generate an absent architecture's URL from a one-architecture build.
    if ("darwin/amd64" in selected) != ("darwin/arm64" in selected):
        raise ValueError("macOS channels require both genuinely built architectures")
    return tuple(p for p in SUPPORTED_PLATFORMS if p in selected)


def artifact_names(version, platforms=None):
    selected = release_platforms(platforms)
    names = []
    for platform in selected:
        system, arch = platform.split("/")
        extensions = ("msi", "zip") if system == "windows" else ("zip", "tar.gz")
        names.extend(f"gocode-{version}-{system}-{arch}.{ext}" for ext in extensions)
    return names


def digest(path):
    if not stat.S_ISREG(path.stat(follow_symlinks=False).st_mode):
        raise ValueError("Release artifact must be a regular file")
    with path.open("rb") as stream:
        return hashlib.file_digest(stream, "sha256").hexdigest()


def generate(version, source, artifacts, output, platforms=None):
    if not re.fullmatch(r"(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)\.(?:0|[1-9]\d*)", version):
        raise ValueError("Invalid stable release version")
    if not re.fullmatch(r"[0-9a-f]{40}", source):
        raise ValueError("Release requires an immutable source commit")
    selected = release_platforms(platforms)
    windows = "windows/amd64" in selected
    macos = "darwin/amd64" in selected
    if not macos and ((output / "install-macos.sh").exists() or (output / "Casks").exists()):
        raise ValueError("Use a fresh versioned output; existing macOS channels must be preserved")
    if not windows and any((output / p).exists() for p in ("install.ps1", "winget", "bucket")):
        raise ValueError("Use a fresh versioned output; existing Windows channels must be preserved")
    base = f"https://github.com/neko233-com/gocode/releases/download/v{version}/"
    names = artifact_names(version, selected)
    hashes = {name: digest(artifacts / name) for name in names}
    output.mkdir(parents=True, exist_ok=True)
    root = pathlib.Path(__file__).resolve().parent.parent
    replacements = {"@VERSION@": version}
    templates = []
    if windows:
        replacements["@MSI_SHA256@"] = hashes[f"gocode-{version}-windows-amd64.msi"]
        templates.append(("install.ps1.in", "install.ps1"))
    if macos:
        replacements["@MAC_AMD_SHA256@"] = hashes[f"gocode-{version}-darwin-amd64.tar.gz"]
        replacements["@MAC_ARM_SHA256@"] = hashes[f"gocode-{version}-darwin-arm64.tar.gz"]
        templates.append(("install-macos.sh.in", "install-macos.sh"))
    for template, target in templates:
        text = (root / "distribution" / template).read_text(encoding="utf-8")
        for key, value in replacements.items():
            text = text.replace(key, value)
        (output / target).write_text(text, encoding="utf-8", newline="\n")
    (output / "SHA256SUMS").write_text("".join(f"{hashes[n]}  {n}\n" for n in names), encoding="utf-8")
    manifest_input = {"version": version, "source": source, "assets": [
        {"path": str((artifacts / f"gocode-{version}-{platform.replace('/', '-')}.zip").resolve()), "platform": platform}
        for platform in selected]}
    (output / "manifest-input.json").write_text(json.dumps(manifest_input, indent=2), encoding="utf-8")
    (output / "release-platforms.json").write_text(json.dumps({"version": version, "source": source, "platforms": selected, "assets": hashes}, indent=2), encoding="utf-8")
    # Keep ProductCode in sync with the MSI packager's stable GUID byte algorithm.
    upgrade = "eed33be1-e465-4a97-a598-a29f430f9324"
    raw = hashlib.sha256(f"{upgrade}:product:{version}".encode()).digest()[:16]
    product = "{" + str(uuid.UUID(bytes_le=raw)).upper() + "}"
    if windows:
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
    if macos:
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
  depends_on macos: ">= :sequoia"
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
    parser.add_argument("--platforms", nargs="+", choices=SUPPORTED_PLATFORMS, default=None,
                        help="Only honestly built platforms; default requires all Windows and both macOS architectures")
    args = parser.parse_args()
    generate(args.version, args.source, args.artifacts, args.output, args.platforms)
