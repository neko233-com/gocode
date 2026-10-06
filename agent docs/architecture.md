# Product contracts

## Documents

Editable source uses the framework's versioned UTF-16 Buffer/snapshots and atomic
transactions. GB-scale browsing uses a file-backed sparse line/byte index and a
bounded page cache; indexing/page reads never block the UI thread. Preserve file
identity, detect external changes, cancel on close, and expose read-only policies
explicitly where full editing/language-service capabilities are limited.

## Services

Generic language servers use the framework LSP client. Route capabilities,
document versions, diagnostics, formatting/navigation and server requests to the
native UI. Copilot completion uses the official LSP, chat the official Go SDK.
VSIX native edits use acknowledged version-checked RPC. Unknown APIs fail clearly.

Ctrl/Cmd+S, palette saves, VSIX Document.save and close confirmation share one
UI-owned queue (32 pending jobs) and one disk writer. UI callbacks freeze path/
immutable snapshot; a 30-second worker hashes/writes at most 8 MiB with fixed
128 KiB blocks. Success is dispatched back to UI, updates the known disk hash,
and marks only the exact saved version clean. Queued saves rebase their disk hash
after acknowledgement. Closed documents and cancelled requests do not start new
writes. Newer edits remain dirty and VSIX gets saved=false. Per-document saveIds
deduplicate success notifications/RPC acknowledgements. Discard waits behind
already accepted writes; failed saves keep the window/modal. Worker shutdown
cancels and waits at most three seconds for an uninterruptible OS write/fsync.

Disk hashes detect external content changes before writing and before rename.
They do not provide a filesystem transaction against unrelated programs; the
last-check/rename interval remains a race. File watchers/conflict resolution UI
and stronger platform-specific coordination are still required for full parity.

## UI

Real terminal processes and VT parsing are worker-owned. Immutable cell snapshots
reach native views through coalesced dispatches; shell input does not pass through
document editing. See terminal.md for PTY/ConPTY, bounded queues/history, isolated
profiles, default-shell highlighting and process lifecycle constraints.

The workbench uses native platform GPU rendering, virtual rows and bounded
measurement/cache state. Window dragging/resizing/maximize/restore and current
DPI hit testing are part of acceptance. Keep the latest VS Code source reference
and record actual visual results, including sidebar/panel/overlays and long text.

## Installation and update

Separate app versions, optional sidecars, user configuration/extensions and
workspaces. Never place user edits in the owned installation payload. Resolve
GitHub reachability and download route with bounded probes, manual overrides and
integrity-checked metadata/artifacts. Stage before switching; preserve rollback.

Do not require a paid installer tool or OS signing certificate. Free metadata
integrity verification is separate from optional platform publisher signing.
