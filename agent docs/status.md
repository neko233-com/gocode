# Milestone ledger

## Native VSIX terminal candidate (2026-10-07)

Process-backed Terminal API and actual owned VSIX/native process acceptance are
implemented with bounds and explicit remaining gaps in terminal.md. Three-repeat
Node protocol/process/env/raw Windows arguments and native Windows API/cwd/env/
Unicode/ANSI/color/focus/exit/dispose/user-close/failure/cleanup gates pass locally;
the 150% output PNG was inspected. Existing source/editor/shell regression gates
remain required. Windows console/GUI and both Mac architectures normal/1.5/2 are
wired into CI. Public core v0.13.0/965678a passes all five jobs in CI 37599839423;
ARM's first existing bitmap timeout after the grid passes unchanged job retry,
without a claimed cause or relaxed checks. go.mod imports the public module with
GOWORK=off/no replace. Independent full Windows strict-cgo/three-repeat race/vet
and all console/GUI native gates pass, including the new real VSIX terminal gate
and existing editor/groups/open/save/terminal/file-watch regressions. Model coverage
is 30.6%, search 81.4%; native pixel gates provide separate evidence. Actionlint,
Python channel policies and diff checks pass. App v0.20.0 source CI, packages and
release/install promotion remain pending. Verified
public/installed v0.19.0/core v0.12.0 remains the current baseline.

## Windows native color v0.19.0 promotion (2026-10-07)

Immutable source/package `4616656105b0b0222b03524f207ee9d53ca335bb` passes all
five jobs in [CI 37590651597](https://github.com/neko233-com/gocode/actions/runs/37590651597).
Windows 2022/2025 amd64, Mac Intel/ARM native/services/packages pass on the first
attempt. Linux race/vet passes; its first no-cgo descendant-close check reports
failure, then the same-source Ubuntu job passes unchanged on retry. No cause or
relaxed assertion/deadline is claimed; the termination observer needs further
stress/diagnostics if this failure recurs. Earlier b69e9cf/37589544875 is not promoted:
both Windows reject actual `??` output. The corrected owned PowerShell 7/5.1
startup selects UTF-8 after profiles; real OEM-437/ASCII regression processes
require decoded ANSI-colored Chinese/emoji output. Local full public-module
strict-cgo/three-repeat race/vet/console/GUI regression plus focused terminal
race/vet/native correction, no-cgo, workflow lint and diff checks pass.

Public godesktop v0.12.0/c42b4b4 is independently imported with GOWORK=off and no
replace. Core all-five CI 37587610645 verifies both Windows 100/150/200% native
reference RGB/mask/opacity/clip/eviction/recovery. Application terminal/split
reports show Windows 2022 color counts 219 and 3287/3287; Windows 2025 counts
59 and 870/870. Both Mac 200% terminal counts are 717; 150% split counts are
ARM 5125/5200 and Intel 7482/7448. Actual Windows terminal/shared split, ARM
terminal and Intel split PNGs were inspected; selected artifact ZIP entries
were range-read with CRC verification. Mac retains normal/1.5/2 native gates.

[Publication 37592393163](https://github.com/neko233-com/gocode/actions/runs/37592393163)
tags that exact source and reuses tested packages. Metadata d6cd857 changes
exactly seven free distribution files; both Python policies pass. ZIP SHA256 is
`f0a7b757c9abf3ffeea698e9123feec5562c5d4078d3bb8d95dca49530114f8c`; MSI is
`bbd429b1d27a88f6ee2da02bda062752bd1a693ff62671c4cd928c06a5546290`.
Actual signed automatic/direct GitHub/manual ghfast.top archive bodies, all
released native gates and actual prior v0.4.0 GUI rollback pass.

The user's stable 0.5.1 launcher applies v0.18→v0.19 through direct GitHub and
selects exact 4616656. Installed GUI/both icon handles/Settings/groups/real VSIX,
actual 1,073,741,824-byte split and unchanged whole hash, highlighted ConPTY and
source preservation pass. Settings shows Up to date (0.19.0), Auto=true/automatic
route and gopls ready. Actual Settings/terminal/GiB survivor PNGs were inspected;
terminal count is 166, shared split counts 2922/3062 at 150%. Config SHA256 remains
d8a85199a0018d88355ab5786881c778dc63596858a58bbe64fde8f799c9ed5f; user PATH SHA256
remains aec04bdd1653d4c8c70dc2b50aec868029787ecf0cc34cbd1f4a9f7df3d30aa3.
Desktop and Start Menu targets remain gocode-launch.exe. Official Copilot reports
authenticated/LSPInitialized/SDKConnected=true, networkPromptSent=false.
Complete VS Code/UI/official Copilot VSIX parity, dedicated third-party color-font
fixtures, Mac native terminal width resize and recorded editor/services gaps remain
active. Copilot follows the accepted SDK/Language Server route.

## Windows shell encoding correction candidate (2026-10-07)

Source b69e9cff47e712fa31a98c5111b33898c9423a36 in CI 37589544875 passes
Linux and Mac ARM; both Windows fail the new real terminal gate at phase 5.
Decoded output is `NATIVE_TRUECOLOR ??` while the command echo contains emoji.
The source is not promoted. Owned PowerShell sessions now explicitly use UTF-8
for Console input/output and native-pipe OutputEncoding after user profiles load.
Real PowerShell 7/5.1 tests begin with OEM 437/ASCII and require decoded colored
Chinese/emoji output. Three-repeat full terminal race/vet and console/GUI native
terminal gates pass locally, retaining decoded output/pixels/resize/interrupt/
exit/source guards. Exact-source cross-platform/package/release/install validation
is pending.

## Windows color public-module candidate v0.19.0 (2026-10-07)

Public core v0.12.0/c42b4b43f0a27452937850871681f26746e39d7f passes all five
jobs in CI 37587610645 and validation/readback WARP diagnosis 37585540569.
All six actual Windows reference RGB/mask reports match 100%; 2022 normal and
2025 200% PNGs were inspected. Core retained original guards, added physical
baseline-phase/direct layer drawing, explicit nonuniform sampling and cgo-tracked
DXIL. Mac ARM 150% viewport passes same-source retry after one timeout.
VERSION 0.19.0 imports public v0.12.0, no replace. Development owned editor/
real VSIX/ConPTY gates pass; final terminal count 166, split counts 2922/3062.
Independent GOWORK=off strict-cgo/three-repeat race/vet and full console/GUI
native regression pass locally. Public-module terminal and shared split reports
retain 166 and 2922/3062 intrinsic color pixels; both actual PNGs were inspected.
Workflow actionlint/ShellCheck and diff checks pass. Model coverage remains 31.3%,
search 81.4%; pixel gates provide separate native evidence. Exact-source five-job
CI/packages/release/install remain pending; public/installed v0.18.0 stays the
baseline. Full VS Code/official Copilot VSIX remains open.

## Windows intrinsic color candidate (2026-10-07)

Local explicit go.work source imports candidate parent 03ed1c7 for development;
independent public-module promotion is pending parent CI 37581933475.
Actual owned Windows editor split, real Node VSIX views and highlighted ConPTY
with decoded `NATIVE_TRUECOLOR 😀` ANSI output pass. Terminal 150% color count
is 92; initial shared split counts are 1774/1898. Both PNGs were inspected.
Windows and Mac now require these intrinsic editor/terminal pixels; existing
native edit/selection/resize/interruption/exit/source preservation gates stay.
Public/installed v0.18.0/10bebad remains the verified baseline until exact source
CI/package/release/installed promotion completes. color-glyphs.md records scope.

## Native color and terminal pixels v0.18.0 promotion (2026-10-07)

Immutable source/package `10bebade1495f234d8bf52cba72491ad598421c9` passes all five
jobs in [CI 37575737909](https://github.com/neko233-com/gocode/actions/runs/37575737909):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off/no replace imports
public godesktop v0.11.0/6706e59, whose exact all-five CI 37574802922 and Intel
ordinary/readback/color/recovery diagnosis 37574803095 pass. Static slot/explicit
LOD sampling fixes the earlier Intel regression without relaxed deadlines.
Independent local strict-cgo/three-repeat race/vet, complete console/GUI Windows
native regression, no-cgo fallback and actionlint/ShellCheck/diff checks pass.
Model coverage is 31.3%, search 81.4%; native pixel gates are separate evidence.

Both Mac architectures pass normal/1.5/2 actual terminal command/string/ANSI/emoji
pixels and editor split colors. ARM 200% terminal (2048×1368) and Intel 150% split
(1920×1230) PNGs were inspected. Their 200% terminal reports each record 717
intrinsic emoji pixels; initial 150% split counts are ARM 5125/5200 and Intel
7482/7448. Owned source artifacts supply exact PNG/JSON evidence; authenticated
range reads retrieved selected ZIP entries with CRC validation after a partial
full-artifact extraction. Windows local and installed 150% terminal/Settings
frames were also inspected. Go/PTY renderer pixels are not browser screenshots.

[Publication 37576841022](https://github.com/neko233-com/gocode/actions/runs/37576841022)
tags exact source and reuses tested packages. Metadata `da39222` changes exactly
seven free distribution files; both Python policy tests pass. Windows ZIP SHA256
is `f5dc457906806ce1de11f4d9b99b7c003ba835927d9471b146ef2006f042d6cb`;
MSI is `45f9ebc0dbb46c10949ca21e67efe13e7e0a049ad1f9fc7817c5520f6bc99a89`.
Actual signed automatic/direct GitHub/manual ghfast.top archive bodies, all
released native gates and real prior v0.4.0 GUI rollback pass.

User stable 0.5.1 launcher updates actual v0.17→v0.18 via direct GitHub; selected
source is 10bebade1495f234d8bf52cba72491ad598421c9. Installed GUI, both icon handles,
Settings, groups/VSIX, actual 1,073,741,824-byte split with unchanged whole hash,
highlighted ConPTY and source checks pass. Settings shows Up to date (0.18.0),
automatic updates/route and gopls ready. Config SHA256 remains
d8a85199a0018d88355ab5786881c778dc63596858a58bbe64fde8f799c9ed5f; user PATH remains
aec04bdd1653d4c8c70dc2b50aec868029787ecf0cc34cbd1f4a9f7df3d30aa3. Desktop and
Start Menu shortcuts still target the stable gocode-launch.exe. Official Copilot
authenticated/LSPInitialized/SDKConnected=true; networkPromptSent=false.

Windows color fonts, Mac terminal native width resize, complete editor
IME/grapheme/bidi/multicursor/accessibility, tabGroups/preview/pin/docking and
multi-window, SCM/DAP/tasks/remote/webviews and full VS Code/GPUI/official Copilot
VSIX parity remain active. Copilot retains the accepted SDK/Language Server route.
See color-glyphs.md for actual bounds, diagnostic scope and maintained gaps.

## Candidate native color and terminal pixels (2026-10-07)

The Mac alpha-only emoji gap is being fixed in the parent Metal atlas with
bounded RGBA pages and one mixed text batch. gocode adds actual initial split
emoji pixels and replaces the previous Mac no-op terminal capture with owned
drawable command/string/ANSI/emoji checks and PNG/JSON evidence. Both Mac jobs
will run normal/1.5/2 terminal density. Local Windows strict-cgo/three-repeat
race/vet and workflow lint pass. Real Windows console/GUI terminal pixel and
native split gates pass; actual GUI output PNG was inspected. Manual hidden
startup caused no rendered frames; the existing normal GUI harness passes.
Public core v0.11.0/6706e59 passes exact all-five CI 37574802922 and Intel
diagnosis 37574803095, with unchanged readiness/drain gates and normal/1.5/2
color/eviction/recovery pixels. VERSION 0.18.0 imports that public module with
GOWORK=off/no replace. Independent app/native/release/install is pending.
Independent public v0.11.0 strict-cgo/three-repeat race and vet now pass locally;
native source CI and tested package/release/install promotion remain pending.
Current installed/public v0.17.0 evidence below remains authoritative.
See color-glyphs.md for scope and retained rendering/terminal limitations.

## Native VSIX editor views v0.17.0 promotion (2026-10-07)

Immutable source/package `aa9d838b8b59bd82d0ce0fe52ffb2225213708c9` passes all five
jobs in [CI 37570381921](https://github.com/neko233-com/gocode/actions/runs/37570381921):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off/no replace imports
public core v0.10.0/8da1ad6, whose exact all-five CI 37567374583 passed. Full local
strict-cgo/three-repeat race/vet and normal console/GUI regressions pass, including
the owned 1024×728/96-DPI window fixture (model 31.3%, search 81.4%).

Real VSIX views share canonical documents while retaining visible identity,
column/selection/ranges and independent reveal; native focus/close, renumbering,
disposed/reopened receipts, ordered UTF-16 edits and acknowledged CRLF save pass.
Hidden opens and causal receipts preserve newer focus. All nine columns, real GiB
service rejection, runtime >32 KiB configuration/cleanup and long-line allocation
bounds are verified. Exact limits and unfinished APIs remain in groups.md.

Mac Intel/ARM pass normal/1.5/2 native gates. Windows CI 100% 1024×728, ARM 200%
2048×1368, Intel 150% 1920×1230, local/installed 150% VSIX/survivor/GiB/Settings
PNG frames were visually inspected. Mac emoji still appears as a solid monochrome
fallback. Early Windows failures were pixel waits sampling centered caption ink;
blank-side sampling fixes the gate and preserves its exact-byte assertion. The
first ARM 200% replacement Undo timeout did not recur in either later exact-source
run; its unchanged gate and native key/history traces remain required and retained.
Windows 2025 and installed GUI verify an actual 1,073,741,824-byte independent split,
survival after original-view close and identical whole-file SHA256.

[Publication 37571358158](https://github.com/neko233-com/gocode/actions/runs/37571358158)
reuses tested artifacts and tags v0.17.0 at aa9d838. Metadata
`eb0de72dfffc1bc06f1235a28cdb5de1c1499431` changes exactly seven channel files;
two Python policy tests pass. Windows ZIP SHA256 is
`858bb5d30893312b796693e766198334deb2c9ec1d1da72e45e8aa146138f5b6`; MSI is
`3ce2d76c302ade8c198258fa0e6e890ad47760bd0c35b08c85eaa5fa41d17a64`.
Real signed automatic/direct GitHub and manual ghfast.top archive bodies pass
integrity. All released native gates, including the new VSIX view gate, pass.
The owned install then actually renders prior v0.4.0/cfcd3513 after rollback.

User v0.16→v0.17 updates through the existing stable launcher via direct GitHub.
Selected GUI is versions/0.17.0/gocode-app.exe/source aa9d838; MSI/launcher baseline
stays 0.5.1. Installed GUI/both icon handles/Settings/source, native groups/VSIX,
actual GiB split and highlighted terminal pass. ConPTY 1.25.260930003 is verified.
Settings shows Up to date (0.17.0), automatic route/updates and gopls ready. Exact
update-config and user-PATH hashes match before/after; desktop/nested Start Menu
still target gocode-launch.exe. Copilot authenticated/LSP/SDK=true with
networkPromptSent=false. Full VS Code/GPUI/official Copilot VSIX, tabGroups/options/
decorations/snippets/undo merging/persistence/docking/multi-window, IME/grapheme/
bidi/multicursor/accessibility and SCM/DAP/remote/webview parity remain active.

## Native VSIX editor views v0.17.0 candidate (2026-10-07)

The real Node bridge now models visible native views separately from canonical
documents, with stable renumbering, disposed/reopened identity and monotonic layout
receipts. Native openTextDocument reads on the bounded worker without showing/focusing
a tab. showTextDocument adds Active/Beside/One–Nine, preserveFocus/UTF-16 selection;
selection/reveal and text edits validate actual view and source identities/versions.
View/column/range/selection events and ordered operations match owned native state.
Manual/reveal scrolling remains independent until native caret movement resumes.

Three-repeat model/editor/actual Node races pass. Actual -groups-vsix-smoke installs
a genuine temporary VSIX, checks hidden/shared views and independent reveal, uses
real mouse focus/close, verifies surviving/disposed references, then awaits actual
CRLF save and preserves the hidden file. Completed group GPU pixels pass at local
150%/process 100%, alongside existing native editor/opener regression. A causal
per-invocation opening receipt fixes the delayed hidden-open/new-focus regression
without weakening its old native gate. Core v0.10.0 is published at immutable
8da1ad62268bc5ef1de58ec8703fa288fcd86250 after all five jobs in CI 37567374583 pass.
go.mod now imports that public module with GOWORK=off/no replace. Independent full
Windows strict-cgo/three-repeat race/vet and all console/GUI native gates pass
(model 31.3%, search 81.4%). Exact app CI and release/install promotion are pending.
The first full run exposed old native palette/notification click coordinates:
eager group initialization moves workspace actions before group headers. Actual
failure pixels showed correct Unicode text but a Chat click instead of Save.
The gate now clicks the rendered action/notification rows and retains exact saved
bytes, extension, settings, maximize/minimize/restore/close assertions. Its isolated
strict-cgo/race rerun and the subsequent complete three-repeat/console/GUI run pass.
Published and installed app remains v0.16.0/source 38f7d15. Full tabGroups/preview/
docking/options/undo merging/IME/accessibility and official Copilot VSIX remain gaps.

First exact-source app CI 37568560220 at 9fb9f2d did not pass: both Windows runners
fail the native palette background wait before Save, while Mac ARM's existing 200% replacement
gate times out awaiting a second grouped Undo prompt. New VSIX gates pass on ARM
normal/1.5/2. Promotion is stopped. Native failure snapshots/DPI and Mac key/history
diagnostics are retained; local owned 96-DPI full-window race passes after matching
probe/child virtualization. Failed gates remain required; further CI diagnosis is
pending, and no application release/install success is claimed.

The Windows failure is now reproduced at the runner's 1024×728/96 DPI: the added
background sample x=700 intersects centered button glyphs, preventing the click
from being sent. Sampling the blank left side fixes it without changing native
Save or its exact-byte assertion. Three-repeat owned small-window races pass.
Initial -window-width/-window-height DIP options allow faithful startup fixtures;
owned forced-DPI failure captures keep matching probe awareness during cleanup.
Diagnostic CI 37569352467 passes all Mac ARM native/package gates, including all
three densities and the unchanged previously timed-out Undo prompt, with actual
key/history traces retained. Full public-module Windows strict-cgo/three-repeat
race/vet plus all normal console/GUI native regressions pass again, with the unit
owned-window fixture forced to the runner's 1024×728/96 DPI. Model coverage is
31.3%, search 81.4%. A fresh exact-source full app CI is still required.

## Native editor groups v0.16.0 promotion (2026-10-07)

Immutable application/package source `38f7d15533361ffc0b7a6e0c9b74d0dc21839962`
passed all five jobs in [CI 37561829837](https://github.com/neko233-com/gocode/actions/runs/37561829837):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off/no replace uses
public core v0.9.0/cb8b077. Full local strict-cgo/three-repeat race/vet and all
console/GUI regressions pass (model 30.7%, search 81.4%). Native right/down groups
share text/history/service identity and retain independent caret/scroll/tabs/pages;
close scope, held toolbar review, origin/closed-group receipts and large-reader
lifetime are verified. Exact bounds and remaining scope are in groups.md.

Both Mac architectures pass normal/1.5/2 owned input/completed Metal pixel gates.
Windows CI 100% 1024×728, Mac ARM 200% 2048×1368, Intel 150% 1920×1230 nested
group PNGs and ARM large-page PNGs were visually inspected. Mac emoji currently
renders as a solid monochrome fallback in those captures; correct emoji appearance
remains a framework rendering gap. Windows 2025 actual 1,073,741,824-byte split
passes independent first/tail pixels, original-view close, survivor read and
identical whole-file SHA256. The separate actual GiB single-line race search
reports 16.7425142 s / 811,864 new Go allocation bytes / 2,147,492,044 I/O bytes in
this scoped run. Existing native VSIX/recovered-gopls/watch/opener/history/search/
tabs/highlighted terminal/MSI and package checks remain green.

[Publication 37562709833](https://github.com/neko233-com/gocode/actions/runs/37562709833)
reuses tested packages and tags v0.16.0 at 38f7d15. Metadata
`5664fdeb45f1ae7f02fe5c03bc77cd5d58948946` changes exactly seven channel files;
two Python policy tests pass. Windows ZIP SHA256 is
`64bba0eb0bbfebf7b9a512affdf3c386c6a48eb237969bda6d6c80fb031b12e9`; MSI is
`34f706718178c8206aae14f56f97fc3af59fa11b3e6cdb6e9024ee4400052e8a`.
Real signed automatic gh-proxy.com, separate direct GitHub and manual ghfast.top
archive bodies pass integrity. All released native gates, including both group
gates, pass; the owned root then actually renders prior v0.4.0/cfcd3513 after rollback.

The user's actual v0.15.0 updater selects v0.16.0/source 38f7d15 via direct GitHub.
Stable MSI/launcher baseline stays 0.5.1. Installed GUI/both icon handles/Settings/
owned source, native groups, actual 1 GiB split and highlighted terminal pass;
ConPTY 1.25.260930003 is verified. Installed 150% GiB split and Settings PNGs were
visually inspected: Up to date (0.16.0), automatic route and gopls ready. Exact
update-config and user-PATH hashes match before/after; both mirrors and stable
desktop/nested Start Menu targets remain. Copilot authenticated/LSP/SDK=true,
networkPromptSent=false. Full group API/persistence/docking/multi-window,
production/GPUI/VS Code and official Copilot VSIX parity remain active.

## Native editor groups v0.16.0 candidate (2026-10-07)

Right/down native splits share canonical editable documents and undo/save history
while retaining independent UTF-16 caret/selection, scroll, tabs and large-file
pages. Scoped group close protects shared dirty resources; asynchronous opens
retain their originating group and captured toolbar clicks reject changed review.
See groups.md for exact ownership, eight-group limit and unfinished API/layout scope.

Full local GOWORK=off public-core v0.9.0 strict-cgo/three-repeat race/vet and all
console/GUI native regressions passed (model 30.7%, search 81.4%). New actual native
group focus/sash/nested split/Unicode/scoped Cancel/Save and shared-index split
gates pass at local 150% and forced process 100% DPI. Actual 1,073,741,824-byte
split passes independent first/tail GPU pages, original-view close, survivor read
and whole-file SHA256. Cross-platform exact source/publication/install evidence
is pending; released and installed version remains v0.15.0/source 9ba8908.

## Workspace history v0.15.0 promotion (2026-10-07)

Immutable application/package source `9ba89081f8aea463853c58a6c0955a9a91c950af`
passed all five jobs in [CI 37557043933](https://github.com/neko233-com/gocode/actions/runs/37557043933):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off/no replace imports
public core v0.9.0/cb8b077, whose exact all-five CI 37555224866 passed. Full local
public-module strict-cgo/race/vet/console/GUI regression passed (model 30.2%, search
81.4%). Native group confirmation/Cancel/All/current-file split and redo preserve
older history/save points; immutable worker receipts guard every identity/version/
caret/stack and expired groups. Complete commits precede callbacks and never save
over external disk bytes. Metadata/worker bounds and remaining gaps are in history.md.

Initial CI 37556152512 exposed a 100% DPI glyph assertion error, reproduced with
owned pixels and corrected without changing the UI/model. A missing-label overlay
still fails with zero glyph ink. Final Windows 100% 1024×728 confirmation (314 ink),
ARM 200% 2048×1368 confirmation (1,091 ink) and Intel 150% 1920×1230 grouped Undo
PNGs were visually reviewed. Both Mac architectures pass normal/1.5/2 native gates.
Windows 2025 actual 1,073,741,824-byte single-line race search reports 32.7752619 s /
748,960 new Go allocation bytes / 2,147,492,044 I/O bytes in this scoped run. Actual
GiB browser/native navigation and existing VSIX/recovered-gopls/watch/opener/terminal/
tabs/search/MSI/package gates remain green; timings are not universal guarantees.

[Publication 37558228545](https://github.com/neko233-com/gocode/actions/runs/37558228545)
reuses tested packages and tags v0.15.0 at 9ba8908. Metadata
`35bc1bd4108ee8568f0087bca4c2ef537416077e` changes seven known files; two policy
tests pass. Windows ZIP SHA256 is
`80606203d889efed6c6b3d38ebb88d3582a97b495a2ca5000534c1f559187c80`; MSI is
`a56f70df8d61d4bcf91a24f3cb5bf9b4159d8e1f001564989c92110a71cb8406`.
Real signed automatic/direct/manual ghfast.top archive bodies, all released native
gates and actual prior v0.4.0/cfcd3513 native GUI rollback pass.

The user's actual v0.14.0 updater selects v0.15.0/source 9ba8908 via direct GitHub.
Stable MSI/launcher baseline stays 0.5.1. Installed GUI/both icon handles/Settings/
owned source, grouped history, 40 tabs, search and highlighted real terminal pass;
ConPTY 1.25.260930003 is verified. The first terminal test invocation omitted the
GPU readback test environment; enabling it passes without a product change.
Actual installed 150% modal and Settings PNGs were visually reviewed: Up to date
(0.15.0), automatic route and gopls ready. Auto=true/mode=auto, both mirrors,
desktop/nested Start Menu stable launcher targets and normalized user PATH remain.
Copilot authenticated/LSP/SDK=true, networkPromptSent=false. Full closed-resource/
provider history, production/GPUI/VS Code/official Copilot VSIX parity stays active.

## Workspace history candidate (2026-10-07)

Replacement groups now support native multi-file confirmation, worker-prepared
all-buffer Undo/Redo and current-file split. Metadata-only bounded groups preserve
newer edits and reopened identities; whole-batch receipts revalidate every caret,
version and history stack before callbacks. Three-repeat race models pass real
replacement/save groups, external disk safety, held receipts, split and shutdown.
Early candidate used an ignored local core modfile. Complete three-repeat race tests
and vet pass (model coverage 30.2%, search 81.4%). Both console/GUI strict-cgo native
replacement/Cancel/grouped Undo/Redo/current-file split pass; actual 150% modal PNG
was visually reviewed. Source now independently imports public godesktop v0.9.0/
cb8b077, whose exact all-five CI 37555224866 passed. GOWORK=off/no replace downloads
the published module. Full local public-module Windows strict-cgo/race/vet and
console/GUI suites pass, including replacement history and existing VSIX/editor/
tabs/search/opener/watch/terminal/large-file regression. Cross-platform/released/
install evidence is pending.
Installed version remains v0.14.0/core0.8.0.
See history.md for exact behavior and remaining closed-resource/provider gaps.

Initial source CI 37556152512 found a Windows 100% DPI glyph assertion error,
reproduced locally with an owned 1280×820 window. Actual gray-on-blue text was
readable but the original all-channel high-coverage predicate counted two pixels.
Corrected contrast counts 314; an ignored overlay binary with the primary label
removed fails with zero ink despite 4,348 blue background pixels. Failure PNG/JSON
is now retained. Fixed 100%/normal 150% complete native gates and vet pass; exact
replacement source CI/promotion is pending. No app UI/model behavior was changed.

## Native workspace replacement v0.14.0 promotion (2026-10-07)

Immutable application/package source `de2a98fb0a041cd88c0ec5ac658bb2fd41a62f9b`
passed all five jobs in [CI 37552538844](https://github.com/neko233-com/gocode/actions/runs/37552538844):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off independently uses
public godesktop v0.8.0/source d5d139d, whose all-five CI 37549368977 also passed.
Local full strict-cgo/race/vet and console/GUI suites pass (search 81.4%). New
replacement planning, prepared all-buffer adoption, per-file saves/conflicts/undo,
blank-editor focus and captured-preview identity accompany existing real VSIX,
opener/watch/recovered-gopls/terminal/tabs/search/large/MSI/package gates.

Mac normal/1.5/2 gates pass actual owned events and completed Metal pixels.
ARM dimensions are 1024×684 / 1536×1026 / 2048×1368; Intel dimensions are
1280×820 / 1920×1230 / 2560×1640. Windows 2022 100% preview, ARM 200% changed
review and Intel 150% Undo PNGs were visually inspected. Windows 2025 actual
1,073,741,824-byte single-line race search reported 32.0390007 s / 748,656 new
Go allocation bytes / 2,147,492,044 measured I/O bytes. Native GiB search and
navigation also passed. Timing is scoped to that run, distinct from prior runs.

[Publication 37553566847](https://github.com/neko233-com/gocode/actions/runs/37553566847)
reused the exact tested packages and published v0.14.0 at de2a98f. Metadata
`7749adfab8d4d68ed4e24a7fd635ded9ecf0349d` changes only seven known channel files;
two Python policy tests pass. Windows ZIP SHA256 is
`5dab3d34039fa7d2622173cad7855f46f7202f76c1051974f7c5941b19156dea`; MSI is
`7ffdbb25771391a2f21a36fc0da2b7a53950a61f9357f2139cae5c77fedfac45`.
Real signed automatic GitHub update plus independent full direct/manual ghfast.top
ZIP integrity pass. Released bytes pass new native replacement and existing
large/four-close/editor/VSIX/terminal/recovered-gopls/watch/opener/UI/tabs/search
gates. Actual prior v0.4.0/source cfcd3513 renders after owned rollback with shared
extensions; health/version-only checks would not prove native startup.

The user's actual v0.13.0 updater selected 0.14.0/source de2a98f via direct GitHub.
Baseline MSI/launcher remains 0.5.1. Installed GUI/icons/Settings/source preservation,
replacement/search/40-tab/caption/highlighted terminal, embedded official ConPTY
1.25.260930003 and update checks pass. Actual preview and Settings 1920×1230 /
150% PNGs were visually reviewed: Up to date (0.14.0), auto route and gopls ready.
Auto=true/mode=auto, both mirrors, desktop/nested Start Menu stable launcher targets
and normalized user PATH are preserved. Copilot authenticated/LSP/SDK=true;
networkPromptSent=false. Replacement is an atomic UI buffer batch followed by
individual disk saves; late conflicts retain undoable dirty text, not filesystem-
wide atomicity. Exact limits/global undo/individual/diff/JS/PCRE2 gaps are in
replace.md. Full production/GPUI/VS Code/official Copilot VSIX parity remains active.

## Native workspace replacement candidate (2026-10-07)

Source uses independently downloaded public godesktop v0.8.0/source d5d139d,
whose all-five CI 37549368977 passed. Native replacement field/preview, worker
complete-set verification, prepared all-buffer commit and existing per-file saves
are implemented. New/dirty Unicode/CRLF sources, captures, bounded expansion and
stale query/edit/caret/reopen/disk/open-hook/late-save cases pass three-repeat race
checks; Node JavaScript supplies the capture oracle. Captured pointer review also
rejects a different preview appearing before release. Full local public-module
strict-cgo/race/vet and console/GUI regression passes (search coverage 81.4%).
Native preview, actual external stale-file rejection, restore/re-preview, two real
saves, preserved unsaved prefix/EOL and blank-area-focus/native Undo pass; 150%
1920×1230 preview was visually reviewed. The first fixture forgot active=0; then
real blank-editor lost focus was found and production input was corrected.
Mac normal/1.5/2, public package/released bytes and installed promotion are pending
for v0.14.0. Current published/installed app remains v0.13.0. Exact limits and
per-file persistence/global-undo/regex compatibility gaps are in replace.md.

## Native workspace search v0.13.0 promotion (2026-10-07)

Immutable source/package `d4a869e24c9acc2d93cd9aefb17f9303064423b2` passed all
five jobs in [CI 37547147426](https://github.com/neko233-com/gocode/actions/runs/37547147426):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off independently uses
public godesktop v0.7.0. Native content search now covers unsaved snapshots,
Unicode folding, case/word/Go regex, include/exclude and nested Git ignore,
grouped virtual results, cancellation and verified UTF-16/file-backed navigation.
Existing editor/VSIX, tabs, opener, close guards, file watch, recovered gopls,
highlighted ConPTY/PTY, MSI lifecycle and free packaging gates stay green.

Actual Windows 2025 race-enabled single-line 1,073,741,824-byte search reported
16.8320796 s / 811,736 new Go allocation bytes / 2,147,492,044 measured I/O bytes.
Actual GiB native query/regex/control/result/tail navigation also passed with
unchanged source hashes. The local non-race 1.53 s / 360,624-byte result is a
separate run, not a cross-device guarantee. Search package coverage is 81.8%.
Two prior fuzz passes and regex/Git oracles are recorded in search.md.

Mac Intel/ARM normal/1.5/2 search captures pass actual owned NSEvent input and
completed Metal pixels. ARM dimensions are 1024×684 / 1536×1026 / 2048×1368;
Intel dimensions are 1280×820 / 1920×1230 / 2560×1640. The dedicated Mac character
probe fixes the ASCII `[` / diagnostic Command-key collision. Windows 100%
literal-results, ARM 150% selected-Unicode and Intel 200% large-navigation PNGs
were visually inspected. Superseded dc2e1f5 CI 37546900949 was cancelled; only
the final d4a869e source is promoted.
Windows 2025 actual GiB tail-navigation PNG was also inspected at 1024×720 / 100%:
Read-only 1.00 GiB, actual byte 1,073,741,573, completed index and needle page.

[Publication 37548090734](https://github.com/neko233-com/gocode/actions/runs/37548090734)
reused those exact tested packages and published v0.13.0 at d4a869e. Metadata
`c708e34ec719c9d1b00dfa76562854bd0876d9f0` changes only seven known channel files;
two Python policy tests pass. Windows ZIP SHA256 is
`e6eaa330329b54d5bf1fc08885259c6c53ed1b37b7ce47aa102d001150f577f7`; MSI is
`cc4be54ecfde6580c1332bf9246412637f84d02e9ba3e1a56e974468f35a1ee0`.
Real signed automatic direct-GitHub update, independent full direct and manual
ghfast.top ZIP body/hash checks pass. Released bytes pass new search and existing
large/editor/VSIX/four-close/terminal/recovered-gopls/watch/opener/UI/tabs gates.
The owned install then renders actual v0.4.0/source cfcd3513 after rollback with
shared extensions; a health-only rollback claim is insufficient.

The user's actual v0.12.0 updater selected 0.13.0/source d4a869e via direct GitHub.
Stable MSI/launcher baseline remains 0.5.1. Actual installed GUI/icons/Settings,
search, 40-tab and measured-caption gates, highlighted terminal, official embedded
ConPTY 1.25.260930003 and update checks pass. Search selected-Unicode and Settings
1920×1230 / 150% PNGs were visually reviewed: Up to date (0.13.0), automatic
route and gopls ready. Auto=true/mode=auto, both mirror URLs, desktop/nested Start
Menu stable launcher targets and normalized user PATH are preserved. Installed
official Copilot authenticated/LSP/SDK=true, networkPromptSent=false. Full
production/GPUI/VS Code/official Copilot VSIX parity remains active; workspace
replace and other explicit search gaps remain open. The accepted SDK/LSP route
is retained.

## Native workspace search candidate (2026-10-07)

The filename-only activity is replaced with native content search, unsaved
snapshots, Unicode folding, case/word/Go-regex, ignore/include/exclude, grouped
virtual rows and verified UTF-16/large-file navigation. One worker/latest request,
debounce/cancellation and identity/version/focus/generation checks protect input.
Standard regex and Git oracles, two fuzz passes and targeted three-repeat race
checks pass. Native 16 MiB and actual GiB HWND gates pass real inputs/pixels,
stale-disk rejection and unchanged source. Plain reader regex initially timed
out on the actual GiB line; chunked literal-prefix candidates with anchored
verification pass the same scenario. Local literal GiB run: 1.53 s / 360,624 new
Go bytes. Native selected-Unicode/large-navigation captures were visually reviewed.
Full local three-repeat strict-cgo/race/vet and console/GUI regressions pass,
including existing tabs/editor/VSIX/opener/close/file-watch/terminal gates. Search
package coverage is 81.8%; final F4 first/reverse navigation has targeted race
coverage. Workflow lint/diff checks pass. Mac/new release/installation promotion
remains pending.
Published/installed app remains v0.12.0; independent core remains public v0.7.0.
See search.md for precise limits, references and remaining search/replace gaps.

## Native overflow v0.12.0 promotion (2026-10-07)

Immutable application/package source `def207c289953b51fe87ab33d28a69e1b7dab3ab`
passed all five jobs in [CI 37541346744](https://github.com/neko233-com/gocode/actions/runs/37541346744):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. GOWORK=off independently uses
public godesktop v0.7.0/source 5e7789b (all-five core CI 37540201901). Final local
public-module three-repeat race/strict-cgo/vet and console/GUI native suites also
pass. New 40-tab GPU/input gates exercise clipping, wheel/drag routing, same-path
reopen capture identity, ordered/MRU navigation and Control release. Existing
VSIX/opener, four close guards, reload/conflicts, recovered-gopls, real highlighted
terminal, Windows 2025 actual GiB/MSI lifecycle and free package gates stay green.

Mac normal/1.5/2 runs use actual native pointer/key/drag events and real CG/NSEvent
wheel deltas with owned coordinate conversion. ARM logical 1024×684 produces
1024×684 / 1536×1026 / 2048×1368; Intel logical 1280×820 produces 1280×820 /
1920×1230 / 2560×1640. Source Windows 2022 captures 1024×728 at 100%. Windows last
tab, ARM 150% stable-release and Intel 200% second-MRU PNGs were visually reviewed.

[Publication 37542564399](https://github.com/neko233-com/gocode/actions/runs/37542564399)
reused those exact tested packages and published stable v0.12.0. Tag source is
verified as def207c. Metadata `c9a424446ce15adf4254387bc4a0c0805c458d76` changes
only seven known channel/hash/install files; two Python policy tests pass. Windows
ZIP SHA256 is `a9677e599420e65e39378cd03576e183ad937ffba925838ff11158f8e613e6da`;
MSI SHA256 is `40921cb3e0d5fa5a9acd39bba477e86d3ef87e1bbb74cfd64e158da6f7396e4c`.
Real signed automatic direct-GitHub update, separate full direct and manual
ghfast.top ZIP downloads pass integrity. Released bytes pass large browsing,
VSIX/edit/save, four close guards, real terminal, recovered-gopls, file-watch,
delayed opening, measured-caption and 40-tab gates. The owned root then actually
renders the original v0.4.0/source cfcd3513 GUI after rollback with shared extensions.

The user's actual v0.11.0 updater selected 0.12.0/source def207c via direct GitHub.
Stable MSI/launcher baseline remains 0.5.1. Installed GUI/icons/Settings/source
preservation, caption and 40-tab native gates, official embedded ConPTY and update
checks pass. Settings/last-tab captures at 1920×1230 / 150% were visually inspected:
Up to date (0.12.0), automatic route and gopls ready. Desktop/nested Start Menu
targets and normalized user PATH remain valid. Auto=true/mode=auto and both mirror
URLs remain intact. Installed official Copilot authenticated/LSP/SDK=true and
networkPromptSent=false. Full production/VS Code/official Copilot VSIX parity
remains active; the accepted official SDK/Language Server route is retained.

## Native overflow candidate (2026-10-07)

Clipped virtualized tab entries, document-instance input identity, manual wheel/
thumb pan, minimal active reveal, ordered page navigation and held-Control MRU
are implemented. Root native input now separates positioned scroll coordinates
from deltas and reports key releases. Terminal/editor/header routing uses the
pointer position. Local strict-cgo console 40-file native GPU/input gate passes,
including actual wheel/drag, reopened captured close target, two MRU hops and
Control release. Last-tab PNG was visually inspected at 1920×1230 / 150%.
Candidate full tests require propagating the ignored modfile through GOFLAGS to
their spawned builds; an initial direct test flag did not propagate, correctly
rejecting compilation against public v0.6.0. With that propagation, the complete
local three-repeat strict-cgo/race/vet and console/GUI native suites pass, including
opener/VSIX, four close guards, large browsing, highlighted terminal and file watch.
No-cgo tests and workflow lint pass. A real native path-key mutant misclosed the
reopened document on pointer release and failed the boundary gate; restoring
instance identity passed. Upstream titlebar/tab CSS and controller blobs remain
identical at refreshed main 1d25d5d. Mac/public promotion is pending;
public core v0.7.0/source 5e7789b passed all five jobs in 37540201901 and is tagged
immutably. gocode now pins that public module with no local replace. Application
publication/installation remains v0.11.0 pending its independent source/package gates.

## Native image / measured captions v0.11.0 promotion (2026-10-07)

Immutable application/package source `05439efb84156feeb14b1d45f76631bd1d0673cc`
passed all five jobs in [CI 37534367695](https://github.com/neko233-com/gocode/actions/runs/37534367695):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. Independent GOWORK=off uses
public godesktop v0.6.0/source 3bf3e4d (core CI 37532765849, all five green).
The new native caption/selection/tab and Windows GPU-logo gates pass; both Macs
capture real completed Metal drawable pixels at normal/1.5/2 density. Existing
opener/VSIX, file reload/conflicts, recovered-gopls, highlighted terminal, Windows
2025 actual GiB/MSI lifecycle and package gates remain green. Exact-source Windows
2022 README, Mac ARM 150%/closed 200% and Intel 200% PNGs were visually inspected.

[Publication 37535500899](https://github.com/neko233-com/gocode/actions/runs/37535500899)
reused those actual tested packages and created stable v0.11.0. Generated metadata
`b6c7af4a7543d2f1e0d0872cc49ba430bd7e6aae` changes only the seven known channel/
hash/install files; two local Python policy tests passed. No separate full-platform
metadata CI is claimed. Windows ZIP SHA256 is
`780fe10b91fafc191312723feaf781f4d1ce17d545cfa343e8d65bf298232530`;
MSI SHA256 is `ce6af9189d4955d47e1ca8fd4e5817a14256219597d5c66e68c7cde148227950`.
Real signed automatic direct-GitHub update, separate full direct and manual
ghfast.top full ZIP downloads passed integrity. Released-byte visual/tab, opener,
VSIX/save, four close modes, large browsing, terminal, recovered-gopls and file-watch
checks passed. The owned root then actually rendered the original v0.4.0/source
cfcd3513 after rollback with shared extensions.

The user's actual v0.10.0 -update command selected 0.11.0/source 05439ef via direct
GitHub. Stable MSI/launcher baseline remains 0.5.1. Installed version/update,
embedded official ConPTY, native visual/tab/opener, VSIX edit/save, recovered-gopls,
file watch/gopls, highlighted terminal, close save, GUI/icons/Settings and source
preservation passed. Actual desktop/nested Start Menu targets, icon handles and
normalized user PATH remain valid. Auto=true/mode=auto and both mirror URLs are
preserved. Installed official Copilot authenticated/LSP/SDK=true and
networkPromptSent=false. Installed README and Settings GPU PNGs were visually
reviewed: complete Latin/CJK names, actual logo, Up to date (0.11.0), automatic
route and gopls ready. Full production/VS Code/official Copilot VSIX parity remains
active; the accepted official SDK/Language Server path is retained.

## Native image / measured captions development (2026-10-07)

The workbench now renders the upstream immutable Code-OSS PNG in the Windows
titlebar (35/16 logical-pixel slot/image from current VS Code main 4861e8b).
Tabs use actual system-font width, preserving final Latin/CJK caption glyphs
before their close controls. New -ui-smoke uses production native views, real
temporary files, completed GPU pixels, native Windows clicks and source bytes;
Mac normal/1.5/2 drawable capture gates are wired for the exact-source CI.
Local 150%-DPI Windows candidate passes used an ignored modfile pointing to the
unreleased core. A real old-width negative control failed the caption/close gate;
the corrected capture was visually inspected. These are candidate results,
not independent public-dependency/CI/release success. Promotion is pending.

Public godesktop v0.6.0 now resolves independently with GOWORK=off, source
3bf3e4dfd1b4a221eb61018f201295b1ce3404f7, all five core jobs green in 37532765849.
The child full local Windows strict-cgo/race/vet/native suite (Repeat=1) passed
with that exact public module, including console/GUI visual gates, opener/VSIX,
four close modes, large browsing, terminal and file watching. No-cgo and two
Python distribution-policy checks passed. Extra real gopls recovery/watch passed;
the first extra invocation lacked GODESKTOP_READBACK and failed its diagnostic
capture, then the correctly configured invocation passed. No source correction
was needed for that harness setup. Final child source CI/publication is next.

## Asynchronous opening/traversal v0.10.0 promotion (2026-10-07)

Immutable application/package source `0a86cec6e6a56aa439c1474c4bfc82a047ce5122`
passed all five jobs in [CI 37527105167](https://github.com/neko233-com/gocode/actions/runs/37527105167):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. Both real Mac VSIX editor gates
now pass the document acknowledgement barrier. This includes worker/queue/race,
native delayed disk/scan, actual VSIX edit/save, gopls/recovery/reload, terminal,
Windows 2025 actual GiB browsing/MSI lifecycle and Mac bundles. Public godesktop
v0.5.3 remains independently pinned with GOWORK=off. Downloaded exact-source
Windows 2022 pending-read/awaited-VSIX GPU PNGs were visually inspected.

[Publication 37528232380](https://github.com/neko233-com/gocode/actions/runs/37528232380)
reused the exact tested packages and created stable v0.10.0. Generated metadata
`ba8555174b53d9e1119b615d3ebdb1d130ad327d` changes only the seven known channel/
hash/install files; two Python hash/input-policy tests passed. No extra full-platform
metadata CI is claimed. Windows ZIP SHA256 is
`e38bbddf98f89521bbfad4247a8eaa12c2a3106cb1ddf6d282d60338634e69b1`;
MSI SHA256 is `65e104577ae4ca81c2bdc2808bdecaa59bddc1647adb508f4232af3cbc8996c6`.
Signed actual automatic direct-GitHub update, separate full direct download and
manual ghfast.top full ZIP download passed integrity. Released-byte opener,
four close modes, VSIX edit/save, large browsing, terminal, recovered-gopls and
file-watch/gopls native checks passed. The owned root rolled back to real
v0.4.0/source cfcd3513 and rendered its GUI with the same extension store.

The user's actual old v0.9.0 -update command applied this release via direct GitHub.
Selected payload/source is 0.10.0/0a86cec; stable MSI/launcher baseline stays 0.5.1.
Installed version/update/embedded ConPTY, native delayed opener, VSIX editor/save,
recovered-gopls, file-watch/gopls, highlighted terminal, close save, GUI/icons/
Settings/source preservation passed. Installed awaited-README and Settings GPU
captures were inspected: the original dirty tab remains, actual new file renders,
Automatic=true/automatic route, Up to date (0.10.0) and gopls ready are visible.
Desktop and actual nested Start Menu shortcuts/icon files and normalized user
PATH are valid. Auto=true/mode=auto and both mirror URLs are preserved. Official
Copilot authenticated/LSP/SDK=true, networkPromptSent=false; no new AI prompt.

Initial source a335ff0 failed both real Mac VSIX editor gates in 37525452237;
Windows/Ubuntu passing did not override it and that source was not published.
Root validation still precedes ui.Run; Explorer is capped and not a recursive
live tree. Full production/VS Code/official Copilot VSIX parity remains active.

## Asynchronous opening/traversal v0.10.0 development (2026-10-07)

Initial source a335ff0 CI 37525452237 passed Windows 2022/2025/Ubuntu and the new real
Mac delayed-open acceptance, but both Mac jobs failed subsequent VSIX editor
acceptance with no active document. Native startup could finish before the Node
document/focus FIFO drained. That source is not released. The corrected bridge
adds a FIFO receipt barrier before commands/providers, covering delayed initialize/
open/focus, cancellation, full/stopped queues and real nested native RPCs.
The corrected source passed five repeated Windows race queue/open/scan regressions,
the full Windows strict-cgo/native suite (Repeat=1), no-cgo and workflow/diff checks.

New source uses one bounded disk opener and batched Explorer scan after the
native window starts. VSIX/Problems/definition navigation waits for actual target
documents; aliases preserve live dirty buffers. Cancelled transfers, closed-path/
new-focus guards and dropped-dispatch large-index cleanup have race regressions.
Local repeated worker/scan/RPC tests and real Windows race-built -open-smoke
passed; owned pending-read/awaited-VSIX GPU PNGs were inspected: typed green text/
dirty tab remain during held I/O, and the actual README renders after awaited
extension navigation. The fixture source files stayed unchanged.

The first full local Windows run failed TestNativeWorkbenchAMD64's fixed-coordinate
save after asynchronous startup. Its old gate checked only editor background,
which can precede loading or the loading banner's removal. The corrected gate
waits for source-row pixels at final input coordinates. Two repeated actual
workbench/OS-close regressions passed. Full local Windows amd64/GOAMD64=v1
strict-cgo/race/vet/console+GUI/native opener/editor/four-close/large/terminal/
file-watch validation passed (Repeat=1), as did no-cgo tests, two channel-policy
Python tests, actionlint and diff checking. Real installed-managed gopls native
formatting/hover/definition/completion/crash
recovery and file-watch/VSIX/new-source-hover also passed from the new local binary.
At this development checkpoint, platform promotion and installation were pending;
the installed app was v0.9.0/source abadd6d. Final promotion evidence is above.

## Editable file watching v0.9.0 promotion (2026-10-07)

Immutable app/package source `abadd6d0d53109f7445a41ee92652ada1491d7ff` passed
all five jobs in [CI 37518877841](https://github.com/neko233-com/gocode/actions/runs/37518877841):
Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu. This includes actual external
atomic replacements, native conflict/cancel/confirmed reload, EOL undo/redo,
VSIX diagnostic changes, actual gopls hover of reloaded source, language recovery,
terminal, Windows 2025 real GiB browsing and MSI lifecycle, and Mac packages.
The passing Windows 2022 conflict/dialog/final owned-GPU captures were downloaded
and visually inspected. Public godesktop v0.5.3/source d22bf93 passed all five
in 37513434533 and is independently pinned with GOWORK=off.

[Publication 37520177277](https://github.com/neko233-com/gocode/actions/runs/37520177277)
reused the exact tested packages and created stable v0.9.0. Generated metadata
`626671a5897052929ff0bfe8c714f93b6ec01773` changes only the seven known channel/
checksum/install files; local two Python hash/input-policy tests passed. No
additional full-platform metadata CI is claimed. Public Windows ZIP SHA256 is
`42fe5d4552df91dc23df58900b2f9a92ea9a5571dc8f9019fcac0b15fe2b2c25`;
MSI SHA256 is `7342039b456539422ec26b1c9e20c988e21467bedddce74d8d66df440907974e`.
Actual signed automatic direct-GitHub update, separate full direct download and
manual ghfast.top full download passed integrity. Released-byte large browsing,
four close modes, VSIX edit/save, terminal, recovered-gopls and file-watch/gopls
native checks passed. The owned root then rolled back to real v0.4.0/source
cfcd3513 and rendered its GUI with the same extension store.

The user's existing stable launcher applied v0.9.0 through its actual old -update
command, direct GitHub. Selected payload/source is 0.9.0/abadd6d; MSI/stable-launcher
baseline remains 0.5.1. Installed version/update metadata, official embedded
ConPTY, actual file-watch/gopls controls and pixels, recovered-gopls, highlighted
terminal, save, GUI/icons/Settings/source preservation, shortcuts/icon files and
normalized user PATH passed. Auto=true/mode=auto is retained. Installed final
reload/Problems and Settings captures were inspected: new external function,
clean tab, gopls ready and Up to date (0.9.0) are visible. Installed official
Copilot authenticated/LSP/SDK=true, networkPromptSent=false. No new AI prompt.
Framework/child harness and coverage ledger remain active: recursive watching,
asynchronous ordinary opens/traversal, huge-browser reindex, diff/merge and full
VS Code/official Copilot VSIX compatibility are unfinished (see files.md).

## Editable disk watching development and negative controls (2026-10-07)

Initial source 6001f54 CI 37515729600 passed Ubuntu and Mac ARM but Windows 2022
failed its new actual-gopls/file-watch native fixture: external atomic rename
returned Access denied during a concurrent gopls read. That source is not released.
The correction uses deletion-sharing Windows observation/hash handles and bounded
sharing-failure rename retries, rechecking the expected disk hash on each attempt.
Actual held-descriptor/long Unicode-path save/retry/external-write/cancel tests
passed repeated local race checks, including ten repeated stale-read/reload/save
sharing runs and ten long Unicode-path descriptor runs. The legacy rename held
handle test failed despite deletion sharing, requiring FileRenameInfoEx/POSIX.
A local exact-size Win32 filename buffer then exposed intermittent invalid paths;
adding the required NUL storage (excluded from FileNameLength) passed the ten runs.
The original CI's other four jobs passed, including MSI on Windows 2025. The final
complete local Windows rerun passed, followed by targeted target/cancel tests,
rebuilt actual-gopls watch/native pixels and the no-cgo suite. Packaging includes
the exact pinned fsnotify BSD notice on Windows/Mac. Final promotion is above.
No native gate
is suppressed and no failed source has been published.

Local committed-source 0ad8fec package extraction rejected the added standalone
FSNOTIFY-LICENSE.txt because prior updater versions permit a fixed root layout.
It was not published; CI 37518524702 also failed Mac ARM package extraction before
being superseded/cancelled. The package now
appends the unmodified BSD notice to existing LICENSE.txt, retaining the legacy
layout and integrity/health checks. Clean-source ZIP/native health and final source
CI passed at abadd6d, as recorded above.

Public godesktop v0.5.3/source d22bf93187911a2fd829c993a6b096338e9a1b64 passed
all five jobs in CI 37513434533 and is independently pinned with GOWORK=off.
The new worker uses fsnotify v1.10.1 parent watches plus bounded reconciliation.
Clean reloads retain versions/EOL/undo; dirty changes show explicit reload/overwrite
controls. Actual file/race tests and native VSIX/gopls acceptance passed locally;
Windows owned clean/conflict/dialog/final captures were visually inspected. The
intentional old-text reload mutant failed and restored source passed. Full Windows
strict-cgo/race/native editor/four-close/large-browser/terminal/watch script passed
with Repeat=1 after the shared bounded reader and parent-recreation test landed.
The first rerun caught the binary-file error classification changing; the restored
Binary/UTF-8 diagnostic passed the final complete rerun. Final actual gopls recovery
and file-watch/gopls modes, no-cgo suite, two channel tests, workflow lint and
PowerShell parsing passed. The final all-platform source/packages/public-byte/
installation evidence is recorded in the v0.9.0 promotion above. Full scope is active;
files.md lists recursive-watch, large-browser and asynchronous-open gaps.

## Language recovery v0.8.1 promotion (2026-10-07)

Immutable source `9a498ab4ae866419a1a8e1c08eb49b15a3be9db5` passed all five
jobs in CI `37509901232`, including both Windows amd64 runners, Mac Intel/ARM,
native recovered-gopls/final-pixel gates, actual GiB browsing and MSI lifecycle.
Publication `37511249130` reused those tested packages and created stable v0.8.1.
The earlier same-source CI `37508299422` passed four jobs, but Windows 2025's
official Copilot LSP initialize reached its 20-second timeout. A local protocol
check passed; the unchanged source passed the complete next run, including that
handshake. No source workaround or weaker gate was used. The corrected first-run
Windows 2022 owned-GPU capture was inspected before publication: greeting(),
retained dirty source, recovered Problems diagnostic and ready status are visible.

Generated channel metadata `5b3a7f309610d5ed095fb38c4e7d8361b6b93392` changes
only the seven known installation/checksum/channel files. Local Python channel
hash/input-policy checks passed. Source gates were not replaced with metadata-only
checks. Actual signed public Windows ZIP SHA256
`e7e0d615ff5748f21bdd354b17667316cf7ef23ce8ebc60a76739665456740cb` passed
automatic ghfast.top, separate full direct GitHub and manual ghfast.top downloads.
Released-byte native large-file/four-close/VSIX-save/terminal/recovered-gopls
acceptance passed. The owned test root then rolled back to v0.4.0/source cfcd3513
and rendered that real prior GUI with the same extension store.

The user's stable v0.5.1 launcher updated the existing v0.7.0 selection to v0.8.1
through its actual -update command via direct GitHub. Selected source is 9a498ab;
MSI/launcher baseline remains 0.5.1. Installed version/runtime, real recovered
gopls/final pixels, native GUI/icon/Settings/source-preservation and update-check
passed. Installed highlighted terminal/resize/interrupt/exit acceptance also passed.
Auto=true/mode=auto remains set. Actual Settings/gopls GPU captures under
.cache/installed-acceptance were visually inspected: Up to date (0.8.1), automatic
route, gopls ready, full greeting() and recovered diagnostic. Shortcut/icon files
and normalized user PATH remain valid. Installed official Copilot protocol reports
authenticated/lspInitialized/sdkConnected=true, networkPromptSent=false; no new AI
prompt was sent. Production/full VS Code and official Copilot VSIX parity remain
active and incomplete.

Each configured language server now has an independent process supervisor with
four exponential-backoff retries in a rolling three-minute failure window. The
fifth failure stops automatic restarts; the native palette can restart/reset
servers explicitly. One generation owns 32 document jobs, eight request slots,
16 events and at most one pending lifecycle/diagnostic UI dispatch. Diagnostic
payloads retain at most 2,000 items and 256 KiB of UTF-8 message text per event.
Disconnect cancels old work and clears only that server's diagnostics/completions;
new initialize replays current eligible unsaved snapshots without stacking hooks.
Result guards include the server session, document identity, version and cursor.

### Development evidence before promotion

Owned real stdio process crash/reinitialize/manual restart, crash-loop budget,
request/job bounds and workbench unsaved replay tests pass. Three randomized
strict-cgo/race workbench repetitions pass, including independent other-server
state and a genuine queued result delivered after the same path is reopened.
Enhanced real gopls native acceptance kills/reaps its own process, edits while
offline, then verifies new diagnostics, hover and completion from the replacement.
Local Windows native acceptance passed; initial capture found the recovered UI
still displayed its old disconnect notice, which is now cleared only when that
server's unchanged notice belongs to the recovered generation. Final console and
GUI subsystem real-gopls checks passed and the corrected owned-GPU capture was
visually inspected: ready status, retained dirty source and recovered diagnostic.
Full Windows amd64 strict-cgo/race/vet, native console/GUI/editor/four-close/
large-file/terminal regression passed. Final supervisor/workbench cases passed
three randomized race repetitions, including 3,000 actual wire diagnostics with
UTF-8/text/item bounds, repaired-runtime manual recovery, and an actual initialize
handler marker before cancellation/reaping. A stopped-after-crash negative control
failed the real-process test; restoring the source passed. Workflow lint/ShellCheck
and diff checks passed. At this development checkpoint, exact cross-platform
promotion was still required and installed stable remained v0.7.0.

Source b767b83787c0cbc9a628e41857efc25928b9a0af passed all five jobs in
37505379810 and publication 37506593322 created v0.8.0. Inspecting downloaded
Windows 2022 GPU evidence then found the final capture still showed the preceding
greet()/Output state despite the successful model checks. A submitted frame count
alone did not prove the newly completed text/panel reached the owned drawable.
v0.8.0 is held as a prerelease without replacing its immutable artifacts/tag;
the user installation was retained at v0.7.0 during correction. Metadata d7b9dbd came
from that publication, not a corrected visual source.

The correction waits three submitted frames and also checks actual owned GPU ink:
yellow ing suffix after greet in the real editor row and the recovered Problems
diagnostic in its existing keyed button bounds. Font metrics/layout/DPI determine
the regions; screenshot bytes are saved only from the exact passing capture.
No product UI marker or artificial acceptance glyph is added. Corrected full
Windows race/vet/native console+GUI/editor/close/large-file/terminal checks passed;
three repeated real-gopls final-pixel runs plus a GUI subsystem run passed. The
positive capture was visually inspected. The later exact-source CI/promotion above
gates this correction.

## Terminal v0.7.0 promotion (2026-10-07)

Real native ConPTY/PTY terminal and shell input highlighting are published at
immutable source `8a0fa07670a3018b42ab86eb2dd45e814099c7c7`. All five source CI
jobs passed in `37498949904`: Windows 2022/2025 amd64, Mac Intel/ARM and Ubuntu.
Publication `37500332017` reused those exact tested Windows MSI/ZIP and Mac
bundles, signed the update metadata and created v0.7.0. Generated channel metadata
is `cacee0d8b5228f63eb37748269721a266530a279`; all five jobs also passed at
that exact metadata commit in manual CI `37500982783`, including MSI lifecycle,
real GiB native browsing, gopls/Copilot and Mac bundle packaging. This evidence
commit changes documentation only; application/package source gates are retained.
Local Windows strict-cgo/race and owned GPU acceptance pass,
including PowerShell 7/5.1, truecolor, Unicode, resize, alternate screen, Ctrl+C,
final exit output, bounded flood/scrollback and owned descendant cleanup. Native
keyboard focus leaves the editor unchanged. See terminal.md for bounds/provenance
and explicit gaps. Independently downloaded source-CI Windows 2022 GPU captures
were visually inspected: distinct pre-Enter command/string colors, readable
truecolor output, native panel background and actual resized shell columns.

Actual signed public v0.7.0 ZIP acceptance passed automatic direct-GitHub download,
native large-file/four-close/VSIX-save/terminal checks and separate full direct
and manual ghfast.top downloads, each verified against signed SHA256
`11bed5aa26e66c76b5fb7eb1db356a15d1f033af2161014c9e27b2dddecc0087`.
The owned root then rolled back to v0.4.0/source cfcd3513 and rendered that real
prior native GUI with the same extension store. Its original baseline is intact.
User installation is promoted and checked separately below.

The user's actual stable v0.5.1 launcher updated the existing v0.6.0 selection
to v0.7.0/source 8a0fa076 via its own -update command using the complete direct
GitHub ZIP. Root remains `C:\Users\14170\AppData\Local\Programs\gocode`;
MSI/launcher baseline stays 0.5.1, actual payload is 0.7.0. Installed -version,
-terminal-runtime-check, real highlighted -terminal-smoke and native GUI/icon/
Settings/source-preservation checks passed. Owned Settings/input/output captures
under .cache/installed-acceptance were visually inspected: Automatic true,
Automatic route, Up to date (0.7.0), gopls ready, distinct input syntax and output
truecolor with correct resized columns. Installed real gopls formatting/hover/
definition/completion/versioned diagnostic clearing passed. -update-check returns
0.7.0/source 8a0fa076; auto=true/mode=auto is retained. Official installed Copilot
check reports authenticated/lspInitialized/sdkConnected=true,
networkPromptSent=false. No new AI prompt was sent. These checks do not establish
full official Copilot VSIX or all VS Code API/function/UI compatibility.
Desktop and Start Menu shortcuts still target the stable gocode-launch.exe;
their installer-owned icon files and normalized per-user PATH remain valid.

### Development negative controls before promotion

Do not promote source 32a4d5a/CI 37490630980 or 3121950/CI 37492140590.
Windows runners needed a raw/VT-input full-screen fixture and a longer bounded
flood deadline under race+coverage. The corrected fixture passed the next runner
tests. Mac real zsh input colors/native resize/Ctrl+C reached exit, but `exit`
inherited interruption status 130; acceptance now explicitly requests `exit 0`.
Small runner fonts and asynchronous GPU completion exposed the exact-RGB/early
snapshot predicate. The gate now waits for actual colored ink in the process-owned
terminal grid and accounts for foreground/background glyph coverage. It retains
both real VT-color and native pixel checks. The later promoted CI gates these fixes.

Bounded IO from source 6f9d398 / CI 37495036441 confirmed Windows 2022 did not
forward either DEC 1049 switch. No primary command was sent because the initial
alternate-state condition never became true. Earlier interpretation as an input
stall/raw-mode problem was incomplete. The same source's Mac ARM/Intel, Ubuntu
and Windows 2025 terminal checks passed. Do not publish it as cross-platform proof.

The correction uses Microsoft's free pinned ConPTY 1.25.260930003, verified native
DLL/helper hashes and embedded executable resources, with no system replacement.
Local full Windows race/strict-cgo/native console+GUI/editor/save/close/large-file
and terminal checks pass. Fresh owned cache extraction and native terminal passed
with an unreachable HTTPS proxy, proving offline resources work. Exact-source
Windows 2022/2025, Mac Intel/ARM, Ubuntu and package CI subsequently passed above.

## Verified foundation

- Public independent gocode submodule; public godesktop dependency now v0.5.2.
- Native Dark Modern workbench, UTF-16 buffers, selections/undo/clipboard/CRLF,
  versioned VSIX edits, language providers and diagnostics.
- Official Copilot LSP/Go SDK real initialization and authenticated synthetic
  completion/chat; native GPU suggestion, Tab acceptance acknowledgement, stream,
  cancellation and retry passed race/strict-cgo with GOWORK=off on Windows.
- Native Windows owned-window Unicode mouse-drag replacement test passed.
- File-backed GiB browsing and huge single-line byte navigation passed real
  strict-cgo/race + process-owned GPU capture. Fixed 768 KiB index; measured
  cumulative index/read allocation 1,911,232 bytes for a 1,073,758,208-byte file.
- Official gopls v0.23.0 native formatting/hover/definition/unsaved completion/
  versioned diagnostics and clearing passed. Main source retained after edits.
- Upstream Code-OSS ICO/PNG/ICNS and MIT provenance retained; Windows icon/version
  resources and actual owned-window taskbar/window icon application implemented.
- `docs/vscode-parity.md` records substantial outstanding capabilities.

## Active objective

1. File-backed GB browsing, byte/line navigation and measured bounded memory.
2. Generic LSP/gopls lifecycle, diagnostics, formatting/hover/definition.
3. Latest VS Code visual reference, icons/resource/bundle metadata and native
   resize/drag/DPI/long-content visual regression.
4. Free MSI/portable/CLI/winget-compatible manifests and macOS bundles/channels.
5. Reachability-aware, configurable, integrity-checked updates and rollback.
6. Install latest validated gocode on this Windows computer and verify launch,
   command, shortcuts/icons/update settings and installed-app behavior.

Distribution/updates are recorded below. Large-file editing, file watching,
broader language capabilities and full parity remain pending. Record immutable commits/runs
and actual deployment evidence when promoting each milestone.
Do not claim the active production/full-parity goal is complete.

Public foundation source `633e754c0c6a695d869317785562678d5f14d756`, CI
`37449327497`: macOS Intel/ARM and Ubuntu passed; both Windows passed native,
editor, Copilot protocol and GiB checks but timed out at gopls navigation.
An actual Windows 8.3 alias reproduced reopening an unsaved document from disk.
The regression fails the old code and passes the canonical-path correction in
three race runs. Public promotion waits for the corrected source's full CI.

Free distribution uses versioned launcher/layout and Windows Installer COM/makecab.

Corrected source `cfcd3513aedc4ec50ae19625fbd2f04446039abe` passed all five jobs
in CI `37451395121` and is the immutable v0.4.0 source release. The parent gitlink
was advanced by godesktop commit `e1af1980d2bcf09391f374500442a66426b2cece`.

Distribution prototype (uncommitted, disposable payloads only) now passed real
per-user MSI install, stable command/native launch, Start Menu/desktop icon targets,
custom-root remembered uninstall and PATH removal. A 0.4.0 -> 0.4.1 synthetic MSI
upgrade retained the root, superseded a stale current.json pointer, launched the
new native payload and uninstalled its registered files. No final user install
exists. Initial custom-root uninstall revealed missing root persistence; RegLocator/
AppSearch with a property restore fixed it. Generated reports/cabinets stay ignored.

The automated disposable MSI acceptance now passes per-user installation, icon/
shortcut targets, corrupted-cabinet rollback with the old product still registered,
normal upgrade, downgrade rejection, native launch, owned pointer cleanup, workspace
preservation and PATH removal. Early removal of the old app revealed a Windows
LUA rollback registration problem; scheduling InstallExecute before old removal
fixed it. The failed prototype was recovered using Windows Installer, including
its advertised registration, files and PATH; no manual registry/delete workaround.

Updater race tests pass real compiled-executable health checks, signed metadata,
bad-route fallback, SHA256 mismatch, ZIP traversal, cancellation, exclusive process
locks, next-launch activation and rollback. Fixed 512 MiB archive/expansion bounds.
An MSI baseline supersedes stale update pointers and remains the rollback target.
Real Windows mouse/keyboard settings persistence and owned GPU capture passed:
`.cache/updates-native-settings.png`. Automatic/manual routing and toggle were saved
without changing the active document. Final published/install evidence is below.

`gocode -install-copilot` installs the embedded npm lockfile with no lifecycle scripts
into the per-user versioned tools directory. This machine's managed runtime passed
real authenticated LSP initialization and SDK connection without an AI prompt.

Prototype promotion gates were: new exact-source CI (including macOS packages and MSI),
published immutable installers/signed manifest/custom channels, real published-byte
update/rollback and final local installation. Public registry acceptance is separate.
Never publish dirty/synthetic prototype packages or private signing keys.

Distribution source `8bccd29b23cd181e074f1c8cdefc6be1ea2e8f59` passed all five
jobs in CI `37459941417` and publication workflow `37461001171`. Real published
Windows ZIP update acceptance then exposed PowerShell 5 backslash ZIP paths.
The strict updater rejected it and kept the immutable v0.4.0 baseline selected.
v0.5.0 was held as a prerelease; no final user install occurred. v0.5.1 uses
canonical ZIP paths and a new package gate that exercises production Stage and
real native Probe on the exact generated Windows/macOS archives before publishing.

v0.5.1 source `e7f2d69011040b6c458cb54fc73f4cff695015b6` passed all five jobs
in CI `37461737830` and artifact publication `37462680852`. Windows PowerShell 5
also drops a separate empty optional mirror argument; the maintained bootstrap
now passes `-update-mirror=` and MSI acceptance executes that actual default CLI
installer with an isolated user config. Released app/installer/ZIP bytes remain
immutable; a corrected `install-windows.ps1` helper is published separately.

Final local install (2026-10-06): corrected PowerShell 5 command installer downloaded
the actual v0.5.1 public MSI over direct GitHub, verified its pinned SHA256 and
installed to `C:\Users\14170\AppData\Local\Programs\gocode`. Version/platform/source
match `0.5.1 windows/amd64 e7f2d69011040b6c458cb54fc73f4cff695015b6`.
Start Menu/desktop shortcuts target the stable GUI launcher with icon metadata;
user PATH includes the install root, launcher VERSIONINFO reports 0.5.1. Both native
icon handles, a real owned-GPU Settings frame and source preservation passed.
Screenshot `.cache/installed-acceptance/updates-native.png` was visually inspected:
Automatic true, Automatic route, GitHub, Up to date (0.5.1), gopls ready.

Installed Copilot runtime/gopls commands succeeded; official SDK/LSP handshake
reports authenticated=true without a prompt. Installed real gopls native formatting,
hover/definition/completion/diagnostic clearing also passed with owned GPU capture.
Live signed public ZIP update from the original v0.4.0 source binary to v0.5.1
passed extraction/health/launcher/native large-file rendering. Direct and manual
ghfast downloads passed signed-asset SHA256, and rollback launched the exact
v0.4.0 source again. These are owned acceptance roots, separate from the final
MSI install. Installed -update-check and -update verify current GitHub metadata
and report up-to-date. No all-feature or full production parity claim.
Unsaved-window close protection is next framework work.

Next editor source (v0.6.0 in development) now depends on public godesktop v0.5.0,
immutable framework source `eff4097c302ed52db98b144352b183ce058120ac`, with all
five jobs green in framework CI `37464670639`. Native OS/custom titlebar closes
defer through its CloseRequested guard. Dirty window/tab confirmation offers Save,
Don't Save and Cancel; Cancel preserves both buffer/version and editing focus.
Close saves use immutable snapshots in a worker and acknowledge only the matching
document version. External disk changes cause save rejection, preserving the
unsaved buffer and other program's source. Disk hash reads/writes are capped at the
8 MiB editable policy with fixed 128 KiB blocks and cancellation checks.

GOWORK=off local race runs passed close/cancel/immutable snapshot/external-change
tests. Real Windows native save, discard, cancel/focus restoration and rejected
external-change close all passed. `.cache/close-native/save.png` was inspected:
centered native confirmation, dimmed workbench, readable buttons and dirty tab.
Full new-source CI, cross-platform workbench close smoke, publication/update and
local deployment of this next editor revision remain pending. The installed app
is still the verified v0.5.1 release. Existing normal Ctrl+S/VSIX saves still use
the synchronous save adapter; move that adapter to acknowledged worker I/O next.

The first new-source CI `37468303977` at `43847119c35831ccea67a9ee8848d56473926078`
passed Mac Intel/ARM and Ubuntu but failed both Windows discard/conflict clicks.
Downloaded owned-window GPU evidence showed a smaller runner viewport. Local
150% DPI additionally exposed physical-client pixels being passed to a DIP pointer
helper. The corrected test converts both actual size and DPI before clicking.
Never promote this failed source or treat a local-only pass as all-platform proof.

Next working revision routes ordinary Ctrl/Cmd+S, palette, VSIX and close saves
through the same bounded actor; production has no synchronous UI disk-save path.
Frozen-path/snapshot writes, serial disk-hash rebase, exact-version acknowledgement,
cancelled/closed requests, 32-job overflow and a discard barrier have real-disk race
tests. Save size is checked before joining the snapshot text. A matching native
saveId coordinates with the framework's next bridge revision; actual VSIX save
smoke verifies saved text/dirty state and one success event. The pure model test
adapter is test-only. Cross-platform native -close-smoke covers four disposable
workspace modes. Public dependency promotion, new exact-source CI, release and
installed update are still pending; installed v0.5.1 is preserved.

Public godesktop v0.5.1 is now pinned (no replace/workspace override), source
`fd5ee95f0fd5df73e8076971abb282d616bd7725`, all five jobs green in framework
CI `37471310727`. Local GOWORK=off child race tests passed with all packages and
three randomized repetitions, including actual OS-close/mouse cases. Native
four-mode workbench close acceptance passed; final exact-dependency VSIX smoke,
conflict screenshot, new child CI and installation promotion follow next.

Exact public v0.5.1 dependency local Windows validation now passed: strict-cgo
amd64 race/vet, console+GUI native smoke, real installed VSIX Document.save with
one event, CRLF disk bytes, native undo/redo/completion, all four native close
modes and large-file browsing. Actual OS-close save/discard/cancel/conflict tests
also passed three repeated randomized package runs. Owned GPU capture
`.cache/close-native/smoke-external-conflict.png` was visually inspected: readable
wrapped conflict reason, unchanged dirty tab and working Save/Don't Save/Cancel.
Actionlint/ShellCheck and diff checks passed. New exact-source CI/promotion is next.

Limits: close confirmation currently disables decisions while its save/barrier is
active; request cancellation is supported at the save API but an interactive
cancel-in-progress control is still needed. A kernel-blocked write/fsync can outlive
the worker context; explicit forced shutdown has a bounded three-second wait.
Regular open/tree I/O, filesystem watchers/transactional conflicts and the broader
editor/services/function coverage ledger remain unfinished.

Promotion audit found a builtin rollback regression in `23ac180...`: installing
the sample VSIX v0.4.0 into the shared user root made the real v0.5.1 executable
fail startup with "extension version already installed". An owned local negative
control ran the actual installed old release, new source, then old release again
and reproduced that failure. This source must not be promoted even if its CI passes.
Builtin payloads are now stored in hidden .gocode-bundled/<version>/ roots; the
shared old installation stays untouched. Cross-platform unit checks verify
idempotent new selection and byte-preserved old manifests. Real old-release GUI,
new managed VSIX save/GUI, then old-release GUI rollback now pass using one owned
extension root. Live signed release acceptance must also render the prior release
after rollback, not merely print its version.

CI `37473079133` did pass all five jobs at `23ac180ed0a36cadd9bcd4f729e092dc71040559`,
including four Mac close modes, Windows DPI-derived discard clicks, GiB browsing,
official gopls/Copilot and MSI/package gates. The separate actual-release rollback
negative control above still prevented promotion. The corrected builtin isolation
passed local real old/new/old native startup plus acknowledged VSIX save, full
strict-cgo race tests, vet, workflow lint and diff checks. Publish only after the
next exact-source CI verifies this final correction.

v0.6.0 is now published and installed (2026-10-06): immutable application source
`242a1a917b189ded070ac00c7ac75f8eba9b6296`, all five jobs green in source CI
`37474545017`, publication workflow `37475908095`. Actual tested Windows MSI/ZIP
and Mac Intel/ARM bundles were reused; signed update metadata and maintained
PowerShell/macOS installers, winget/Scoop/Homebrew channels were generated at
metadata commit `2ff7885cd40be9b5a4a94e29d51598d760674f18`. Registry acceptance
remains separate. No immutable tag or released artifact was overwritten.

Real signed public ZIP acceptance staged/health-checked/launched v0.6.0 from the
original v0.4.0 binary, ran native large-file/four-close/VSIX-save checks, then
rolled back to source `cfcd3513aedc4ec50ae19625fbd2f04446039abe` and actually
rendered that prior app using the same extension store. Automatic routing chose
gh-proxy.com despite GitHub probe reachability=true. A separate full direct
GitHub download timed out/failed size verification; it did not change selection.
Manual gh-proxy download independently passed signed SHA256. No direct-download
success claim for this run. The owned test root was rolled back, separate from
the user's final installation.

The real installed v0.5.1 launcher then applied v0.6.0 via its own -update command,
verified public metadata/archive/native health and selected exact source 242a1a9.
Install root remains `C:\Users\14170\AppData\Local\Programs\gocode`; MSI baseline
and stable launchers remain 0.5.1, current payload is 0.6.0. Auto=true/mode=auto,
installed -update-check returns 0.6.0/source 242a1a9. Start Menu/desktop/PATH remain
valid. Installed GUI/icon/settings/source-preservation, actual close save, VSIX
Document.save/CRLF/undo/redo/completion and real gopls native checks passed.
Installed official Copilot check reports authenticated=true, lspInitialized=true,
sdkConnected=true, networkPromptSent=false. No new AI prompt was sent by this check.
Owned `.cache/installed-acceptance/updates-native.png` was visually inspected:
Automatic true, Automatic route, Up to date (0.6.0), gopls ready. The source harness
continues to track the incomplete full VS Code/official Copilot VSIX/UI parity.
