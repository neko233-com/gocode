# Native workspace replacement contract

Source checked 2026-10-07: stable VS Code 1.140.0 / main
`a4a3dff1c3d7b48bf44c5c1ea54447b36a80b053`. Search CSS blob
`d6709f396f3f0cd2fd6163e1757c519a81d872ac` is unchanged from 8ce9c47.
[Replace service](https://github.com/microsoft/vscode/blob/a4a3dff1c3d7b48bf44c5c1ea54447b36a80b053/src/vs/workbench/contrib/search/browser/replaceService.ts)
provides the model-edit / text-file-save reference. Numeric captures, escaped
newline/tab/backslash, whole matches and per-capture case operations follow
[replacement patterns](https://github.com/microsoft/vscode/blob/a4a3dff1c3d7b48bf44c5c1ea54447b36a80b053/src/vs/workbench/services/search/common/replace.ts).
Complete JS/PCRE2 compatibility is not claimed.

## Interaction and planning

Ctrl/Cmd+Shift+H or the search chevron exposes the replacement field. Its caret,
select-all/paste and Tab order are independent of query/document input. Enter or
Preview prepares a plan; Replace all first previews if needed, then applies the
reviewed plan. Virtual rows show bounded red before / green after values; clicking
a preview row does not move the original caret. Replacement typing leaves the
query result generation unchanged.

One existing cancellable worker/latest queue owns preparation. Complete successful
search results are required: incomplete/error/unreadable/changed reports must be
refined or rerun. Sources are rescanned and the entire match set, byte offsets,
UTF-16 ranges and digests compared. Added/shifted matches invalidate the plan.
Open snapshots require original tab identity/version. Closed files use os.OpenRoot,
regular/writable checks, bounded reads and post-read file identity/size/time.
Planning never writes disk or mutates a live Buffer; new private buffers transfer
to the UI once. Snapshot size is checked before joining its line strings.

Limits: 128 matched files, 5,000 results, 8 MiB source/output per file, 64 MiB
aggregate source plus output and 4,096 replacement bytes. Regex expansion and
normalized CRLF output are checked. These account retained source/output, not
process RSS; bounded history, protocol slices and worker temporaries are additional.
Large-file browsing stays read-only and never enters this writer.

Literal replacement stays literal. Regex supports $0/$&, $1–$99 with JS numeric
fallback, $$, prefix/suffix and known named captures, newline/tab/backslash and
simple Unicode u/U/l/L per-capture operations. Go/RE2 syntax remains explicit;
full JS named-missing/case-mapping/UTF-16 operations, preserve-case and PCRE2 need
further compatibility.

## Commit and save

Public godesktop v0.8.0 Snapshot.Prepare computes prospective text, changes,
selection and undo state off UI. CanCommit checks identity/version/selection;
a reopened same-version Buffer is a different identity. Apply checks all expected
disk hashes on the worker. UI rechecks documents and pending saves, resolves closed
files without stealing focus, then preflights again after open-notification hooks.
All buffers commit without interleaved callbacks; change notifications follow the
complete batch. Earlier unsaved source, history and LF/CRLF survive. Existing
background saves mark only exact saved versions clean. Changed caret/content or
closed/reopened documents require a fresh preview.

Buffer adoption is atomic in one UI turn. Disk writes are individual atomic saves,
not a filesystem-wide transaction against other programs or power loss. A late
conflict may leave earlier files saved and later buffers dirty. Unsaved counts/
errors are visible, dirty replacements remain undoable and external bytes are
preserved. Per-document Undo produces dirty text without silently rewriting disk.
v0.15.0 adds grouped workspace undo/redo, native confirmation and current-file
split through public core v0.9.0; exact native/release/install evidence and
closed-resource/provider gaps are in history.md. Individual replacement controls,
full diff editor, encoding/multi-root and large-file editing remain open.

## Evidence

GOWORK=off independently imports public v0.8.0 without replace. Planner tests cover
unsaved/closed Unicode/CRLF, complete-set staleness, incomplete reports and source/
expansion bounds. Actual Node JavaScript supplies the capture-expansion oracle.
Three-repeat race models hold real worker receipts for changed query/content/
caret/reopened identity, all-file disk preflight, open-hook mutation, real saves,
undo and late actual external save conflicts. Full local strict-cgo/race/vet and
console/GUI native regression passes; search package coverage is 81.4%.

Owned native -replace-smoke passes actual query/regex/replacement typing, red/
green completed GPU pixels, external disk-stale rejection, restored re-preview,
actual pointer-down/new-preview/pointer-up rejection of changed reviewed content,
two real saved files with unsaved prefix/CRLF, blank-editor focus and native Ctrl+Z.
Preview/stale/saved/undo PNG/JSON stages are retained. 1920×1230 / 150% preview
was visually reviewed (5,700 red / 8,282 green pixels). The first fixture forgot
to activate its document and missed unsaved input; it is fixed. A subsequent run
exposed actual lost focus on blank-editor clicks; production input now focuses
the closest real line and the unchanged native undo scenario passes.

Immutable v0.14.0/source de2a98f passed all five source jobs in CI 37552538844,
including Windows console/GUI and Mac Intel/ARM normal/1.5/2 native gates.
Publication 37553566847 reused tested packages. Released-byte replacement and
other native gates, signed actual direct/mirror ZIP integrity, original v0.4.0
native rollback and the user's installed 0.14.0 GUI/replace/SDK-LSP checks pass.
Windows 100% preview, ARM 200% changed-review, Intel 150% Undo and installed
150% preview/Settings PNGs were visually reviewed. Exact source/hash/install
scope and remaining parity gaps are in status.md.
