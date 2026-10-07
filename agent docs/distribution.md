# Free distribution contract

## Public and installed v0.23.0 (2026-10-08)

Immutable source 47623823dd88fbb45422d096b545631be7e6b77b passes all five
jobs in [CI 37678679723](https://github.com/neko233-com/gocode/actions/runs/37678679723)
attempt 1. [Publication 37682493872](https://github.com/neko233-com/gocode/actions/runs/37682493872)
reuses those tested packages and tags the same source. Seven generated metadata
files at 05404af95162921c89617d314b7ff6edb7feb60e pass both Python policies;
published Ed25519 manifest/source/asset digests verify. Windows ZIP is
19,129,156 bytes, SHA256
05cff9bb4179f4316c28bfa8cd76f9eb28bd37e5e22cda02a2332ed86e4fe9e3;
MSI is 16,113,664 bytes, SHA256
8ed5cf7f46af4f939cc4f8f619f4445f89e825b48ab2a1655723b2614f95b3fe.

.cache/live-release-check/current-v023.log exits 0 after signed automatic direct
GitHub full ZIP, separate actual manual direct and ghfast.top full bodies, all
25 ordered console/GUI acceptance gates and real v0.4.0/cfcd351 GUI/VSIX rollback.
The owned .cache/live-update-050/installed/native-acceptance.json pins exact
source, executable hashes and ordered native gates. The earlier v0.22.0 direct
failures below remain history; current success does not erase failed downloads.

The user's original 0.5.1 MSI/stable launcher selects v0.23.0/exact source via
direct GitHub, GitHubReachable=true. Installed validation exits 0 for all nine
native fixture modes plus actual console/GUI Auto Save/minimized/version gates,
real 1,073,741,824-byte shared split/identical source SHA, native UI/Settings/icons,
keymaps/VSIX/terminal/Git and authenticated official SDK/LSP without a prompt.
Both installed minimized JSONs prove IsIconic, save/didSave 1/1 then 2/2,
dirty=false, View/GPU 2->2 before restoration. User update configuration/PATH
hashes, absent keyboard/autosave settings and both stable shortcut bytes,
targets/icons/arguments/working directories remain intact. Exact hashes and
.cache/installed-{update,validation,copilot}-v023.log evidence are in status.md.

## Historical v0.22.0 promotion

v0.22.0/source 0b5966171e5e183e74a9fecbeccce391cd1cf229,
five-platform source CI 37651719075 and publication 37654475129 reusing tested
artifacts. Seven metadata files at 787be53d2662c4c8d85417371474636190604b03 pass
both Python policies. Actual signed automatic/manual gh-proxy.com full ZIP bodies,
released native gates and genuine v0.4.0 GUI/VSIX rollback pass. Direct GitHub
archive body checks fail twice on this network; rejected incomplete downloads
do not count as direct-route proof. Actual user update uses gh-proxy.com.
User selected payload is 0.22.0; stable MSI/launcher baseline remains 0.5.1.
Installed native File/keymaps/groups/real VSIX, actual GiB split, shell/VSIX
terminals/Git and official SDK/LSP health without a prompt pass. Auto=true/mode=auto,
settings/PATH hashes and both stable shortcut targets/icons match baseline.
Exact hashes and source references are in status.md and distribution/SHA256SUMS.

## Distribution contract

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

Built-in sample VSIX payloads live under the extension root's hidden
.gocode-bundled/<version>/ store. They never upgrade the shared user VSIX store;
rollback executables retain their expected old builtin installation there. The
native workbench selects its own embedded builtin and preserves other user VSIXs.
Acceptance must launch both the new and real prior release with the same user
extension root; a -version/health-only rollback check does not prove GUI startup.

The local installation target is the user's per-user Programs directory, with a
gocode command and Start Menu/desktop access; verify real installed launch/update
settings. Never install test payloads over an unrelated application.

Historical v0.14.0 install: source de2a98f payload selected by the original
v0.5.1 MSI stable launcher through its actual -update command and signed/hashed
public ZIP. The baseline installer and launcher file versions remain 0.5.1 until
an MSI upgrade; -version reports the selected 0.14.0 source. The embedded official
ConPTY runtime, actual native highlighted terminal, icons/GUI/settings, native
search/replacement, tabs and measured-caption gates passed after installation.
Released-byte gopls recovery and external file-watch/reload/conflict/VSIX controls
also pass; these remain separate from the installed GUI/SDK checks.
Auto=true/mode=auto is retained. The owned update
root rolled back to the original v0.4.0 and rendered its GUI with shared extensions.
v0.14.0 automatic direct GitHub routing and a separate full direct download succeeded;
manual ghfast.top also passed signed SHA256. The earlier v0.6.0 direct timeout
remains historical evidence; a range probe alone never establishes body success.
The unmodified fsnotify BSD notice is appended to existing packaged LICENSE.txt.
An added root FSNOTIFY-LICENSE.txt was rejected by the real legacy-layout package
gate and was not published. Keep this old-client layout compatibility check.
