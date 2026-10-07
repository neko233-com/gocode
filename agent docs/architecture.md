# Product contracts

## Documents

Editable source uses the framework's versioned UTF-16 Buffer/snapshots and atomic
transactions. GB-scale browsing uses a file-backed sparse line/byte index and a
bounded page cache; indexing/page reads never block the UI thread. Preserve file
identity, detect external changes, cancel on close, and expose read-only policies
explicitly where full editing/language-service capabilities are limited.

## Services

Native startup uses an empty model plus bounded Explorer/open workers. Live
documents are UI-owned; disk jobs build private buffers, transfer once and dispose
unadopted large indexes before continuing. Tickets guard focus/closed identity;
VSIX/LSP/Problems navigation waits for the actual target. See files.md for queue,
traversal/timeout bounds and the pre-ui.Run root/service setup still performed.

Generic language servers use the framework LSP client. Route capabilities,
document versions, diagnostics, formatting/navigation and server requests to the
native UI. Copilot completion uses the official LSP, chat the official Go SDK.
VSIX native edits use acknowledged version-checked RPC. Unknown APIs fail clearly.

Standard language processes restart independently with a bounded crash budget.
Only immutable source jobs/results cross threads; ready events replay current
unsaved snapshots. UI hooks do not accumulate on restart. Process generations
invalidate old results even when the same path/version is reopened. Per-session
document/request/event bounds and real-process/native recovery gates are in
docs/language-servers.md and status.md.

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
last-check/rename interval remains a race. Open editable-file parent watches,
bounded disk reconciliation, clean reloads, dirty conflict confirmation and
explicit hash-checked overwrite are implemented in files.md. Recursive workspace
watching, diff/merge and stronger coordination remain required for full parity.

## UI

Workspace replacement registers bounded metadata-only history groups before
callbacks. A dedicated worker prepares real undo/redo stack transitions from
immutable core history snapshots; complete UI preflight precedes all-buffer
adoption and notifications. Closed/reopened/newer history splits preserve other
documents. Undo/redo never writes disk. Exact limits and gaps are in history.md.

Workspace search owns one cancellable worker and one latest queued immutable
request. It streams literals/Go regex across unlimited line lengths and returns
bounded previews/byte/UTF-16 ranges. Version/instance/focus/generation and worker
digest/coordinate verification precede navigation. See search.md for execution,
traversal/ignore/regex limits, actual GiB evidence and remaining replace/API scope.

Replacement previews re-scan immutable sources on that worker. Public v0.8.0
Snapshot.Prepare computes text/selection/history off UI; all-buffer identity/
version/caret and disk preflight precede UI adoption. Open hooks run before the
final check, change hooks after all commits. Per-file saves follow; late failures
retain undoable dirty text and visible counts. This is not filesystem-wide
atomicity. Limits and evidence are in replace.md.

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

## Immutable images and native visual evidence

The titlebar bitmap is decoded/copied once before Run and reused by views through
the independently published godesktop v0.6.0 API. Native GPU residency/lifetime
is owned by the framework; application image identity is immutable. Mac visual
acceptance reads only the owned completed drawable through testing/metalprobe.
Tabs use platform UI-font measurements; complete captions retain a separate close
target. See ui.md for the current source pin, pixel/action gates and scope gaps.
