# Standard language servers

The editor manages standard LSP stdio processes through godesktop's bounded
JSON-RPC transport. [LSP 3.17](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)
and [official gopls](https://go.dev/gopls/) are the protocol/reference sources.

`gocode -install-gopls` installs `golang.org/x/tools/gopls@v0.23.0` into gocode's
per-user tool directory using Go's verified module download. It does not replace
a global gopls. Default startup prefers that version, then a PATH gopls. A missing
server leaves the native editor usable; no server executable is downloaded merely
by opening an untrusted workspace.

The native command palette also offers **Install Go Language Support** and
**Install TypeScript Language Support**, with the actual pinned VSIX version in
each title. The equivalent explicit installation commands are:

```text
gocode -install-language-extension go
gocode -install-language-extension typescript
```

These install original Open VSX packages into the selected extension directory:
`golang.go` 0.50.0 with gopls v0.23.0, and
`vscode.typescript-language-features` 1.95.3 with its actual TypeScript 5.6.3
tsserver and the Apache-2.0 typescript-language-server 6.0.1 stdio wrapper.
Go installation needs Go 1.27; TypeScript needs Node.js >=22.22.2 and npm.
Dependencies live under `<UserConfigDir>/gocode/tools/language-extensions`.
Nothing is installed globally. Opening a workspace does not install dependencies.
Reinstallation verifies the complete original VSIX inventory and existing tool
hashes before reusing them without network access.

These recognized packages activate an explicit **native LSP adapter**, providing
completion, diagnostics, formatting, hover and definition through real servers.
Their original JavaScript activation, debugger/testing features and manifest
commands are not executed or advertised as working Node callbacks. Other VSIX
packages continue to use the documented partial Node extension API. This route
does not claim arbitrary vscode-languageclient compatibility or official
Microsoft Marketplace access. Go 0.56.1 exceeds the public core's 256 KiB
manifest limit; choosing that catalog version still fails explicitly. The
versioned language installation command does not silently downgrade a catalog
selection or rewrite/repack its VSIX.

Disable/uninstall immediately invalidates that adapter's UI session, diagnostics
and completion receipts after the settings acknowledgement, then closes its owned
server process tree. Re-enable creates a fresh server identity without stacking
document hooks. Package deletion remains deferred until Reload Window. An
explicit reinstall cancels only its package's pending deletion and preserves a
user's disabled preference. Explicit user LSP configurations stay independent and
retain request precedence; automatic Go fallback cannot resurrect a disabled or
uninstalled adapter. Supported TypeScript IDs include typescriptreact and
javascriptreact for `.tsx`/`.jsx`.

```text
gocode -language-extension-check go
gocode -language-extension-check typescript
```

These process-only checks require the actual installed packages and verify real
initialize/completion/definition/format edits/type diagnostics and shutdown in an
owned temporary workspace. They emit source inputs, original package/dependency
hashes, actual process identities and ordered gates. On Windows they retain
observed Job process handles and require them to be dead after shutdown. They do
not substitute for native Go/TypeScript window acceptance or a released-byte
installation check. Current evidence and remaining gaps are recorded in
[language extensions](../agent%20docs/language-extensions.md).

For other servers, put a JSON array at `<UserConfigDir>/gocode/lsp.json`, or pass
`-lsp-config` explicitly. Commands are executable paths and argument arrays,
without shell evaluation. Configuration is user-owned, not read from workspace
files. Example:

```json
[
  {
    "name": "gopls",
    "languages": ["go"],
    "command": "gopls",
    "settings": { "gopls": { "staticcheck": true } }
  },
  {
    "name": "python",
    "languages": ["python"],
    "command": "pyright-langserver",
    "arguments": ["--stdio"]
  }
]
```

Implemented: UTF-16 capability negotiation; full/incremental synchronization
according to server capabilities; ordered didOpen/change/save/close, including
server-requested save text; workspace/configuration; completion arrays/lists and
TextEdit/InsertReplaceEdit/additional edits applied in one transaction; versioned
diagnostics/clearing; formatting; plaintext hover in Output; first definition or
LocationLink navigation. Snippet insertion is not advertised. Unknown/unavailable
features report an explicit error. Completion sources merge deterministically
with VSIX results instead of discarding each other's asynchronous responses.

Requests run on workers and return with document/version/cursor guards. Transport
cancellation and process shutdown are bounded. Standard source snapshots are
limited to 2 MiB so JSON escaping cannot exceed the transport's 16 MiB ceiling.
Larger native documents remain editable/browsable under their own policy, without
sending an invalid/truncated source snapshot. Server workspace analysis may have
its own resource costs; no general claim of GiB-scale semantic analysis is made.

Shortcuts: Ctrl+Space completion, Shift+Alt+F formatting, F12 definition,
Ctrl/Cmd+K hover. These are also available in the native command palette.
`gocode -lsp-smoke` remains Go-only and verifies a real configured gopls in a disposable module:
formatting, hover, definition, unsaved-prefix completion, diagnostics and clearing,
while retaining the main function and all unrelated source. The separate
`-typescript-lsp-smoke` uses the installed TypeScript adapter and its own typed
fixture. Releases now use local validation; historical cross-platform CI
receipts remain in the engineering ledger.

Each server restarts independently after an unexpected transport/process exit,
with 250/500/1000/2000 ms backoff and at most four automatic retries within three
minutes. A fifth failure stops retries; the native palette's Restart Language
Servers command resets the budget. Startup cancellation also reaps a server stuck
in initialize. Restart uses the existing user configuration, not a workspace file.
Old requests/diagnostics/completions are invalidated; the replacement receives
didOpen with the latest eligible unsaved document snapshots and versions. Hooks
are installed once. Shutdown has one shared three-second acknowledgement.
The child owns a suspended-before-resume Windows Job with an explicit pipe
handle list, or a Unix process group, rather than relying on killing only the
root process. Windows cancellation/disable also kills tsserver descendants;
completed joins and actual observed process-handle exits are checked separately.
Unix children that deliberately escape their group are outside this ownership
contract. A timeout is an explicit shutdown failure, not evidence of completion.

Document queues have 32 slots per generation, requests eight, event queues 16;
each lifecycle/diagnostic stream has only one pending native UI callback. Overflow
restarts that server rather than silently losing document synchronization.
Diagnostics retain at most 2,000 entries/256 KiB message text per event, with
2 KiB per message. Full source/protocol policy remains as above. -lsp-smoke now
also kills its owned real gopls, edits while offline, and verifies replayed unsaved
diagnostics, hover and completion after reinitialization. Source/release evidence
and cross-platform completion status remain in agent docs/status.md.

Not yet implemented: code actions/rename/references/symbols, semantic tokens,
inlay hints, completion resolve/snippet UI, multi-location picker, watched-file
registration and full configuration UI. Server applyEdit
requests currently return `applied:false`; that capability is not advertised.
