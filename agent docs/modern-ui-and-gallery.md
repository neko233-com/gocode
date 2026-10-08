# Modern Windows UI and native VS Gallery protocol

2026-10-08: application VERSION is 0.24.0; the consolidated unpublished candidate
now consumes immutable public core v0.17.0 with GOWORK=off/no replace. Its module
sum is `h1:2NF49iSveCrnfJATCyc1KS46Yobp7L4jRtHFjSyg6Fk=`. The independent
consumer verified the public source at `996b5ff`, after core's five-platform
CI 37713329201 passed. Earlier complete application checks below used public16
and do not validate this new dependency or popup integration. Local native,
packaging and exact released/installed-byte checks for the consolidated source
remain required; no application tag or automatic Action is made here.

## Native popup shadow integration (candidate)

File menus and submenus use one native `ShadowStyle` with black alpha .14,
Blur12, zero offsets and zero spread. Quick Input, Revert/reload confirmation,
dirty-close confirmation and grouped-history confirmation use black alpha .15,
Blur20 and the same zero offsets/spread. Blur is twice the core's DIP Gaussian
sigma. The measured source is stable MIT Code-OSS 1.141.0,
`2a59476c9bfcb90b3ddc372c36762471b7dfad1c`: cached style.css lines60/61 define
LG/XL and menu.ts lines1042/1266 select LG; immutable
[quickInput.css](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/vs/platform/quickinput/browser/media/quickInput.css)
and
[dialog.css](https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/src/vs/base/browser/ui/dialog/dialog.css)
select XL. Latest pinned main `9a89cf962f1d34463974058e9ef2b59f2bc33f15`
roundedCorners.css preserves the 8-DIP floating surfaces; shadows.css explicitly
preserves LG/XL while suppressing main-part shadows. Native style values are
implemented in Go; no CSS renderer, browser surface or copied blur asset is used.

The floating body is a direct positioned child of its root Stack. This removes
the artificial tight Row/Column ancestor clip while retaining the actual window
and rounded ancestor clips. The existing menu clamp, layer order, widths,
radii, body/row/button keys and callbacks are preserved. Quick Input remains
at y6 with its existing centered width. Centering still clamps to zero in
undersized windows. Revert keeps explicit height140 plus20 per error line,
including its prior two-DIP spare bottom space; dirty-close keeps its existing
114/22/20 formula, and history keeps height138. Each popup adds one GPU instance
without changing its hit area, measurement or adding product workers/timers.
The existing text wrapper preserves one blank status row for an empty string:
the initial Revert body is160DIP with Cancel at body.y+96; single-document close
is156DIP with Cancel at body.y+94. These are the pre-shadow product dimensions.

`-popup-shadow-smoke` is a separate Windows/cgo acceptance, with three owned
Window Runs, private files/configuration, actual native header/menu/cancel input,
24 fixed GPU PNG captures and a version/commit-bound JSON report. It waits three
actual completed frames per capture and checks the real HWND presentation/clock,
GPU dimensions against DIP viewport and actual DPI, body/control bounds, outside
halo input, real rounded/rectangular ancestor masks, scheduler/GPU idle and
shutdown/context/worker closure. Overlapping submenu input probes must miss
both panels. A separate uniform Simpson/Gaussian rounded-mask reference predicts
halo RGB; per-channel tolerance1 plus aggregate darkening rejects an absent
dark-theme halo. Synthetic negative checker tests are CPU admission evidence,
not native GPU evidence. The helper clears diagnostic drawable scaling, never
uses global desktop input/capture, preserves failed reports before reuse and
requires private scratch removal before writing current.json. Original workbench
45 phases and other native acceptance guards are unchanged.

Independent public17/GOWORK=off strict-cgo2/race/shuffle/count3 popup style,
Gaussian, absent-halo and overlapping-menu controls pass in 1.157s. Focused
existing Quick Input/menu/Revert/dirty-close/history controls pass strict-cgo3
in 36.577s and no-cgo3 in 36.888s. Full vet in both modes and race/no-cgo app
builds exit0; git diff --check exits0. The initial stamped build was blocked by
restricted child Git reads, then a peer's temporary duplicate transport method;
neither ran a native window. The final stamped build succeeds after that peer
source is compile-ready. Logs/owned-source receipt are in .cache/popup-shadow-cpu
and .cache/popup-shadow-*-*.log. These are CPU/build checks; the new three-Run
hardware/WARP popup gate, installed UI and packaging remain pending.
The first owned hardware-policy native attempt at actual DPI144/scale1.5 passes
base/File/submenu/Quick Input capture and halo/input gates, then fails phase4:
the fixture incorrectly expects140DIP instead of the original160DIP Revert body.
Its exact209-source snapshot, executable, failed JSON, four GPU PNGs and log are
preserved in .cache/native-combined24/popup-first-failure; receipt SHA256 is
40141eae23582fde6f6ca1cb0e7cc0b72420b4873548c6e47f4dbfcac7006a0b.
The next two attempts correctly refuse to overwrite that failure. The owned
process is gone and private TMP is empty. The correction changes only fixture
expected row-derived dimensions/button offsets, asserts empty/nonbusy status and
the exact one-document close plan, and preserves both product dialog sources.
New blank/one-line/CRLF/trailing-line contracts and all popup CPU controls pass
three strict-cgo/race/shuffle repeats in1.155s and no-cgo repeats in0.077s;
strict vet/diff checks pass. Logs are .cache/popup-shadow-correction-r2.
New-source full native proof is still required; the early captures do not prove
the remaining modal/ancestor/idle/closure phases or the forced-WARP path.
The second attempt passes all eight Run0 captures, including source-derived
Revert/close bounds, real Cancel callbacks and rounded ancestor halo/hit gates,
then rejects the original250ms idle check: S/C/T56→57 while Views stays57 and
requests stays72. Its worker sampled before the current View's pending frame
was submitted. Immutable209-source/EXE/log/eight-PNG/failed-JSON evidence is
.cache/native-combined24/popup-second-failure, receipt SHA256
53d1020a02fb0f20125faf2061939646928559c7fa2c19ba5a3d15fc45333907.
The fixture now waits off UI, bounded2s, for S/C/T/Views and InFlight0 to remain
stable50ms before starting the unchanged250ms zero-increment guard. It admits
no extra frame during that guard and creates no product timer or GPU request.
The former failure is retained; complete new-source native proof remains pending.
The third source-bound repeat attempt passes three hardware-policy processes,
each with three Runs/24captures, then the forced-WARP process expires phase4 at
35420ms. Its first five captures pass pixels, but per-stage submissions grow
749→1225→1658→2558→3520. The fixture requests another frame whenever InFlight
is nonzero even after reaching the three-completion floor, producing a slow-GPU
self-wake loop. All four owned PIDs are gone; private TMP/config are absent.
The immutable209-source/program/log/31-artifact archive receipt is SHA256
eb5c3cd24affab1e62e769f3cfffb695cd6807ab806799434ea658a47086b698
at .cache/native-combined24/popup-third-failure. This is not a WARP/full pass.
The next fixture requests drawing only below its completion floor. At that
floor it enters the single existing worker, which waits for actual quiescence
before every capture or native pointer job; the original capture InFlight0
guard, RGB tolerance and250ms idle guard remain strict. CPU controls parse the
actual source condition and prove that the former slow-completion loop fails
to quiesce while the correction retains the three-frame floor. Three strict-cgo/
race/shuffle CPU repeats pass1.190s and strict vet passes; real new-source WARP
acceptance remains required. Product render scheduling and35/40s bounds are unchanged.
The corrected public17/GOWORK=off native r4 gate passes three strict-cgo/race/
shuffle repetitions in108.134s (whole command114.116s). Six owned processes
execute18actual Window Runs and144GPU captures: hardware policy204088/182456/
176800 and forced-WARP191440/232208/228488. Every process verifies24captures,
three completed-frame floors, original RGB tolerance1/absence-sensitive halo,
source-derived body/Cancel coordinates, unchanged dirty source, outside-body
and rounded-ancestor native hits, original250ms zero-render idle and full HWND/
old-Context/worker/GPU closure. Hardware is an adapter selection policy; the
reports bind actual DXGI/committed-DIB presentation and actualDPI/drawable scale.
Three private TMP/config/GOTMP directories are empty. The209-source hash map
stays4419cbf9682b470ffc638d20742311acd53ff8b8ff6c65a27b9e1f320c493eea.
Receipt .cache/native-combined24/popup-r4-receipt.json SHA256 is
485acc5392ce97eda9e13c0cabdd5a629605f37ca62efbf047ea88c2e8dfb2fc;
log SHA256 isdbcb8e61bc52af60203a05aede68ecdb18a6b003844688860f04ecba1f7cd31d.
The three earlier failed archives remain unchanged. This proves the owned native
floating-shadow integration, not complete IDE parity or final packaged/installed
bytes; consolidated full-source and local signed-package gates remain required.
This small integration addresses floating elevation and clipping. Full VS Code
visual/API parity and official Marketplace access are not established by it.

## Public v0.23.0 baseline

Current public/installed v0.23.0 is immutable source
47623823dd88fbb45422d096b545631be7e6b77b on public core v0.16.0. Five platforms
pass attempt 1 in [CI 37678679723](https://github.com/neko233-com/gocode/actions/runs/37678679723),
and [publication 37682493872](https://github.com/neko233-com/gocode/actions/runs/37682493872)
reuses those tested packages. Actual released and installed console/GUI Auto Save,
minimized receipts and rounded Revert pass; installed 150% File/Settings/Revert/
extension-detail GPU captures were inspected. The v0.22.0 native File menus,
VS Code/JetBrains keymaps, radii and descendant clipping remain shipped behavior.
Full pixel/API parity is unfinished. Native GPU shadows and independent extension
detail scrolling were outside v0.23.0; fixed-header scrolling is now the
unpublished v0.24.0 candidate below. Exact public evidence is in status.md.

## v0.24.0 native extension editor candidate

The reference is stable Code-OSS 1.141.0 at
2a59476c9bfcb90b3ddc372c36762471b7dfad1c, specifically extensionEditor.ts and
extensionEditor.css; pinned source/license/hash evidence is in
[extension details](extension-details.md). Native title/close, metadata/actions
header and 36-DIP navigation remain fixed above an independently clipped body.
The viewport owns scroll/focus separately from sidebar search/list and documents.
Native font measurement drives Unicode wrapping, narrow reflow, fitting and
cached virtual rows. Bounds are 64 KiB description, 512 wrapped description rows,
4096 commands and 1024 materialized rows including bounded overscan; this is
a row budget, not a total-element count.

Tab-switch followed immediately by End previously consumed a zero-extent key.
Same-turn plan preparation against actual viewport bounds fixes that product
defect. Body wheel/navigation and the existing VS Code Ctrl+W/JetBrains Ctrl+F4
close paths preserve the underlying document. The real private VSIX acceptance
registers 2000 isolated Node callbacks and executes the final command1999; it
does not claim 2000 executions or arbitrary extension API compatibility.
Enable/disable uses real private persistence and cancellable worker receipts.
Fixed reports/screenshots, empty owned scratch and unchanged settings verify
idempotence. Original pre-version-bump GPU evidence is preserved separately in
.cache/extension-detail-pre-version-bump; .cache/extension-detail is reusable
latest candidate evidence and must not inherit the old report hashes.

Quick Input's native input, result rows and popup now use rounded descendant
clipping, with 3/4/8-DIP inner/row/outer surfaces. Save All requires dirty
documents and is disabled while saveBusy or fileActions.busy is set. These
changes consume public core16 clipping; no local shadow API is needed.

The first full public16 Windows default-three suite failed after 765.457s
(.cache/app24-public-windows-first-failure.log; main 531.350s/coverage 34.6%,
shuffle seed 1791410668665064700). One detail run clicked management before its
acknowledged state had a fresh committed native callback/tree. The old workbench
fixture's hardcoded contribution coordinates missed the new fixed-header layout
in all three rounds after its real Unicode source was saved; the original
.cache/app24-workbench-pointer-first-failure.png and notification-RGB failure
remain retained. This is a failed full suite, not a successful package run.

Fixture corrections wait at least three completed GPU frames after management
acknowledgement before rereading actual bounds. The workbench test uses a
test-only UI mailbox for two stable keys, bound to PID/sequence/View generation/
selected tab and fresh completed frames. Product main bytes are unchanged by
that overlay. Actual pointer input and original notification RGB, Unicode disk,
source/icon/settings/restore/close/smoke assertions remain intact.

Both corrected fixtures now pass public16/GOWORK=off, debug=0/default adapter,
strict-cgo2/race/shuffle/count=3 in 72.035s, seed 1791411978500786200
(.cache/app24-native-fixtures-targeted.log). Detail takes 15.71/13.56/13.41s,
each with two real HWND/Node lifecycles and ten gates; workbench takes
10.97/8.58/8.71s using actual contribution-tab bounds {491,232,180,35} and
command bounds {311,325,936,42}. Completed/tree generations advance 21→27,
21→27 and 22→28. The full default-three rerun passes 833.454s in
.cache/app24-public-windows-final.log (main 498.701s/34.6%). Package repeats,
vet/build/AMD64 PE and all console/GUI native modes pass. Independent full
no-cgo three repeats also pass 170.494s; private Go tool telemetry is removed
after the owned test process exits. Root inspects all three current v0.24.0
GPU captures. Exact-source CI and released/installed-byte proof remain separate.

The header still uses a generic icon and manifest/catalog description. Actual
VSIX icons, README/CHANGELOG/Markdown images and richer contribution editors,
scrollbar drag, overlay shadows and complete VS Code UI/API remain open. A
bounded 27-gate released-byte checker and separate installed console/GUI detail
reports are prepared, with original 8m/60s guards and v23 snapshots retained;
that preparation is not publication or installed v0.24.0 evidence.

## Historical v0.22.0 rounding evidence

Native rounded UI was first public and installed in v0.22.0/source
0b5966171e5e183e74a9fecbeccce391cd1cf229 on public core v0.15.0. All five source
jobs 37651719075 and tested-package publication 37654475129 pass. Actual final
Windows 2025 File menu and Windows 2022 JetBrains Search Everywhere CI captures
were inspected, together with installed 150% File menu/Settings GPU pixels.
The framework's descendant clipping fixes a real missing capability; it does
not establish full VS Code pixel/API parity. Earlier candidate checks below
are retained as history. Exact installed/released-byte evidence is in status.md.

The user's latest steering requests current VS Code rounding and the official
extension store. Actual MIT Code-OSS 1.141.0/2a59476c9bfcb90b3ddc372c36762471b7dfad1c
modernUI sources are inspected: roundedCorners, editorBorder, padding, tabs,
activityBar, commandCenter, titlebar and shadows CSS. Latest main is now verified
as 9a89cf962f1d34463974058e9ef2b59f2bc33f15 on 2026-10-08. Existing Microsoft MIT/license provenance
remains in assets/code-oss; these CSS rules are translated into native Go elements.

Windows controls/list rows use 4px radii and native hover fills; menus/Quick Input
use clipped 8px surfaces. Editor framing includes tabs/breadcrumbs/content inside
a 1px stroke with 8px outer/7px inner clipping, without a new outer margin.
The panel remains outside the editor card. Selected activity items use an inset
rounded fill. The framework needed real descendant clipping, implemented in the
public godesktop v0.15.0; a background radius alone cannot clip painted child content.
Workspace-linked native 45-phase acceptance passes a standalone run. Captures
now wait three completed GPU submissions after asynchronous acknowledgements,
so a saved model identity cannot be presented with an older Untitled screenshot.
Full final independent public-framework/repeated/native/visual evidence follows.

Core immutable 4d62ed73a5513d8cbc281e05fe9ea43a74fd61ea passes all five jobs in
CI 37642213032 and is published as v0.15.0. The application independently fetches
it with GOWORK=off/GOPROXY=direct/no replace, and go mod verify passes. Full
workspace-linked three shuffled strict-cgo/race repeats pass (247.071s, model
30.3% coverage). No-cgo and vet pass. Native saved Chinese tab/content and
extension detail PNGs are inspected after the three-submission capture fix.
Final full public-module script/source CI/released-byte installation follows.

The final independent public-module full Windows script passes three shuffled
race/strict-cgo repeats (242.704s), vet and all console/GUI native gates including
both new 45-phase runs, existing editor/groups/large pages/search/replace/terminal/
VSIX/Git/file watch. Private scratch cleanup succeeds. Final File menu and selected
JetBrains Settings GPU captures are visually inspected. Public no-cgo, workflow
lint, distribution policies and diff checks pass; exact source CI/release/install
are still required before promoting the local user installation.

## Native store protocol and actual service limit

Microsoft's [official FAQ](https://code.visualstudio.com/docs/supporting/FAQ#extensions),
rechecked on 2026-10-08, says alternative products including Code-OSS forks are
not permitted to access Visual Studio Marketplace.
This is distinct from MIT Code-OSS source/API compatibility. No authorization
from Microsoft is present in this task. Official Marketplace access/compatibility
is not claimed. The default public catalog remains Open VSX with local VSIX import.

The native -extension-gallery-url HTTPS-base option implements the VS Gallery
query/download/install protocol for an authorized compatible/private service.
It can address the official service only when separately authorized. The native
UI names the selected service and new/replacement windows inherit its endpoint.
No Microsoft product/client identity is spoofed, no access restriction is bypassed,
and credentials/query tokens are rejected in endpoint URLs. Actual source contracts
come from platform/extensionManagement/common/extensionGalleryService.ts,
extensionGalleryManifestService.ts, extensionGalleryManifest.ts and extensionManagement.ts
at the exact stable source. Query uses the same Target/SearchText, bounded page,
flags and Accept version; native gocode User-Agent stays distinct.

The v0.24.0 candidate adds bounded bare publisher.extension and @id exact
queries. Installed/enabled/disabled filters stay local and ordinary free text
retains its original search behavior. Open VSX named win32-x64/latest then
universal/latest metadata is reread at an immutable version; identity, version,
platform and HTTPS/host bounds must match. Superseding a query cancels stale
network requests and rejected/queued receipts. Authorized Gallery uses exact
ExtensionName=7, Target=8 and ordinary SearchText=10 from the pinned stable
extensionGalleryManifestService.ts. Query redirects remain on the authorized
HTTPS host; selected packages retain archive identity/version/download bounds.

Windows x64/universal version selection rejects Mac packages/prereleases. Responses
are limited to 1MiB/20 entries/128 versions, requests to the existing cancellable
single worker, downloads to 64MiB and manifests to the existing archive caps.
The selected immutable package URL is retained; extraction rejects mismatched
manifest identity/version. Gallery assets/redirects stay on the authorized host
or known Microsoft gallery CDNs for that official endpoint.

Three race/strict-cgo repeats (2.137s) use a real TLS server, real ZIP VSIX and real
Node host: protocol headers/body, x64 selection over universal/Mac, download,
identity validation, extraction, actual contributed command and cleanup pass.
Different selected version and foreign/credential URLs are rejected without
changing the installed fixture. This is actual protocol/process evidence for the
fixture, not proof of live official Marketplace access or arbitrary extension APIs.

The installed v0.23.0 official Copilot SDK/LSP health check in
.cache/installed-copilot-v023.log reports authentication, LSP initialization and
SDK connection true, networkPromptSent=false. This verifies the accepted SDK/LSP
path; full official Copilot VSIX compatibility remains unfinished. Microsoft's
FAQ was rechecked on 2026-10-08: alternative products, including Code-OSS forks,
are not permitted to access the official Marketplace. Separate service
authorization is absent here; the native authorized Gallery implementation and
default live Open VSX remain the implemented scope.

The actual installed v0.23.0 -extension-catalog-check golang exits 0 and records
target=win32-x64, installed=false in .cache/installed-catalog-v023-golang.json.
It resolves real Open VSX versions tooltitudeteam.tooltitude 1.56.5,
toga4.go-tdt-outline 0.0.2 and ZencoderAI.zencoder 3.85.9007 with platform-specific
package/SHA256 URLs. The original golang.go query exits 1 immediately with
"no actual Windows x64/universal extension versions resolved"; raw failure stays
in .cache/installed-catalog-v023.json. This is a query-specific search/resolution
limit, not evidence of a broken extension or network failure. These checks only
read live Open VSX metadata; they install no catalog VSIX, prove no arbitrary
extension activation, and do not access Microsoft's official Marketplace.

The unpublished exact-query CLI now resolves golang.go and @id:GoLang.Go to
canonical golang.Go 0.56.1/universal through actual named then immutable Open VSX
metadata. Ordinary golang still returns the three win32-x64 versions above.
Reports are .cache/extension-query-live-cli-{golang-go,id-golang-go,golang}.json,
all installed=false; no live catalog VSIX is downloaded, installed or executed.
The v23 free-text empty result remains historical query evidence. Genuine TLS,
real generated VSIX/Node, platform/version/identity/404/cancellation and bounds
tests pass with public16, three strict-cgo/race repeats 13.819s and no-cgo 3.791s.
Exact source/hash/protocol provenance is in extension-details.md. These tests
verify the authorized/private adapter and Open VSX scope; Microsoft's FAQ
restriction and absence of separate authorization remain unchanged. The official
SDK/LSP health proof remains v23 evidence, not official Copilot VSIX compatibility.
