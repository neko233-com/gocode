# Native Git / Source Control

Candidate v0.21.0 uses installed Git in a cancellable background worker and the
public godesktop v0.13.0 native UI. Ctrl/Cmd+Shift+G opens Source Control. A
non-repository workspace can be initialized. Staged and working-tree resources
have individual/all stage or unstage actions; clicking a resource opens a
read-only side-by-side diff. A Unicode commit message and Commit button commit
the actual index. Existing identity, hooks and signing configuration are used;
the application never invents an identity or changes global Git configuration.

Porcelain v2 NUL records preserve literal spaces, bracket pathspecs, tabs,
newlines, Unicode and rename origins. Linked worktrees resolve their actual
index; SHA-1/SHA-256 objects are supported. Index SHA256 and HEAD are checked
before actions. An external index/HEAD change requires refresh. Dirty open
documents must be saved before staging their disk contents. Unstaging changes
only the index, including unborn repositories and renames. Commit preserves
unstaged disk bytes. Failed commands retain the message and report real errors.

Diff uses bounded immutable before/after snapshots and Git's no-index algorithm
with external diff/textconv disabled. Original and working/index sides, aligned
line numbers, syntax colors, red/green backgrounds and synchronized virtual
vertical scrolling are native. Binary files show metadata. Files above 8 MiB
report the limit and remain available through the existing bounded file browser.
Unmerged and submodule resources report their distinct unsupported diff modes.

One owned worker/queue, 30-second operation cancellation, 4 MiB status,
16,384 changed resources, 8 MiB per blob/patch, 64 MiB streaming index hash,
64 KiB stderr/message and 1 MiB path selection/input bound memory. Diff rows
are capped at 131,072 and source lines at 65,536; displayed code retains the
existing 400-rune line bound. Status refreshes every two seconds while SCM is
visible. Windows starts suspended, inherits only the three stdio handles, joins
a kill-on-close Job Object before resume, and kills descendants on cancellation.
Mac/Linux use an owned process group; independently detached Unix processes are
not claimed to be contained. Reader draining is bounded after process exit.

Real Git tests cover stage/unstage/index-only commit, Unicode/CRLF, literal
paths/rename/deletion/binary, stale index, ignored files, linked worktrees,
SHA256 repositories, real conflicts/submodules and rejection of a 1 GiB diff
without modifying it. Real helper executables verify exact Unicode stdin,
stderr/stdout caps and owned descendant cancellation. Parser fuzzing checks
malformed records, paths and resource bounds. Controller tests preserve dirty
documents and round-trip actual Git hunk alignment including empty/no-EOL cases.

`-scm-smoke` creates only an owned temporary repository and local fixture
identity. Actual HWND/drawable pointer clicks stage, unstage, restage and commit;
the gate checks actual HEAD/index/disk and both original/staged GPU red/green
and syntax pixels. It captures `working-diff`/`staged-diff` PNG+JSON under
GOCODE_SCM_SCREENSHOTS or .cache/scm-native. The message path exercises native
input handling with Unicode events; it does not claim OS keyboard/IME coverage.
Windows console/GUI and Mac normal/1.5/2 density run in source CI.

Local native Windows gate passes; exact five-platform source CI, immutable
release bytes and installed promotion are pending. Do not infer those from
unit tests. Latest authoritative installed version remains v0.20.0.

Remaining: editable/inline/hunk diffs, conflict resolution/merge editor,
submodule/history/branch/remote/stash UI, multi-repository discovery,
SCM providers and built-in Git extension API, configurable Git authentication
and progress, full diff keyboard/horizontal navigation and accessibility.
Git preflight and command execution are separate operations: cross-process
atomic isolation of concurrent external writers is not guaranteed. Working
diff snapshots can become older than subsequent disk edits; refreshing/selecting
the resource again rereads disk. Existing Git locks still govern mutations.

References: [Git status](https://git-scm.com/docs/git-status),
[add](https://git-scm.com/docs/git-add), [restore](https://git-scm.com/docs/git-restore),
[diff](https://git-scm.com/docs/git-diff) and
[VS Code Source Control](https://code.visualstudio.com/docs/sourcecontrol/overview).
