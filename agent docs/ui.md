# Native workbench visual contract

Reference checked 2026-10-07: VS Code stable 1.140.0 and main
`4861e8bae38121e9793aad2fac4ae37e922ed525`. Source measurements:
[titlebar CSS](https://github.com/microsoft/vscode/blob/4861e8bae38121e9793aad2fac4ae37e922ed525/src/vs/workbench/browser/parts/titlebar/media/titlebarpart.css)
uses a 35-pixel icon slot and centered 16-pixel image;
[tab CSS](https://github.com/microsoft/vscode/blob/4861e8bae38121e9793aad2fac4ae37e922ed525/src/vs/workbench/browser/parts/editor/media/multieditortabscontrol.css)
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
pixels. It no longer estimates proportional Latin/CJK names by rune count. Tabs
still need scrolling/overflow, truncation policy and full VS Code tab actions.

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
overlapped the close target. Restored measurement sizing passed. Public-dependency
and exact-source Mac/Windows CI promotion remain pending in the status ledger.
