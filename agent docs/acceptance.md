# Acceptance contract

- Editable text: UTF-16/CRLF/selection/undo/save, immutable snapshots, rejected
  overlapping/stale edits, extension/provider version agreement.
- Large files: actual >=1 GiB baseline (higher user-selected target when supplied),
  bounded index/cache/read buffers, cancellation, first/middle/tail and huge-line
  navigation, file-change handling, responsive native frames and memory report.
- Language: real generic server initialization, edits/diagnostics, completion,
  formatting, hover/definition, save/close/restart and stale-result suppression.
  Real process crash/reinitialize, latest unsaved didOpen, independent other-server
  state, crash-loop stop/manual repair, actual initialize cancellation and bounded
  diagnostic/request/job queues are required. -lsp-smoke kills its owned real
  gopls and verifies recovery in the native window. Lifecycle results also guard
  document identity when a closed path is reopened at the same version.
  The final Windows recovery gate inspects actual completed-token/Problems glyph
  ink in process-owned pixels; a render submission counter alone is insufficient.
- Native: Windows amd64 PE/GOAMD64=v1/race/strict-cgo, owned HWND input/GPU pixels,
  drag/resize/restore/DPI; macOS Intel/ARM real AppKit/Metal behavior.
- Visual: pin VS Code source, snapshot font/theme/viewport/DPI, inspect meaningful
  editor/large-file/diagnostics/chat/settings/update states. Assertions alone do
  not replace visual review.
- Distribution: real MSI install/uninstall/upgrade in an owned root, portable and
  CLI install, icons/shortcuts/PATH, update probe/hash/metadata/rollback tests,
  published asset checks, then the explicitly requested local install.
- Terminal: real ConPTY/PTY, pre-Enter PSReadLine/zsh input colors, ANSI/truecolor,
  Unicode/alternate screens, actual shell grid after window resize, Ctrl+C/exit,
  bounded output history and descendant cleanup. Windows console/GUI subsystem
  probes wait on the owned process sequentially and assert owned GPU color pixels.
  Mac Intel/ARM native smoke does not by itself establish a Mac screenshot claim.

Normal CI never needs a paid Copilot prompt, certificate or notarization service.
Real-account AI acceptance is an additional synthetic-workspace check.

v0.7.0/source 8a0fa076 passed all five source jobs in 37498949904, publication
37500332017, actual signed public ZIP/native update and real v0.4.0 GUI rollback.
Windows 2022 source-CI GPU captures and installed v0.7.0 Settings/terminal GPU
captures were visually inspected. Installed real terminal/gopls and official
Copilot protocol checks passed; account check networkPromptSent=false. See
status.md for immutable metadata-CI and installation evidence and explicit gaps.

Close/save acceptance uses real immutable-snapshot disk writes and a deterministic
UI acknowledgement mailbox to reproduce newer edits, serial saves, cancellation,
queue bounds, external changes and closed documents. Native Windows tests send
actual OS WM_CLOSE and mouse/keyboard confirmation; derive client DIP from physical
size/DPI rather than assuming one viewport. The cross-platform -close-smoke modes
save/discard/cancel/external invoke native RequestClose in disposable workspaces,
render the modal, exercise acknowledgement and verify disk outside UI callbacks.
Mac Intel/ARM execute these workbench modes in CI; no Mac screenshot claim follows
from a successful smoke alone. -editor-smoke calls an actual VSIX Document.save,
verifies one save event, then native undo/redo and completion versions.
