# Native keymaps and repeatable Windows acceptance

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
