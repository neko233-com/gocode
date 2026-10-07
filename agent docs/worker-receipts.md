# Background acknowledgements under UI queue pressure

The native framework v0.16.0 processes UI dispatches while hidden/minimized and
bounds its pending queue at 1024 callbacks. A false Dispatch result may mean
temporary overload as well as shutdown. Product workers must not treat the first
rejection as permanent closure or drop an acknowledgement for committed I/O.

`internal/uidispatch.Retry` retains one callback in its existing caller, attempts
admission immediately, then retries with one 10 ms timer. It creates no worker
goroutine. The actor lifetime cancels both retry and an accepted callback that
has not yet executed; successful admission does not itself acknowledge work.
Workers continue to publish immutable captures, and only dispatched callbacks
change live model state. Short read/write/network request contexts cannot guard
a final acknowledgement because their timeout/cancellation may precede delivery.

File opens retain atomic transfer ownership and dispose an unadopted large-file
index on cancellation. Both transfer and the final busy/next-job acknowledgement
retry. File watch, search, workspace history, Git, language-server lifecycle and
terminal workers retain their own actor contexts rather than exiting on overload.
Workspace scan separates its actor lifetime from its 30 s scan deadline so a
cancelled scan still clears Explorer busy state and reports cancellation.

Large-file index progress retries within the reader lifetime. Each page request
has a separate delivery context: newer navigation cancels old read and receipt,
while the current 15 s read timeout can still clear loading. The delivery context
remains alive after admission until actual callback execution. Search debounce
likewise cancels an older pending receipt when the query changes. This prevents
superseded page bodies or timers from accumulating behind a full queue.

Update settings, verified apply completion and automatic ticks retry under the
update actor lifetime. Copilot startup installation of UI hooks, final chat result
and final chat-login result also retry; existing generation and document-version
guards remain authoritative. Transient completion/progress/status-only callbacks
remain best effort. Extension RPC entry admission continues to fail the request
before mutation when the native queue rejects it; it does not strand a worker or
claim a committed save succeeded.

CLI overrides use visited flags, so absent settings preserve stored policy,
explicit `-auto-save-delay=0` is rejected, and explicit empty/unknown modes are
rejected. Session overrides do not write user configuration.

## Verification

`worker_dispatch_test.go` uses genuine disk reads/searches/writes, an owned Git
repository and an owned stdio language-server process. Every test UI admission
first rejects, then holds a real worker receipt until the test UI thread applies
it. Coverage includes both open receipts, subsequent jobs, search/history survival,
cancelled scan acknowledgement, cancelled retry shutdown, real Git status/index
and genuine LSP hover bytes. The page test forces 20 real reads to reach a rejected
delivery, proves all superseded workers stop while admission stays closed, then
holds the current receipt and checks real page delivery and reader shutdown.
`internal/filewatch/retry_test.go` verifies actual Unicode/CRLF disk changes before
and after a rejected receipt. Fourteen actual FlagSet parse cases cover CLI absence,
single/both overrides, delay bounds, explicit zero and invalid mode values.

The final stronger page supersession check passes three workspace-linked
strict-cgo/race repeats (app 2.705 s, file watch 1.820 s). The associated original
open/search/history/replace/Git/LSP/watch cases, all new rejection tests and CLI
parsing pass three independent public-module strict-cgo/race repeats with
GOWORK=off/GOPROXY=off: app 9.160 s, file watch 5.086 s, uidispatch 1.253 s.
They use published core v0.16.0/source
5178f551a179351391af6eaab62f3f5cf150350e and the default public module cache,
private workspace cache/TMP/GOTMP paths and Git ceiling. The fixed log is
`.cache/worker-retry-public.log`. Workspace-linked vet passes; final combined vet,
full/native/source CI checks are recorded by the promotion ledger after all
concurrent actor tests are complete. No native GUI was run for these worker tests.

Restricted sandbox runs of the real Git/history cases are not validation passes:
private `.git/config` writes and atomic replacement in sandbox AC/Temp were denied.
An early history assertion records the actual unsaved replacement status before
Ctrl+Z/Escape; the original saved-state/cancel guard remains intact. The approved
private-temp independent check above proves real Git writes and actual history
replacement/undo/redo without changing those guards.

The production native file/extension manager adapters also accept an injected
dispatcher for real headless worker checks. Three independent public-core16
strict-cgo/race/shuffle repeats pass (1.638s), followed by full vet (9.491s).
Actual chooser completion, Save As disk/snapshot/path/hash/URI receipts and real
VSIX install/disable/enable/error results survive rejected admission and clear
busy without repeating the I/O. Cancel-before-start creates no file, permanent
rejection stops on actor shutdown, and an older Save As snapshot cannot emit
save for newer edits; that new dirty path is rescheduled for automatic saving.
Private roots and atomic-write temp files are clean. Final full/native/source CI
promotion remains in the current status ledger.

Further real-writer review reproduces a Save As overlap in both receipt orders:
an extension could save the original URI while a new-path copy waited for its
UI receipt, then falsely clean the new URI even though its disk held older text.
Nonempty shared-save admission and queued starts now exclude active file actions;
receipt/start checks also require the frozen path to match the owned document.
Three public-module strict-cgo/race/shuffle repeats pass (5.362s) with actual
Node VSIX, both held receipt orders, a completion-started Save As ahead of an
already queued job, frozen-path hash preservation and subsequent genuine latest
new-path save. The original negative control fails both orders in 0.100s.

The first full public-module native run exposes an async-open fixture timing
assumption. New UIWake starts the hidden read before View adds its 32px Opening
toolbar; old tab bounds click the toolbar rather than the tab. Diagnostics prove
sequence 4->4/editing=false. Phase 4 now waits for three pending-state frames
before resolving its native replay bounds. Three replays prove 4->5/editing=true,
then pass the unchanged stale-VSIX/disk/pixel/source/timeout guards. Actual-disk
worker regressions pass three strict-cgo/race repeats (1.514s); no production
focus guard is changed. Initial failure/replay logs remain in the status ledger.

Auto Save and built-in keyboard persistence now share a bounded settings writer:
one coalescing disk writer and one latest-error publisher, each with a capacity-one
queue. A new selection cancels older rejected/admitted failures; shutdown cancels
UI delivery immediately while draining the latest disk value within the original
3s budget. A blocked error receipt cannot delay subsequent writes. Public-core16
strict-cgo/race/shuffle tests pass three repeats (app 5.803s, uidispatch 1.251s),
including both real malformed-directory writers, 512 coalesced selections, latest
disk content while UI admission stays rejected, stale callbacks after success/
stop, idempotent shutdown and no duplicate/temp configuration files. User settings
are not used by these tests.

Terminal startup/output/final/error receipt retries now belong to each tab's
lifetime. Closing a tab cancels rejected or admitted-late callbacks and completes
its pending request once, even while the controller remains alive. Preparation
also combines request/tab cancellation; the Session itself keeps the tab lifetime
so a natural process exit still publishes its final receipt. Three public-core16
strict-cgo/race/shuffle repeats pass (4.107s): 16 actual ConPTY processes across
two eight-slot batches, rejected startup/output/error, held startup after close,
real Unicode echo, real exit code 7, per-tab worker completion and PID termination.
An isolated old-controller-lifetime overlay fails the original closed-worker guard
(2.088s). Logs reuse .cache/terminal-dispatch-{public,negative}.log. No GUI is used.
