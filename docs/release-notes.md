# gocode v0.24.0 Windows candidate

This candidate consolidates the native Windows workbench and real Go/TypeScript
language extensions on public godesktop v0.17.0. Future releases are tested,
packaged and uploaded locally against immutable tags. Automatic push/PR Actions
and remote release packaging are retired.

File/submenus, Quick Input and confirmation dialogs now use native rounded
shadows, clipping and body hit testing. Extension details keep their metadata
and actions visible while measured, virtualized content scrolls. Built-in VS Code
and JetBrains shortcuts persist only after the settings writer acknowledges the
actual final write. Existing editing, tabs/splits, search/replace, Git, terminal,
file watching and large-file workflows remain part of acceptance.

Install language support from the command palette or
`-install-language-extension go|typescript`. Pinned original VSIX packages use
native LSP adapters for completion, diagnostics, formatting, hover and definition.
Go uses golang.Go 0.50.0 and gopls v0.23.0; TypeScript uses
vscode.typescript-language-features 1.95.3, tsserver 5.6.3 and
typescript-language-server 6.0.1 with Node.js 24. Reinstallation verifies existing
files. Disable/uninstall releases the owned server tree, and recovery replays
unsaved content while rejecting results from obsolete sessions.

The current catalog's Go 0.56.1 manifest exceeds the published framework's
manifest bound. Explicit native installation selects its documented pinned
version; an arbitrary catalog request is never silently substituted. Open VSX
is the default catalog. A separately authorized Gallery endpoint can be
configured. Original language-extension JavaScript activation, debugger/testing
APIs, full VSIX compatibility and full VS Code parity remain unfinished.
Copilot follows the accepted official SDK/Language Server path.

Windows amd64 MSI and ZIP are this candidate's distribution target. macOS
continues to use its previous v0.23.0 artifacts and channels. Clean committed
source must pass repeated strict-cgo/race native tests, no-cgo tests, vet,
workflow lint and distribution checks. The local publication runner then checks
the signed package, 35 ordered native gates, actual MSI lifecycle and genuine
prior-release rollback before uploading a draft and verifying every downloaded
asset's bytes. Source-specific native passes are recorded in the
[milestone ledger](../agent%20docs/status.md); final package/signing/installation
and publication remain pending until their real receipts exist.
