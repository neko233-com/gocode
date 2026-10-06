# Native workbench visual contract

Reference checked 2026-10-07: VS Code stable 1.140.0 and main
`1d25d5df846ea8edf8d0d6e2c742e8dda9d4916b`. Source measurements:
[titlebar CSS](https://github.com/microsoft/vscode/blob/1d25d5df846ea8edf8d0d6e2c742e8dda9d4916b/src/vs/workbench/browser/parts/titlebar/media/titlebarpart.css)
uses a 35-pixel icon slot and centered 16-pixel image;
[tab CSS](https://github.com/microsoft/vscode/blob/1d25d5df846ea8edf8d0d6e2c742e8dda9d4916b/src/vs/workbench/browser/parts/editor/media/multieditortabscontrol.css)
uses a 120-pixel fit baseline with a content minimum. These are source references,
not a claim that gocode matches every VS Code pixel or feature.

Windows titlebar reuses the same upstream Code-OSS PNG as the installation assets.
Its pinned source, SHA256 and MIT notice remain in assets/code-oss/PROVENANCE.md.
PNG decoding and immutable RGBA copying occur once before native startup; each
view reuses that bitmap. godesktop uploads to a private native GPU texture while
resident, including after device recovery. macOS retains the native traffic-light
area and bundle icon rather than adding a duplicate titlebar application icon.

Tab sizing uses actual DirectWrite/CoreText UI-font measurements. The caption has
room for its icon, insets and separate close target, with at least 120 logical
pixels. It no longer estimates proportional Latin/CJK names by rune count.
Native overflow uses a clipped Viewport with cached measured widths, binary
search and visible entries/spacers. The current
[upstream tab controller](https://github.com/microsoft/vscode/blob/1d25d5df846ea8edf8d0d6e2c742e8dda9d4916b/src/vs/workbench/browser/parts/editor/multiEditorTabsControl.ts)
uses scrollYToX and a default 3-pixel scrollbar; gocode follows those behaviors.
Wheel and draggable thumb retain manual pan until an explicit activation,
document-set or viewport-size change requires minimal active-tab reveal.
Tab/close keys belong to document instances, preserving captured input across
index changes and rejecting a reopened instance of the same path. Ctrl+Tab
freezes MRU order until Control release/focus cancellation; Ctrl+Shift+Tab
reverses, Ctrl/Cmd+PageUp/Down use tab order, Ctrl/Cmd+W retains dirty-close guards
and the terminal's word-delete chord. Width policy, pinned/wrapped/preview tabs,
MRU switcher overlay, tab reorder/drop, split groups and full menus remain gaps.

Candidate `-tabs-smoke` opens 40 real files, captures completed GPU viewport,
selected indicator and scrollbar pixels, exercises native ordered/MRU keys,
positioned horizontal wheel and owned thumb drag, then replaces a pressed close
target with a new document at the same path/index before releasing. It checks
the replacement/neighbor survived and every file byte remained unchanged.
Windows uses owned HWND messages and temporarily restores only its UI thread's
keyboard-state table; Mac uses owned native pointer/key NSEvents and real
CG/NSEvent scroll deltas plus Quartz-to-owned-view coordinate conversion. The
unposted CGEvent lacks an AppKit window attachment, so it enters scrollWheel's
shared native delivery method after that conversion. No global input is posted.
Windows console/GUI gates pass at 1280×820 logical / 150%; Mac normal/1.5/2 and
public dependency/release promotion remain pending for this candidate.

`gocode -ui-smoke` opens actual main.go, README.md and a Chinese Markdown file in
an owned temporary workspace, renders the production view, checks final caption
glyphs and the selected indicator in completed GPU pixels, selects a tab and
closes README, then verifies every source byte is unchanged. Windows uses actual
owned HWND pointer messages; macOS uses native view/model controls. Windows checks
the actual blue/white titlebar PNG, separately from WM_GETICON/bundle icon gates.

Windows capture is PID/title-owned winprobe readback. Mac capture uses the public
testing/metalprobe completed-drawable copy, without desktop capture or recording
permission. CI runs Mac normal, 1.5 and 2 drawable-density gates and uploads PNGs.
Assertions must be followed by visual inspection of meaningful passing captures.

Local Windows candidate checks passed at 1280×820 logical pixels / 150% DPI.
Initial README and selected/closed-tab PNGs are in .cache/workbench-ui. Restoring
the previous rune-based sizing produced a real native failure: README.md caption
overlapped the close target. Restored measurement sizing passed. Final public
v0.6.0 dependency and source 05439ef passed all five jobs in CI 37534367695; stable
v0.11.0 was published and installed. Actual released-byte and installed visual/tab
checks passed, alongside signed routes and real prior-source GUI rollback.

Exact-source capture sizes: Windows 2022 1024×728 at 100%; installed Windows
1920×1230 for 1280×820 logical pixels / 150% DPI. Mac ARM actual logical viewport
was screen-constrained to 1024×684, yielding 1024×684 / 1536×1026 / 2048×1368.
Mac Intel viewport was 1280×820, yielding 1280×820 / 1920×1230 / 2560×1640.
Windows source/installed README and installed Settings, Mac ARM 150%/closed 200%
and Intel 200% passing captures were visually inspected. Metal drawable readback
contains the native-rendered workbench; AppKit traffic lights/window chrome are
separate native controls and are not part of the drawable PNG.
