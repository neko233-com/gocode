# Acceptance contract

- Editable text: UTF-16/CRLF/selection/undo/save, immutable snapshots, rejected
  overlapping/stale edits, extension/provider version agreement.
- Large files: actual >=1 GiB baseline (higher user-selected target when supplied),
  bounded index/cache/read buffers, cancellation, first/middle/tail and huge-line
  navigation, file-change handling, responsive native frames and memory report.
- Language: real generic server initialization, edits/diagnostics, completion,
  formatting, hover/definition, save/close/restart and stale-result suppression.
- Native: Windows amd64 PE/GOAMD64=v1/race/strict-cgo, owned HWND input/GPU pixels,
  drag/resize/restore/DPI; macOS Intel/ARM real AppKit/Metal behavior.
- Visual: pin VS Code source, snapshot font/theme/viewport/DPI, inspect meaningful
  editor/large-file/diagnostics/chat/settings/update states. Assertions alone do
  not replace visual review.
- Distribution: real MSI install/uninstall/upgrade in an owned root, portable and
  CLI install, icons/shortcuts/PATH, update probe/hash/metadata/rollback tests,
  published asset checks, then the explicitly requested local install.

Normal CI never needs a paid Copilot prompt, certificate or notarization service.
Real-account AI acceptance is an additional synthetic-workspace check.
