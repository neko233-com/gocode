# Product engineering harness

Current objective: a native Go alternative inspired by Zed/GPUI and VS Code,
with GB file browsing, language/extension/AI services, free installation/update
channels, local deployment and visual validation. The workbench stays native.

Read [status](status.md), [architecture](architecture.md), [acceptance](acceptance.md)
and [distribution](distribution.md). Update their evidence/limitations as work lands.
Real PTY/shell/native lifecycle and syntax-highlighting contracts are in
[terminal](terminal.md).
Open editable-file watching/reload/conflict contracts are in [files](files.md).
The existing docs/vscode-parity.md remains the API/function coverage ledger.

Reference: latest stable VS Code 1.140.0 and current main
`1d25d5df846ea8edf8d0d6e2c742e8dda9d4916b` (2026-10-07). Use source measurements,
colors/fonts/layout and native screenshots, rather than remembered old UI details.

Production promotion requires real core/editor/process/native/visual/installer
checks and exact public commits. Future goals must not be described as completed.

Titlebar image, measured caption and owned native visual evidence are in [ui](ui.md).
