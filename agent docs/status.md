# Milestone ledger

## Terminal v0.7.0 promotion (2026-10-07)

Real native ConPTY/PTY terminal and shell input highlighting are published at
immutable source `8a0fa07670a3018b42ab86eb2dd45e814099c7c7`. All five source CI
jobs passed in `37498949904`: Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu.
Publication `37500332017` reused those exact tested Windows MSI/ZIP and Mac
bundles, signed the update metadata and created v0.7.0. Generated channel metadata
is `cacee0d8b5228f63eb37748269721a266530a279`; all five jobs also passed at
that exact metadata commit in manual CI `37500982783`, including MSI lifecycle,
real GiB native browsing, gopls/Copilot and Mac bundle packaging. This evidence
commit changes documentation only; application/package source gates are retained.
Local Windows strict-cgo/race and owned GPU acceptance pass,
including PowerShell 7/5.1, truecolor, Unicode, resize, alternate screen, Ctrl+C,
final exit output, bounded flood/scrollback and owned descendant cleanup. Native
keyboard focus leaves the editor unchanged. See terminal.md for bounds/provenance
and explicit gaps. Independently downloaded source-CI Windows 2022 GPU captures
were visually inspected: distinct pre-Enter command/string colors, readable
truecolor output, native panel background and actual resized shell columns.

Actual signed public v0.7.0 ZIP acceptance passed automatic direct-GitHub download,
native large-file/four-close/VSIX-save/terminal checks and separate full direct
and manual ghfast.top downloads, each verified against signed SHA256
`11bed5aa26e66c76b5fb7eb1db356a15d1f033af2161014c9e27b2dddecc0087`.
The owned root then rolled back to v0.4.0/source cfcd3513 and rendered that real
prior native GUI with the same extension store. Its original baseline is intact.
User installation is promoted and checked separately below.

The user's actual stable v0.5.1 launcher updated the existing v0.6.0 selection
to v0.7.0/source 8a0fa076 via its own -update command using the complete direct
GitHub ZIP. Root remains `C:\Users\14170\AppData\Local\Programs\gocode`;
MSI/launcher baseline stays 0.5.1, actual payload is 0.7.0. Installed -version,
-terminal-runtime-check, real highlighted -terminal-smoke and native GUI/icon/
Settings/source-preservation checks passed. Owned Settings/input/output captures
under .cache/installed-acceptance were visually inspected: Automatic true,
Automatic route, Up to date (0.7.0), gopls ready, distinct input syntax and output
truecolor with correct resized columns. Installed real gopls formatting/hover/
definition/completion/versioned diagnostic clearing passed. -update-check returns
0.7.0/source 8a0fa076; auto=true/mode=auto is retained. Official installed Copilot
check reports authenticated/lspInitialized/sdkConnected=true,
networkPromptSent=false. No new AI prompt was sent. These checks do not establish
full official Copilot VSIX or all VS Code API/function/UI compatibility.
Desktop and Start Menu shortcuts still target the stable gocode-launch.exe;
their installer-owned icon files and normalized per-user PATH remain valid.

### Development negative controls before promotion

Do not promote source 32a4d5a/CI 37490630980 or 3121950/CI 37492140590.
Windows runners needed a raw/VT-input full-screen fixture and a longer bounded
flood deadline under race+coverage. The corrected fixture passed the next runner
tests. Mac real zsh input colors/native resize/Ctrl+C reached exit, but `exit`
inherited interruption status 130; acceptance now explicitly requests `exit 0`.
Small runner fonts and asynchronous GPU completion exposed the exact-RGB/early
snapshot predicate. The gate now waits for actual colored ink in the process-owned
terminal grid and accounts for foreground/background glyph coverage. It retains
both real VT-color and native pixel checks. The later promoted CI gates these fixes.

Bounded IO from source 6f9d398 / CI 37495036441 confirmed Windows 2022 did not
forward either DEC 1049 switch. No primary command was sent because the initial
alternate-state condition never became true. Earlier interpretation as an input
stall/raw-mode problem was incomplete. The same source's Mac ARM/Intel, Ubuntu
and Windows 2025 terminal checks passed. Do not publish it as cross-platform proof.

The correction uses Microsoft's free pinned ConPTY 1.25.260930003, verified native
DLL/helper hashes and embedded executable resources, with no system replacement.
Local full Windows race/strict-cgo/native console+GUI/editor/save/close/large-file
and terminal checks pass. Fresh owned cache extraction and native terminal passed
with an unreachable HTTPS proxy, proving offline resources work. Exact-source
Windows 2022/2025, Mac Intel/ARM, Ubuntu and package CI subsequently passed above.

## Verified foundation

- Public independent gocode submodule; public godesktop dependency now v0.5.2.
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
without changing the active document. Final published/install evidence is below.

`gocode -install-copilot` installs the embedded npm lockfile with no lifecycle scripts
into the per-user versioned tools directory. This machine's managed runtime passed
real authenticated LSP initialization and SDK connection without an AI prompt.

Prototype promotion gates were: new exact-source CI (including macOS packages and MSI),
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

v0.5.1 source `e7f2d69011040b6c458cb54fc73f4cff695015b6` passed all five jobs
in CI `37461737830` and artifact publication `37462680852`. Windows PowerShell 5
also drops a separate empty optional mirror argument; the maintained bootstrap
now passes `-update-mirror=` and MSI acceptance executes that actual default CLI
installer with an isolated user config. Released app/installer/ZIP bytes remain
immutable; a corrected `install-windows.ps1` helper is published separately.

Final local install (2026-10-06): corrected PowerShell 5 command installer downloaded
the actual v0.5.1 public MSI over direct GitHub, verified its pinned SHA256 and
installed to `C:\Users\14170\AppData\Local\Programs\gocode`. Version/platform/source
match `0.5.1 windows/amd64 e7f2d69011040b6c458cb54fc73f4cff695015b6`.
Start Menu/desktop shortcuts target the stable GUI launcher with icon metadata;
user PATH includes the install root, launcher VERSIONINFO reports 0.5.1. Both native
icon handles, a real owned-GPU Settings frame and source preservation passed.
Screenshot `.cache/installed-acceptance/updates-native.png` was visually inspected:
Automatic true, Automatic route, GitHub, Up to date (0.5.1), gopls ready.

Installed Copilot runtime/gopls commands succeeded; official SDK/LSP handshake
reports authenticated=true without a prompt. Installed real gopls native formatting,
hover/definition/completion/diagnostic clearing also passed with owned GPU capture.
Live signed public ZIP update from the original v0.4.0 source binary to v0.5.1
passed extraction/health/launcher/native large-file rendering. Direct and manual
ghfast downloads passed signed-asset SHA256, and rollback launched the exact
v0.4.0 source again. These are owned acceptance roots, separate from the final
MSI install. Installed -update-check and -update verify current GitHub metadata
and report up-to-date. No all-feature or full production parity claim.
Unsaved-window close protection is next framework work.

Next editor source (v0.6.0 in development) now depends on public godesktop v0.5.0,
immutable framework source `eff4097c302ed52db98b144352b183ce058120ac`, with all
five jobs green in framework CI `37464670639`. Native OS/custom titlebar closes
defer through its CloseRequested guard. Dirty window/tab confirmation offers Save,
Don't Save and Cancel; Cancel preserves both buffer/version and editing focus.
Close saves use immutable snapshots in a worker and acknowledge only the matching
document version. External disk changes cause save rejection, preserving the
unsaved buffer and other program's source. Disk hash reads/writes are capped at the
8 MiB editable policy with fixed 128 KiB blocks and cancellation checks.

GOWORK=off local race runs passed close/cancel/immutable snapshot/external-change
tests. Real Windows native save, discard, cancel/focus restoration and rejected
external-change close all passed. `.cache/close-native/save.png` was inspected:
centered native confirmation, dimmed workbench, readable buttons and dirty tab.
Full new-source CI, cross-platform workbench close smoke, publication/update and
local deployment of this next editor revision remain pending. The installed app
is still the verified v0.5.1 release. Existing normal Ctrl+S/VSIX saves still use
the synchronous save adapter; move that adapter to acknowledged worker I/O next.

The first new-source CI `37468303977` at `43847119c35831ccea67a9ee8848d56473926078`
passed Mac Intel/ARM and Ubuntu but failed both Windows discard/conflict clicks.
Downloaded owned-window GPU evidence showed a smaller runner viewport. Local
150% DPI additionally exposed physical-client pixels being passed to a DIP pointer
helper. The corrected test converts both actual size and DPI before clicking.
Never promote this failed source or treat a local-only pass as all-platform proof.

Next working revision routes ordinary Ctrl/Cmd+S, palette, VSIX and close saves
through the same bounded actor; production has no synchronous UI disk-save path.
Frozen-path/snapshot writes, serial disk-hash rebase, exact-version acknowledgement,
cancelled/closed requests, 32-job overflow and a discard barrier have real-disk race
tests. Save size is checked before joining the snapshot text. A matching native
saveId coordinates with the framework's next bridge revision; actual VSIX save
smoke verifies saved text/dirty state and one success event. The pure model test
adapter is test-only. Cross-platform native -close-smoke covers four disposable
workspace modes. Public dependency promotion, new exact-source CI, release and
installed update are still pending; installed v0.5.1 is preserved.

Public godesktop v0.5.1 is now pinned (no replace/workspace override), source
`fd5ee95f0fd5df73e8076971abb282d616bd7725`, all five jobs green in framework
CI `37471310727`. Local GOWORK=off child race tests passed with all packages and
three randomized repetitions, including actual OS-close/mouse cases. Native
four-mode workbench close acceptance passed; final exact-dependency VSIX smoke,
conflict screenshot, new child CI and installation promotion follow next.

Exact public v0.5.1 dependency local Windows validation now passed: strict-cgo
amd64 race/vet, console+GUI native smoke, real installed VSIX Document.save with
one event, CRLF disk bytes, native undo/redo/completion, all four native close
modes and large-file browsing. Actual OS-close save/discard/cancel/conflict tests
also passed three repeated randomized package runs. Owned GPU capture
`.cache/close-native/smoke-external-conflict.png` was visually inspected: readable
wrapped conflict reason, unchanged dirty tab and working Save/Don't Save/Cancel.
Actionlint/ShellCheck and diff checks passed. New exact-source CI/promotion is next.

Limits: close confirmation currently disables decisions while its save/barrier is
active; request cancellation is supported at the save API but an interactive
cancel-in-progress control is still needed. A kernel-blocked write/fsync can outlive
the worker context; explicit forced shutdown has a bounded three-second wait.
Regular open/tree I/O, filesystem watchers/transactional conflicts and the broader
editor/services/function coverage ledger remain unfinished.

Promotion audit found a builtin rollback regression in `23ac180...`: installing
the sample VSIX v0.4.0 into the shared user root made the real v0.5.1 executable
fail startup with "extension version already installed". An owned local negative
control ran the actual installed old release, new source, then old release again
and reproduced that failure. This source must not be promoted even if its CI passes.
Builtin payloads are now stored in hidden .gocode-bundled/<version>/ roots; the
shared old installation stays untouched. Cross-platform unit checks verify
idempotent new selection and byte-preserved old manifests. Real old-release GUI,
new managed VSIX save/GUI, then old-release GUI rollback now pass using one owned
extension root. Live signed release acceptance must also render the prior release
after rollback, not merely print its version.

CI `37473079133` did pass all five jobs at `23ac180ed0a36cadd9bcd4f729e092dc71040559`,
including four Mac close modes, Windows DPI-derived discard clicks, GiB browsing,
official gopls/Copilot and MSI/package gates. The separate actual-release rollback
negative control above still prevented promotion. The corrected builtin isolation
passed local real old/new/old native startup plus acknowledged VSIX save, full
strict-cgo race tests, vet, workflow lint and diff checks. Publish only after the
next exact-source CI verifies this final correction.

v0.6.0 is now published and installed (2026-10-06): immutable application source
`242a1a917b189ded070ac00c7ac75f8eba9b6296`, all five jobs green in source CI
`37474545017`, publication workflow `37475908095`. Actual tested Windows MSI/ZIP
and Mac Intel/ARM bundles were reused; signed update metadata and maintained
PowerShell/macOS installers, winget/Scoop/Homebrew channels were generated at
metadata commit `2ff7885cd40be9b5a4a94e29d51598d760674f18`. Registry acceptance
remains separate. No immutable tag or released artifact was overwritten.

Real signed public ZIP acceptance staged/health-checked/launched v0.6.0 from the
original v0.4.0 binary, ran native large-file/four-close/VSIX-save checks, then
rolled back to source `cfcd3513aedc4ec50ae19625fbd2f04446039abe` and actually
rendered that prior app using the same extension store. Automatic routing chose
gh-proxy.com despite GitHub probe reachability=true. A separate full direct
GitHub download timed out/failed size verification; it did not change selection.
Manual gh-proxy download independently passed signed SHA256. No direct-download
success claim for this run. The owned test root was rolled back, separate from
the user's final installation.

The real installed v0.5.1 launcher then applied v0.6.0 via its own -update command,
verified public metadata/archive/native health and selected exact source 242a1a9.
Install root remains `C:\Users\14170\AppData\Local\Programs\gocode`; MSI baseline
and stable launchers remain 0.5.1, current payload is 0.6.0. Auto=true/mode=auto,
installed -update-check returns 0.6.0/source 242a1a9. Start Menu/desktop/PATH remain
valid. Installed GUI/icon/settings/source-preservation, actual close save, VSIX
Document.save/CRLF/undo/redo/completion and real gopls native checks passed.
Installed official Copilot check reports authenticated=true, lspInitialized=true,
sdkConnected=true, networkPromptSent=false. No new AI prompt was sent by this check.
Owned `.cache/installed-acceptance/updates-native.png` was visually inspected:
Automatic true, Automatic route, Up to date (0.6.0), gopls ready. The source harness
continues to track the incomplete full VS Code/official Copilot VSIX/UI parity.
