# Product engineering harness

Current objective: a native Go alternative inspired by Zed/GPUI and VS Code,
with GB file browsing, language/extension/AI services, free installation/update
channels, local deployment and visual validation. The workbench stays native.

Read [status](status.md), [architecture](architecture.md), [acceptance](acceptance.md)
and [distribution](distribution.md). Update their evidence/limitations as work lands.
Real PTY/shell/native lifecycle and syntax-highlighting contracts are in
[terminal](terminal.md).
Intrinsic emoji and actual cross-platform terminal GPU acceptance are in
[color glyphs](color-glyphs.md).
Open editable-file watching/reload/conflict contracts are in [files](files.md).
The existing docs/vscode-parity.md remains the API/function coverage ledger.

Reference: latest stable VS Code 1.140.0 and current main
`3f07e1aba32acacb8b08ae91bfdc954b580ad1fd` (2026-10-07). Use source measurements,
colors/fonts/layout and native screenshots, rather than remembered old UI details.

Production promotion requires real core/editor/process/native/visual/installer
checks and exact public commits. Future goals must not be described as completed.

Titlebar image, measured caption and owned native visual evidence are in [ui](ui.md).
Workspace search, streaming/ignore/stale-navigation bounds and evidence are in [search](search.md).
Replacement preview, prepared buffer commits and per-file save/failure contracts
are in [replace](replace.md).

Workspace grouped undo/redo, confirmation/split and preparation are in
[history](history.md).

Native split ownership, independent views, scoped close and shared large readers
are in [groups](groups.md).
