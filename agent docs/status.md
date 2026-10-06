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

No installer/updater/local installation is verified yet. Large-file editing,
language crash restart and broader parity remain pending. Source/CI integration
for these milestones is in progress; record the immutable commit/run on promotion.
Do not claim the active production/full-parity goal is complete.

Public foundation source `633e754c0c6a695d869317785562678d5f14d756`, CI
`37449327497`: macOS Intel/ARM and Ubuntu passed; both Windows passed native,
editor, Copilot protocol and GiB checks but timed out at gopls navigation.
An actual Windows 8.3 alias reproduced reopening an unsaved document from disk.
The regression fails the old code and passes the canonical-path correction in
three race runs. Public promotion waits for the corrected source's full CI.

Free distribution is now in development: versioned launcher/layout and Windows
Installer COM + makecab build a real MSI. Prototype payload command/version works;
install/uninstall/upgrade and updater/local deployment are not yet verified.
