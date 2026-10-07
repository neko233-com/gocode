# Native color and terminal pixels

Public core v0.11.0 preserves actual CoreText color-font RGBA in the Metal
atlas. Ordinary glyphs keep foreground tint. Mixed glyph pages retain painter
order in one draw; the active cache remains at most 16 MiB/16 pages, with actual
in-flight byte accounting. The parent color-glyphs.md records implementation and
independent CoreText/opacity/clip/eviction/recovery pixel gates.

`-groups-smoke` now records intrinsic yellow emoji pixels in each native editor
viewport. Mac's initial shared split requires actual 😀 color, distinct from
the brown string foreground. Existing selection, editing, scrolling, sash,
close/save and shared-document assertions remain required.

The previous non-Windows terminal capture helper returned nil without reading
pixels. Terminal acceptance now uses the same process-owned HWND/Metal drawable
capture as editor acceptance, clips to the actual terminal grid and checks
command/string/ANSI truecolor ink. It saves PNG and JSON counts even when no
screenshot directory is supplied. Readback is enabled explicitly for this
native diagnostic. The real Mac PTY command outputs 😀; its intrinsic yellow
pixels must survive the surrounding ANSI foreground color. Both Mac jobs run
normal/1.5/2 density terminal checks and retain their actual artifacts.

At v0.18.0 Windows keeps real ConPTY command/output/window resize/interrupt/exit
and native color thresholds; its core still uses monochrome color-font coverage.
The candidate below adds Windows intrinsic color. Mac checks grid height changes;
its terminal diagnostic still lacks an independent native window-width resize.
The fixture does not establish full shell integration, terminal links/styles,
IME/grapheme/bidi, accessibility or complete VS Code parity.

Local Windows strict-cgo/three-repeat race/vet and actionlint checks pass. Actual
console and GUI terminal colors/resize/interrupt/exit and native split gates
pass; the GUI output PNG was inspected. Manual hidden-window startup caused a
no-frame timeout; the repository's normal ProcessStartInfo/CreateNoWindow GUI
harness passes unchanged. This is diagnostic launch scope, not a product fix.
Immutable core source 6706e59bd83b07705eed1e2d378dc954dd2a2970 passes all five
jobs in [CI 37574802922](https://github.com/neko233-com/godesktop/actions/runs/37574802922)
and Intel diagnosis 37574803095, including both Mac normal/1.5/2 pixel gates.
Static texture-slot/explicit-LOD sampling resolves the earlier Intel readiness
and recovery regression without relaxed gates. Actual Intel normal/ARM normal
and 200% PNGs were inspected. VERSION 0.18.0 now imports public v0.11.0 with
GOWORK=off/no replace. Independent application native CI/release and installation
remain pending. Public/installed app stays v0.17.0 until promotion is complete.
Independent public-module strict-cgo/three-repeat race/vet now pass locally.

Public/installed v0.18.0 source 10bebade1495f234d8bf52cba72491ad598421c9 now passes
all five jobs in CI 37575737909; publication 37576841022 reuses those packages.
Both Mac architectures pass normal/1.5/2 terminal and editor color pixels.
Actual ARM 200% terminal and Intel 150% split PNGs were inspected. Each 200%
terminal report has 717 intrinsic yellow emoji pixels; independent split counts
are ARM 5125/5200 and Intel 7482/7448 at 150%. Exact release hashes, signed
direct/mirror bodies, rollback, installed actual GiB/GUI/Settings/ConPTY and
unchanged config/PATH evidence are recorded in status.md. Full parity stays open.

## Windows candidate

Public core v0.12.0 now pins c42b4b43f0a27452937850871681f26746e39d7f and
passes all five jobs in CI 37587610645. Both Windows normal/150/200% reference
RGB and masks match 100%; actual 2022 normal and 2025 200% PNGs were inspected.
Shader indexing/cgo tracking and baseline-phase/direct-layer rendering fixes
preserve original deadlines/assertions. Mac ARM's first 150% viewport timeout
passes on same-source retry; no physical-driver cause is claimed.
VERSION 0.19.0 now imports public v0.12.0 without replace. Independent full
GOWORK=off strict-cgo/three-repeat race/vet and console/GUI native regression pass
locally. Public-module terminal count is 166 and split counts are 2922/3062;
both actual PNGs were inspected, and real Node/editor/terminal gates pass.
Workflow lint/diff checks pass. Exact-source CI/package/release/install promotion
is pending. The older development record below is historical and is superseded
by this public-core state.

Parent source 03ed1c7186b0a95071db3a034317729f1c28ca3e retains native Windows
COLR/COLRv1 RGBA and ordinary R8 in a bounded mixed D3D12 batch. Exact parent
CI 37581933475 is running; immutable public-module promotion is pending.
Local explicit go.work development verifies actual Windows editor split and
ConPTY output emoji, while the public dependency remains v0.11.0 until promotion.

Both Windows and Mac split gates require intrinsic yellow 😀 pixels in each
initial shared viewport. The Windows real ConPTY command now emits 😀 inside
the ANSI truecolor run, and the decoded frame must contain the full
`NATIVE_TRUECOLOR 😀` sequence with that foreground. Completed terminal GPU
pixels must retain intrinsic emoji color in addition to command/string/ANSI ink.
All existing native input/resize/interruption/exit and shared-source assertions
remain required. The native Windows 150% candidate terminal and split PNGs were
inspected: terminal color count is 92; split counts are 1774/1898. Real Node VSIX
shared view/UTF-16/CRLF/save acceptance passes. Independent published-module,
full source/package/release and installed upgrade gates remain pending.

Dedicated SVG/bitmap/currentColor third-party fonts remain unverified in the
parent. Existing grapheme/bidi/IME, physical display migration, Mac native terminal
window-width resize and full VS Code/official Copilot VSIX limits remain open.
