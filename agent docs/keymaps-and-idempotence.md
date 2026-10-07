# Native keymaps and repeatable Windows acceptance

Public/installed v0.22.0/source 0b5966171e5e183e74a9fecbeccce391cd1cf229
passes all five exact-source CI jobs 37651719075 and package-reuse publication
37654475129. Both Windows jobs retain three shuffled race/strict-cgo repeats;
package binaries run serially (-p=1), with unchanged native guards/assertions.
Released and actual installed 45-phase native keymap checks pass. The actual
installed configuration/PATH and both stable shortcuts match baseline. Final
Settings/File PNGs were inspected at 150%; full release evidence is in status.md.

The user explicitly requested idempotent tests and built-in JetBrains/VS Code
shortcuts. Both presets are native Go data, require no extension, and drive the
same command dispatcher and menu/Quick Input binding labels. VS Code is default.
Settings > Keyboard Shortcuts and the two Use ... Keymap commands switch at once.
The bounded keyboard.json in the OS user configuration/gocode directory persists
the choice; -keymap vscode|jetbrains overrides the current session. Replacement
and new windows inherit the current preset. Disk writes run on one worker with a
one-item coalescing queue and explicit shutdown. Re-selecting the current preset
or saving identical bytes performs no write and leaves no temporary file.

Windows JetBrains bindings derive from the official GoLand Windows reference:
https://www.jetbrains.com/help/go/reference-keymap-win-default.html and
https://resources.jetbrains.com/storage/products/goland/docs/GoLand_ReferenceCard.pdf
(checked 2026-10-07). No JetBrains art/code/keymap package is copied. Supported
commands include Find Action (Ctrl+Shift+A), Go to File (Ctrl+Shift+N), open-editor
Recent Files (Ctrl+E), Search Everywhere (two unmodified Shift taps within 500ms),
Save All (Ctrl+S), Close Editor (Ctrl+F4), Settings (Ctrl+Alt+S), Format (Ctrl+Alt+L),
Definition (Ctrl+B), Hover (Ctrl+Q), Find/Replace in Files (Ctrl+Shift+F/R), line
navigation (Ctrl+G), Explorer/SCM (Alt+1/9) and Terminal (Alt+F12). Ctrl+Alt+S is
excluded from Alt-only menu mnemonics. Shift typing, repeat and focus loss cancel
the double-tap gesture. Ordinary copy/paste/select/undo remain native editor actions.

Full JetBrains refactoring/debug/editing parity is not claimed. Reserved Ctrl+W
does not fall through to VS Code Close Editor; expanding/shrinking selection is
still pending. Recent Files lists this window's open MRU documents, not persistent
history. Search Everywhere includes native commands/workspace filenames, not
symbols/classes. Unimplemented preset actions have no misleading menu binding.

## Actual verification

- Three-repeat strict-cgo/race keymap tests pass dispatch, exact menu labels,
  command/file distinction, real LSP request selection, disabled VS Code fallback,
  double-Shift typing/focus cancellation and configuration size/error bounds.
- Configuration tests switch and save three times in the same isolated directory;
  only keyboard.json survives, identical writes preserve mtime, invalid profiles
  cannot alter the saved choice, and reads reject files above 4 KiB.
- The native Windows gate now has 45 phases. Real HWND pointers switch both
  Settings presets; actual modifier/key messages exercise Ctrl+Shift+A/N/P,
  Ctrl+Alt+S and both Shift press/release taps. Final disk configuration must be
  exactly vscode. File/Unicode/VSIX guards from the original 33 phases remain.
- TestNativeWorkbenchAcceptanceIsIdempotent builds the actual race executable
  and runs that complete native gate twice with the same private TMP/TEMP/APPDATA
  and evidence directories. Initial both runs pass (44.27s); the expanded isolated
  acceptance then passes three repeats (121.712s), each running twice. After each, scratch contains
  zero temporary workspaces, VSIX or saved files, the user-settings marker and
  directory are unchanged, and evidence filenames/count do not accumulate.
- Actual new window/disabled-extension reload/deferred uninstall/Open Folder
  processes pass three race repeats (30.994s). Their owned kill-on-close Windows
  Job Object includes replacement windows/hosts/terminals on assertion failure.
  Same-title windows are selected by PID. Cross-process shell edit verification
  uses WM_GETTEXT per Microsoft's GetWindowTextW contract:
  https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-getwindowtextw.

Ignored full-suite failure logs retain earlier desktop input interference.
Opt-in GODESKTOP_TEST_INPUT_ISOLATION=1 validates bounded WM_COPYDATA replay
through the real HWND handlers while physical mouse/keyboard/focus changes cannot
mutate the fixture. Explicit replayed cancellation remains testable. Ordinary
application input is unchanged. The parent rounded-clipping record describes
the transport bounds. Final captures wait three completed GPU submissions after
the asynchronous actor acknowledges model changes.

Windows CI's aggregate job budget is 20 minutes: the expanded app race package
alone takes about 244 seconds locally, versus the earlier 99-second suite, before
existing GiB/Copilot/gopls/MSI checks. The race package budget is eight minutes;
native acceptance/dialog guards remain 60/10 seconds. No assertion is relaxed.

The complete Windows script isolates TMP/TEMP/APPDATA and the extension root,
restores its environment, verifies the resolved cleanup root is its own direct
temporary child, and removes that tree even on failure. Executables and evidence
use stable bounded paths; read-only dependency caches are reusable. Full final
public-module regression, exact-source CI and publication/install follow.

Final independent public-core v0.15.0 Windows script passes three shuffled race/
strict-cgo repeats (app 242.704s) and all console/GUI native regression gates,
including both 45-phase workbench runs and scratch-root cleanup. No-cgo, vet,
actionlint/ShellCheck, distribution policies and diff checks pass. File popup and
selected JetBrains Settings PNGs are inspected. Exact source CI and published/
installed-byte checks follow before promotion.

Initial source CI 37644269599 catches Shift+Alt+F being routed as Alt+F during
real gopls formatting. A full m.input dispatch test fails on the original code;
the Alt mnemonic mask now excludes Shift as well as Control/Command. Three race/
strict-cgo repeats pass (2.027s). The native LSP fixture now sends the actual
Ctrl+K Ctrl+I chord, preserving the rule that Ctrl+K alone does not issue hover.
Real Windows formatting/hover/definition/completion/diagnostics/crash recovery/
final pixels, the 45-phase gate and actual Git commit pass after the correction.

Final targeted input routing passes three strict-cgo/race repeats (1.960s),
including Ctrl+W preserving the source behind a read-only Git diff and excluding
Control/Alt/Command-modified Shift taps from Search Everywhere. The native shell
selector now requires visible/enabled owned dialog windows, excluding stale cached
HWNDs; diagnostics include edit/window visibility and enablement. Three actual
idempotence/lifecycle repeats pass (151.237s): six full 45-phase native runs leave
empty private scratch, unchanged settings and a stable evidence count. Previous
Windows 2025 CI 37646262818 failure is retained in status.md; that source is not
published. Final-source CI/released/installed-byte checks remain required.

CI 37648319182 passes Windows 2025, both Macs and Ubuntu; Windows 2022 observes
a correctly owned but hidden (visible=0/enabled=1) shell dialog. Its cause is not
claimed. Windows package binaries now run with go test -p=1 so native shell/GPU
acceptance does not compete with parallel GiB/parser/process suites. Default
race repeat count remains three and native guards/cleanup assertions are unchanged.
Local independent serial Windows script Repeat=1 passes (app 81.809s), all
console/GUI native gates, vet and owned scratch cleanup. Final three-repeat
source CI remains the publication gate.

Final-source CI satisfies that gate on Windows 2022 and 2025. Isolated scratch
tests remove their owned roots and reuse stable evidence names. One earlier
manual diagnostic used inconsistent drawable/input density and timed out; it is
not valid product evidence. Its owned directory
C:/Users/14170/AppData/Local/Temp/gocode-scm-native-4191533845 initially remains because
automatic approval review rejected checked literal deletion with 'blocked by policy'
and no further reason. No alternate deletion mechanism was used to bypass it;
global temporary-directory emptiness is not claimed.

2026-10-08 cleanup is resolved: the same literal owned path is revalidated for
its Gocode acceptance Git identity, exact two entries and absolute temp boundary,
then successfully removed under current permissions. The two earlier approval
review rejections remain historical evidence. Fifteen explicitly named inactive
fixtures from the failed restricted local tests are also removed from the
workspace's private Go temp root after path/age/reparse/live-process checks.
No broad deletion of user temporary directories or live build caches is used.

The v0.23.0 candidate also repairs persistence error delivery under UI overload.
VS Code/JetBrains selection uses a capacity-one coalescing writer and a separate
latest-error publisher shared with Auto Save. New selections/stop cancel old
errors, including accepted but delayed callbacks; shutdown still drains the latest
configuration within 3s. Three public-core16 strict-cgo/race repeats pass together
with actual Auto Save/Node VSIX/save-path regressions (app 5.803s). Real malformed
directory failures, 512 selections, actual latest config with UI rejection,
shutdown and temporary-file cleanup are tested without user configuration.
