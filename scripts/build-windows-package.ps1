param(
    [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
    [string]$OutputDirectory='',
    [switch]$AllowDirty,
    [switch]$SkipMSI
)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
if(-not $Version){$Version=(Get-Content -LiteralPath (Join-Path $taskRoot 'VERSION') -Raw).Trim()}
if (-not $OutputDirectory) {$OutputDirectory=Join-Path $taskRoot 'dist'}
$taskOutput=[IO.Path]::GetFullPath($OutputDirectory)
$taskStage=Join-Path $taskRoot ('.cache/package-'+$Version+'-'+[Guid]::NewGuid().ToString('N'))
$taskPayload=Join-Path $taskStage 'payload'
$taskVersion=Join-Path $taskPayload ('versions/'+$Version)
New-Item -ItemType Directory -Path $taskVersion,$taskOutput -Force | Out-Null
$taskNames=@('GOWORK','CGO_ENABLED','GOARCH','GOAMD64','GOEXPERIMENT')
$taskSaved=@{};foreach($taskName in $taskNames){$taskSaved[$taskName]=[Environment]::GetEnvironmentVariable($taskName,'Process')}
Push-Location -LiteralPath $taskRoot
try {
    $taskCommit=(& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) {throw 'A Git source revision is required.'}
    $taskDirty=(& git status --porcelain).Count -gt 0
    if($taskDirty -and -not $AllowDirty){throw 'Release packaging requires a clean committed source; use -AllowDirty only for disposable development acceptance.'}
    if($taskDirty){$taskCommit+='-dirty'}
    $env:GOWORK='off';$env:CGO_ENABLED='1';$env:GOARCH='amd64';$env:GOAMD64='v1';$env:GOEXPERIMENT='cgocheck2'
    & (Join-Path $PSScriptRoot 'build-windows-resources.ps1') -Version $Version
    Copy-Item -LiteralPath (Join-Path $taskRoot 'gocode_windows_amd64.syso') -Destination (Join-Path $taskRoot 'cmd/gocode-launcher/gocode_windows_amd64.syso')
    $taskFlags="-s -w -X main.version=$Version -X main.sourceCommit=$taskCommit"
    & go build -trimpath "-ldflags=$taskFlags" -o (Join-Path $taskVersion 'gocode-app.exe') .
    if($LASTEXITCODE -ne 0){throw 'Console app build failed.'}
    & go build -trimpath "-ldflags=$taskFlags -H=windowsgui" -o (Join-Path $taskVersion 'gocode-app-gui.exe') .
    if($LASTEXITCODE -ne 0){throw 'GUI app build failed.'}
    $env:CGO_ENABLED='0'
    & go build -trimpath '-ldflags=-s -w' -o (Join-Path $taskPayload 'gocode.exe') ./cmd/gocode-launcher
    if($LASTEXITCODE -ne 0){throw 'Command launcher build failed.'}
    & go build -trimpath '-ldflags=-s -w -H=windowsgui' -o (Join-Path $taskPayload 'gocode-launch.exe') ./cmd/gocode-launcher
    if($LASTEXITCODE -ne 0){throw 'GUI launcher build failed.'}
    & go build -trimpath '-ldflags=-s -w -H=windowsgui' -o (Join-Path $taskPayload 'gocode-maintenance.exe') ./cmd/gocode-maintenance
    if($LASTEXITCODE -ne 0){throw 'Maintenance helper build failed.'}
    $taskManifest=@{owner='neko233-com/gocode';schema=1;version=$Version;source=$taskCommit}|ConvertTo-Json
    [IO.File]::WriteAllText((Join-Path $taskPayload 'base.json'),$taskManifest,[Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath (Join-Path $taskRoot 'LICENSE') -Destination (Join-Path $taskPayload 'LICENSE.txt')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'README.md') -Destination (Join-Path $taskPayload 'README.md')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'assets/code-oss/code.ico') -Destination (Join-Path $taskPayload 'gocode.ico')
    Copy-Item -LiteralPath (Join-Path $taskRoot 'assets/code-oss/LICENSE.txt') -Destination (Join-Path $taskPayload 'CODE-OSS-LICENSE.txt')
    $taskZip=Join-Path $taskOutput "gocode-$Version-windows-amd64.zip"
    if(Test-Path -LiteralPath $taskZip){throw "Artifact exists: $taskZip"}
    # Windows PowerShell 5 Compress-Archive writes backslash ZIP entry names.
    # ZIP paths must be canonical forward slashes for the strict updater.
    Add-Type -AssemblyName System.IO.Compression
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $taskArchive=[IO.Compression.ZipFile]::Open($taskZip,[IO.Compression.ZipArchiveMode]::Create)
    try{foreach($taskFile in (Get-ChildItem -LiteralPath $taskPayload -File -Recurse)){
        $taskRelative=$taskFile.FullName.Substring($taskPayload.Length+1).Replace('\','/')
        [void][IO.Compression.ZipFileExtensions]::CreateEntryFromFile($taskArchive,$taskFile.FullName,$taskRelative,[IO.Compression.CompressionLevel]::Optimal)
    }}finally{$taskArchive.Dispose()}
    $taskCheckSource=$taskCommit.Replace('-dirty','');$taskProbe='true';if($taskDirty){$taskProbe='false'}
    & go run ./cmd/gocode-packagecheck -archive $taskZip -platform windows/amd64 -version $Version -source $taskCheckSource "-probe=$taskProbe"
    if($LASTEXITCODE -ne 0){throw 'Generated updater archive extraction/health failed.'}
    & (Join-Path $taskPayload 'gocode.exe') -version
    if($LASTEXITCODE -ne 0){throw 'Packaged command launcher failed.'}
    if(-not $SkipMSI){& (Join-Path $PSScriptRoot 'build-msi.ps1') -PayloadDirectory $taskPayload -Version $Version -OutputPath (Join-Path $taskOutput "gocode-$Version-windows-amd64.msi")}
    Write-Output "Payload: $taskPayload"
    Get-FileHash -Algorithm SHA256 -LiteralPath $taskZip
} finally {
    foreach($taskName in $taskNames){[Environment]::SetEnvironmentVariable($taskName,$taskSaved[$taskName],'Process')}
    Pop-Location
}
