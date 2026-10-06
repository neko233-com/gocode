v0.5.1 corrects Windows PowerShell 5 ZIP paths and validates every actual generated
updater archive through production extraction and native executable health checks.
v0.5.0's Windows updater archive was rejected safely; its immutable assets remain.

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
