gocode v0.24.0 gives extension details a native editor with a fixed metadata/actions
header, 36-DIP navigation bar and independently clipped, measured, virtualized
content. It continues to use public Go/godesktop v0.16.0.

Details and Feature Contributions keep their header/navigation visible while the
body scrolls. Native proportional-font measurement controls Unicode wrapping and
narrow-window reflow. Content is bounded to 64 KiB of description, 512 wrapped
description rows, 4096 commands and 1024 materialized visible rows with bounded
overscan. Immediate tab-switch followed by End now prepares the actual viewport
extent before handling the key. Pointer/wheel, page/home/end navigation and the
existing VS Code Ctrl+W/JetBrains Ctrl+F4 close paths operate on the detail editor
without editing the underlying document.

Real native acceptance installs a private generated VSIX, registers 2000 Node
command callbacks and navigates to and executes its final details.command.1999
callback. Enable/disable operations use the existing cancellable worker and real
private settings. Repeated acceptance reuses exactly four fixed report/PNG names,
leaves private scratch empty and preserves pre-existing settings.

Catalog searches accept both publisher.extension and @id:publisher.extension.
Open VSX named metadata checks Windows x64/latest, falls back to universal and
rereads the selected immutable version with identity/platform bounds. Actual
read-only queries golang.go and @id:GoLang.Go resolve golang.Go 0.56.1/universal;
ordinary search and installed/enabled/disabled filters retain their behavior.
These metadata checks did not install the live extension. Authorized/private
Gallery services use the upstream exact-name query criterion while retaining
native gocode identity, HTTPS/host/size/version/archive guards and cancellation.

Quick Input now applies native rounded descendant clipping to its input, result
rows and popup. Save All is disabled while a save or file action is busy.

Publication requires independent public-module strict-cgo/race repeated tests,
the complete native Windows suite, successful exact-source CI and signed
released-byte/installed validation. The release checker runs 27 ordered native
gates, including separate console/GUI detail reports bound to actual completed
PID/version/source; the final ordered marker also binds both payload hashes.
Existing process deadlines, complete archive integrity, genuine prior-version
rollback and preservation of user
settings, PATH and shortcuts remain requirements. Detailed capabilities,
limitations and exact validation evidence are in the
[milestone ledger](../agent%20docs/status.md).

Actual per-extension VSIX icons, README/CHANGELOG rendering, overlay shadows,
complete VS Code UI/API/debug/tasks and official Copilot VSIX parity remain open.
The accepted Copilot integration uses the official SDK/Language Server path.
Default live catalog access is Open VSX; Microsoft's official Marketplace still
requires separate authorization, which is absent here.
