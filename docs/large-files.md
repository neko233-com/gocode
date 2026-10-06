# File-backed large-file browsing

Source files up to 8 MiB use the framework's versioned editor. Larger regular
UTF-8 files use a read-only file handle with a background scanner, a sparse index
and one asynchronous native page. This does not implement large-file editing.

- Scanner block: 128 KiB plus UTF-8 carry; UTF-8 and NUL validation is incremental.
- Index: at most 32,768 checkpoints / 786,432 bytes. In-place coarsening increases
  spacing when it fills, instead of retaining an entry for every line.
- Page: at most 128 rows with at most 4,096 preview bytes per row. A line longer
  than a read block ends the page immediately, avoiding a full huge-line scan
  before the first display. Indexed line navigation can jump past that line.
- Byte view: at most 64 KiB, direct random access, valid UTF-8 boundaries and
  line/byte-column coordinates when that position has been indexed.
- A page read has a cancellation/generation guard. File identity/size/mtime are
  checked before and after reads; an external change requires reopening.
- Closing cancels scan/page/progress workers and closes the file handle. No page
  is editable or saved, and no huge full-text snapshot is sent to extensions/AI.

Use Ctrl/Cmd+G with a 1-based line, or `:0` / `:1073741824` for a 0-based byte.
The scrollbar uses indexed lines after completion, or bytes during indexing.
Arrow/Page keys, Home/End, wheel and scrollbar drag are supported. Byte windows
wrap bounded text; Ctrl/Cmd+C copies the bounded current page. Before completion,
unindexed line navigation shows progress and retries when the index advances.

Windows amd64 local strict-cgo/race acceptance on 2026-10-06 wrote **1,073,758,208
actual UTF-8 bytes**, verified first/middle/tail, then replaced newlines to create
one actual GiB line and verified its tail:

| Measurement | Result |
| --- | --- |
| First 8-row page | 82 ms |
| Complete index, race enabled | 8.26 s |
| Fixed index allocation | 786,432 bytes |
| Cumulative index + tested read allocations | 1,911,232 bytes |
| 100 direct single-line tail reads | 23.6 ms |

These timings are machine-specific; the allocation measurement excludes fixture
creation, OS file cache, GPU state and unrelated application services. Real native
GiB browsing also passed with a GPU readback from the process-owned window. CI
runs measured GiB and native GiB acceptance on Windows 2025, plus smaller native
long-line fixtures on Windows 2022/2025 and macOS Intel/ARM.

Commands:

```powershell
$env:GOCODE_LARGEFILE_GIB='1'
go test -race -run '^TestGiBBrowsing$' -v ./internal/largefile
go run -race . -largefile-smoke -largefile-smoke-mib 1024
go run . -goto-byte 1073741824 C:\data\huge.log
```

Whole-file search, encoding conversion and large-file editing remain separate
work. The editable framework Buffer still uses per-line slices; the file-backed
viewer does not pretend that model scales to arbitrary GiB edits.
