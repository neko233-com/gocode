gocode v0.22.0 prioritizes Windows workbench menus and extension interactions,
using the public native Go/godesktop v0.15.0 framework.

File, Edit, Selection, View, Go, Run, Terminal and Help now have distinct native
popups, submenus, separators, shortcut labels, disabled actions and keyboard
selection. Unpressed hover, Alt mnemonics, F10/arrows/Enter/Escape and narrow
menubar overflow are handled. The command center is centered in the window;
Ctrl+P opens file Quick Open and Ctrl+Shift+P opens a filtered command palette.

Current Code-OSS Modern UI sources guide rounded controls, 8px menus/Quick Input
and the editor's 1px rounded frame. Descendant GPU clipping applies to text,
intrinsic emoji, images and nested backgrounds; rounded hit areas reject corners.

File New Text File creates an untitled editable buffer. Open File/Open Folder,
Save As and Install from VSIX use process-owned Windows shell dialogs on a
separate STA. Unicode paths/emoji, cancellation and overwrite prompts feed
bounded immutable snapshot saves. New targets commit without replacing a racing
file; existing targets retain hash conflict guards. Save All/close confirmation
route untitled documents through Save As. New windows/reload/folder transitions
retain source files and close the old services before replacement startup.

Extensions has native search/details, actual manifest contributions, local VSIX
installation, persistent enable/disable and deferred uninstall/reload. Real
Open VSX searches use cancellable bounded workers. Downloads explicitly resolve
Windows x64/universal versions, check available SHA256 and validate the selected
VSIX identity/version before extraction. Disabled extensions are absent from the
next real Node host. Closing a detail editor restores source focus.

The native -extension-gallery-url option supports an authorized compatible VS
Gallery service: source-derived query headers/flags, bounded responses, Windows
x64/universal package selection and actual VSIX installation. The selected store
name appears in the workbench. Microsoft's Marketplace requires separate service
authorization for alternative products; default catalog remains Open VSX. Real
TLS/ZIP/Node tests prove the protocol and installed command, not official access.

VS Code and JetBrains keyboard presets are built in and switch through native
Settings or the command palette, with persisted choices and matching menu labels.
JetBrains supports Find Action/Go to File/Recent Files/double Shift, Save All,
close/format/definition/hover/search/replace/terminal/settings for existing features.
Reserved unsupported shortcuts cannot accidentally invoke VS Code close/new file.
Repeat acceptance isolates temporary workspaces/settings/extensions, cleans them
after each run and reuses evidence filenames; owned process jobs clean failures.

Owned Windows console/GUI acceptance exercises actual pointers, shell dialogs,
exact Chinese/emoji disk saves, keyboard/hover, genuine VSIX actions and actual
new/replacement application processes. Public-module strict-cgo/race/PE/native
and existing GiB/editor/Git/terminal/LSP/Copilot regressions remain required.
Publication reuses exact-source CI packages; free update/install channels retain
the stable launcher, automatic/direct/mirror routing and rollback checks.

Full VS Code UI/API parity remains unfinished. Unimplemented menu commands stay
disabled. Rich extension README/icons/rating/theme rendering, Auto Save, persistent
Recent/workspace history and full debug/task/editing APIs are still pending.
Open VSX is a separate catalog from Microsoft's Marketplace. Copilot keeps the
accepted official SDK/Language Server integration; official Copilot VSIX parity
is not claimed. New native file dialogs target Windows; Mac feature work is
deferred. Exact source/test/install evidence is in agent docs/windows-workbench.md.
