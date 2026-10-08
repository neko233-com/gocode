# Recognized VSIX native language adapters

This is the unpublished consolidated Windows candidate using independent public
godesktop v0.17.0 with GOWORK=off. The application remains native Go/platform GPU;
CPU/process/model evidence and Root's separately bound native Go/TypeScript
executions are recorded below. These do not promote a published v0.24.0 or an
actual-user installation. Root owns the final combined suite, release and
installed-byte receipts.
No Actions were triggered for this work.

## Original packages and upstream provenance

The supported explicit install route preserves these original public packages:

| Package | Original bytes | SHA256 | Native server |
| --- | ---: | --- | --- |
| golang.Go 0.50.0 | 607925 | 7b43b91174adc47c3bb0aa331faab32d2333cb4d12fba43ba7d2ad3fe35b07a6 | gopls v0.23.0 |
| vscode.typescript-language-features 1.95.3 | 11510122 | b1494642a46d1b139ab799e2ee0ae2453bdcef6ed1a718acca034445cfd9d87f | original bundled tsserver 5.6.3 plus typescript-language-server 6.0.1 |

Primary immutable download origins are
[Go 0.50.0](https://open-vsx.org/api/golang/Go/0.50.0/file/golang.Go-0.50.0.vsix)
and [TypeScript 1.95.3](https://open-vsx.org/api/vscode/typescript-language-features/1.95.3/file/vscode.typescript-language-features-1.95.3.vsix).
Both actual VSIX license files are MIT; bundled TypeScript retains its own
license in the original archive. Source provenance is
[golang/vscode-go v0.50.0](https://github.com/golang/vscode-go/tree/v0.50.0) and
the package's recorded eclipse-theia/vscode-builtin-extensions repository,
derived from [VS Code 1.95.3 TypeScript sources](https://github.com/microsoft/vscode/tree/1.95.3/extensions/typescript-language-features).
The wrapper is Apache-2.0
[typescript-language-server v6.0.1](https://github.com/typescript-language-server/typescript-language-server/releases/tag/v6.0.1),
tag revision cf9c47b. Its primary package metadata requires Node >=22.22.2 and
provides lib/cli.mjs. The embedded npm lock uses the real registry tarball
integrity, npm ci --ignore-scripts and no global package mutation. Actual local
Node v24.14.0 satisfies this requirement.

The actual Go 0.50.0 manifest is 237502 bytes, with 35 original extension files
and 2939168 expanded bytes. Its complete inventory SHA256 is
295f83601de947c1e057cb1de787e361e769f2185f59f22f3fba89dbb82852c9.
TypeScript has a 66809-byte manifest, 235 original extension files and 55988842
expanded bytes, inventory SHA256
89806dd3685da5012779fec123fa12310f3613077ebba5355de6625435254cb6.
The existing public store guards remain 256 KiB manifest, 4096 entries, 16 MiB
per file and 64 MiB expanded bytes. Original golang.Go 0.56.1 manifest271556 and
0.54.0 manifest265592 exceed the cap; their downloaded bytes and metadata are
retained in .cache/language-extension-audit. No archive was compacted/repacked,
no public core tag/bound changed, and selecting a catalog latest version is not
silently downgraded. The pin command displays its actual version explicitly.

## Compatibility and lifetime

Recognized packages are excluded from the partial Node host and executable
manifest-command inventory. They activate native LSP adapters after verification
of the original complete file inventory and runtime hashes. Original JS
activate(), Go debugger/testing, TS-specific JS APIs and arbitrary
vscode-languageclient/official Copilot VSIX compatibility remain incomplete.
The supported standard provider features are completion, type diagnostics,
formatting, hover and definition. Official unauthenticated Microsoft Marketplace
fork access is not authorized; this installer calls pinned public Open VSX only.

UI hooks are composed once even with zero initial servers. Reconciliation owns
binding identity on the UI thread, retains an unchanged config, clears removed
sessions/diagnostics/completions before cancellation and rejects old lifecycle
events/requests by membership and generation. FS/download/npm/go work stays on
workers. Up to16 total configured/native servers retain the existing per-session
document32/request8/event16 queues, UTF-16 coordinates and 2 MiB source ceiling.
Explicit user lsp.json remains independent and takes request precedence;
implicit Go fallback cannot revive disabled/uninstalled adapter-owned support.
TypeScript uses typescript/react and javascript/react language IDs, the original
installed tsserver path, disabled automatic typing acquisition and its memory
option512 MiB; this does not claim a total IDE/server RSS cap or GiB semantics.

Disable/uninstall closes the corresponding server after its real settings
acknowledgement. Package deletion is deferred; explicit reinstall cancels only
its pending uninstall and preserves Disabled. A canceled old manager receipt
cannot change a new actor's busy/settings state. Dependencies stay in fixed
per-user version directories for verified reuse. Install commands are finite
120s owned process invocations, with shared stdout/stderr512 KiB ceiling. The
actual service has no120s lifetime ceiling and uses the separate owned transport.
Windows Job creation/assignment happens while suspended, before resume; Unix
uses an owned group. See [language processes](language-processes.md) for Root's
transport and actual descendant/sibling evidence. No public17 API was changed.

## Real local validation and retained failures

Go's first real go install + original VSIX + server check passed32.05s.
TypeScript's first immutable package/npm installation reached actual completion
and definition but failed a literal Windows URI comparison: the server lowercases
the drive and encodes the colon. The corrected guard requires the actual same
file identity and expected symbol line. An intermediate diagnostic consumer
still used literal URI equality and timed out60.51s; it was corrected by the same
actual file-identity check. One non-escalated process invocation returned EOF;
its cause is unclassified, and its log remains separate. Elevated real Windows
repeats pass. An initial model-test nil diagnostics map was a fixture setup
failure, corrected before current repeats. Transient duplicate Client.Close and
not-yet-defined native TS struct fields were coordinated integration compilation
failures, not native passes. All original logs are retained.

Final pre-explicit-reinstall strict-cgo2/race/shuffle/count3 on public17 passes:
Go main5.190s, package4.016s, language-ID mapping1.057s; TypeScript main29.354s,
package16.587s. Logs are final-go-strict-race.log and
final-typescript-strict-race.log in .cache/language-extension-audit. Actual
results include greeting completion, definition to the real owned source,
format14/18 edits applied through editor.Apply, and actual Go cannot-use / TS
not-assignable type diagnostics. Seven ordered process-only gates pass.
Windows TypeScript reports actual three-PID Job snapshots (root, tsserver and
its worker); retained OS handles are required dead after check/disable/uninstall.
Go observes its root handle; transient go-list descendants are covered by the
owned transport contract and Root's independent descendant tests rather than a
fabricated complete Go process inventory.

The final explicit-reinstall edge changes only language_extensions.go,
extension_manager.go and the focused test. Its stale-actor strict/race/count3
passes1.179s. The normal Windows sandbox denied replacement of the same private
settings file; this negative receipt remains pending-uninstall-stale-ack-
diagnostic.log, while the actual Windows/private-root elevated positive is
pending-uninstall-stale-ack-strict.log. Full real Go/TS edge strict/race/shuffle
count3 repeats subsequently pass3.559s and27.697s respectively, including actual
persisted settings, preserved Disabled, deferred removal and original-package
reinstall. Their logs are final-go-explicit-reinstall.log and
final-typescript-explicit-reinstall.log. Current fixed source hashes are recorded
in the cache receipt; the older5.190/29.354s results remain pre-edge evidence.

No-cgo focused count3 passes main2.583s/package0.514s/mapping0.427s, with optional
real-package tests explicitly skipped unless their owned cache is supplied.
After the explicit-reinstall edge, no-cgo count3 passes
main0.349s/package0.041s/mapping0.032s, and both focused vet modes exit0 again.
Pure no-cgo tests do not supply native evidence.
The genuine archive/runtime tests explicitly forbid network on verified reinstall
and require the original receipt bytes unchanged; altered manifest and omitted
file inventory are rejected. Actual model tests disable/re-enable/uninstall,
require process closure, remove the actual VSIX directory and reinstall it.

Root's native gates reuse the fixed ignored prepare/main.go helper.
It creates native-profile/{APPDATA,extensions,TMP,screenshots}, copies verified
tool bytes and installs the original cached VSIX without network or native
execution. native-roots.json deliberately leaves appSource/appBinary/hash null
until Root binds its actual frozen executable. It is a preparation receipt,
not a native or released-byte acceptance marker.

All observed check/disable/uninstall server handles have exited. The fixed owned
test TMP/TEMP root is independently verified empty after bounded named inactive
compile-cache cleanup (no live path references/reparse entries). The initial
cleanup receipt writer used a nonexistent double-gocode path after the delete
loop completed; temp-cleanup.json records the corrected independent final read.
Reusable tool/package roots, all original failures and the separate native-profile
preparation receipt are preserved; no user temp or application configuration was
cleaned.

## Whole-module CPU boundary after integration

With public17 and GOWORK=off, the actual CGO_ENABLED=0 whole-module command
`go test -v -p=1 -shuffle=on -count=3 -timeout=12m ./...` passes, main179.626s;
the subsequent whole-module `go vet ./...` also exits0 in session47276. This
includes real owned console-process/transport tests, not native windows or GPU
execution. Native font-width fallback under CGO0 is not a native measurement.
The full204013-byte log is .cache/language-extension-final-nocgo/full.log,
SHA256 a57b94afb08718972d6f6cb8e847cf8376c3ab4fa94c2e7c1aa6a976c1f55150.
Vet produced no output; the empty Tee did not create a vet.log file.

The source manifest was observed during the early run after Root's initial
freeze, then again after test/vet completion, before the later native LSP
ProcessClosed guard. It records264 actual repository compile/embed/module/version
inputs plus15 generated Go test-cache inputs and348 repository boundary files.
Preserved source-before.json SHA256 is
c47a9883cf2c7e01069ebd81cb0fbf711a15bdd01d51ca9fcf57a27d7f1db1cb;
source-after.json is
9475308c224d5efd465b8fe7e6950e17d6671dc14e53ae7461fe8578c0a60137.
There were no added/removed repository paths. Four local-release PowerShell
scripts changed outside Go's compile inputs. The ordinary selected
popup_shadow_test.go also changed4723→8170 bytes during this run: its owner
added the r4 source-derived InFlight scheduling regression around12:01 and
separately passed focused strict-cgo/race/count3 in1.190s. The179.626s whole-module
result belongs to the earlier inputs and does not prove this added test or the
subsequent native LSP ProcessClosed guard. Final frozen native/local-release
validation must carry its own exact source receipt.

The owned TMP is independently empty; the private APPDATA contains seven Go
tool telemetry files and no user application settings. The network-free native
profile is unchanged, native-roots.json SHA256
8ca5b18ffd2674b6c338e26a56d1104fffe2683aeefa14b4584fdce49c2cdeda.
The prior adapter receipt is preserved byte-for-byte as
.cache/language-extension-audit/current-before-full-nocgo.json.snapshot;
current.json links the new boundary/result while retaining all earlier real
protocol passes and failures. It still records nativeExecuted=false and does not
promote the prepared profile to a native, release or actual-user installation
receipt.

## Typed TypeScript native failure and exact-sequence diagnosis

Root's first actual native source209 manifest is
74b0e797c22f046f44ed055fd9cfe6cb6eb242eee73e40206aea67d50bbe2d10,
preserved with its executable/logs in .cache/native-combined24/language-native-r1.
The genuine Go native fixture passes5.591s, all13 phases and final GPU checks,
with the old two-PID Job closed=true/error=nil. TypeScript actually passes
formatting, definition, function hover, greeting completion and the initial
missing-name diagnostic. It then remains in phase6 after deleting the missing
line and exits2 at90372ms under the original90s fixture deadline; the outer120s
owned process guard does not fire. Phase7 is never reached, so this failed TS run
has no acceptance screenshot. Its root/tree are reaped and the original source,
scratch hashes and cleanup remain preserved, not rebound to the correction.

The independent ignored typed-sequence process probe uses the exact original
source and tsconfig, actual verified VSIX/TS5.6.3/TLS6.0.1 and production owned
transport. In5240ms it observes five actual versions: original didOpen, three
real formatting edits, greeting→greet, the returned greeting InsertReplaceEdit,
then deletion of the missing source line. Real function hover and definition
also match. Original publishDiagnostics payload fields show2304/missing present
through version4; after deletion, only code6133/severity4 remains:
"'value' is declared but its value is never read." The server publishes a
replacement list, so this is not evidence of a missing empty-clear event or
consumer version rejection. All five payloads omit the optional LSP version
field; local receive-time versions are explicitly distinguished from a server
version. JSON indentation is normalized, not claimed as byte-exact RPC framing.

Actual probe receipt is
.cache/language-extension-audit/typed-sequence/result/current.json,
13350B/SHA256 f629b2ab4cc8deec1bc5d4b6b5968066359b26e9cc7c190d4de67aa0054dbd78;
diagnosis.json is6713B/SHA256
97645d276fd5a48ad6bc83277e90190bb4ff05cb1d17a75afadf1f974a9deaa9.
It runs no GUI/GPU and preserves all real diagnostics, including the Hint.
Observed Job PIDs174008/228136/188904 are dead through retained-handle checks;
Close completes and its private TMP is empty. The earlier seven-gate check is
still separate evidence and does not stand in for this typed editing sequence.

## Root's r2 native Go and TypeScript proof

Root changes only the TS fixture to export its value and restartValue. This
removes genuine unused-local Hints while retaining the original requirement
that the diagnostic list become empty, all13 phases, real completion/formatting/
definition/hover, crash/reinitialize, unsaved replay, final GPU pixel guards and
the90s fixture/120s owned process limits. Product diagnostics are not filtered.

Both actual native invocations pass against source209 manifest
f7dbbac27e3b201ed63b80ed40b366ee9f251c96969d5033ff1c4c8005834b26:
Go5125ms and TypeScript5528ms. The original Go Job216600/231192 and TS
Job141744/188244/231988 close=true/error=nil; outer roots231184/224660 are
also reaped. Root actually inspects the GPU screenshots. The fixed receipt is
.cache/native-combined24/language-native-r2/receipt.json; its separate
cleanup-and-validation.json verifies all recorded PIDs absent, private TMP/local
cache absent and the prepared profile unchanged, preserving gopls scratch before
cleanup. Source209 archive and original r1 failure remain available.
Receipt SHA256 is89b7cf9004c0d388166302cb7445364627fe6094dc9b40b712f9ce9dfeaac4d2;
cleanup SHA256 is18c0e5c3e08a15d6346bfd16631da86223d551b8bdbfe88cc6011a56688f86d5.

This r2 proof precedes the separate view.go correction from PlainText to the
protocol-consistent TypeScript status name with actual measured width. Its
receipt is not rebound to that newer source or to a final combined suite,
release package or actual-user installation.

## Root's r3 actual native status-label verification

The new source209 manifest is
18140ca8a6417181ecbf766bf9a8078b9eeb08c10f64aa965769e4f373058d94;
binary SHA256 is
defa38833f8811ce7a9d51492b64f1b5aebd63f89f336307164e900bce0e0589.
Both genuine native invocations again pass: Go5114ms and TypeScript5576ms,
with all original13 phases and final GPU guards. Root actually views the
1920x1230/DPI144 screenshots and confirms the TypeScript status label and
uncut text. Old Go Job231792/228932 and TS Job221036/233728/234196 close=true/
error=nil. App roots192200/225536 exit0; the outer120s owned reports require
rootReaped/treeClosed. No fixture/pixel/deadline guard is loosened.

The fixed .cache/native-combined24/language-native-r3/receipt.json is explicitly
before cleanup: it reports four private TMP entries, zero Go compilation TMP
entries and47 local-cache files. These upstream gopls scratch/cache entries
are not a compilation failure or proof of empty scratch. Root retains their
bounded inventories before cleanup and will provide a separate cleanup receipt;
no future cleanup hash is inferred here. The prior r1 failure, r2 proof and
whole-no-cgo source-change boundary remain separate. Root's later actual GiB,
combined-suite, package and installed-byte checks still need their own results.
The before-cleanup receipt SHA256 is
6a560793428c6aa4f84770691cb37cbac802516755dfb7ea89a79f8ddf5d9591.

The current independent CheckFramework consumer also passes0.808s against
public17 with no replacement, actual matching module sums/ZIP/Origin;
.cache/native-combined24/framework-current-consumer/framework.json. Root
preserves the original51-byte defaultcache.info and replaces it atomically with
the actual199-byte official-download metadata. This is framework-consumer
evidence, not an additional language/native/released-byte execution.

## Actual CLI reinstall completion

The final read-only workflow review found that the CLI installation branch
returned a genuine successful package receipt without finishing deferred
uninstall settings, although the command-palette manager already did so. If a
user uninstalled without Reload Window and then explicitly reinstalled through
the CLI, next startup could remove the successfully reinstalled package. The
CLI now reads current settings after successful installation and calls the same
verified finish helper before printing its success receipt. It clears only that
package's pending uninstall and preserves Disabled and other package settings.

The new opt-in TestActualLanguageExtensionCLIReinstall builds the actual CGO0
application and executes both install flags with original cached Go/TypeScript
VSIX/tool bytes under private APPDATA/extension roots. Real CLI receipts must
report the pinned identity/hash and verified reuse; owned processes must be
reaped. Actual startup removal preserves the reinstalled package, repeated
installation retains the exact settings bytes, and the disabled adapter remains
disabled. A denied HTTP/HTTPS proxy ensures the verified reuse path cannot pass
by downloading dependencies again. Ordinary test runs without explicit original
package cache skip this live-package control.

Focused strict-cgo2/race/shuffle/count3 passes19.137s; no-cgo/shuffle/count3
passes5.987s. Each mode covers three actual Go and three actual TypeScript
control rounds, with two successful CLI reuse invocations per round. Both
package vet modes exit0. Logs and exact main/test hashes are recorded separately
in .cache/language-extension-cli-reinstall. No GUI/GPU/native window, network
installation or user profile mutation occurs. This later CLI branch correction
is not silently folded into the preceding source209 native r3 receipt; final
clean-source/released-byte checks remain Root-owned.

## Runtime fallback provenance and explicit provider precedence

Final workflow review also finds two same-window cases that startup-only checks
cannot establish. Re-enabling/installing a native adapter appended it after
independent user providers, reversing the startup request precedence. A window
started without Go VSIX/user config could also retain its automatically selected
gopls after first installing Go support, even after disabling the new adapter.
An explicit user provider named gopls must remain independent, so names cannot
identify this automatic process.

The child Config now carries AutomaticGoFallback as internal, non-JSON provenance
set only by LoadConfig's actual managed/PATH automatic branch. Explicit user JSON
does not acquire it. Startup and the extension worker share the same installed-Go
or persistent-go-marker check, including Disabled, unsupported-version and
pending-Uninstall cases with no active native Go config. The admitted UI receipt
stores this suppression monotonically for the actor. Reconciliation clears and
cancels only automatically sourced fallback bindings; explicit same-name gopls
bindings and other user providers retain their sessions. TypeScript-only
installation leaves automatic Go support intact.

Retained native bindings and new native bindings precede independent bindings;
existing independent order/session identities are preserved. The existing
reverse request scan therefore keeps the same explicit-user precedence through
install, disable and re-enable. The16-server cap is checked before cancellation
or published binding mutation. Existing membership/generation receipt guards and
once-composed document/request hooks remain in place. Process equality compares
native inputs; independent provenance stays on its unchanged binding until a new
window, rather than being inferred from the JSON equality helper.

The owned synthetic stdio selection control uses the actual automatic loader,
real owned provider processes and real hover requests/results to distinguish
which provider was selected. It verifies TypeScript-only fallback retention,
persistent-marker/disabled-Go first-install removal, recognized unsupported-Go
suppression, enable/disable/uninstall without resurrection, and explicit same-name
gopls plus last-user priority with unchanged session/order. It supplies protocol
routing evidence, not original VSIX/gopls compatibility or native-window proof.
Focused strict-cgo2/race/shuffle/count3 passes2.630s; no-cgo count3 passes1.020s;
affected app/languageserver vet exits0 in both modes. Exact files/logs are recorded
in .cache/language-priority-provenance/current.json. These later routing edits
remain outside the preceding source209 native receipt; Root's final clean-source
production regression and released-byte acceptance are still required.

Root subsequently preserves all209 tested source files and the actual upstream
Go overlay files byte-exact, inventories the47 local-cache files, verifies every
recorded old-server/app PID is absent, and removes only the ordinary private
TMP/local-cache trees after owned process retirement. The actual post-run
cleanup-and-validation.json SHA256 is
523f78e848ddc987eeecdffd9845c5d557f189333a9ea0eecf8de400ae3040e7;
TMP/local-cache are absent and compiler TMP is empty. This additive receipt
does not alter the before-cleanup receipt or claim a signed released package.
