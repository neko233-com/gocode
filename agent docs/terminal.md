# Native terminal contract

Theme reference (2026-10-07): VS Code main bf519293a7de013c7e3897185784065b0799c9f2,
extensions/theme-defaults/themes/dark_modern.json and terminalColorRegistry.ts.
Dark Modern terminal foreground is #CCCCCC; unset terminal.background inherits
panel #181818. Native terminal default/cell backgrounds now follow that panel
color. Copied application icon provenance retains its earlier immutable source.

Terminal process I/O, VT parsing and shell preparation run in owned workers.
Only immutable cell snapshots and native view/input state cross the UI thread.
No Electron, xterm.js or browser surface is involved.

Public v0.18.0 cross-platform owned drawable color/emoji PNG/JSON gates and the
previous Mac no-op capture gap are documented in color-glyphs.md. Mac terminal
window-width resize and full native color fonts on Windows remain unverified.
Both Mac architectures pass normal/1.5/2 terminal density; actual ARM 200% PTY
output shows native emoji details and ANSI/shell highlighting. Release and
installed source/hash/ConPTY/settings/rollback evidence is in status.md.

## Processes and shells

Windows uses official Microsoft.Windows.Console.ConPTY 1.25.260930003 from
Windows Terminal v1.25.2733.0. The system Windows 2022 conhost strips DEC 1049
switches (microsoft/terminal issue 381); bounded owned-fixture IO proved no
alternate-screen notification reached the parser. It was not an input stall.
The modern redistributable gives both Windows runners the same supported VT
behavior. NUPKG SHA256 is 6d6f8b008c655d814c498d8205d4012dcd4d0b6dfc1096461d0a185d1e3caf03.
Individual amd64 DLL/helper sizes and SHA256 are pinned in runtime.go; NuGet
repository re-signing never bypasses these exact native-byte checks. MIT license
is retained in the managed cache. No OS console/DLL or registry is replaced.

Windows resource builds embed those verified binaries as PE RCDATA 7701/7702;
only generated ignored syso/cache files hold binaries. Source Git contains no
native runtime payload. New executables extract to an owned versioned cache,
check hashes, then load the absolute DLL with restricted dependency search.
Offline MSI/portable/update execution needs no download. The signed updater ZIP
retains the old flat executable layout, so prior released updaters can consume
it. Package checks invoke -terminal-runtime-check on the actual staged executable.
Development/test builds without resources use the same pinned HTTPS/cache
installer; -install-terminal-tools prepares it explicitly. Session close still
drains output and owns job cleanup. Loader holds one DLL for process lifetime.

- Windows: CreatePseudoConsole, STARTUPINFOEX, suspended startup and a job with
  KILL_ON_JOB_CLOSE before resume. Independent output/reply/input readers prevent
  full-duplex pipe deadlocks. Original process/thread/attribute handles close.
- macOS/Linux: creack/pty, a new session/controlling terminal. Close collects
  owned descendants while the shell exists, kills their groups/processes and
  closes its PTY. Detached/disowned POSIX jobs and process-identity races require
  further supervision; don't claim the same containment guarantee as Windows jobs.
- Default Windows shell is installed pwsh then Windows PowerShell. Per-session
  PSReadLine colors support PowerShell 7/5.1; existing profiles remain intact.
- Default Mac shell honors SHELL, falling back to /bin/zsh. For zsh, an owned
  temporary ZDOTDIR sources the user's startup files before embedded upstream
  zsh-syntax-highlighting 0.8.0. Source/license/hash is under terminal/assets.
  Bash/custom shells retain their own configured input-highlighting behavior.
- Isolated acceptance uses owned startup/history settings. It never writes
  user dotfiles/history or sends commands to an existing terminal/session.

## Bounds and view

Maximum 8 tabs; 2..400 columns, 2..160 rows; history at most 1,000 lines.
Each input/paste is at most 64 KiB UTF-8. Request and writer queues each have
64 slots, errors report rejected input/backlog. Output blocks and terminal
string controls are bounded at 8 KiB; oversize OSC/DCS/APC/PM/SOS are discarded.
C1 controls respect UTF-8 continuation boundaries. OSC clipboard and hyperlink
side effects are disabled. Visible grapheme content is capped at 256 bytes.
Snapshot publication is coalesced to 60 Hz with one pending UI dispatch per tab.
No allocation grows with lifetime output bytes; history has fixed line bounds.

Native grid uses godesktop v0.5.2 TextAdvance with Consolas/Menlo; MeasureText keeps
the label's height/layout size. Mac negative-control CI found padded label widths
are not cell advances. Corrected framework source ff69721 passed all five jobs
in CI 37488721747 and is published as the immutable v0.5.2 public module.
Adjacent cells with matching colors/underline share a run; wide continuations
remain zero-width in immutable snapshots. Native colors/backgrounds/cursor and
selection are painted by the platform GPU. Window/panel sizes update the real
PTY. Raw/alternate-mode programs get VT key sequences; Ctrl+C goes to the shell.
Natural exit drains final output before publishing the exit code; cancellation
unblocks VT reply writers and kills the owned process. Shutdown wait is 3 seconds.

## Evidence and gaps

Local Windows amd64 strict-cgo/race tests passed real PTY Unicode, truecolor,
alternate/primary screen, input/resize, exact final output/exit, cancellation,
descendant termination, 5,000-line flood/scroll bounds, fragmented/oversize/C1
controls and PSReadLine input colors before Enter (PowerShell 7 and 5.1).
Deterministic VT reply-queue overflow verifies cancellation unblocks a writer
holding the emulator mutex before publishing errors. A timed output flood alone
is not this regression: Windows conhost can answer queries or drain them itself.
Native -terminal-smoke passed actual input colors, output truecolor, shell-reported
columns after resize, Ctrl+C during a 60-second command, exit and unchanged
editor version. Owned GPU captures input-highlight.png/output-resized.png were
visually inspected under .cache/terminal-native (not tracked). Actual Windows
HWND resize now waits for shell-reported new columns; reading the prior frame
initially produced an incorrect test expectation. Input/output syntax colors
are also counted in owned GPU pixels. Console and GUI subsystem binaries passed;
the PowerShell helper explicitly waits on the owned GUI process. Hide-window
startup prevented rendering in an early helper, so it now suppresses only console
allocation and lets the tested native window render.

Source `8a0fa07670a3018b42ab86eb2dd45e814099c7c7` passed all five jobs in CI
37498949904, including real Windows 2022/2025 console/GUI, Mac Intel/ARM terminal
and package checks. Publication 37500332017 reused these packages for v0.7.0.
The downloaded Windows 2022 input/output GPU captures were visually inspected.
Real signed public ZIP update/native-terminal/prior-release-GUI rollback passed;
separate direct GitHub and manual ghfast.top full-body SHA256 checks passed.
All five generated-metadata jobs also passed in 37500982783 at cacee0d8.
Installed v0.7.0 real terminal/GUI/Settings and inspected GPU captures passed;
immutable installation evidence is recorded in status.md.

This is not proof of full VS Code terminal parity. Missing: VSIX terminal API, full shell
integration/command history, terminal settings/profiles UI, clickable links,
Windows UI Automation/Mac accessibility, IME composition, full bold/italic/bidi
glyph handling, advanced keyboard protocols, arbitrary rich graphics and POSIX
detached job supervision. Keep failed sources and exact promoted evidence in status.
