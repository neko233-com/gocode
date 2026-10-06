# Standard language servers

The editor manages standard LSP stdio processes through godesktop's bounded
JSON-RPC transport. [LSP 3.17](https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/)
and [official gopls](https://go.dev/gopls/) are the protocol/reference sources.

`gocode -install-gopls` installs `golang.org/x/tools/gopls@v0.23.0` into gocode's
per-user tool directory using Go's verified module download. It does not replace
a global gopls. Default startup prefers that version, then a PATH gopls. A missing
server leaves the native editor usable; no server executable is downloaded merely
by opening an untrusted workspace.

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
`gocode -lsp-smoke` verifies a real configured gopls in a disposable module:
formatting, hover, definition, unsaved-prefix completion, diagnostics and clearing,
while retaining the main function and all unrelated source. CI runs this on both
Windows runners and macOS architectures without a paid AI request.

Not yet implemented: code actions/rename/references/symbols, semantic tokens,
inlay hints, completion resolve/snippet UI, multi-location picker, watched-file
registration, crash restart/backoff and full configuration UI. Server applyEdit
requests currently return `applied:false`; that capability is not advertised.
