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

Windows keeps the existing real ConPTY command, output, owned window resize,
interrupt, exit and native color thresholds. Windows native color fonts remain
a separate framework gap. Mac currently checks terminal grid height changes;
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
