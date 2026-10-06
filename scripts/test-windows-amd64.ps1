param([ValidateRange(1,20)][int]$Repeat = 3)
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
$taskNames = @('GOWORK','CGO_ENABLED','GOARCH','GOAMD64','GOEXPERIMENT','CC','CXX','GOCODE_SCREENSHOT','GOCODE_LARGEFILE_SCREENSHOT','GOCODE_TERMINAL_SCREENSHOTS','GOCODE_FILEWATCH_SCREENSHOTS','GOCODE_OPEN_SCREENSHOTS','GOCODE_CONPTY_DIR','GODESKTOP_READBACK')
$taskSaved = @{}
foreach ($taskName in $taskNames) { $taskSaved[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
Push-Location -LiteralPath $taskRoot
try {
    $env:GOWORK='off'
    $env:CGO_ENABLED='1'
    $env:GOARCH='amd64'
    $env:GOAMD64='v1'
    $env:GOEXPERIMENT='cgocheck2'
    $env:CC='gcc'
    $env:CXX='g++'
    $env:GODESKTOP_READBACK='1'
    $env:GOCODE_SCREENSHOT=Join-Path $taskRoot '.cache/workbench-windows.png'
    $env:GOCODE_LARGEFILE_SCREENSHOT=Join-Path $taskRoot '.cache/largefile-native-acceptance.png'
    $env:GOCODE_TERMINAL_SCREENSHOTS=Join-Path $taskRoot '.cache/terminal-native'
    $env:GOCODE_FILEWATCH_SCREENSHOTS=Join-Path $taskRoot '.cache/filewatch-native'
    $env:GOCODE_OPEN_SCREENSHOTS=Join-Path $taskRoot '.cache/open-native'
    Invoke-CheckedGo run ./cmd/gocode-terminaltools -output (Join-Path $taskRoot '.cache/conpty-runtime')
    $env:GOCODE_CONPTY_DIR=Join-Path $taskRoot '.cache/conpty-runtime'
    & (Join-Path $PSScriptRoot 'build-windows-resources.ps1')
    Invoke-CheckedGo test -race -shuffle=on "-count=$Repeat" -timeout=5m '-coverprofile=coverage.out' ./...
    Invoke-CheckedGo vet ./...
    New-Item -ItemType Directory -Path bin -Force | Out-Null
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w' -o bin/gocode.exe .
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w -H=windowsgui' -o bin/gocode-gui.exe .
    & ./bin/gocode.exe -workspace . -extensions-dir .cache/extensions -smoke
    if ($LASTEXITCODE -ne 0) { throw 'Workbench smoke failed.' }
    Invoke-CheckedGUI -workspace . -extensions-dir .cache/extensions -smoke
    & ./bin/gocode.exe -ui-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native workbench logo/caption/tab GPU acceptance failed.' }
    Invoke-CheckedGUI -ui-smoke
    & ./bin/gocode.exe -tabs-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native overflow/routing/identity/MRU tab acceptance failed.' }
    Invoke-CheckedGUI -tabs-smoke
    & ./bin/gocode.exe -search-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native workspace search/UTF-16/stale/large-file navigation failed.' }
    Invoke-CheckedGUI -search-smoke
    & ./bin/gocode.exe -extensions-dir .cache/extensions -open-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native delayed disk/scan and awaited VSIX opening acceptance failed.' }
    Invoke-CheckedGUI -extensions-dir .cache/extensions -open-smoke
    & ./bin/gocode.exe -extensions-dir .cache/extensions -editor-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Versioned native editor acceptance failed.' }
    foreach ($taskCloseMode in @('save','discard','cancel','external')) {
        & ./bin/gocode.exe -extensions-dir .cache/extensions -close-smoke $taskCloseMode
        if ($LASTEXITCODE -ne 0) { throw "Native close acceptance failed: $taskCloseMode" }
    }
    & ./bin/gocode.exe -extensions-dir .cache/extensions -largefile-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native large-file browsing acceptance failed.' }
    & ./bin/gocode.exe -extensions-dir .cache/extensions -terminal-smoke
    if ($LASTEXITCODE -ne 0) { throw 'Native real terminal highlighting/resize/interrupt acceptance failed.' }
    Invoke-CheckedGUI -extensions-dir .cache/extensions -terminal-smoke
    & ./bin/gocode.exe -extensions-dir .cache/extensions -filewatch-smoke -lsp=false
    if ($LASTEXITCODE -ne 0) { throw 'Native external file watch/VSIX acceptance failed.' }
} finally {
    foreach ($taskName in $taskNames) { [Environment]::SetEnvironmentVariable($taskName,$taskSaved[$taskName],'Process') }
    Pop-Location
}
