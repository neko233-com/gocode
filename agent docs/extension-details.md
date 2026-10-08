# Native extension details and exact catalog queries

2026-10-08: this is the unpublished application v0.24.0 candidate following
public/installed v0.23.0/source 47623823dd88fbb45422d096b545631be7e6b77b.
It uses independent public core v0.16.0/source
5178f551a179351391af6eaab62f3f5cf150350e with GOWORK=off/no replace. The
application checkout base is bad4e20f9b59a833e0e00bfd9288758925f4c5b8 plus
uncommitted functional changes. No v0.24.0 tag, exact-source CI promotion,
released-byte installation or complete VS Code parity is established here.

## Fixed upstream source and native layout

The measured reference is stable VS Code 1.141.0 at immutable
2a59476c9bfcb90b3ddc372c36762471b7dfad1c:

- [extensionEditor.ts](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/vs/workbench/contrib/extensions/browser/extensionEditor.ts)
- [extensionEditor.css](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/vs/workbench/contrib/extensions/browser/media/extensionEditor.css)

The existing Microsoft MIT provenance remains in assets/code-oss. Read-only
copies are .cache/vscode-menu-reference/stable-extensionEditor.ts and
stable-extensionEditor.css. Their SHA256 values are respectively
940a0eab9bf213f9418e6fd30e8de990cfd91dc076020036a8bf5dd90012b120 and
276f9e86b88950c9ef4382d91d23dbabe5801731cf3cc2e088743cdcbb363f25.
These measurements are translated into native Go elements; the application
does not use the upstream README webview.

The editor keeps its title/close row, metadata/actions header and 36-DIP
navigation bar fixed above an independently clipped native Viewport. The
navigation bar contains 35-DIP controls and a one-DIP selected underline;
its following separator is accounted for separately. Details and Feature
Contributions retain their full identities even when their labels are fitted.
The header follows the upstream 20-DIP top/12-DIP bottom inset and 12-DIP icon
gap. Its generic extension icon is 128 DIP, decreases to 64 below 430 DIP and
is omitted below 220 DIP. Narrow action rows stack rather than overflow.

The detail body owns scroll, measured content extent and keyboard focus apart
from extension-sidebar search/list scroll and the underlying document. Wheel
input is admitted only inside its actual native viewport bounds. Unmodified
Up/Down, PageUp/PageDown and Home/End navigate the body; Escape closes it.
Characters consumed while detail has focus do not modify the document.
Closing, selecting another extension or changing the detail tab resets the
corresponding state. Pointer focus elsewhere returns routing to that surface.
The existing VS Code Ctrl+W and JetBrains Ctrl+F4 close paths operate on the
detail editor; this does not claim every VS Code/JetBrains shortcut is covered.

## Native text and bounded content

[extension_detail.go](../extension_detail.go) and
[extension_detail_view.go](../extension_detail_view.go) build and cache a plan
from the selected metadata/status/commands and actual native width. Width or
metadata changes rebuild the plan and clamp scroll to the new extent. Native
ui.MeasureText supplies proportional-font widths; UTF-8 repair and Unicode
grapheme segmentation precede wrapping. Whole clusters are retained, including
combining marks and joined emoji. Text that cannot fit even one whole cluster
is omitted/truncated explicitly instead of painting beyond the viewport.

| Bound | Implemented behavior |
| --- | --- |
| 64 KiB per text input | Prefix limited before segmentation/shaping; invalid UTF-8 and NUL repaired within the same bound |
| 512 wrapped description rows | Explicit truncation notice; identifier/version/status have smaller per-field row budgets |
| 4096 contribution commands | Body plan capped; additional contributions produce a display-limit notice |
| 1024 visible rows | Only intersecting rows plus bounded overscan are materialized; exact spacer heights retain the full scroll extent |

Each contribution has a fixed 66-DIP row. Titles use at most two measured
lines, command IDs are fitted separately, and stable keys identify callbacks
at the end of a long list. The native tree limit is a row budget, not a claim
that the entire element tree contains at most 1024 elements. Cached plans
avoid reshaping unchanged metadata every frame. Exponential prefix probes
followed by binary search avoid shaping the remaining 64-KiB paragraph for
every short wrapped row.

## Real fast-tab navigation defect and correction

The initial owned Windows native acceptance failed after 55.107s in
.cache/extension-detail-native-initial.log. The 54.994s diagnostic failure in
.cache/extension-detail-native-diagnostic.log showed the selected contribution
tab with 2001 plan rows and maxScroll=131602.67, while scroll stayed zero across
actual frame counts 255 through 2215. The focused-key diagnostic then failed
after 8.487s: key=35/modifiers=0, focused=true, consumed=true, afterScroll=0.
Its raw log is .cache/extension-detail-native-key-diagnostic.log.

The tab callback resets the detail state. A subsequent End message can arrive
before the next Draw establishes the new content extent. Routing the key using
that zero extent consumes the user's input without moving scroll; later layout
cannot recover the lost key. The production prepareExtensionDetailViewport
now resolves the selected metadata/commands against actual viewport bounds on
the same UI turn before wheel or navigation-key handling. It preserves the
newly moved scroll when the next View builds the same plan.

TestExtensionDetailImmediateTabNavigationBeforeNextLayout covers this ordering
without starting a native window. The same original real native test then
passes with count=1 in .cache/extension-detail-native-fast-tab-fix.log, total
16.879s: each test runs two private native child processes. The earlier failures
remain retained; the callback/frame/process deadlines are not relaxed.

## Native acceptance and idempotence

[extension_detail_acceptance_windows.go](../extension_detail_acceptance_windows.go)
creates a real private ZIP VSIX, installs it into the owned fixture and starts
the public framework's isolated Node extension host. The manifest contributes
2000 commands and activation registers their real callbacks. Native navigation
reaches the final command, details.command.1999, and its click invokes the real
Node callback, which must return executed:1999. This is 2000 registered command
callbacks plus a verified final-command execution, not 2000 executions.

The ten original gates cover actual native font widths and bounded wrapping,
pointer/wheel with fixed header, outside-wheel isolation/other activity,
resize rewrap without document editing, final contribution/Node execution,
actual disable/enable worker persistence, bottom-scroll resize clamping, both
keymap page/close/reset paths, and unchanged real disk/D3D12 submissions.
Snapshots read UI-owned state through admitted callbacks. File/ZIP/process/
report/PNG work remains in the worker; it is cancelled and joined at shutdown.

TestNativeExtensionDetailViewportAndIdempotence repeats the owned process twice
under private TMP/TEMP/TMPDIR and APPDATA, with a kill-on-close Windows Job
Object owning its process tree. Each successful child must leave scratch empty
and preserve the one pre-existing settings marker byte-for-byte. Its evidence
directory must contain exactly four nonempty fixed artifacts: current.json,
details.png, narrow.png and contributions-bottom.png. Repeating overwrites
these names and creates no accumulating screenshot or runtime directories.
The report is limited to 4096 bytes and published by a prepared temporary file
and rename after all gates pass. Original source bytes/dirty state are checked
throughout; the final report records the real disk SHA256 and native PID/backend.

The final count=3 strict-cgo/race/shuffle acceptance also passes in
.cache/extension-detail-native-three.log: three test repeats take 13.97s,
13.38s and 13.44s, total 41.877s, shuffle seed 1791409813952344500. Each repeat
runs two owned native children, so all six complete the same ten original
gates and four-file/scratch/settings assertions. Headless
TestExtensionDetailNativeMeasuredWidths explicitly skips when native shaping
has no owned Run; that skip is not native font evidence.

After the real fast-tab correction, independent public16/GOWORK=off focused
strict-cgo/race/shuffle/count=3 tests also pass in 1.910s, seed
1791410314837020100 (.cache/extension-detail-final-race.log). The real private
VSIX/Node fixture and immediate-tab navigation regression pass each round;
native measured widths explicitly skip in each headless round. CGO_ENABLED=0
count=3 passes in 0.151s (.cache/extension-detail-final-nocgo.log). The validation
owner also confirms full vet EXIT0 separately with no-cgo and strict-cgo; their
empty output emitted no retained vet log file. These checks accompany the
actual six native children and standalone font/GPU proof below.

The separate console -extension-detail-smoke then exits 0 in 5.562s, PID 224564,
with its owned process/job closed. .cache/extension-detail-fixed.log records the
pass. The original report is preserved byte-for-byte at
.cache/extension-detail-pre-version-bump/current.json. It contains all ten gates, 50 description plan
rows, native font width 51.149414 and maximum measured line 932.9214 within
viewport width 988 DIP. It records 2000 manifest commands, actual execution of
details.command.1999, bottom scroll 131602.67, direct3d12 and 25 submissions.
Real disk SHA256 is
d79793cb0662042ea31b20a1f6d7c1189035f41975190a2bb3368c194527e52b.
Private TMP is empty; APPDATA contains only the original unchanged 37-byte
settings marker. The report's version is 0.23.0 and source is development:
this is the uncommitted v0.24.0 candidate, not an immutable released v0.24.0.
The later source CI/release must establish that exact immutable identity.

The pre-version-bump snapshot contains exactly these four original artifacts:

| Artifact in .cache/extension-detail-pre-version-bump | Bytes | SHA256 |
| --- | --- | --- |
| current.json | 901 | 2368c649db148d5119e0a917e035e779bdb4978f9996f581f4aefefd487179d1 |
| details.png | 620597 | 59d3aa6735f41ed7b7d231c34ef82c1de47e78750d10491e0c2115b82e650eb6 |
| narrow.png | 429975 | ee873c49ad14de8d9b18cfcd19cc4c0bb3761d77f8ae5beb90bc23d5bcf3ab09 |
| contributions-bottom.png | 201693 | f5ca9038f4d8f150bb77bbfb3e6bc7c212f510a7ca0d595167831ca41fa79c63 |

.cache/extension-detail is the reusable latest-evidence directory. A subsequent
Windows suite can replace it with version 0.24.0 candidate reports; the original
0.23.0/development identities and hashes above belong only to the preserved
pre-version-bump snapshot. Such a replacement does not change the six-run or
standalone historical proof and is recorded separately after its real outcome.

Root actually viewed all three GPU PNGs and the JSON. Wide/narrow captures show
the fixed header/navigation bar, independently clipped/wrapped body and active
card rounding; body text does not escape the current content clip. The bottom
capture visibly reaches contributions 1993-1999 with the scroll thumb at the
bottom. The generic icon remains an observed limitation, not per-extension
icon/README parity. These captures verify this fixture, not every extension,
window width or upstream VS Code pixel.

The first standalone attempt is a separate ignored-launcher failure, preserved
in .cache/extension-detail-fixed-hidden-launch-negative.log (PID 216760,
exit status 2). HideWindow=true supplied SW_HIDE to the native HWND, preventing
the first visible Draw and hitting the 60-second guard. Replacing that ignored
helper launch setting with CREATE_NO_WINDOW hides its console while allowing
the owned native window to draw. It then passes; product source and original
50/60-second fixture/process guards are unchanged. This failure is distinct
from the earlier real fast-tab extent defect and the six successful native
repeat children.

## Exact catalog IDs and actual service evidence

[extension_catalog.go](../extension_catalog.go) classifies ordinary search,
publisher.extension and @id:publisher.extension with bounded input. Installed,
enabled and disabled filters stay local, can combine with an exact ID, and
retain their existing filtering semantics. Exact IDs compare case-insensitively.
Unknown filters/malformed explicit IDs become bounded errors. The sidebar uses
this classification instead of suppressing every query containing @; it also
rejects unrelated stale exact-ID cards.

Actual Open VSX free-text query golang.go returns a legitimate empty response.
Named metadata first tries win32-x64/latest, then universal/latest. A returned
canonical identity/version is reread through its immutable platform/version
endpoint; identity, version and platform must match before a result is offered.
The selected install path retains archive identity/version and existing size/
HTTPS/download verification. Superseding a query cancels its request and any
obsolete rejected/queued UI receipt. A current failed request ends busy with its
own error; 404 on both latest platform endpoints is a legitimate empty result.

The actual unpublished no-GUI CLI exits 0 for golang.go and @id:GoLang.Go,
resolving canonical golang.Go 0.56.1/universal and its fixed version download/
SHA256 URLs. Ordinary golang still resolves tooltitudeteam.tooltitude 1.56.5,
toga4.go-tdt-outline 0.0.2 and ZencoderAI.zencoder 3.85.9007/win32-x64.
Fixed reports are .cache/extension-query-live-cli-{golang-go,id-golang-go,golang}.json.
All report installed=false; these live checks download/install/execute no
registry VSIX. The installed v0.23.0 empty-query report remains historical
evidence and is not rewritten as a package or network failure.

Authorized/private Gallery exact queries use ExtensionName criterion 7, with
Target=8 and ordinary SearchText=10. Those values were read from stable
2a59476c src/vs/platform/extensionManagement/common/extensionGalleryManifestService.ts,
blob 53be55bdffd20fc62daa11f5be27db0922087a6a. Its retained source SHA256 is
695d8b00d1bf8bb22c382d0e919858ef31ccfb69206d3251704a30368e98f8a9.
Native gocode identity, same-authorized-host HTTPS query redirects and existing
page/version/response/download/archive bounds remain enforced.

.cache/extension-query-validation.json records the three exact query-file
hashes, public v0.16.0 module sum, actual CLI bytes/base-head-dirty stamp and
independent checks. Focused strict-cgo/race/shuffle/count=3 passes in 13.819s;
no-cgo passes in 3.791s; vet/build/diff checks pass and private validation temp
is empty. Genuine TLS tests cover Windows/universal fixed-version resolution,
404, cancellation, platform/identity/version/host/response limits and obsolete
worker receipts. A private authorized TLS Gallery actually downloads/validates/
installs its generated VSIX and executes its isolated Node callback. Initial
sandbox loopback/VCS-read failures are retained separately from successful
permission-correct runs. These are focused transport/process checks, not the
forthcoming full application CI or installed-byte proof.

## Prepared release and installed checks

The ignored v0.23.0 release/install helpers and 25-gate native marker are saved
as byte-exact .v023.snapshot files before preparation. The original check.exe
and check-installed.ps1 remain unchanged. The separate check-v024.exe adds
console extension-detail and gui-extension-detail, giving 27 ordered gates;
each executable has an independent fixed four-file report directory and a
report bound to its actual completed PID/version/source. Pure file tests reject
the old 25 gates, either missing new gate, incorrect order/root/source/console
or GUI hash, changed payload bytes and bounded/cancelled writes. Race/strict-cgo
three repeats pass 1.757s and no-cgo three repeats pass 0.596s; vet and helper
build pass using public16/GOWORK=off. These are integrity tests and preparation,
not native release or installed v0.24.0 success. The original 8m release limit,
signed full-byte download/rollback checks and installed 60s owned-PID guards
remain unchanged. Fixed .cache/live-release-check/v024-preparation.json records
helper hashes and the preserved v23 evidence; actual release/install runs remain
pending.

An independent review of these ignored helpers finds a count-only inner detail
gate check: ten wrong, duplicate or reordered names could pass, and the first
PowerShell checker accepted an unknown name. That negative control and seven
pre-correction helper snapshots are retained. Revision R2 now requires the exact
ten case-sensitive gate names in order in both Go and PowerShell, in addition
to all original PID/source/root/payload/order guards. The native execution,
27 outer gates, 8m total limit, signed downloads, rollback and installed 60s
guards are byte-identical after their validation boundary. Pure strict-cgo/race
three repeats pass 1.830s, no-cgo 0.680s; all 17 PowerShell negative/positive
cases pass three repeats in 0.855s, both scripts parse and scratch is empty.
The source-bound preparation record is
.cache/live-release-check/v024-preparation-gates-r2.json. It explicitly records
nativeExecuted=false and no release source; this stronger helper is still only
prepared until a corrected immutable source passes CI and real released bytes
are published and exercised.

## Remaining scope

The Details body currently shows actual manifest/catalog description and
identity/version/status. It does not render the extension README, CHANGELOG,
Markdown images/links, dependencies, extension packs, marketplace ratings or
arbitrary contributed feature editors. The generic extension icon does not
claim actual per-extension icon parity. The painted scroll indicator currently
provides position feedback; it does not claim upstream scrollbar-drag parity.

Default live service evidence is Open VSX, with authorized/private Gallery
fixtures and local VSIX compatibility kept distinct. Microsoft's
[Marketplace FAQ](https://code.visualstudio.com/docs/supporting/FAQ#extensions)
is recorded in modern-ui-and-gallery.md: separate official service authorization
is absent. This milestone grants no Microsoft Marketplace access, establishes
no arbitrary/full extension API compatibility and does not complete official
Copilot VSIX, full VS Code UI/debug/tasks or README/icon parity. The accepted
official Copilot SDK/LSP integration remains the previously recorded path.

## Final independent Windows candidate regression

The complete public16/no-replace/GOWORK=off Windows script uses default Repeat=3,
strict-cgo2/race/shuffle/p1 and the unchanged 12m package/process/scenario guards.
It exits 0 in 833.454s, main 498.701s/34.6%; raw latest log is
.cache/app24-public-windows-final.log. All packages, vet, builds, AMD64 PE and
console/GUI native modes pass, including detail viewport/management, the real
2000-command private VSIX, both keymaps, Auto Save/minimized receipts, File/shell,
actual Git/file-watch/terminal/language, groups/tabs and bounded large readers.
The original 765.457s failed run and stale-pointer PNG remain immutable. The two
repaired fixtures wait for three actual GPU completions and stable-key bounds,
without changing production main or loosening real click/pixel/disk/time guards.

Independent no-cgo shuffle/count3/p1 full tests pass 170.494s (main 80.678s),
.cache/app24-public-nocgo-final.log. Owned scratch is empty; seven private Go
telemetry files are removed after process closure under exact path/type/boundary
checks (.cache/app24-nocgo-cleanup.json), leaving settings empty. Public16 exact
module origin/sums and go mod verify are rechecked; source gofmt (241 nonignored
Go files), both workflow/actionlint/ShellCheck and all eight PowerShell AST checks
pass. Source/cleanup/actual four detail artifacts are bound by
.cache/app24-full-validation.json. Root inspects all three latest actual GPU
captures, now reporting version 0.24.0 and checkout base bad4e20; the pre-bump
snapshot and hashes remain separate. These are unpublished dirty-candidate
checks. Exact immutable application CI, publication, 27 released-byte gates and
actual-user installation remain required before promotion.

## Immutable CI failure and synchronization correction

Source 1549cac2d112bb8050b663f18c9e3cc9cc56760d is immutable and untagged.
[CI 37698299244](https://github.com/neko233-com/gocode/actions/runs/37698299244)
attempt 1 passes Ubuntu, macOS Intel and ARM, but fails Windows 2022 and 2025.
Their main package reports 485.330s/34.6% and 552.160s/34.6% respectively;
the aggregate 12m alarm does not fire. All six native detail repeats report
the actual public16 resize-lag error at resize-description. Windows 2025 also
reports Access denied on the geometry request's atomic replacement. Later
Windows GiB/services/MSI/packaging steps are skipped. Their diagnostic artifacts
are not tested release packages. No source tag or release is created.

The corrected capture helper waits for three real GPU completions after the
action, checks the current owned client dimensions before and after capture,
and retries only the known public16 resize-lag error or a changing image size.
Other capture/identity failures remain fatal. The existing 50-second acceptance
context and 60-second child guard remain. Reports record actual completion,
image size and resize retries for each capture stage; layout state alone is
never accepted as proof of a newly rendered image.

Both directions of the private geometry observer use bounded 4KiB reads with
FILE_SHARE_READ/WRITE/DELETE and cancellable atomic replacement. The original
10-second context, 32-request budget, sequence/PID/view-generation/completed-frame
checks, actual USER32 clicks and unchanged production main.go remain. Actual
held-reader negative controls demonstrate that adding delete sharing alone
does not guarantee MoveFileEx replacement of an open Windows destination.
Known sharing/lock/access-denied errors retry only within the existing context;
unknown failures are immediate. Cancellation preserves the previous immutable
request and removes the owned .pending file. This is test transport, not a
change to user-file saving or a claim that arbitrary occupied files can be
overwritten. Fixed failure logs remain separate from corrected local evidence.

An initial corrected targeted run preserves a further fixture failure in
.cache/app24-ci-fix-native-first-failure.log (main 150.775s). The three normal
workbench repetitions pass at actual 96 DPI/small window, but two detail runs
reach their original deadline because direct test-only focus dispatch is
followed by PageDown before the new viewport exists. An old in-flight completion
can satisfy a simple frame>previous check. The fixture now requires its exact
profile/Details tab and nonzero content/rows after the focus receipt, then three
actual GPU completions before the unchanged USER32 key replay. Product main,
extension detail UI and keyboard behavior are byte-identical.

The final targeted three-repeat public16/GOWORK=off strict-cgo/race/shuffle run
passes with debug=1, actual DPIUNAWARE 96 DPI and small-window conditions, main
66.068s/whole 72.023s, seed 1791415409992643500. Fixed log:
.cache/app24-ci-fix-native-final-second.log. The normal workbench runs take
10.57/8.34/8.30s; each detail repetition has two actual HWND/Node children and
takes 14.05/11.92/11.80s. Original ten gates/2000 commands/two keymaps/four fixed
artifacts and private scratch/settings/Go temp cleanup pass. This focused proof
does not replace the complete default-three script or forthcoming exact-source
Windows CI. Publication and actual-user installation remain pending.

The corrected complete default-three public16/GOWORK=off script then exits 0
in 837.6336332s, main 486.174s/34.7%, debug=0/default adapter policy.
.cache/app24-ci-fix-full-windows.log retains every package/vet/build/AMD64 PE
and standalone console/GUI native result; the original 12m aggregate package
alarm and per-process/dialog/50/60s/real pixel/disk guards are unchanged.
The focused non-GUI regression also passes three strict-cgo/race repeats in
10.110s, including actual held-reader replacement/cancellation and unchanged
production-source overlay compilation.

Root visually inspects details/narrow/contributions-bottom at actual 150%.
The report's three capture floors 6/12/18 are met by actual completed 6/12/18,
with client/image sizes 1920x1230 and 1372x1196 and no resize retries. Header/
navbar stay fixed, rounded ancestor boundaries contain the rewrapped body, and
the final real contribution 1999 is visible and executes in Node. Unicode/CRLF
disk SHA256 d79793cb0662042ea31b20a1f6d7c1189035f41975190a2bb3368c194527e52b
is unchanged. This is PID 218468/version 0.24.0/base 1549cac2 with dirty correction,
not that failed immutable source's CI or a released executable.

.cache/app24-ci-fix-validation.json binds exact public16 origin 5178f551,
module sum, six corrected fixture sources and byte-identical main/detail/view,
failure and passing logs, four fixed artifacts and actual cleanup. The final
owned detail PID and all workspace-bin/private-temp gocode/Node/gopls candidates
are absent; private settings/temp are empty. New exact-source CI is the next
gate; no tag/release or installed v0.24.0 success is inferred from this local pass.

## Corrected immutable source and new CI

The corrected candidate is immutable source
2d136492229d22fa502be4749d3048c1f256fbee. Its parent is the failed 1549cac2
candidate; the change contains only six Windows native acceptance/test files
and these two engineering records. Independent read-only Git-blob comparison
against the completed local source manifest finds all nine bound files
byte-identical, zero CRLF/LF-only substitutions. All 316 other tracked blobs
match 1549cac2, including 160 non-test product Go files. Production main.go,
extension_detail.go and extension_detail_view.go are unchanged; the correction
therefore preserves the actual UI/keyboard implementation described above.

go.mod and go.sum are unchanged, consuming public godesktop v0.16.0 at
5178f551a179351391af6eaab62f3f5cf150350e, with module sum
h1:OKJYTXhzfV1Vr5uVsYdgx9FrOiyYnZGVtHMXOaS73yc= and no replace. The parent
repository's committed and staged gitlink both remain bad4e20f9b59a833e0e00bfd9288758925f4c5b8.
The source-bound receipt is .cache/app24-ci-fix-immutable-verification.json,
SHA256 28feb422083cdb526b74d3e68a0d7c1542b7fa8379dcbae2b0a5a4a626d53877.
It retains the original .cache/app24-ci-fix-validation.json SHA256 and does not
overwrite its dirty-base identity, old failed source logs or native artifacts.

[CI 37703702524](https://github.com/neko233-com/gocode/actions/runs/37703702524)
attempt 1 targets this exact source on all five platforms. The initial captured
jobs are Windows 2022/113072955012, Windows 2025/113072954796, macOS ARM/
113072954987, Intel/113072954980 and Ubuntu/113072955099; all are in progress
in .cache/ci-v024-37703702524-initial.json. This is an initial state, not a final
CI result. The failed 1549cac2 run is not rerun to conceal its failures.

Read-only GitHub tag-ref and release-by-tag requests both return HTTP 404 for
v0.24.0 at verification time. Publication, the prepared 27 released-byte gates
and installed promotion remain pending. No native window, release, Git state
or product source is changed during this verification; only these subsequent
engineering-record additions are prepared for the later evidence commit.

## Subsequent native preset persistence failure

CI 37703702524/source 2d136492229d22fa502be4749d3048c1f256fbee subsequently
fails Windows 2022/job 113072955012. Detail resize/capture, bounded geometry
mailboxes and both-keymap detail navigation pass. Its sole main failure is the
separate 45-phase workbench idempotence fixture: second native run reports
"native keymap changes did not persist exact final preset" after 31.33s;
main is 442.279s/34.7%. The complete job log is retained under
.cache/ci-v024-37703702524-windows-2022-job.log. The completed Windows 2025/
job 113072954796 independently fails a later native GiB search gate: its
complete Windows validation and real GiB search model (36.97s) pass, but
-search-smoke -search-smoke-mib 1024 times out after approximately 91.6s.
Subsequent GiB split/services/MSI/package steps are skipped; its full retained
log is .cache/ci-v024-37703702524-windows-2025-job.log. These are distinct
failures, and the keyboard fixture correction is not GiB search proof.
No rerun of that source, tag, release or installed promotion erases either.

The phase-44 final disk assertion ran in View after rendered preset/Command
Palette success, while the real capacity-one settings writer was independent.
Its existing close-and-drain was deferred until after the assertion. The fix
keeps the actual native palette/key check, ends ui.Run, explicitly invokes the
same bounded 3s keyboard drain, then clears its deferred handle and performs
the final disk assertion outside the UI. The file must exist, be regular and
<=4KiB, and contain exact vscode configuration; readKeymap's absent-file default
alone is insufficient. Failure reports retain actual profile and read error.

A nongui test uses the production settings actor, selectKeymap and real
writeKeymap, holds only the final actual write, observes the correct vscode
model with a still-jetbrains file, and verifies that the existing drain waits
and yields the exact final file without extra staging files. Missing default,
extra fields, bad JSON and oversized real files are rejected. Together with
existing actual settings-failure/cancellation tests, three strict-cgo/race/
shuffle repeats pass 1.504s (.cache/app24-keymap-drain-nongui.log). This proves
the required ordering; the original terse CI diagnostic does not reveal which
specific read/share race occurred. Product persistence and key dispatch code
remain untouched.

The corrected native repeat passes strict-cgo/race/shuffle/count3 in 83.711s,
whole 89.823s, with six actual owned runs under debug1/default adapter. Every
run records requested 1024x728 DIP separately from actual window DPI 96 and
client 1024x728 pixels, completes all 45 unchanged phases, and verifies exact
final vscode bytes after shutdown drain. Private settings/scratch/temp are
empty; the retained log is .cache/app24-keymap-drain-native-final.log. The
earlier 92.173s default-size pass is separately preserved and is not claimed
as small-window evidence.

The complete default-three Windows script subsequently passes 819.360s,
main 487.831s/34.7%, with public16/GOWORK=off, debug0/default adapter and
normal-window policy against the frozen dirty source;
.cache/app24-keymap-drain-full-windows.log. All original native console/GUI,
pixel, command, source-byte, disk and keymap checks remain. The default
workbench independently records DPI 144/client 1920x1230 pixels for its
1280x820 DIP request. Package 12m, owned process 60s and writer drain 3s are
unchanged. This script's native search is 16MiB; the separate CI 1024MiB
timeout remains an unresolved distinct gate.

The final fixed detail report is .cache/extension-detail/current.json,
PID 177484/version 0.24.0/base source 2d13649 with dirty fixture correction.
Its ten gates, 50 measured description rows, 2000 commands/actual command1999,
font width 51.149414 and maximum line 932.9214 below viewport width 988 all
pass. Real completed capture floors are 6/13/18; actual completed frames
match each floor, with 1920x1230 and two 1372x1196 images and zero resize
retries. Direct3D12 submits 38 frames and original disk SHA256 remains
d79793cb0662042ea31b20a1f6d7c1189035f41975190a2bb3368c194527e52b.

The fixed artifacts and SHA256 are current.json/1443B
eda9d7be3d752d8439bcfe9b3d2c2d88c8b5caf2074721fa489ea2d0482762c3,
details.png/620542B b652b2edb0738f7ec1f6cb961ca430c4f0e9b78308eea603c3f8f8f5f803b5f4,
narrow.png/429880B 264ed67ac25f21e210bad4600a5fd71fafe89d0355ed0c5d601fcbbdbefd74be,
contributions-bottom.png/201598B
4fa8694871e71ff8763a77db34f0099acfa338c62edc6b425e07c7542066c956.
At 08:24:07+08, all six targeted PIDs and final detail PID are absent; owned
process candidates and private settings/temp are empty. Fourteen frozen
source hashes are unchanged. The new source-bound receipt is
.cache/app24-keymap-drain-validation.json, SHA256
9562b573ce614081d3c4d85400551231752fff68150c4b8f480f3b102c12d45c.
Earlier failure/pass receipts are retained. Formal release source is null;
new immutable CI, the separate GiB search correction, publication and
installed promotion remain pending.

## Subsequent completion acknowledgement

The 819.360s record is preserved with all eighteen tested sources under
.cache/app24-keymap-drain-before-ack before a subsequent CPU-only correction.
A real-file probe seeds vscode, holds the old JetBrains write and queues final
vscode: the original void stop returns at 3.0001667s, so the existing vscode
file passes, then the late old actual write commits JetBrains. Its retained
strict-cgo/race probe is .cache/app24-keymap-drain-stop-timeout-negative.log,
4.085s. Normal native passes remain valid; they do not establish a completed
writer after an arbitrary three-second shutdown timeout.

The shared WithDrain variant exposes the existing writerDone channel, with
the writer loop/error publisher/queues/timers and legacy void API behavior
unchanged. The native workbench fixture checks actual completion after stop
without any additional wait: missing/open completion fails before reading,
and completion still requires the exact real file. No OS cancellation is
treated as I/O completion.

New public16/GOWORK=off strict-cgo/race/shuffle/count3 CPU regressions pass
10.656s; no-cgo count3 passes 0.457s, and both full vets exit 0. Actual old-write
timeout/coincidental-file rejection, final-write completion, late error
cancellation, both keymaps and idempotent stop/storage are covered. Private
settings/temp are empty. The new nineteen-source CPU receipt is
.cache/app24-keymap-drain-ack-cpu-validation.json, SHA256
3e64f5faca8cbac9cea9316ee6038549ed008774a72213c832d558c5dec06fe9.
This is new dirty source, not the old native-pass source or an immutable CI
result. Its subsequent actual native strict-cgo/race/shuffle/count3 passes
85.305s (whole 87.765s): six owned processes each record actual DPI 96 and
1024x728 client pixels, complete all 45 phases and require true writer
completion plus exact final file. Private directories are empty and nineteen
source hashes remain frozen. The separate log/receipt are
.cache/app24-keymap-drain-ack-native.log and
.cache/app24-keymap-drain-ack-native-receipt.json. A new default-window/debug0
complete Repeat3 script subsequently passes 829.761s, main 505.467s/34.8%;
.cache/app24-keymap-drain-ack-full-windows.log. All original console/GUI,
pixel, real Node/VSIX, source-byte, disk and preset guards remain, with
package 12m/owned process 60s/writer 3s unchanged. The complete native/full
source-bound receipt is .cache/app24-keymap-drain-ack-validation.json,
11210B/SHA256 a9c61a6aee0eeaf1048c4d4d45b7f8c2e5ca44558a9efc25c66bc9728dd618ad.
At 09:25:58+08 all six targeted PIDs and latest detail PID 222592 are absent;
owned process candidates and private settings/temp are empty. Nineteen bound
source hashes remain frozen, and the GUI slot is explicitly handed back.
The default script's native search remains 16MiB; exact immutable CI and the
independent CI 1024MiB gate remain pending.
The original writer body SHA256 after newline normalization is
c2ed7ac181bac070c72e7b153af8d7e35a208eab465767db66109aa3a0ad8d35,
identical to the newly exposed implementation body.

The latest fixed detail report belongs to PID 222592/version 0.24.0/base2d13649
with the new dirty ack source. Its ten gates, 50 description rows, native font
width 51.149414, maximum measured line 932.9214 below viewport 988, 2000
commands and actual command1999 pass. Completed capture floors/actual counts
are 6/12/17 with 1920x1230 and two 1372x1196 images, zero resize retries,
37 Direct3D12 submissions and unchanged source disk SHA256
d79793cb0662042ea31b20a1f6d7c1189035f41975190a2bb3368c194527e52b.
The final four artifact SHA256s are current.json/1443B
f75cfc4d6c06aa1bddeb34767a60e54d254537c1c69ec017b430785de222d58e,
details.png/620371B 2808f227ebdc1706bff2e31379a1690ac4175533f89e1444562767299596fd89,
narrow.png/429596B 8345629fa1a98ee27a8efd1e0ec90c7b4a6597e995d5891e538713ef7149becf,
contributions-bottom.png/201313B
934e4c138d46fec130edd730a28802ab53a5d500990f60c4a35d0980962f8ab4.
The earlier 819s source/archive/receipt remain unchanged; publication and
installed v0.24.0 promotion are not inferred from the newer local pass.
