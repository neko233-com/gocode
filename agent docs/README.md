# Product engineering harness

Current objective: a native Go alternative inspired by Zed/GPUI and VS Code,
with GB file browsing, language/extension/AI services, free installation/update
channels, local deployment and visual validation. The workbench stays native.

Read [status](status.md), [architecture](architecture.md), [acceptance](acceptance.md)
and [distribution](distribution.md). Update their evidence/limitations as work lands.
Real PTY/shell/native lifecycle and syntax-highlighting contracts are in
[terminal](terminal.md).
The existing docs/vscode-parity.md remains the API/function coverage ledger.

Reference: latest stable VS Code 1.140.0 and current main
`ff275a2dfa5cebe351c45e6c7c3d18002bcb2162` (2026-10-06). Use source measurements,
colors/fonts/layout and native screenshots, rather than remembered old UI details.

Production promotion requires real core/editor/process/native/visual/installer
checks and exact public commits. Future goals must not be described as completed.
