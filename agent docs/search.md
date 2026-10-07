# Workspace search contract

Source reference checked 2026-10-07: VS Code stable 1.140.0 / main
`8ce9c47a85608cc74351a6b0e60ef36979884661`.
[Search CSS](https://github.com/microsoft/vscode/blob/8ce9c47a85608cc74351a6b0e60ef36979884661/src/vs/workbench/contrib/search/browser/media/searchview.css)
defines the 26-pixel search input, 22-pixel result rows and source spacing. The
native sidebar uses these measurements with the existing Dark Modern surfaces.
It provides query/case/whole-word/regex controls, include/exclude fields, ignore
toggle, Search/Cancel, grouped highlighted results, wheel and keyboard navigation.
Ctrl/Cmd+Shift+F focuses search; Enter submits; Down/Up select; Enter/F4 navigate.
Clicking a result restores editor input even while the Search activity stays open.

## Execution and bounds

One worker and one latest pending request own traversal/search/navigation checks.
Input changes invalidate older generations, cancel active work and debounce for
200 ms. Each request has a 30-second deadline. UI publishes immutable Buffer
snapshots; snapshot text is joined on the worker, one file at a time. No file read,
regex scan, source hash or large-file seek is performed in an input/view callback.
Shutdown cancels work and waits at most three seconds for an uninterruptible OS read.

Traversal uses os.OpenRoot and batches of 128 directory entries, with no symlink
recursion or .git payload search. Separate limits: 50,000 files / 16 MiB returned
paths, 1,000,000 entries, 10,000 directories, 1,024 queued directories, depth 64,
8,192 compiled ignore rules, 64 KiB ignore files, 2,048 rules per file, 4,096-byte
query/filter/glob, 256 expanded filter patterns and 5,000 matches. Regex AST cost
is bounded before compilation. Open editor overlays are limited to 128 snapshots
and 64 MiB source; exceeding a limit is visible, never silently treated as complete.

Literal matching uses fixed chunks/overlap; insensitive literals fold Unicode
SimpleFold cycles with original-byte maps. It preserves Kelvin/long-s/Greek case
equivalence without copying an entire file. Standard Go regex runs against an
io.RuneReader, restarting from ReaderAt with a consumed preceding-rune context
to preserve anchors and word boundaries. Literal regex prefixes use chunked
candidate scanning plus anchored verification; this avoids a real GiB long-line
timeout seen in the initial plain-reader implementation. No maximum line length
is imposed. General regex complexity/I/O can still hit the visible request deadline.

Source-only byte positions and UTF-16 ranges accompany each bounded preview and
match digest. Open snapshots supersede disk, including ignored/new editor paths.
Disk file identity/size/time is rechecked after scanning; changed results are dropped.
Before navigation, a worker rechecks matched bytes and UTF-16 coordinates against
the actual frozen editor snapshot or owned file. UI verifies document version/
instance identity, focus sequence and navigation/query generation before selection.
Stale/reopened/edited results cannot change the newer caret or write source.
Large results use the existing file-backed byte viewport at the actual match offset.

## Patterns and scope

Include/exclude support slash/basename globs, **, classes and bounded brace
expansion. Parent/nested .gitignore plus .git/info/exclude use last-match priority,
anchoring, directory rules and negation; excluded directories are pruned following
[Git's documented semantics](https://git-scm.com/docs/gitignore/). Binary NUL headers
are skipped. Ignored open editor snapshots remain searchable through explicit filters.

This is Go/RE2 regex syntax, including multiline expressions; PCRE2 lookaround and
backreferences are not supported. Global Git excludes, .ignore variants, complete
VS Code settings/encoding/provider integration and result-tree collapse remain
gaps. Native workspace replacement is verified and published in v0.14.0;
[replace.md](replace.md) records preview/commit/save behavior, verification and
remaining global-undo/individual/diff/regex compatibility gaps. Full VS Code
search/replace parity is not yet achieved.

## Evidence

Standard regexp oracle/property/fuzz tests cover greediness, anchors, empty
matches, arbitrary lookahead, Unicode/CRLF, literal chunk boundaries and folding.
Git check-ignore supplies an independent ignore oracle. Worker tests hold receipts
to reproduce replaced queries, cancellation, immutable unsaved snapshots, stale
focus, UTF-16 selection and query/editor input isolation. A malformed pattern that
escaped the wrapper was found by fuzzing; raw-pattern validation fixes it and its
regression seed is retained. Two 15-second fuzz passes completed 1,327,043 and
725,966 executions before the final complexity bound.

Actual local 1,073,741,824-byte single-line literal search: 1.53 s, 2,147,492,044
bytes of measured search/position/preview I/O and 360,624 new Go allocation bytes.
These are one local run, not a cross-hardware performance guarantee. The native
16 MiB and 1 GiB -search-smoke gates pass real HWND typing/clicks, unsaved results,
case/word/regex/ignore, Unicode selection, actual disk-stale rejection, file-backed
tail navigation and unchanged source hashes. Selected-Unicode and large-navigation
GPU PNGs were visually inspected at 1920×1230 / 150%.

Immutable v0.13.0 source `d4a869e24c9acc2d93cd9aefb17f9303064423b2` passed all
five jobs in [CI 37547147426](https://github.com/neko233-com/gocode/actions/runs/37547147426),
including Windows console/GUI, Windows 2025 actual GiB and Mac Intel/ARM native
normal/1.5/2 drawable-density gates. The Windows 2025 race-enabled actual GiB
literal scan reported 16.8320796 s / 811,736 new Go allocation bytes and the same
2,147,492,044 measured I/O bytes. This is distinct from the local non-race result.
Mac character probes construct actual owned-view NSEvents; punctuation is kept
separate from virtual-key codes (`[` must not become Command). No global input
is posted. ARM 150% selected-Unicode, Intel 200% large-navigation and Windows
100% literal-result PNGs were visually inspected.
Windows 2025 actual GiB tail-navigation pixels at 1024×720 / 100% were also
reviewed: Read-only 1.00 GiB / byte 1,073,741,573 and the actual needle page.

[Publication 37548090734](https://github.com/neko233-com/gocode/actions/runs/37548090734)
reused the tested packages. Public released bytes passed native search alongside
existing acceptance, real signed automatic/direct/manual mirror integrity and
actual prior-source GUI rollback. The user's actual installed 0.13.0 GUI passed
the same search gate; selected-Unicode and Settings PNGs were visually reviewed
at 1920×1230 / 150%. See status.md for exact package hashes and installation scope.
