# Owned language server process lifetime

2026-10-08 unpublished Windows candidate, independently consuming public
godesktop v0.17.0. This is process/protocol evidence; it does not establish a
native UI pass, a signed production package or complete extension API support.

The public framework's `lsp.Start` kills its direct process. A TypeScript server
can launch other processes that hold inherited stdout. The application therefore
uses public `lsp.Connect` with an independently owned transport: Windows creates
a suspended process, passes only the three explicit pipe handles, assigns its
KILL_ON_JOB_CLOSE Job before ResumeThread, then owns root wait and pipe shutdown.
The platform ownership implementation derives from the frozen nativeguard
milestone ed00509a56e113219ad1f51e7349c4728b459988aad035911abce65bf76fc2fc.
Windows environment names use the actual case-insensitive ordinal OS comparison
and last-value overrides, preserving private TMP/APPDATA isolation.

The service has no acceptance-style 120-second lifetime ceiling. Parent context
cancellation or Close kills the owned tree, closes stdin/stdout/stderr, and joins
the root wait, cancellation observer and stderr-drain worker. One shared Close
acknowledgement has a three-second join bound; repeated/concurrent Close does not
restart that wait. Stderr goes to a bounded-memory discard stream. Protocol
message/queue limits remain those of the public LSP transport. ProcessClosed
requires the complete acknowledgement and no close error, rather than merely
the root PID ending. Client.Close reports transport close errors as well.

Windows ProcessIDs queries only the exact owned Job, including nested child
jobs, with at most 256 IDs. An incomplete/oversized observation is an error;
the job handle is synchronized against Close. The API is worker-side and does
not search global processes by executable name. See Microsoft's
[Job process list](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-jobobject_basic_process_id_list)
and [query API](https://learn.microsoft.com/en-us/windows/win32/api/jobapi2/nf-jobapi2-queryinformationjobobject).
Darwin/Linux use an owned process group; a native Job-style complete PID query
is unavailable there and the observation API returns an explicit error. A
deliberately detached Unix descendant is outside this group ownership contract.

Real console-only controls start a root and a descendant holding stdout, plus
an unrelated sibling. Direct Close and parent cancellation kill/reap the root
and descendant; cancellation also rejects an unanswered RPC. Natural root exit
closes the remaining descendant without waiting indefinitely for inherited EOF.
Exact Windows Job snapshots include root/child and exclude the sibling; retained
OS handles prove the first two exited while the sibling remains alive. Sixteen
concurrent repeated Close calls succeed afterward. Already-cancelled/missing
lifetimes and an oversized command are rejected before creating a process.

Three strict-cgo/race/shuffle repeats pass in 6.188s in
`.cache/lsp-process-lifecycle/strict-race-with-tree.log`. The final matching
no-cgo repeats pass in 1.358s in `no-cgo-with-tree.log`; strict vet exits zero
in `vet-with-tree.log`. These are console process controls with no native window.
Earlier pre-observation
controls pass in 6.196s and no-cgo repeats in 1.460s; their logs remain separate.
The earlier Windows constant type compile failure was corrected before these
passes. Final combined source/package/native validation remains required.
Actual Go/TypeScript installed VSIX/server evidence is recorded separately in
[language extensions](language-extensions.md).
