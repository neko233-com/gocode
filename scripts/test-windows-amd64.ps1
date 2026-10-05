param([ValidateRange(1,20)][int]$Repeat = 3)
$ErrorActionPreference = 'Stop'
if (-not [Environment]::Is64BitOperatingSystem) { throw 'A 64-bit Windows OS is required.' }
$taskCompiler = (& gcc -dumpmachine).Trim()
if ($LASTEXITCODE -ne 0 -or $taskCompiler -notmatch '^x86_64-.*mingw') { throw "Expected x86-64 MinGW-w64, found $taskCompiler" }
function Invoke-CheckedGo {
    & go @args
    if ($LASTEXITCODE -ne 0) { throw "go $args failed with exit code $LASTEXITCODE" }
}
$taskRoot = Split-Path -Parent $PSScriptRoot
$taskNames = @('CGO_ENABLED','GOARCH','GOAMD64','GOEXPERIMENT','CC','CXX','GOCODE_SCREENSHOT')
$taskSaved = @{}
foreach ($taskName in $taskNames) { $taskSaved[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
Push-Location -LiteralPath $taskRoot
try {
    $env:CGO_ENABLED='1'
    $env:GOARCH='amd64'
    $env:GOAMD64='v1'
    $env:GOEXPERIMENT='cgocheck2'
    $env:CC='gcc'
    $env:CXX='g++'
    $env:GOCODE_SCREENSHOT=Join-Path $taskRoot '.cache/workbench-windows.png'
    Invoke-CheckedGo test -race -shuffle=on "-count=$Repeat" -timeout=5m -coverprofile=coverage.out ./...
    Invoke-CheckedGo vet ./...
    New-Item -ItemType Directory -Path bin -Force | Out-Null
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w' -o bin/gocode.exe .
    Invoke-CheckedGo build -trimpath '-ldflags=-s -w -H=windowsgui' -o bin/gocode-gui.exe .
    & ./bin/gocode.exe -workspace . -extensions-dir .cache/extensions -smoke
    if ($LASTEXITCODE -ne 0) { throw 'Workbench smoke failed.' }
    & ./bin/gocode-gui.exe -workspace . -extensions-dir .cache/extensions -smoke
    if ($LASTEXITCODE -ne 0) { throw 'GUI workbench smoke failed.' }
} finally {
    foreach ($taskName in $taskNames) { [Environment]::SetEnvironmentVariable($taskName,$taskSaved[$taskName],'Process') }
    Pop-Location
}
