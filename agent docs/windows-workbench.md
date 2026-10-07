# Windows workbench correction

The current user priority is Windows first, with actual VS Code File menus and
extension interactions. Mac/IME development is deferred. Public/installed app
v0.21.0 remains installed until application promotion completes. Public core
v0.14.0 is verified and imported independently with GOWORK=off.

## Upstream source and license

VS Code stable 1.141.0 resolves to 2a59476c9bfcb90b3ddc372c36762471b7dfad1c
(checked with GitHub API on 2026-10-07). Native Go menu labels/groups, keyboard
behavior and measurements derive from these MIT licensed Code-OSS sources:

- src/vs/workbench/browser/parts/titlebar/menubar.contribution.ts
- src/vs/workbench/contrib/files/browser/fileActions.contribution.ts
- src/vs/base/browser/ui/menu/menubar.css (8px horizontal caption padding)
- src/vs/base/browser/ui/menu/menu.ts (13px font, 24px action rows, separators)
- src/vs/workbench/contrib/extensions/browser/media/extensionsViewlet.css
  (41px search region, extension header/description/publisher/actions)

Source links use https://github.com/microsoft/vscode/blob/2a59476c9bfcb90b3ddc372c36762471b7dfad1c/.
Copyright (c) Microsoft Corporation; license notice is retained in
assets/code-oss/LICENSE.txt. These sources are translated into native Go elements;
no Electron/browser UI is introduced. The actual inspected source snapshots are
cached under ignored .cache/vscode-menu-reference. Existing icon provenance stays
in assets/code-oss/PROVENANCE.md.

## Implemented candidate

File/Edit/Selection/View/Go/Run/Terminal/Help have separate popup trees with
separators, shortcut labels, disabled entries, selection, check state and nested
menus. Pointer hover switches menus and submenu selection. Escape, arrows,
Home/End, Enter, F10 and Alt mnemonics use the same command registry. The command
center is centered in the client window; narrow widths move menus into More.
Unimplemented commands are visibly disabled, rather than opening an unrelated
palette. Native popup placement stays inside the current viewport.

File New Text File creates a real untitled buffer. Open File, Open Folder,
Save As and Install from VSIX use owned Windows IFileDialog on a dedicated STA,
with Unicode paths, overwrite confirmation and cancellation. File/network/disk
work stays off the rendering thread. New target commits are atomic and do not
overwrite a concurrently created target; existing targets retain the bounded
hash-checked replacement policy. Save/Save All/dirty close route untitled buffers
through Save As. Snapshot version checks preserve newer unsaved edits. Save As
invalidates cached caption width and publishes the new file identity. Reload/
folder transitions use existing save/discard/cancel guards; replacement process
launch happens after the old UI/workers/services/Node host exit.

Ctrl+P opens a centered file Quick Open; Ctrl+Shift+P opens the filtered command
palette. They no longer render inline inside the editor. Native selection/click,
Enter, arrows/Escape and bounded Unicode/paste queries share actual commands.

Extensions has installed filters (@installed/@enabled/@disabled), a native detail
editor, exact manifest command contributions, persistent enable/disable, VSIX
installation, deferred uninstall and reload-required actions. Disabled extensions
are omitted from the next actual Node host; uninstall removes only direct owned
version directories after that host exits. Generic command IDs are matched by
manifest ownership rather than guessed publisher prefixes.

Typing an ordinary extension query searches the real Eclipse Open VSX REST API:
https://github.com/eclipse-openvsx/openvsx/wiki/Registry-API. A cancellable single
worker coalesces requests after 300ms; metadata is limited to 1MiB/20 results,
downloads to 64MiB and manifests to 256KiB. Search can return a Mac URL even with
targetPlatform=win32-x64; installation explicitly resolves the selected immutable
Windows x64 version, falling back only to universal, verifies metadata identity/
platform, checks published SHA256 when present and validates VSIX manifest
identity/version before extraction. Native UI explicitly names Open VSX.

## Evidence and remaining scope

- Windows -windows-workbench-smoke passes actual owned pointer File/New/Save As,
  real shell filename controls, exact Chinese/emoji disk bytes, Open/cancel,
  filtered distinct native Ctrl+P/Ctrl+Shift+P and pointer VSIX details/disable/
  enable/uninstall. Screenshots under .cache/windows-workbench were inspected;
  clipped detail buttons and stale Save As tab measurements were corrected.
- Three-repeat race tests pass immutable Unicode/CRLF Save As, exclusive target
  collision/cancel preservation, menu/Quick Open behavior, real enabled/disabled
  Node commands, exact contribution ownership and owned uninstall preservation.
  Real TLS fixtures prove Windows/universal resolution and bad SHA256 rejection.
- Public core v0.14.0 is immutable 08c8e355110b8e7241411e36781c2a1919d81eb5;
  all five core jobs pass in CI 37624236198. The application independently fetches
  that public module with GOWORK=off/no replace and all module hashes verify.
  Complete public-module Windows checks and exact source/release/install follow.
- Final rounded core v0.15.0 is immutable 4d62ed73a5513d8cbc281e05fe9ea43a74fd61ea;
  all five source jobs pass in CI 37642213032. Independent GOWORK=off/GOPROXY=direct
  fetch/no replace and module verification pass. Three workspace-linked shuffled
  strict-cgo/race repeats pass (app 247.071s), plus no-cgo/vet and workflow lint.
  Actual saved Chinese filename/content and extension detail captures are inspected
  after waiting three completed GPU submissions for acknowledged changes.
- The workspace-linked full three-repeat strict-cgo/race suite passes (app
  98.933s). Updated native regression still checks real pointer Save, exact disk
  bytes and an actual bundled VSIX command via its detail/contribution controls.
  Vet rejects raw uintptr-to-pointer vtable access; typed COM objects remove it.
  Race native acceptance then exposes lost out-parameter lifetime in forwarding
  calls; both syscall wrappers now use Go's documented uintptrescapes contract.
  Actual race/strict-cgo shell dialogs, Alt+F/F10/arrows/hover and native acceptance
  pass with that correction, without weakened checks or changed deadlines.
- -extension-catalog-check golang makes real Go HTTPS search/version requests:
  tooltitude 1.56.5, go-tdt-outline 0.0.2 and zencoder 3.85.9007 resolve win32-x64;
  installed=false. Output is retained in ignored .cache/open-vsx-live-windows.json.
  This proves catalog/Windows metadata, not third-party extension execution.

Actual replacement/new-window lifecycle now passes three race repeats: disabled
state survives real process replacement, deferred uninstall removes the exact
owned VSIX after the old host exits, and the actual shell folder picker opens a
new process with the selected workspace. Windows CREATE_NO_WINDOW suppresses
only the console; SW_HIDE would also hide the new workbench and is not used.
Late extension management acknowledgements preserve a newer palette/editor.
Virtual extension detail close restores the prior source; source edit/save actions
are disabled while the read-only detail editor owns the center. Production shell
cancellation uses bounded FindWindowEx loops rather than allocating callbacks
per dialog. The new keymap and repeatable 45-phase gate is recorded in
keymaps-and-idempotence.md.

Full VS Code parity is not complete. Disabled editing/debug/task/appearance
commands, Auto Save, persistent Recent/workspace history, rich README/changelog/
icon/rating detail rendering and complete extension API compatibility remain.
Open VSX is not Microsoft's Marketplace. Availability/download/install does not
prove a third-party extension's APIs work. Official Copilot remains the accepted
SDK/LSP route; official Copilot VSIX compatibility is not claimed. Mac file
dialogs and new Windows acceptance are intentionally not advertised on Mac.
