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

The existing signing key is still needed for production `Validate`. Free
distribution tools are used throughout. A pending/prepared candidate or separate
unsigned native/tooling proof does not satisfy the complete-release verifier.

The first actual clean-source Prepare at
`249b057d60f66d715fba49ec863285e1254147cd` completes all five ordered source checks.
Packaging then exits1 after389ms before any app/package build: importing the
WorkDirectory helpers enables StrictMode Latest, and clean `git status` emits
no object whose `.Count` can be read. Its original package failed JSON/stderr,
five successful checks and incomplete marker remain in the immutable
`.cache/local-release/0.24.0-249b057d60f6-r1/` root; private roots were cleaned.
The packager now counts `@(& git status --porcelain)` and explicitly checks Git's
exit status. Real unchanged clean249 status reproduces the original null error
and validates the array count0 under Windows PowerShell5.1, without executing
packaging. The other builder Count/index sites use explicit arrays or typed
object[] arguments; no additional scalar/null change was needed. A new clean
source commit and fresh Prepare must rerun the exact five checks; the successful
249 receipts cannot be relabeled for the corrected packager source. The ignored
candidate249 MSI runner remains bound to249 and is not revised or executed here.

## MSI raw-command correction (2026-10-08)

Clean source f4a795d4a4cc09ce67f72a93364f15363c077d48 subsequently passes all
five Prepare source checks and produces genuine MSI/ZIP bytes. Its unsigned
original ZIP also passes all35 packaged native gates in111.793s. Those receipts
remain bound to f4a; they are not relabeled for the corrected publication helper.
The exact-source record is [local candidate f4a](local-candidate-f4a.md).

The first real prior-MSI install reaches the original120s deadline without a
verbose log. A same-owner, same-SW_HIDE missing-package negative control isolates
the parser: generic all-token quoting waits5026ms and exits2 after timeout;
MSI raw syntax returns1619 in83ms with a5806-byte real Installer log. The retained
result is `.cache/native-combined24/msi-quote-control-r1/execution/results.json`,
SHA256 ee49cbc883d2cc07a211489e32deff904681f6c595d4414dd9f9eafc564d8ef6.
A missing-package1619/log proves entry into the engine, not property-value
interpretation or an installation transaction.

`GocodeReleaseProcess.RunMSI` now fixes the executable to the original system
x64 msiexec and serializes only the explicit `/i` or `/x`, `/qn`, `/norestart`
and `/L*v` grammar. Switches remain bare; canonical absolute path operands are
quoted; unique ASCII public-property identifiers use `NAME="value"`, doubling
literal quotes. Empty values remain `NAME=""`. Null/NUL/CR/LF tokens, ambiguous
or relative paths, duplicate switches/properties, unsupported options and
oversized inputs reject before spawn. This follows Microsoft's
[MSI command-line syntax](https://learn.microsoft.com/en-us/windows/win32/msi/command-line-options),
[property identifiers](https://learn.microsoft.com/en-us/windows/win32/msi/restrictions-on-property-names)
and [empty property values](https://learn.microsoft.com/en-us/windows/win32/msi/setting-public-property-values-on-the-command-line).
The generic Quote function and ordinary Go/PowerShell argv serialization remain
unchanged. Only this MSI path sets STARTF_USESHOWWINDOW/SW_HIDE; suspended
Job-before-Resume ownership,64 arguments,4MiB per captured stream,120s MSI
deadline and the original3s tree/root and3s reader joins remain enforced.

The corrected two execution files pass a real f4a/prior23 lifecycle preflight:
10984ms, outer PID183364, root/tree closure. The validation directory actually
contains spaces and Unicode; MST/INSTALLDIR paths and logs are used by Installer.
Prior installation takes2421ms/exit0; corrupted upgrade236ms/1603 retains prior;
upgrade2625ms/0 installs candidate; rejected downgrade115ms/1603 retains candidate;
uninstall2145ms/0 removes the isolated product. Native installed launch, original
registrations, PATH, workspace and all four original package hashes pass; private
TMP/config are removed. The result is
`.cache/native-combined24/msi-parser-fixed-preflight-r2/execution/current.json`,
SHA256 906c8eaff6ee96da1ebdc1c6c433d57547095613820835909bce0a716df66e77.
Execution-source hashes are unchanged before/after: C#
33c01cd658c2db4b3f82396a1922c30bb31d17e7cb5c42a105ac39a223807471,
candidate MSI script bd3ca3fc72b422807e7765ab7f1c09d7494811b0971061e2fdd1aa4e3046bbc7,
functions424c3a1c0f9ce8764a5ae9091d156d566e1d95776f75178ecf7784dc5ca73706.
The original quoted failure and the separate r1 pre-execution harness failure
remain preserved. This preflight uses the uncommitted corrected harness with
original f4a packages; it is not a final-source complete release.

The test script adds pure bounded grammar controls and a separate root-only
`-MSIControls` opt-in. Pure controls include Unicode/space paths, literal quote
doubling, empty/equal-containing values, rejected private/malformed identifiers,
token/path/length/deadline rejection and the original generic argv convention.
The actual opt-in preserves missing-package process/log evidence and a fresh
minimal Type19 parser fixture. Its sequence1 error formats the real property
before any transaction/file/registry action, requiring exact embedded quote,
equals, Unicode and space values, intentional1603 and unchanged unregistered
product state. Root executes these actual controls in final-r7: all102 parser/
process/MSI controls pass. Missing1619 writes5790B/log SHA256
77efabb9428848fff126343eb2d9434b9fb31ec4df71a06918c6882573f51e3d;
the intentional Type19 abort1603 writes41468B/log SHA256
93faa7cf7f5f792fbe4f7f843c05186518ea9793c6fb9089d4f9822f80230f4a.
The actual formatted property is `Embedded "Quotes" = White Space 世界`;
product state stays-1 before/after and both owned process trees close. This
proves actual Installer property parsing, not an installed product transaction.
A fresh owner-validated `-MSIEvidenceDirectory` preserves original logs/results;
an existing nonempty directory rejects rather than overwriting older proof.

AST, native C# compilation and three pure90-control repeats pass. Root also
passes95 ordinary parser/process controls three times in7.765s; log SHA256
d34aa0500cc8289697218d962d433c8fe8ac5245d54abaabcf34f33703efa382.
Workflow
actionlint/ShellCheck and pinned PSScriptAnalyzer Warning/Error checks pass.
New-source whole regression, trusted signing, final complete-release verification,
tag/upload and user installation remain pending; no Actions are dispatched.

## Explicit older-host language compatibility

Public23's genuine production-signed native smoke fails with the newly installed
native Go/TypeScript packages enabled (535ms/exit1, document-sync/command failure)
and passes with only those exact IDs disabled in a separate owned copy
(497ms/exit0). Actual A/B receipt SHA256 is
0ac2f7b0cb4c533f1ca399937776c055906988e7b64830c0f96ae13791215012 under
`.cache/prior23-native-compat-2ff-r1/execution/current.json`. Original signed ZIP,
package inventories, profile and source2ff are preserved; both process trees
close and disposable directories are removed. No new-signed Activate/Rollback
is performed in this control.

The subsequent rollback producer uses the production rollback function with an
explicit extension root. It writes only the two native IDs to Disabled in its
disposable acceptance store, preserving other Disabled/Uninstall preferences.
Its required compatibility observation names this limited scope and binds three
retained files under rollback-compatibility/: original-state.json,
applied-state.json and final-state.json. It records complete original pinned
Go/TypeScript receipts from actual byte verification before and after the old
native run. The publisher preserves these three files before removing private
stage; strict verification requires their exact paths/hashes, actual narrow
Disabled state, unchanged preferences during the run and compiled upstream
archive/inventory identities. Missing/old compatibility evidence cannot complete
a release. Original program-byte, production-signature, ordered native/service,
MSI,60s old-smoke and cleanup guards are unchanged.

This owned compatibility state is deliberately distinct from user rollback:
production CLI rejects an older-host selection while verified native adapters
are enabled and asks the user to disable them. It never silently changes the
user's store. Actual package acceptance for the new producer remains Root-owned
and requires a fresh immutable source/Prepare; prior2ff proof does not cover
this later change. The production signing key is still missing, and there is no
new tag, Actions dispatch, upload, complete receipt or installed promotion.
