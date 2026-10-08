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

## Current-navigation acceptance outcomes (2026-10-08, native pending)

Immutable source 2d136492229d22fa502be4749d3048c1f256fbee fails the separate
Windows 2025 native GiB gate in CI 37703702524/job113072954796 after about91.6s.
Complete Windows validation and the actual GiB model test pass first. Its last
artifact is a correct single large-file result at phase11; the subsequent
large-navigated checkpoint is absent. No final phase/worker elapsed report was
captured, so the actual CI timeout cause remains unconfirmed.

Independent ignored diagnostics retain all165 pre-change non-test Go/module/
version inputs and the source bytes. The pre-correction archive is
.cache/search-smoke-audit-37703702524/before-acceptance-correction-r5/snapshot.json,
SHA256af5f1b267c0eb27217b3de6b7349750359ceb18ee2131eecaae0f2664973dac9.
They distinguish the failed immutable CI source from the later frozen three-file
keymap-acknowledgement change. Actual non-race public16 local GiB diagnostics pass
all six stages at144-DPI/1920x1230 in23.072s and, with a separately corrected
owned-window coordinate helper,96-DPI/1024x720/Scale1 in23.039s. The latter uses
the target HWND's actual awareness domain and real resize/GPU metadata; monitor
DPI remains144. The earlier r3 small-window helper failure at693x506 remains
preserved and is not rebadged as1024x720 proof. All owned Job descendants/PIDs
and private program/config/TMP are absent. These are pre-correction binaries,
not native evidence for the new acceptance source or proof of the CI cause.

A real-file CPU negative control proves an acceptance defect: the original
actor's30.0015612s request context expires, the original Verify closure returns
actual DeadlineExceeded at30.0125489s, and original phase12 stays12/failure=nil.
A real asynchronous NotExist open clears busy and also leaves that branch
pending. Exact original phase12 statement tokens/pixel/offset/quit guards are
independently checked. This control gates before invoking the original Verify;
it is not a slow-disk benchmark or a native90s reproduction. Held successful real
Verify receipts are correctly discarded after a newer model focus/query gesture,
and fresh actual query/Verify/file-backed Bytes succeeds. Original ignored
negative logs/receipts and r3/r4 source/binary/artifacts remain immutable.

The acceptance-only correction forwards existing requestOpen/validate/work/done
exactly once and observes the armed current path/query generation/navigation/
document/focus. Definitive current open/Verify errors are reported directly;
superseded receipts retain their existing protections and get a distinct cause.
It does not infer failure from the deliberately retained phase9 stale message.
Page errors belong to the matching acknowledged byte page; old errors before
replacement or while loading are ignored. Production model/actors/page APIs,
their queues/shutdown and30s/15s worker limits are unchanged. All thirteen native
phases, clicks, match counts, UTF-16/offset checks, completed GPU pixels/targets
and actual source-byte/hash checks remain. Checkpoint metadata adds elapsed time.

The90s watchdog now reads immutable atomic scalar/string progress and calls the
actual thread-safe Context.Quit, allowing normal actor/document/workspace defers
to run. The publisher checks timeout after storing its Context, covering a timer
that fires before the first View. The final source hash remains mandatory;
timedOut or elapsed>=90s rejects PASS afterward, including delayed timers. This
is a logical acceptance deadline and cooperative close: it does not prove a hard
90s exit if native startup never returns/publishes a Context or an OS hash read
never returns. The separate owned-process runner's120s Job bound is independent.

Focused public16/GOWORK=off/GOPROXY=off strict-cgo/race/shuffle/count3 passes
104.487s; no-cgo/count3 passes91.767s, including the original real30s deadline in
each repetition. Actual private9,437,197-byte files supply Run/Verify/Bytes,
NotExist and changed-file page errors. Tests count real forwarding boundaries,
hold focus/query/rearmed/cancelled receipts, verify fresh positive recovery,
ignore old messages/page errors, prove detached snapshots and exercise1000
concurrent timer/publication interleavings per repetition. Both complete vet
modes pass. No UI Run/GPU/native gesture is executed by these CPU checks.
New-source real GiB/default/96-DPI/WARP acceptance, exact-source full CI and
promotion remain pending; this fixture improvement does not declare the original
CI timeout fixed or alter public core v0.16.0/installed app v0.23.0.
