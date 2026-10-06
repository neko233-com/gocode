v0.6.0 protects unsaved native windows and tabs with Save, Don't Save and Cancel.
Cancel restores editing focus; failed saves keep the window and unsaved buffer.
Native Windows/macOS close requests follow the framework's guarded close path.

Ctrl/Cmd+S, command palette, VSIX Document.save and close saves now share a bounded
background snapshot writer. New edits during a save remain dirty. Content changes
from another program reject the write instead of overwriting that program's file.
VSIX acknowledgement and notifications emit one success event per saved revision.
File-watch/conflict resolution and filesystem coordination are future work.

Builtin sample extensions use versioned managed storage, preserving the shared
user extension directory so a prior application can still start after rollback.

gocode adds free Windows x64 MSI/portable packages and macOS Intel/Apple Silicon
bundles, pinned command installation scripts and maintained winget/Scoop/Homebrew
manifests. Public package-registry acceptance is separate from these custom channels.

The native update settings select direct GitHub, automatic reachability/latency
routing or a manual HTTPS mirror. Updates verify publisher Ed25519 metadata and
archive SHA256, stage versioned payloads and check the executable's version/source
before choosing it for the next launch. The current editor stays open. CLI rollback
restores a verified previous version. User files/settings stay outside the install.

Windows packaging uses built-in Installer COM/makecab with per-user PATH, shortcut
and Code-OSS icon integration. Acceptance exercises install, damaged-cabinet
rollback, upgrade, downgrade rejection, real native launch and owned uninstall.
Copilot sidecars can now be installed from the embedded npm lockfile with
`gocode -install-copilot`; Node.js 24/npm is required for optional extension/AI hosts.

This is an engineering preview, not full VS Code/GPUI parity. Native bounded GiB
browsing, standard LSP/gopls and official Copilot SDK/LSP build on v0.4.0. Remaining
capabilities and validation limits are recorded in agent docs and docs/vscode-parity.md.
