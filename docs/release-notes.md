gocode v0.23.0 makes File Auto Save and Revert File functional in the native
Windows workbench, using public Go/godesktop v0.16.0.

Auto Save supports off, afterDelay, onFocusChange and onWindowChange. File toggles
the checked action; native Settings configures all four modes and the delay.
Bounded scheduling uses the existing immutable, hash-checked atomic writer.
Newer edits remain dirty, conflicts require review, and untitled/read-only large
files cannot produce automatic Save As dialogs. Session flags and new windows
preserve the chosen policy; idempotent settings avoid duplicate writes.

Actual minimized-window UI receipts no longer wait for rendering. Window-change
and delayed saves complete disk/clean/save events while View/GPU submissions
remain stopped. Bounded, cancellable worker retries retain acknowledgements under
UI queue pressure, including file/scan/search/history/Git/language/terminal/update
services. Superseded large-page/debounce receipts are cancelled. Closed terminals
stop per-tab retries and reap their actual processes. Save As excludes overlapping
original-path saves, and frozen path/version checks keep newer edits dirty.
Coalesced keyboard/Auto Save persistence continues behind rejected UI errors;
superseded errors and late shutdown callbacks are cancelled.

Revert uses asynchronous real file reads and explicit dirty-discard confirmation,
preserving newer edits, cancelled requests, missing/binary/oversized sources and
document identity. Close/Reload/history dialogs use actual rounded descendant
clipping; scrolling keeps lower Settings controls reachable. Current Code-OSS
1.141.0 and main-source rounding/provenance remain the native visual reference.

Strict-cgo/race, genuine Node VSIX, owned HWND/GPU/disk, repeated private-root
idempotence and independent public-module checks gate publication. Release assets
reuse the successful exact-source CI packages. Free MSI/ZIP/CLI channels, signed
integrity metadata, automatic/manual mirror routing and stable launcher rollback
are retained; no paid certificates or installer tooling are required.

Full VS Code UI/API/debug/task/refactoring parity remains unfinished. Save
participants (VSIX onWillSave/waitUntil, LSP willSaveWaitUntil, formatting/code
actions on save), settings scopes/hot reload and per-resource exclusions remain
open. Default catalog is Open VSX; the native Gallery adapter requires separate
authorization for Microsoft's official Marketplace. Copilot keeps the accepted
official SDK/Language Server path; full official Copilot VSIX parity is not
claimed. New native product acceptance targets Windows amd64; existing shared
Metal regressions remain required.
