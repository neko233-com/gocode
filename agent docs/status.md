# Milestone ledger

## Verified foundation

- Public independent gocode submodule; godesktop dependency now v0.4.0.
- Native Dark Modern workbench, UTF-16 buffers, selections/undo/clipboard/CRLF,
  versioned VSIX edits, language providers and diagnostics.
- Official Copilot LSP/Go SDK real initialization and authenticated synthetic
  completion/chat; native GPU suggestion, Tab acceptance acknowledgement, stream,
  cancellation and retry passed race/strict-cgo with GOWORK=off on Windows.
- Native Windows owned-window Unicode mouse-drag replacement test passed.
- File-backed GiB browsing and huge single-line byte navigation passed real
  strict-cgo/race + process-owned GPU capture. Fixed 768 KiB index; measured
  cumulative index/read allocation 1,911,232 bytes for a 1,073,758,208-byte file.
- Official gopls v0.23.0 native formatting/hover/definition/unsaved completion/
  versioned diagnostics and clearing passed. Main source retained after edits.
- Upstream Code-OSS ICO/PNG/ICNS and MIT provenance retained; Windows icon/version
  resources and actual owned-window taskbar/window icon application implemented.
- `docs/vscode-parity.md` records substantial outstanding capabilities.

## Active objective

1. File-backed GB browsing, byte/line navigation and measured bounded memory.
2. Generic LSP/gopls lifecycle, diagnostics, formatting/hover/definition.
3. Latest VS Code visual reference, icons/resource/bundle metadata and native
   resize/drag/DPI/long-content visual regression.
4. Free MSI/portable/CLI/winget-compatible manifests and macOS bundles/channels.
5. Reachability-aware, configurable, integrity-checked updates and rollback.
6. Install latest validated gocode on this Windows computer and verify launch,
   command, shortcuts/icons/update settings and installed-app behavior.

Distribution/updates are being validated below. Large-file editing, language
crash restart and broader parity remain pending. Record immutable commits/runs
and actual deployment evidence when promoting each milestone.
Do not claim the active production/full-parity goal is complete.

Public foundation source `633e754c0c6a695d869317785562678d5f14d756`, CI
`37449327497`: macOS Intel/ARM and Ubuntu passed; both Windows passed native,
editor, Copilot protocol and GiB checks but timed out at gopls navigation.
An actual Windows 8.3 alias reproduced reopening an unsaved document from disk.
The regression fails the old code and passes the canonical-path correction in
three race runs. Public promotion waits for the corrected source's full CI.

Free distribution uses versioned launcher/layout and Windows Installer COM/makecab.

Corrected source `cfcd3513aedc4ec50ae19625fbd2f04446039abe` passed all five jobs
in CI `37451395121` and is the immutable v0.4.0 source release. The parent gitlink
was advanced by godesktop commit `e1af1980d2bcf09391f374500442a66426b2cece`.

Distribution prototype (uncommitted, disposable payloads only) now passed real
per-user MSI install, stable command/native launch, Start Menu/desktop icon targets,
custom-root remembered uninstall and PATH removal. A 0.4.0 -> 0.4.1 synthetic MSI
upgrade retained the root, superseded a stale current.json pointer, launched the
new native payload and uninstalled its registered files. No final user install
exists. Initial custom-root uninstall revealed missing root persistence; RegLocator/
AppSearch with a property restore fixed it. Generated reports/cabinets stay ignored.

The automated disposable MSI acceptance now passes per-user installation, icon/
shortcut targets, corrupted-cabinet rollback with the old product still registered,
normal upgrade, downgrade rejection, native launch, owned pointer cleanup, workspace
preservation and PATH removal. Early removal of the old app revealed a Windows
LUA rollback registration problem; scheduling InstallExecute before old removal
fixed it. The failed prototype was recovered using Windows Installer, including
its advertised registration, files and PATH; no manual registry/delete workaround.

Updater race tests pass real compiled-executable health checks, signed metadata,
bad-route fallback, SHA256 mismatch, ZIP traversal, cancellation, exclusive process
locks, next-launch activation and rollback. Fixed 512 MiB archive/expansion bounds.
An MSI baseline supersedes stale update pointers and remains the rollback target.
Real Windows mouse/keyboard settings persistence and owned GPU capture passed:
`.cache/updates-native-settings.png`. Automatic/manual routing and toggle were saved
without changing the active document. No final installed-app live release update yet.

`gocode -install-copilot` installs the embedded npm lockfile with no lifecycle scripts
into the per-user versioned tools directory. This machine's managed runtime passed
real authenticated LSP initialization and SDK connection without an AI prompt.

Remaining promotion gates: new exact-source CI (including macOS packages and MSI),
published immutable installers/signed manifest/custom channels, real published-byte
update/rollback and final local installation. Public registry acceptance is separate.
Never publish dirty/synthetic prototype packages or private signing keys.

Distribution source `8bccd29b23cd181e074f1c8cdefc6be1ea2e8f59` passed all five
jobs in CI `37459941417` and publication workflow `37461001171`. Real published
Windows ZIP update acceptance then exposed PowerShell 5 backslash ZIP paths.
The strict updater rejected it and kept the immutable v0.4.0 baseline selected.
v0.5.0 was held as a prerelease; no final user install occurred. v0.5.1 uses
canonical ZIP paths and a new package gate that exercises production Stage and
real native Probe on the exact generated Windows/macOS archives before publishing.
