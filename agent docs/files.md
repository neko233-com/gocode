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
Windows observation/hash reads use non-inheritable CreateFile handles sharing
read/write/delete, including extended Unicode/long paths. Windows modern native
FileRenameInfoEx/POSIX replacement preserves those readers' old descriptors;
the original legacy os.Rename held-handle negative control still failed despite
deletion sharing and prevented a premature success claim. Unsupported OS/filesystem
classes fall back to ordinary rename with the same bounded retry. The modern path
checks unrelated deletion-sharing conflicts before replacing and does not ignore
read-only attributes. Other processes may omit deletion sharing: atomic
replacement retries only Windows sharing/lock/access-denied failures for at most
two seconds with cancellation and a disk-hash check before every save retry.
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

## Asynchronous opens and Explorer traversal

Native startup constructs an empty workbench, then opens explicit CLI files and
scans Explorer on separate workers. Root canonicalization/Stat and optional
service discovery still occur before ui.Run; recursive traversal and document
contents no longer execute inside native UI callbacks. Headless newModel is a
synchronous fixture helper, not the production entry point.

The UI owns at most 32 pending open jobs and one active transfer. One disk worker
resolves physical paths and loads regular UTF-8 text up to 8 MiB, or constructs a
bounded large-file index. It never touches an existing live buffer. Dispatch
transfers ownership once; the worker waits for acknowledgement and disposes
duplicate/discarded indexes before the next job. Atomic ownership handles an
accepted dispatch dropped during window shutdown. Cleanup remains off the UI.

Navigation tickets prevent slow results stealing newer focus. Windows 8.3 and
symlink aliases reuse the original unsaved document; closed-path tickets reject
older opens but permit an explicit later reopen. Cancel clears pending jobs and
rejects even a completed read awaiting UI receipt. Cached opens check request
cancellation. VSIX showTextDocument, Problems and LSP definition navigation wait
for the actual target; superseded VSIX requests return an error. Extension edit/
selection/save paths and generic diagnostic URIs resolve in RPC/service workers.
UI findDocument performs only lexical/cache lookup.

Explorer reads 128 entries per directory batch, retaining at most 250 files,
256 queued directories, depth 64 and 32,768 visited entries. It skips symlink
directories and existing .git/.cache/node_modules/bin/vendor exclusions. Sorted
results expose scan limits/unreadable directories instead of implying complete
workspace coverage. A late scan can populate Explorer but cannot start obsolete
default navigation. CLI goto binds to the final requested file, even when that
file is cached and an earlier open is still pending.

Workers use 30-second cooperative contexts; OS calls may remain uninterruptible.
One blocked read retains its slot without spawning replacement workers. Queues
remain bounded and shutdown waits at most three seconds. Scan cancellation is
independent of explicit file opens. Recursive live Explorer watching and complete
tree/navigation behavior remain unfinished.

-open-smoke deliberately holds worker latency, then executes the real disk
reader/scanner and actual bundled Node VSIX. Windows sends owned native character
messages, resizes the HWND and clicks Cancel/tab controls; GPU gates inspect the
newly typed row and awaited README row. Mac exercises the same native model/view
actions without claiming Windows pointer messages or screenshots. Source files
are compared afterward. Race tests cover serial transfer/disposal, cancelled
receipts, queue bounds, shutdown indexes, closed aliases, newer focus, dirty
buffers, startup barriers/goto and traversal limits.

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

Source 6001f54 failed Windows 2022 native gopls/file-watch CI 37515729600:
the second external fixture rename returned Access denied while gopls was reading.
It was not published. Go 1.27 syscall.Open shares read/write but omits delete;
both our readers and unrelated readers can interfere with rename. A real held
descriptor regression now proves our long Unicode-path reader permits rename.
Actual save regressions hold a legacy reader across rename, release it, edit the
disk during retry, and cancel retry: only the authorized unchanged revision may
be written. The native external-editor fixture also uses the bounded Windows
replacement path; actual replacement/notification/pixel requirements remain.
Read-only and nonregular save targets are rejected before creating a temporary
save file and at every retry; no permission override or silent replacement occurs.

## Remaining scope

This watches open editable files, not the entire recursive workspace. Native
ordinary opens/traversal are asynchronous in the new source; platform promotion
and installed-release evidence must be read from status.md. Huge read-only browser index
invalidation/reopen, deleted-file restoration, diff/merge conflict review, nested
workspace watchers, VSIX filesystem-watch API and LSP watched-file registrations
remain unfinished. The last disk-check/rename interval is not a transaction
against an unrelated external writer. Full VS Code/production parity is active.

References: [fsnotify upstream](https://github.com/fsnotify/fsnotify/tree/v1.10.1)
and pinned [VS Code disk model](https://github.com/microsoft/vscode/blob/bf519293a7de013c7e3897185784065b0799c9f2/src/vs/workbench/services/textfile/common/textFileEditorModel.ts).
