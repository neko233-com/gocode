# Modern Windows UI and native VS Gallery protocol

Final native rounded UI is public and installed in v0.22.0/source
0b5966171e5e183e74a9fecbeccce391cd1cf229 on public core v0.15.0. All five source
jobs 37651719075 and tested-package publication 37654475129 pass. Actual final
Windows 2025 File menu and Windows 2022 JetBrains Search Everywhere CI captures
were inspected, together with installed 150% File menu/Settings GPU pixels.
The framework's descendant clipping fixes a real missing capability; it does
not establish full VS Code pixel/API parity. Earlier candidate checks below
are retained as history. Exact installed/released-byte evidence is in status.md.

The user's latest steering requests current VS Code rounding and the official
extension store. Actual MIT Code-OSS 1.141.0/2a59476c9bfcb90b3ddc372c36762471b7dfad1c
modernUI sources are inspected: roundedCorners, editorBorder, padding, tabs,
activityBar, commandCenter, titlebar and shadows CSS. Latest main is now verified
as 12701a51749f8b57aab5d567c66f7727c88c3eb4. Existing Microsoft MIT/license provenance
remains in assets/code-oss; these CSS rules are translated into native Go elements.

Windows controls/list rows use 4px radii and native hover fills; menus/Quick Input
use clipped 8px surfaces. Editor framing includes tabs/breadcrumbs/content inside
a 1px stroke with 8px outer/7px inner clipping, without a new outer margin.
The panel remains outside the editor card. Selected activity items use an inset
rounded fill. The framework needed real descendant clipping, implemented in the
public godesktop v0.15.0; a background radius alone cannot clip painted child content.
Workspace-linked native 45-phase acceptance passes a standalone run. Captures
now wait three completed GPU submissions after asynchronous acknowledgements,
so a saved model identity cannot be presented with an older Untitled screenshot.
Full final independent public-framework/repeated/native/visual evidence follows.

Core immutable 4d62ed73a5513d8cbc281e05fe9ea43a74fd61ea passes all five jobs in
CI 37642213032 and is published as v0.15.0. The application independently fetches
it with GOWORK=off/GOPROXY=direct/no replace, and go mod verify passes. Full
workspace-linked three shuffled strict-cgo/race repeats pass (247.071s, model
30.3% coverage). No-cgo and vet pass. Native saved Chinese tab/content and
extension detail PNGs are inspected after the three-submission capture fix.
Final full public-module script/source CI/released-byte installation follows.

The final independent public-module full Windows script passes three shuffled
race/strict-cgo repeats (242.704s), vet and all console/GUI native gates including
both new 45-phase runs, existing editor/groups/large pages/search/replace/terminal/
VSIX/Git/file watch. Private scratch cleanup succeeds. Final File menu and selected
JetBrains Settings GPU captures are visually inspected. Public no-cgo, workflow
lint, distribution policies and diff checks pass; exact source CI/release/install
are still required before promoting the local user installation.

## Native store protocol and actual service limit

Microsoft's official FAQ explicitly says alternative products cannot directly
access Visual Studio Marketplace:
https://code.visualstudio.com/docs/supporting/faq#can-i-use-the-visual-studio-marketplace-with-other-products
This is distinct from MIT Code-OSS source/API compatibility. No authorization
from Microsoft is present in this task. Official Marketplace access/compatibility
is not claimed. The default public catalog remains Open VSX with local VSIX import.

The native -extension-gallery-url HTTPS-base option implements the VS Gallery
query/download/install protocol for an authorized compatible/private service.
It can address the official service only when separately authorized. The native
UI names the selected service and new/replacement windows inherit its endpoint.
No Microsoft product/client identity is spoofed, no access restriction is bypassed,
and credentials/query tokens are rejected in endpoint URLs. Actual source contracts
come from platform/extensionManagement/common/extensionGalleryService.ts,
extensionGalleryManifestService.ts, extensionGalleryManifest.ts and extensionManagement.ts
at the exact stable source. Query uses the same Target/SearchText, bounded page,
flags and Accept version; native gocode User-Agent stays distinct.

Windows x64/universal version selection rejects Mac packages/prereleases. Responses
are limited to 1MiB/20 entries/128 versions, requests to the existing cancellable
single worker, downloads to 64MiB and manifests to the existing archive caps.
The selected immutable package URL is retained; extraction rejects mismatched
manifest identity/version. Gallery assets/redirects stay on the authorized host
or known Microsoft gallery CDNs for that official endpoint.

Three race/strict-cgo repeats (2.137s) use a real TLS server, real ZIP VSIX and real
Node host: protocol headers/body, x64 selection over universal/Mac, download,
identity validation, extraction, actual contributed command and cleanup pass.
Different selected version and foreign/credential URLs are rejected without
changing the installed fixture. This is actual protocol/process evidence for the
fixture, not proof of live official Marketplace access or arbitrary extension APIs.

The installed v0.22.0 official Copilot SDK/LSP health check reports authentication,
LSP initialization and SDK connection true, networkPromptSent=false. Microsoft's
FAQ was rechecked on 2026-10-08: alternative products, including Code-OSS forks,
are not permitted to access the official Marketplace. Separate service
authorization is absent here; the native authorized Gallery implementation and
default live Open VSX remain the implemented scope.
