# Native editor groups

Released v0.17.0 extends the tree to nine groups and Ctrl/Cmd+1..9, matching
ViewColumn.One–Nine. Real VSIX view identity/visibility/column/range/selection,
hidden worker opens and guarded show/selection/reveal are implemented; status.md
records exact all-five CI, immutable publication and installed evidence. It uses
public core v0.10.0 with GOWORK=off/no replace. Historical v0.16.0 below retains
its earlier eight-group/native-only contract.

The v0.17 bridge uses one TextDocument per native canonical resource and one
TextEditor per actually visible eligible view. Hidden tabs are excluded, identities
survive column renumbering, and closed views/reopened document instances reject
stale edits/save/selection/reveal. Snapshot generations and complete coordinate
preflight guard events; at most nine native views and 128 ordered/pending host
operations are admitted. Native service documents are limited to 2 MiB.

Hidden openTextDocument reads use the shared cancellable worker. showTextDocument
supports Active/Beside/One–Nine, preserveFocus and UTF-16 selection. Its invocation
receipt captures origin/layout before the read so newer focus or closed targets
invalidate late focusing results. Selection/reveal/edit ordering is acknowledged
by native state. Manual/reveal scroll stays independent until caret movement.
The real VSIX gate uses shared Unicode/CRLF content, hidden resources, independent
reveal, actual mouse focus/close, surviving reference renumbering, disposed edits
and disk-acknowledged save. Local 150% mutated and 100% survivor PNGs were visually
inspected. Current independent public-module/platform/publication status is in
status.md. Full tabGroups, editor options/decorations/snippets and undo merging
remain unfinished.

Renderer previews decode at most 400 runes and minimap length queries stop at 70;
an actual 7 MiB Unicode-line benchmark holds preview allocation to <=4096 bytes/op.
Core short position queries on an actual 8 MiB line allocate zero objects. These
are scoped measurements, not whole-frame performance claims.

## v0.16.0 historical release contract

Released v0.16.0 uses public godesktop v0.9.0 with GOWORK=off and no replace.
Released/installed versions and immutable CI evidence remain in status.md.

Visual thesis: Dark Modern planar editor groups use restrained borders and native
font measurements. Content thesis: every group contains its own tabs, breadcrumb
and source, while navigation, panel and status remain workspace-wide. Interaction
thesis: one focused caret, independent view state, and live 4-DIP sash resizing.
The reference is VS Code stable 1.140.0/main 26e0111cea3247abadfdd27f991a15a6a13f1c85,
including editorActions.ts and editorgroupview.css in the MIT Code-OSS repository.
This source comparison does not establish full pixel parity.

## Ownership and actions

At most eight leaves form a binary horizontal/vertical split tree. Ctrl/Cmd+\
splits right; adding Shift splits down. Ctrl/Cmd+1..8 focuses groups in visual
tree order. The native toolbar offers both splits and close group. Mouse input
focuses the target group before editor hit testing; wheel input over an inactive
source or tab strip updates that view without moving focus. Each group owns tab
membership, MRU/reveal/overflow state, selection, caret and scroll. Sashes clamp
each side to a feasible minimum (180 DIP horizontally, 100 vertically).

One canonical document owns text, dirty/save points, undo/redo, disk identity and
LSP/VSIX lifecycle across all its views. A split shares the actual Buffer; it does
not emit another document-open event. Inactive selections transform through real
UTF-16 change ranges, including multiline/CRLF and non-BMP input. The focused
view projects its selection into the shared Buffer before ordinary editing.
Closing one shared view neither discards dirty text nor closes its services.
Closing the last view retains the existing dirty-save confirmation.

Close group prompts only for dirty resources unique to that group. Cancel keeps
the group; Save awaits the real background disk queue; Discard releases its unique
resources. Shared dirty resources remain open and untouched on disk. Toolbar
clicks retain the pressed group and membership/current-document generation;
changed or expired review rejects the old release action. Background open tickets
retain their originating group and reject a group closed before delivery.

## Huge files

Each large-file view owns page/request/cancellation/navigation state, sharing the
thread-safe file-backed sparse index and ReadAt reader. A root resource keeps the
reader alive when its original view closes; the last document close cancels all
views and releases the file. Pages remain bounded; split rendering never loads
the entire file. Long-line display uses measured pane width and rune cell width.
Large files remain read-only and are not sent wholesale to language/AI services.

## Verification and gaps

Three-repeat race tests cover shared text/protocol events, transformed carets,
independent views/shared undo and save points, group MRU, eight-group limit,
close scope, held real-file receipts and shared large-reader lifetime. Full local
public-module strict-cgo/race/vet and console/GUI regression passed. Owned native
-groups-smoke uses actual Unicode input, group focus, horizontal sash drag,
nested vertical splits, stale held close click, Cancel and actual scoped Save.
Completed syntax glyph pixels are required in each viewport. Both console and
GUI gates passed at local 150% and forced process 100% DPI.

-groups-large-smoke streams actual files, independently renders first/tail pages,
closes the original view and reads again from the survivor, then verifies the
whole file SHA256. Local 32 MiB and actual 1,073,741,824-byte runs passed. Exact
source 38f7d15 passes all five jobs in CI 37561829837, including actual GiB splits
on Windows 2025 and both gates at normal/1.5/2 on both Mac architectures.
Publication 37562709833 reuses those packages. Released group/native/route/rollback
and installed GUI/actual GiB split/Settings gates pass. Exact hashes and source
evidence are in status.md. Windows 100%, ARM 200%, Intel 150% and installed 150%
PNG frames were visually reviewed. Mac emoji appears as a solid monochrome fallback
in these captures; correct emoji glyph/color rendering remains a framework gap.
Those v0.16/v0.17 captures are historical. Public/installed v0.18.0 imports
core v0.11.0 and passes actual intrinsic Mac emoji pixels in both split views;
both Mac normal/1.5/2 source gates and selected PNGs are verified in color-glyphs.md.

Full group persistence/restoration, tab move/drop/docking, configurable layouts,
multi-window and complete VSIX tabGroups/options/decorations remain gaps.
The native gate currently drags the horizontal sash and checks nested vertical
geometry; it does not independently prove vertical-sash native dragging. Complete
IME/grapheme/bidi/multicursor/accessibility, SCM/DAP and official Copilot VSIX
compatibility remain separate unfinished goals.
