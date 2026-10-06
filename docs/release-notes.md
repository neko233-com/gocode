gocode v0.11.0 replaces the Windows titlebar's debug placeholder with the actual
upstream Code-OSS icon, rendered as an immutable native GPU image. Tabs use real
system-font measurements, so README.md and Chinese filenames retain their final
caption glyphs before the close button. Current VS Code stable 1.140.0 / main
4861e8b supplies the visual measurements; the MIT asset provenance is retained.

The public godesktop bitmap API provides premultiplied RGBA, aspect-preserving
clipping, static upload reuse, bounded 128-image / 64 MiB native CPU residency,
frame pinning and fence-owned textures across eviction/device recovery. A new
native UI gate checks actual completed caption/selection pixels, tab selection
and close, and unchanged real source files. Mac Intel/ARM additionally capture
owned Metal drawable pixels at normal, 1.5 and 2 density, without desktop capture.

Existing bounded asynchronous file opening/Explorer startup, acknowledged VSIX
document synchronization, editable-file reload/conflict handling, generic language
server recovery, real highlighted ConPTY/PTY terminals and read-only GiB browsing
remain available. Copilot follows the official SDK/Language Server route while
VSIX compatibility continues to expand. Free MSI/ZIP/Mac packages and maintained
CLI/custom winget/Scoop/Homebrew distribution use signed/hash-verified updates.

This remains an engineering preview. Full VS Code/GPUI parity, the full official
Copilot VSIX, tab overflow, recursive workspace watching, DAP/SCM, full search and
advanced editing remain unfinished. Exact-source CI, released-byte rollback and
installed-app evidence are recorded in agent docs/status.md as they pass.
