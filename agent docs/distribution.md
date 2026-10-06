# Free distribution contract

Planned maintained channels: Windows unsigned MSI and portable ZIP, PowerShell/CLI
installation and update, winget-compatible manifests/submission; macOS .app/archive
and free command installation. Record any external registry acceptance separately
from manifest generation. A generated manifest is not a published package.

Use Windows Installer COM/database APIs and built-in cabinet tooling or another
verified fee-free path; do not depend on paid installer maintenance/certificates.
OS code signing/notarization is optional and outside the required free path.

Artifacts need deterministic names, source/version metadata and publisher-controlled
hash/manifest verification. Mirror routing must not weaken executable integrity.
Direct GitHub reachability, automatic fallback and explicit endpoint overrides
need reproducible success/failure tests. Updates must stage safely, preserve user
data and allow rollback when extraction/startup fails.

Icons are sourced from the referenced microsoft/vscode Code-OSS repository with
its exact resource revision/license retained. Keep installer, shortcut, executable
and macOS bundle icon metadata synchronized.

The local installation target is the user's per-user Programs directory, with a
gocode command and Start Menu/desktop access; verify real installed launch/update
settings. Never install test payloads over an unrelated application.
