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

## UI

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
