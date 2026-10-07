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
