# External editable-file contract

Open editable files use fsnotify v1.10.1 parent-directory watches, not inode-only
file watches. This covers ordinary writes and temporary-file/atomic replacement.
Watch registration failures, overflow and disappeared parent directories are
reconciled: metadata every two seconds, content every 30 seconds, registrations
retried after parent recreation. These are bounded worker operations.

The worker accepts 128 editable paths, one coalesced subscription snapshot and
one pending UI acknowledgement/body. It reads one file per 100 ms tick, at most
8 MiB through 128 KiB chunks, validates regular UTF-8/NUL-free text, and compares
the descriptor and final pathname identity/size/mtime before publishing. A racing
replacement is retried. Native event storms cannot queue multiple text bodies.
Cancellation rejects pending callbacks; shutdown waits at most three seconds for
an uninterruptible OS read. Initial small-file opening also uses this bounded
reader instead of an unbounded Stat/ReadFile race.

UI applies results only to the same open-document identity, buffer version and
acknowledged disk hash. Clean buffers adopt the new disk revision. Buffer.Reload
in public godesktop v0.5.3 preserves monotonic versions, bounded undo/snapshots,
selection and LF/CRLF. The ordinary change event reaches VSIX, LSP and completion
invalidation; disk reload does not report a successful Document.save event.

Dirty buffers retain text/version/history and show a persistent conflict banner.
Reload from Disk requires an explicit discard confirmation; Cancel preserves
edits. The confirmed action reads the current disk contents again. A newer VSIX
edit while that read is pending cancels the discard and requires review/retry.
Overwrite Disk explicitly saves local text against the hash shown by the banner;
the existing writer still verifies that hash before writing and before rename.
Deleted, nonregular, binary and oversized replacements preserve editor contents.
Watching pauses while accepted saves are queued/running and reconciles after UI
acknowledgement, including unrelated external writes after our atomic rename.

## Evidence and gates

Real file/race tests exercise atomic replacement, deletion/recreation, paused
saves, event storms, one pending body, shutdown, input bounds, queued stale reads,
closed/reopened identity, explicit reload versus a newer VSIX edit, EOL/undo and
an external write between our disk write and UI acknowledgement. An intentional
mutant reloaded the old buffer text: the real-watch regression failed, then the
restored source passed. Native acceptance activates the real bundled VSIX by its
command (its manifest does not auto-activate on Go); an earlier harness attempt
without activation timed out rather than fabricating diagnostic success.

-filewatch-smoke uses actual owned files and atomic replacements. It checks clean
reload, dirty preservation, rendered confirmation/cancellation, EOL undo/redo and
actual VSIX diagnostic clear/reappearance. With configured gopls it also verifies
hover for the reloaded function after undo/redo. Windows sends actual process-owned
mouse messages to banner/confirm buttons and gates owned GPU function suffix,
banner/modal colors and final Problems glyph ink. Mac invokes the same UI actions
in the real native workbench; this alone is not a Mac mouse/screenshot claim.
CI runs both VSIX-only and actual-gopls native modes on Windows/Mac, and portable
race/no-cgo policy tests. Local Windows captures are in .cache/filewatch-native
and .cache/filewatch-native-lsp; final source CI/public installation evidence is
recorded in status.md only after it passes.

## Remaining scope

This watches open editable files, not the entire recursive workspace. Initial
workspace traversal, canonical-path resolution and ordinary UI open callbacks
still perform synchronous filesystem operations. Huge read-only browser index
invalidation/reopen, deleted-file restoration, diff/merge conflict review, nested
workspace watchers, VSIX filesystem-watch API and LSP watched-file registrations
remain unfinished. The last disk-check/rename interval is not a transaction
against an unrelated external writer. Full VS Code/production parity is active.

References: [fsnotify upstream](https://github.com/fsnotify/fsnotify/tree/v1.10.1)
and pinned [VS Code disk model](https://github.com/microsoft/vscode/blob/bf519293a7de013c7e3897185784065b0799c9f2/src/vs/workbench/services/textfile/common/textFileEditorModel.ts).
