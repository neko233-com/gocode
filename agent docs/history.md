# Workspace history contract

Source reference checked 2026-10-07: VS Code main
`491cf07046747e11065bf5cb1cb96cba7ab81782`.
[UndoRedoService](https://github.com/microsoft/vscode/blob/491cf07046747e11065bf5cb1cb96cba7ab81782/src/vs/platform/undoRedo/common/undoRedoService.ts)
provides the cross-resource confirmation, stack-top verification, preparation and
split reference. The implementation below is independent native Go code.

Replacement registers one metadata-only group before change callbacks run.
Groups are bounded to 32, with at most 128 path/tab-identity/revision members;
they never retain closed document pointers or full source. Eviction leaves normal
per-buffer history intact. Ctrl/Cmd+Z offers Undo in N Files, Undo this File and
Cancel. Ctrl/Cmd+Shift+Z and Ctrl+Y redo a valid group. A newer history entry in
another member or a closed/reopened member splits to the current file, preserving
the protected file. Current-file choice removes the workspace grouping.

A dedicated cancellable worker, one latest queue and 30-second job deadline
prepare genuine core stack transitions from immutable HistorySnapshots. Complete
UI preflight checks document ownership/tab identity and every prepared version,
caret and history stack before any commit. All buffer commits precede callbacks.
Worker shutdown cancels, invalidates receipts and waits at most three seconds.
Typing/new extension edits while preparation is pending invalidate the batch.
Confirmation is revalidated after the modal. No undo/redo performs disk writes:
existing explicit save transactions handle later saves and external hash conflicts.

Candidate evidence: three-repeat race tests pass real saved replacement groups,
all-or-nothing held receipts, newer edits/caret/reopen/modal changes/group expiry, current-file
split, preserved earlier local undo, bounded metadata and cancelled worker shutdown.
Real external disk bytes survive undo and redo. The expanded -replace-smoke checks
owned native Cancel/All/current-file controls, shortcut redo and completed modal
GPU text/button pixels. Both Windows console/GUI strict-cgo native gates pass;
the actual 1920×1230 / 150% confirmation PNG was visually reviewed. Complete
candidate race tests pass; vet's two unkeyed external test literals were corrected.
Source independently downloads public godesktop v0.9.0/cb8b077 with GOWORK=off
and no replace; its all-five CI 37555224866 passes. Full local public-module
Windows strict-cgo/race/vet/console/GUI regression passes. Cross-platform/released/
install evidence is pending. Full VS Code history
persistence across closed resources, compound undo groups, provider edits/multi-root
and full diff editor remain open.
