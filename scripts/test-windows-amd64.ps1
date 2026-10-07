param([ValidateRange(1,20)][int]$Repeat = 3,[switch]$UseLocalFramework)
$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitOperatingSystem) { throw 'A 64-bit Windows OS is required.' }
$taskCompiler = (& gcc -dumpmachine).Trim()
if ($LASTEXITCODE -ne 0 -or $taskCompiler -notmatch '^x86_64-.*mingw') { throw "Expected x86-64 MinGW-w64, found $taskCompiler" }
function Invoke-CheckedGo {
    & go @args
    if ($LASTEXITCODE -ne 0) { throw "go $args failed with exit code $LASTEXITCODE" }
}
function Invoke-CheckedGUI {
    # PowerShell may return immediately for a GUI subsystem binary. Wait on the
    # exact owned process so GPU acceptance stays sequential and exit is checked.
    $taskInfo = New-Object System.Diagnostics.ProcessStartInfo
    $taskInfo.FileName = (Resolve-Path './bin/gocode-gui.exe').Path
    $taskInfo.Arguments = [string]::Join(' ', $args)
    $taskInfo.UseShellExecute = $false
    $taskInfo.CreateNoWindow = $true
    $taskInfo.RedirectStandardOutput = $true
    $taskInfo.RedirectStandardError = $true
    $taskGUI = [System.Diagnostics.Process]::Start($taskInfo)
    $taskOutput = $taskGUI.StandardOutput.ReadToEndAsync()
    $taskError = $taskGUI.StandardError.ReadToEndAsync()
    try {
        if (-not $taskGUI.WaitForExit(60000)) { $taskGUI.Kill(); throw 'GUI acceptance timed out.' }
        [IO.File]::WriteAllText((Join-Path (Get-Location) '.cache/gui-smoke.log'), $taskOutput.Result)
        [IO.File]::WriteAllText((Join-Path (Get-Location) '.cache/gui-smoke-error.log'), $taskError.Result)
        if ($taskGUI.ExitCode -ne 0) { throw "GUI acceptance failed with exit $($taskGUI.ExitCode): $($taskError.Result)" }
    } finally { $taskGUI.Dispose() }
}
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskNames = @('GOWORK','CGO_ENABLED','GOARCH','GOAMD64','GOEXPERIMENT','CC','CXX','TMP','TEMP','APPDATA','GOCODE_SCREENSHOT','GOCODE_LARGEFILE_SCREENSHOT','GOCODE_TERMINAL_SCREENSHOTS','GOCODE_FILEWATCH_SCREENSHOTS','GOCODE_OPEN_SCREENSHOTS','GOCODE_AUTOSAVE_SCREENSHOTS','GOCODE_AUTOSAVE_MINIMIZED_REPORT','GOCODE_CONPTY_DIR','GODESKTOP_READBACK','GODESKTOP_TEST_INPUT_ISOLATION')
$taskSaved = @{}
foreach ($taskName in $taskNames) { $taskSaved[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
Push-Location -LiteralPath $taskRoot
$taskTempParent = [IO.Path]::GetFullPath([IO.Path]::GetTempPath()).TrimEnd('\')
$taskNativeRoot = Join-Path $taskTempParent ('gocode-validation-' + [guid]::NewGuid().ToString('N'))
$taskExtensions = Join-Path $taskNativeRoot 'extensions'
try {
	New-Item -ItemType Directory -Path $taskNativeRoot -Force | Out-Null
	$env:TMP=$taskNativeRoot
	$env:TEMP=$taskNativeRoot
	$env:APPDATA=Join-Path $taskNativeRoot 'settings'
    if ($UseLocalFramework) {
        $env:GOWORK=(Resolve-Path -LiteralPath (Join-Path $taskRoot '../go.work')).Path
        Write-Output 'Development validation uses the local framework workspace; independent published-module validation is still required before promotion.'
    } else {
        $env:GOWORK='off'
    }
    $env:CGO_ENABLED='1'
    $env:GOARCH='amd64'
    $env:GOAMD64='v1'
    $env:GOEXPERIMENT='cgocheck2'
    $env:CC='gcc'
    $env:CXX='g++'
    $env:GODESKTOP_READBACK='1'
    $env:GODESKTOP_TEST_INPUT_ISOLATION='1'
    $env:GOCODE_SCREENSHOT=Join-Path $taskRoot '.cache/workbench-windows.png'
    $env:GOCODE_LARGEFILE_SCREENSHOT=Join-Path $taskRoot '.cache/largefile-native-acceptance.png'
    $env:GOCODE_TERMINAL_SCREENSHOTS=Join-Path $taskRoot '.cache/terminal-native'
    $env:GOCODE_FILEWATCH_SCREENSHOTS=Join-Path $taskRoot '.cache/filewatch-native'
    $env:GOCODE_OPEN_SCREENSHOTS=Join-Path $taskRoot '.cache/open-native'
    $env:GOCODE_AUTOSAVE_SCREENSHOTS=Join-Path $taskRoot '.cache/auto-save-native'
    $env:GOCODE_AUTOSAVE_MINIMIZED_REPORT=Join-Path $taskRoot '.cache/auto-save-minimized/current.json'
    Invoke-CheckedGo run ./cmd/gocode-terminaltools -output (Join-Path $taskRoot '.cache/conpty-runtime')
    $env:GOCODE_CONPTY_DIR=Join-Path $taskRoot '.cache/conpty-runtime'
    & (Join-Path $PSScriptRoot 'build-windows-resources.ps1')
    # Default-three 45/66-phase lifecycle/minimized suites exceed the old 8m
    # aggregate package alarm on CI. Keep each owned-process/dialog guard.
    # Keep actual shell/GPU fixtures away from simultaneous GiB/parser/process
    # package tests on small Windows runners. All package/repeat guards remain.
    Invoke-CheckedGo test -p=1 -race -shuffle=on "-count=$Repeat" -timeout=12m '-coverprofile=coverage.out' ./...
    Invoke-CheckedGo vet ./...
    New-Item -ItemType Directory -Path bin -Force | Out-Null
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w' -o bin/gocode.exe .
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w -H=windowsgui' -o bin/gocode-gui.exe .
    & ./bin/gocode.exe -workspace . -extensions-dir $taskExtensions -smoke
    if ($LASTEXITCODE -ne 0) { throw 'Workbench smoke failed.' }
    Invoke-CheckedGUI -workspace . -extensions-dir $taskExtensions -smoke
    & ./bin/gocode.exe -ui-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native workbench logo/caption/tab GPU acceptance failed.' }
    Invoke-CheckedGUI -ui-smoke
    & ./bin/gocode.exe -windows-workbench-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native File/shell dialog/Quick Input/VSIX management failed.' }
    Invoke-CheckedGUI -windows-workbench-smoke
    & ./bin/gocode.exe -extension-detail-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native extension detail viewport/font/VSIX management failed.' }
    Invoke-CheckedGUI -extension-detail-smoke
    & ./bin/gocode.exe -auto-save-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native Auto Save/window activation/Revert acceptance failed.' }
    Invoke-CheckedGUI -auto-save-smoke
    & ./bin/gocode.exe -auto-save-minimized-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native minimized Auto Save/disk/event/GPU-idle acceptance failed.' }
    Invoke-CheckedGUI -auto-save-minimized-smoke
    & ./bin/gocode.exe -tabs-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native overflow/routing/identity/MRU tab acceptance failed.' }
    Invoke-CheckedGUI -tabs-smoke
    & ./bin/gocode.exe -groups-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native editor groups/shared edits/sash/close scope failed.' }
    Invoke-CheckedGUI -groups-smoke
    & ./bin/gocode.exe -groups-vsix-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native VSIX view/column/selection/reveal/disposal/save failed.' }
    Invoke-CheckedGUI -groups-vsix-smoke
    & ./bin/gocode.exe -groups-large-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native shared-index/independent large split failed.' }
    Invoke-CheckedGUI -groups-large-smoke
    & ./bin/gocode.exe -search-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native workspace search/UTF-16/stale/large-file navigation failed.' }
    Invoke-CheckedGUI -search-smoke
    & ./bin/gocode.exe -replace-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native workspace replace preview/stale/save/undo failed.' }
    Invoke-CheckedGUI -replace-smoke
    & ./bin/gocode.exe -extensions-dir $taskExtensions -open-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native delayed disk/scan and awaited VSIX opening acceptance failed.' }
    Invoke-CheckedGUI -extensions-dir $taskExtensions -open-smoke
    & ./bin/gocode.exe -extensions-dir $taskExtensions -editor-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Versioned native editor acceptance failed.' }
    foreach ($taskCloseMode in @('save','discard','cancel','external')) {
        & ./bin/gocode.exe -extensions-dir $taskExtensions -close-smoke $taskCloseMode
        if ($LASTEXITCODE -ne 0) { throw "Native close acceptance failed: $taskCloseMode" }
    }
    & ./bin/gocode.exe -extensions-dir $taskExtensions -largefile-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native large-file browsing acceptance failed.' }
    & ./bin/gocode.exe -extensions-dir $taskExtensions -terminal-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native real terminal highlighting/resize/interrupt acceptance failed.' }
    Invoke-CheckedGUI -extensions-dir $taskExtensions -terminal-smoke
    & ./bin/gocode.exe -terminal-vsix-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native VSIX terminal PID/cwd/env/input/events/process cleanup failed.' }
    Invoke-CheckedGUI -terminal-vsix-smoke
    & ./bin/gocode.exe -scm-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native real Git index/HEAD/diff/stage/unstage/commit failed.' }
    Invoke-CheckedGUI -scm-smoke
    & ./bin/gocode.exe -extensions-dir $taskExtensions -filewatch-smoke -lsp=false
    if ($LASTEXITCODE -ne 0) { throw 'Native external file watch/VSIX acceptance failed.' }
} finally {
    foreach ($taskName in $taskNames) { [Environment]::SetEnvironmentVariable($taskName,$taskSaved[$taskName],'Process') }
    Pop-Location
	$taskResolved = [IO.Path]::GetFullPath($taskNativeRoot)
	if ([IO.Path]::GetDirectoryName($taskResolved).TrimEnd('\') -ne $taskTempParent -or [IO.Path]::GetFileName($taskResolved) -notmatch '^gocode-validation-[0-9a-f]{32}$') { throw 'Refusing cleanup outside the owned validation root.' }
	if (Test-Path -LiteralPath $taskResolved) { Remove-Item -LiteralPath $taskResolved -Recurse -Force }
	if (Test-Path -LiteralPath $taskResolved) { throw 'Validation left its owned temporary directory.' }
}
