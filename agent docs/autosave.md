# Native Auto Save and Revert File

Auto Save and Revert File were disabled menu placeholders in public v0.22.0.
Public and installed v0.23.0 implements both on Windows in native Go, sharing the real save actor
and external-file watcher. It follows MIT Code-OSS 1.141.0 at
2a59476c9bfcb90b3ddc372c36762471b7dfad1c, filesConfigurationService.ts and
files.contribution.ts/fileActions.contribution.ts under the existing Microsoft
source/license provenance. Desktop defaults to off/1000ms. File/Auto Save toggles
off/afterDelay; Settings selects four modes and delay. onFocusChange includes
window deactivation; onWindowChange requires a distinct native window event.

The UI owns at most 128 dirty scheduling records. One timer worker accepts one
coalesced deadline and at most one unacknowledged dispatch; it never reads mutable
buffers. Delay resets on changes. Focus modes follow actual document/group/editor
focus or native activation, never capture/keyboard cancellation. Owned file/close/
reload/history dialogs do not create new focus saves. Existing delayed saves
pause during those interactions and resume afterward.

One automatic job at a time enters the existing frozen-snapshot writer when
manual queues/close barriers are idle. Existing 32-job/30-second/8MiB save,
disk hash/read-only/missing/atomic-replacement limits remain. Only the exact saved
version becomes clean and emits existing didSave/onDidSave. Newer edits stay
dirty and are scheduled again. Same-version failures stop retry until a new edit
or policy/manual action. Untitled buffers never open Save As automatically,
file-backed large buffers stay read-only, and disk conflicts require review.
Editable 2–8MiB files can save despite the lower language/extension service cap.

User gocode/autosave.json holds files.autoSave/files.autoSaveDelay with a 4KiB
input limit and delay 100..600000ms. Missing config defaults off; malformed config
is reported without enabling saves. Semantic equality preserves bytes/mtime.
One coalesced writer uses fsync/close/atomic replacement and cleans temp files.
Shutdown cancels deadlines/pending automatic jobs before existing disk workers
stop; already accepted atomic writes cannot be undone retroactively. A clipped
scrollable Settings viewport keeps lower update controls accessible.

Revert uses real asynchronous reads and explicit dirty discard confirmation.
It freezes path/identity/version/hash and prioritizes the selected target within
128 watch slots, including the 129th open file, then restores normal watches.
Old normal/cancelled receipts, newer edits, closed/renamed resources and changed
baselines cannot discard newer text. Cancel, deletion, binary/oversized content
and shutdown clear busy and retain buffers. Untitled/large Revert is disabled.
The Reload confirmation can cancel an already pending read. Close/Reload modal
surfaces use actual rounded descendant clipping.

## Published and installed evidence (2026-10-08)

Immutable application source 47623823dd88fbb45422d096b545631be7e6b77b uses
independently fetched public core v0.16.0/source
5178f551a179351391af6eaab62f3f5cf150350e, GOWORK=off/no replace.
The full independent Windows default-three strict-cgo/race script passes
(main 423.075s/coverage 33.6%). [Exact-source CI 37678679723](https://github.com/neko233-com/gocode/actions/runs/37678679723)
attempt 1 passes all five platforms; Windows 2022 main is 442.628s/33.7% and
Windows 2025 is 512.883s/33.7%, with original scenario/process/pixel guards intact.
[Publication 37682493872](https://github.com/neko233-com/gocode/actions/runs/37682493872)
tags the same source and reuses those packages; metadata is at
05404af95162921c89617d314b7ff6edb7feb60e.

The real released-byte check exits 0 with all 25 ordered console/GUI gates,
signed automatic and manual direct GitHub full ZIPs, manual ghfast.top full ZIP
and genuine v0.4.0 GUI/VSIX rollback. Windows ZIP SHA256 is
05cff9bb4179f4316c28bfa8cd76f9eb28bd37e5e22cda02a2332ed86e4fe9e3.
The actual user updater then selects v0.23.0/exact source through direct GitHub.
Installed console and GUI Auto Save/Revert gates pass all 66 phases, alongside
existing menus/keymaps/VSIX/Git/terminal and the real GiB shared split. Actual
150% File/Settings/rounded Revert/extension-detail GPU captures were inspected.

Both installed minimized reports prove the following while actual IsIconic=true:

| Mode | saveId / didSave | Dirty | View calls | GPU Submitted |
| --- | --- | --- | --- | --- |
| onWindowChange | 1 / 1 | false | 2->2 | 2->2 |
| afterDelay | 2 / 2 | false | 2->2 | 2->2 |

Restoration occurs afterward. Reports reuse
.cache/installed-auto-save-minimized/current.json and
.cache/installed-auto-save-minimized-gui/current.json; source/release/install logs
and payload hashes are pinned in status.md. The actual user's keyboard.json and
autosave.json remain absent, with update config/PATH and stable shortcuts intact.
Native GPU shadows and independent extension-detail scrolling are outside v0.23.0.
Save participants, full settings scopes/hot reload, format/code actions on save
and per-resource exclusions remain open.

## Development evidence before promotion

The following checkpoints retain the original failures and pending states from
development; the published/installed proof above supersedes those pending gates.

2026-10-08 local real-config tests pass three strict-cgo/race repeats (1.141s).
Workspace-linked real-writer tests pass three race/shuffle repeats (3.300s):
Unicode/CRLF debounce, held acknowledgement/new edits, tab/terminal/group/window
focus, same-version failures, off/close/reopen/shutdown, real large/service-cap
separation, 128 slots/reclamation/manual priority, no-writer disarm, modal negative
controls, delay input and one-writer config coalescing/cleanup. Real watcher/Revert
plus existing conflict/save tests pass three strict-cgo/race/shuffle repeats
(33.309s), leaving a clean private workspace temp root. Final native GPU controls,
full regression/public dependency/source CI/release/installed validation remain
pending; public and installed v0.22.0 remain unchanged.

Full settings scopes/hot reload, VSIX onWillSave/waitUntil and LSP willSave/
willSaveWaitUntil, format/code actions on save, per-resource exclusions and full
product parity remain open. Existing service/API proof does not imply these APIs.

Final candidate's 66-phase native Windows gate passes actual File checkmark,
Settings four modes/delay, five real save receipts/Unicode CRLF disk writes,
editor-to-Settings/terminal/tab blur, independent window activation with capture/
keyboard negatives, no duplicate saves, dirty close cancellation, Revert cancel/
confirmed disk restoration without a fake save event, external conflict and
untitled preservation. Actual 150% File/Settings/rounded Revert PNGs were inspected.
Three race/strict-cgo idempotence repeats (121.330s) each run the whole gate twice:
six complete native runs leave empty private scratch, unchanged user settings and
the same six evidence filenames. ConPTY is verified once before the isolated
child processes and reused by explicit path; the earlier sandbox/cold-cache
startup timeout is not claimed as a passed check. Startup failure now reports
immediately instead of waiting for the original 60-second guard.

Real Node VSIX tests pass three race/shuffle repeats (1.418s): automatic and
actual JS Document.save share the real writer; old snapshot acknowledgements do
not falsely save newer text; duplicate ticks generate no event; off preserves
edits, stale manual versions are rejected, and saveIds 1/2/3 match real disk/
Unicode CRLF/document events in order. Final writer/watcher/keymap/VSIX repeats
pass together (37.648s), and delay-field/modal keyboard priority passes three
repeats (1.099s). Workspace-linked full no-cgo suite passes (app 24.502s) after
local loopback access and a private-cache Git discovery boundary are provided;
the initial restricted-network and parent-repository negative-control failures
are retained in ignored logs. Vet, workflow lint/ShellCheck and both Python
policies pass. Full default-three Windows workspace regression is running;
default script still validates GOWORK=off, with explicit -UseLocalFramework only
for development. Independent public module/source CI/release/install remain open.

Latest main is rechecked as 9a89cf962f1d34463974058e9ef2b59f2bc33f15 on
2026-10-08; stable remains 1.141.0. Main roundedCorners.css SHA256
461987fb6faa588a3c2869ae671bb98c23675bdbaf1fdccce12ca1b52fdd407b and editorBorder.css
eaaeb56b733712c00c0cce7c3579201cb00188283b50e7f95a6b93b8e6614ffe exactly match
the recorded stable files. notificationsDialogs.css was also inspected; native
dialogs remain a partial translation, not a claim of complete pixel parity.
CLI -auto-save/-auto-save-delay are session overrides, and new/replacement windows
inherit current policy without waiting for the asynchronous settings writer.

Full first workspace Windows run fails only the old update-Settings native test:
its fixed click coordinates land inside the newly added Auto Save section. Actual
control coordinates are corrected, retaining source/config/window assertions;
three targeted strict-cgo/race repeats pass (22.244s). The full final run remains
required. A separate source audit then finds that core Dispatch only drains on
Draw and stalls worker receipts while actually minimized/hidden. Its UIWake fix
and a separate actual-minimized disk/clean/event/GPU-idle acceptance are being
implemented; synthetic activation and six earlier visible runs are not evidence
that minimized Auto Save works. Public/installed v0.22.0 remains unchanged.

Separate actual-minimized Auto Save acceptance now passes three strict-cgo/race/
shuffle repeats (25.572s), each running twice: six owned windows prove IsIconic,
onWindowChange disk/saveId1/didSave1/clean, then afterDelay disk/saveId2/didSave2/
clean while still minimized. View calls and native GPU Submitted remain equal to
the minimized baseline, allowing previous submissions to complete. A 300ms
quiet interval produces no duplicate saves; restoration and new GPU work occur
only after both saved states are verified. Private scratch is empty, settings
marker/entries are unchanged and repeated evidence has only current.json.
Console/GUI flags and CI JSON artifact wiring preserve this actual native proof.
Final full workspace/public-module/source CI/release/install remain pending.

Final frozen bd2f48c candidate passes the whole independent published-core16
Windows default-three script: all shuffled strict-cgo/race packages (main
423.075s, model coverage 33.6%), vet/build/PE and every console/GUI native gate.
The public minimized report proves two actual disk/clean/save receipts with
View/GPU Submitted 2->2, followed by restoration. Current six GPU PNGs reuse the
same names; actual 150% rounded Revert/checked File/Settings states were inspected.
The script's private validation root is empty afterward. Auto Save and keyboard
settings use a separate latest-error publisher so rejected UI admission does not
delay the newest real config write; new selections/stop cancel late obsolete
errors. Review also repairs real Save As/original-path save overlap, with frozen
path checks before queued starts and receipts. Exact-source app CI 37676083407,
public released bytes and installed selection still require confirmation.
