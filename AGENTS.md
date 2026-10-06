# gocode harness

Read `agent docs/README.md` and `agent docs/status.md`. Maintain these contracts
with every functional milestone. User instructions override repository guidance.

- Keep the workbench native Go/godesktop. No Electron/browser UI shell.
- godesktop is a public dependency: use GOWORK=off for independent validation.
- Perform file/network/LSP/update work in cancellable workers with bounded memory.
  UI callbacks only mutate UI state and publish immutable snapshots.
- Validate upstream services with real protocols and native UI, not fabricated
  authentication/completion success. Keep explicit compatibility gaps recorded.
- Large files require file-backed bounded-memory browsing and measured GB tests.
- Installer/update changes require owned install-root, integrity/rollback, PATH,
  shortcut/icon and launch checks. Preserve all user workspaces/settings.
- Use only free packaging/distribution tooling; paid signing/notarization is not
  required. Maintain manifests/scripts and source/license provenance.
- Pin latest VS Code main/stable references for visual comparisons. Native frames
  are captured only from the process-owned HWND/drawable.
- After source/CI/package changes, run relevant Go/race/native checks, actionlint
  and git diff --check. Publish immutable releases and advance the parent gitlink.
- Never commit tokens, credentials, private manifest-signing keys, installed
  runtimes, caches or generated installer/binary artifacts.
