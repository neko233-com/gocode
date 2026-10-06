gocode v0.12.0 adds native tab overflow: clipped, visible entries preserve measured
Latin/CJK captions, while positioned wheel input and a draggable 3-pixel scrollbar
pan without scrolling the editor. Activation and window-size changes minimally
reveal the selected tab. Current VS Code stable 1.140.0 / main 1d25d5d supplies the
source reference; titlebar image and MIT asset provenance are retained.

Tab/close targets now follow document instances rather than indexes or paths, so
captured input cannot close a replacement reopened at the same position. Held
Ctrl+Tab/Shift+Tab uses a frozen recent-document order until Control release;
Ctrl/Cmd+PageUp/Down navigates tab order and Ctrl/Cmd+W retains dirty-close guards.
Terminal word deletion and wheel routing remain scoped to their native panes.

Public godesktop v0.7.0 adds native clipped Viewport geometry, passive keyed bounds,
positioned wheel deltas and key-release events. A 40-real-file native gate checks
GPU selection/scrollbar/clipping, wheel/drag, captured reopen identity, ordered/MRU
keys and unchanged source bytes. Windows console/GUI and Mac Intel/ARM normal,
1.5 and 2 drawable-density gates use only the process-owned HWND/Metal output.

Existing bounded asynchronous file opening/Explorer startup, acknowledged VSIX
document synchronization, editable-file reload/conflict handling, generic language
server recovery, real highlighted ConPTY/PTY terminals and read-only GiB browsing
remain available. Copilot follows the official SDK/Language Server route while
VSIX compatibility continues to expand. Free MSI/ZIP/Mac packages and maintained
CLI/custom winget/Scoop/Homebrew distribution use signed/hash-verified updates.

This remains an engineering preview. Full VS Code/GPUI parity, the full official
Copilot VSIX, pinned/preview tabs, tab reorder/split groups, recursive workspace
watching, DAP/SCM, full search and advanced editing remain unfinished. Exact-source
CI, released-byte rollback and
installed-app evidence are recorded in agent docs/status.md as they pass.
