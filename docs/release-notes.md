gocode v0.21.0 adds native Source Control using real installed Git and public
godesktop v0.13.0. Ctrl/Cmd+Shift+G opens repository status, initialization,
individual/all staging and unstaging, a Unicode commit message and index-only
commit. Existing identity/hooks/signing settings are respected. Dirty open
documents and stale HEAD/index snapshots reject unsafe actions; unstaging and
committing preserve working-tree bytes.

Clicking a resource opens a native read-only side-by-side diff with aligned line
numbers, syntax colors, red/green changes and virtual synchronized scrolling.
Original/staged/worktree text is derived from exact bounded snapshots. Literal
Unicode/CRLF/rename paths, SHA256 repositories and linked worktrees are covered;
binary metadata, large-file bounds, conflicts and submodules fail explicitly.
Returning to an editor tab restores editing, and delayed diff jobs preserve newer
focus. Escape/Ctrl+W closes the diff; page/arrow/home/end navigate its rows.

Git operations use a bounded cancellable worker. Windows launches suspended with
only owned stdio handles and joins a kill-on-close Job Object before execution;
Mac/Linux use owned process groups. Actual descendant cancellation, Unicode
stdin, stdout/stderr caps, malformed status records and real Git repository
fixtures complement the native pointer stage/unstage/restage/commit gate.
Windows console/GUI and both Mac architectures retain GPU diff PNG/JSON evidence,
including Mac normal/1.5/2 densities. Existing terminal/VSIX/large-file/editor/LSP
regressions remain required.

Publication requires successful exact-source five-platform CI and reuses its
tested packages. Free signed metadata, direct/mirror updates, rollback and user
settings/PATH preservation remain supported. Copilot continues through the chosen
official SDK/Language Server route while VSIX compatibility expands.

Full diff editing/hunk staging, merge/history/branch/remote/multi-repository and
extension SCM APIs remain pending, alongside full VS Code/GPUI/Copilot VSIX
parity. Precise bounds and source/release/installed evidence are recorded in
agent docs/scm.md and status.md.
