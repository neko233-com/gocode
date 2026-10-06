# Free distribution contract

Maintained channels: Windows unsigned MSI/portable ZIP and pinned PowerShell install;
macOS Intel/ARM .app/tar.gz, updater ZIP and shell install; custom winget manifests,
Scoop bucket and Homebrew cask. Generated manifests are not public registry acceptance.

Use Windows Installer COM/database APIs and built-in cabinet tooling or another
verified fee-free path; do not depend on paid installer maintenance/certificates.
OS code signing/notarization is optional and outside the required free path.

Artifacts need deterministic names, source/version metadata and publisher-controlled
hash/manifest verification. Mirror routing must not weaken executable integrity.
Direct GitHub reachability, automatic fallback and explicit endpoint overrides
need reproducible success/failure tests. Updates must stage safely, preserve user
data and allow rollback when extraction/startup fails.

`VERSION` owns the package version; immutable source SHA is embedded in the payload.
`scripts/test-msi.ps1` uses isolated names/UpgradeCodes/cache roots; never the real
Programs install. It covers corrupt-package transaction rollback, upgrade, native
launch, icon shortcuts, remembered roots and cleanup. Old products are removed only
after InstallExecute successfully copies the new files. Cleanup runs only for an
installed maintenance component and never during a major upgrade.

The release workflow requires a successful exact-main-source CI run and reuses those
actual tested artifacts. It signs update-manifest.json with the Actions publisher
secret, refuses existing releases, publishes artifacts, then maintains distribution
metadata, bucket/ and Casks/. Private keys stay outside Git. The signature key is
free Ed25519 integrity metadata, unrelated to OS certificate/notarization fees.

Updater layout: baseline base.json, signed versions/V payloads, current.json and
previous.json pointers. Health verifies version/platform/source before selection.
No running editor is replaced or closed. Updates select the next launch. The updater
rejects unknown/symlink/traversal files and preserves unknown user content. SHA256
must succeed independently of route speed; manual mode uses exactly the chosen
HTTPS prefix. Automatic routes currently include ghfast.top and gh-proxy.com and
also probe direct GitHub. Default auto checks at launch and every 24 hours.

CLI: -update-check, -update, -update-rollback; -configure-updates -update-mode
auto/direct/mirror -update-mirror https://prefix/ -updates-auto true/false. Per-user
updates.json contains route settings. Source builds can save preferences but need
a versioned package for activation. Interrupted/failed staging leaves selection
unchanged; existing version directories are never overwritten. Retrying a failed
health check of an already staged version currently requires a newer release.

Icons are sourced from the referenced microsoft/vscode Code-OSS repository with
its exact resource revision/license retained. Keep installer, shortcut, executable
and macOS bundle icon metadata synchronized.

The local installation target is the user's per-user Programs directory, with a
gocode command and Start Menu/desktop access; verify real installed launch/update
settings. Never install test payloads over an unrelated application.
