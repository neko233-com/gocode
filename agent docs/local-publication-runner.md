# Local publication runner

The consolidated Windows candidate uses local source checks and packaging.
The runner does not dispatch Actions. The published/installed v0.23.0 and
immutable core v0.17.0 remain separate from this unpublished candidate.

`scripts/publish-local-release.ps1` has four explicit modes. `Plan` reads the
checker-generated ordered gate/check contract. `Prepare` requires the exact
clean committed source, independently verifies the actual public framework,
runs the five source checks and builds the genuine Windows ZIP/MSI and channels.
It needs no signing key and performs no staged native acceptance or installer
transaction. Its `prepared-candidate.json` deliberately has `complete=false`.
Prepare, Validate and Publish reject an explicit version that differs from the
repository VERSION before creating private directories or starting a process.

`Validate` can consume a fresh root or that exact preparation. Continuation
rechecks current clean HEAD, tracked-byte InputDigest and actual module Origin,
checksums and cache files. It verifies all five checks' identities, order,
commands, PID, successful exit, process/tree closure and actual captured logs;
it also rehashes the original checker, ordered MSI/ZIP and complete channel
directory. A changed source, rewritten receipt, changed log/package/tool,
missing or extra channel, unsuccessful previous attempt or retained private
directory rejects continuation. Original preparation queries/logs are preserved;
fresh queries use `resume-` filenames. Previously prepared source checks and
packages are reused only after these checks succeed.

Validation uses the existing production manifest key outside both checkouts;
it never generates or rotates a key. It signs and stages the original prepared
ZIP, installs the pinned Go/TypeScript extensions into the private extension
root, executes the generated native plan, tests the actual candidate MSI with
an isolated MST instance and runs a real older signed release for rollback.
The final Go verifier remains authoritative. `receipt.json` is published locally
only after all gates, original package-byte checks and private directory cleanup
succeed; a pending or prepared marker is not complete release evidence.

`Publish` is an independent explicit invocation. It verifies the complete
receipt and immutable local/remote tag source, creates or resumes only a
matching draft, uploads missing allowlisted assets, downloads every asset and
compares actual bytes/SHA256 before publishing the draft. A different tag source,
already published release or unknown draft asset rejects the operation. This
milestone has not executed signing, tag creation, upload or publication.

The Windows long process owner creates a suspended process, assigns a private
KILL_ON_CLOSE Job with an explicit inherited handle list, then resumes it.
It bounds each raw log stream to 4MiB and fails on overflow, joins readers and
reaps the root/owned descendants before acknowledging closure. Root approved a
20-minute outer source-suite ceiling; original package/test and native fixture
deadlines remain unchanged. Environment names use Windows ordinal case-insensitive
comparison with last-value overrides. Private TMP/config paths, native readback
and input isolation are explicit. A private Go telemetry `off YYYY-MM-DD` mode
is created before the first Go process and hash-checked before owned cleanup;
the user's global telemetry setting is untouched. Explicit builder WorkDirectory
parameters accept only new/empty, unreparsed directories below the repository
cache; the publisher places all intermediate payload/MSI work under private TMP.

The MSI fixture consumes original final MSI/ZIP and genuine prior MSI/ZIP bytes.
Its diagnostic MST changes only isolated product/upgrade/component identities,
registry/search roots and shortcut/install names. It applies the generated MST
to another original MSI copy and reads back the real property/component/registry/
directory/shortcut/upgrade tables and unchanged File metadata. Actual transactions
must prove corrupted-upgrade rollback, replacement of the prior registration,
candidate-byte installation, rejected downgrade preservation, installed native
acceptance and uninstall of unique shortcuts/products. Cleanup attempts each
owned product independently and checks original registrations, packages and PATH.
No synthetic package result is accepted as proof for final release bytes.

CPU validation records are under `.cache/local-release-ps-validation/`.
Windows PowerShell 5.1 parser/process controls pass three repeats with 73 controls
each in 7.813s total; the final repository VERSION equality correction adds
three pure positive/mismatch/malformed controls and an actual publisher process
that rejects mismatch before directory creation/build. All 77 controls pass
three final Windows PowerShell 5.1 repeats in 9.225s total.
The final staged whitespace check identified one extra EOF LF in the previously
untracked helper. Original bytes are preserved as `pre-eof-functions-r4.ps1`;
only that one byte was removed. All 77 controls pass another three repeats in
9.333s; AST and PSScriptAnalyzer Warning/Error checks remain clear. The r5 freeze
map supersedes r4 while preserving its receipt and original source snapshot.
They include actual console argv/Unicode/environment/EOF,
timeout kill/reap, a bounded output flood and a descendant process; prepared-file
controls use clearly labeled placeholders and do not claim installer/native proof.
Actionlint/ShellCheck and PowerShell AST checks pass. PSScriptAnalyzer 1.25.0
reports zero Warning/Error findings and 75 Information findings for positional
parameters in MSI/COM helper calls; those informational findings are retained.
Read-only `git diff --check` passes.

Historical 0.5.0-to-0.5.1 file-only MSI preparation succeeds with real
GenerateTransform/ApplyTransform/table readback. Four original package hashes,
original registrations and per-user PATH remain unchanged; fresh isolated product
GUIDs remain unregistered. Prior failed COM/parser preparations are preserved.
This is a file-only tooling proof, not a final v0.24.0 installer transaction.
The final-source freeze map and validation record bind the exact script bytes.

Root still needs to execute the final clean-source `Prepare`, then provide the
existing key and genuine prior artifacts for `Validate`. Final source-suite,
production ZIP/MSI, signed envelope, all packaged native gates, real installer
transactions, rollback and remote publication remain pending until their actual
source/byte-bound receipts exist. Free distribution tools are used throughout.
