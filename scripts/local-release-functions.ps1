Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$script:ReleaseProject = Split-Path -Parent $PSScriptRoot
$script:ReleaseUTF8 = [Text.UTF8Encoding]::new($false)

function Write-ReleaseJSON {
    param([string]$Path, $Value)
    $taskJSON = $Value | ConvertTo-Json -Depth 40
    if ($script:ReleaseUTF8.GetByteCount($taskJSON) -gt 524288) { throw 'Release JSON exceeds 512KiB.' }
    $taskTemporary = $Path + '.pending'
    [IO.File]::WriteAllText($taskTemporary, $taskJSON + "`n", $script:ReleaseUTF8)
    Move-Item -LiteralPath $taskTemporary -Destination $Path -Force
}
function Read-ReleaseJSON {
    param([string]$Path)
    $taskFile = Get-Item -LiteralPath $Path
    if ($taskFile.PSIsContainer -or $taskFile.Length -gt 524288 -or ($taskFile.Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Invalid bounded release JSON.' }
    $taskText = [IO.File]::ReadAllText($taskFile.FullName, $script:ReleaseUTF8)
    [GocodeReleaseJson]::Validate($taskText)
    return $taskText | ConvertFrom-Json
}
function Assert-ReleaseOwnedPath {
    param([string]$Path, [string]$Root)
    $taskAbsolute = [IO.Path]::GetFullPath($Path)
    $taskRoot = [IO.Path]::GetFullPath($Root).TrimEnd('\', '/')
    if (-not $taskAbsolute.StartsWith($taskRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Owned path escapes its explicit root.' }
    for ($taskAncestor = $taskAbsolute; ; $taskAncestor = Split-Path -Parent $taskAncestor) {
        if (Test-Path -LiteralPath $taskAncestor) {
            $taskItem = Get-Item -LiteralPath $taskAncestor -Force
            if (($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Reparsed release path is forbidden.' }
        }
        if ($taskAncestor.Equals($taskRoot, [StringComparison]::OrdinalIgnoreCase)) { break }
        if (-not $taskAncestor) { throw 'Owned path ancestor escaped its root.' }
    }
    return $taskAbsolute
}
function Initialize-ReleaseWorkDirectory {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Explicit builder parameter creates only a new or empty owner-validated cache directory, never deletes existing entries.')]
    param([string]$Path)
    $taskPath = Assert-ReleaseOwnedPath $Path (Join-Path $script:ReleaseProject '.cache')
    if (Test-Path -LiteralPath $taskPath) {
        if (-not (Test-Path -LiteralPath $taskPath -PathType Container) -or @(Get-ChildItem -LiteralPath $taskPath -Force).Count -ne 0) { throw 'Explicit builder work directory must be new or empty.' }
    } else { New-Item -ItemType Directory -Path $taskPath -Force | Out-Null }
    return $taskPath
}
function Get-ReleaseFile {
    param([string]$Path, [string]$Root)
    $taskPath = Assert-ReleaseOwnedPath -Path $Path -Root $Root
    $taskFile = Get-Item -LiteralPath $taskPath
    if ($taskFile.PSIsContainer) { throw 'Release evidence must be a regular file.' }
    return [ordered]@{ path = $taskPath.Substring([IO.Path]::GetFullPath($Root).TrimEnd('\', '/').Length + 1).Replace('\', '/'); bytes = $taskFile.Length; sha256 = (Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() }
}
function Assert-ReleasePlan {
    param($Plan)
    if (@($Plan.platforms).Count -ne 1 -or $Plan.platforms[0] -cne 'windows/amd64' -or ($null -ne $Plan.pendingCommands -and @($Plan.pendingCommands).Count -ne 0)) { throw 'Only a fully wired Windows plan is publishable.' }
    if (@($Plan.checks).Count -lt 1 -or @($Plan.gates).Count -lt 1 -or @($Plan.checks).Count -gt 64 -or @($Plan.gates).Count -gt 64) { throw 'Invalid bounded release plan.' }
    $taskNames = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($taskName in $Plan.checks) {
        if ($taskName -cnotmatch '^[a-z][a-z0-9-]{0,79}$' -or -not $taskNames.Add($taskName)) { throw 'Invalid or duplicate source check.' }
    }
    $taskNames.Clear()
    foreach ($taskGate in $Plan.gates) {
        if ($taskGate.id -cnotmatch '^[a-z][a-z0-9-]{0,79}$' -or -not $taskNames.Add($taskGate.id) -or @($taskGate.args).Count -lt 1 -or @($taskGate.args).Count -gt 32 -or $taskGate.gui -isnot [bool]) { throw 'Invalid or duplicate native gate.' }
        if ($taskGate.timeoutNanoseconds -notin @(60000000000L, 120000000000L)) { throw 'Native gate must retain its exact 60/120-second deadline.' }
        foreach ($taskArgument in $taskGate.args) { if ($taskArgument -isnot [string] -or $taskArgument.Length -gt 4096 -or $taskArgument.Contains([string][char]0)) { throw 'Invalid gate argument.' } }
    }
}
function Assert-ReleaseOrder {
    param([object[]]$Actual, [object[]]$Expected)
    if ($Actual.Count -ne $Expected.Count) { throw 'Release evidence count differs from generated plan.' }
    for ($taskIndex = 0; $taskIndex -lt $Expected.Count; $taskIndex++) {
        if ([string]$Actual[$taskIndex] -cne [string]$Expected[$taskIndex]) { throw 'Release evidence identity/order differs from generated plan.' }
    }
}
function Assert-ReleaseBoundFile {
    param($File, [string]$Root, [switch]$Nonempty)
    if ($File.path -isnot [string] -or $File.path -cnotmatch '^[A-Za-z0-9._/-]+$' -or @($File.path.Split('/') | Where-Object { $_ -in @('', '.', '..') }).Count -ne 0 -or $File.sha256 -isnot [string] -or $File.sha256 -cnotmatch '^[0-9a-f]{64}$' -or $File.bytes -isnot [ValueType] -or $File.bytes -is [bool] -or $File.bytes -lt 0 -or [Math]::Truncate([double]$File.bytes) -ne $File.bytes -or ($Nonempty -and $File.bytes -le 0)) { throw 'Prepared file metadata is invalid.' }
    $taskActual = Get-ReleaseFile (Join-Path $Root $File.path) $Root
    if ($taskActual.path -cne $File.path -or $taskActual.bytes -ne $File.bytes -or $taskActual.sha256 -cne $File.sha256) { throw 'Prepared evidence bytes differ from the original preparation.' }
}
function Assert-ReleasePrepared {
    param($Prepared, [string]$Root, [string]$Version, [string]$Source, [string]$InputDigest, $Framework, $Plan, $Commands, [string]$Checker)
    if ($Prepared.schema -ne 1 -or $Prepared.complete -isnot [bool] -or $Prepared.complete -or $Prepared.prepared -isnot [bool] -or -not $Prepared.prepared -or $Prepared.needsExistingSigningKey -isnot [bool] -or -not $Prepared.needsExistingSigningKey -or $Prepared.version -cne $Version -or $Prepared.source -cne $Source -or $InputDigest -cnotmatch '^[0-9a-f]{64}$' -or $Prepared.inputDigest -cne $InputDigest) { throw 'Only the same incomplete prepared source may continue validation.' }
    foreach ($taskAbsent in @('privateTMPAbsent', 'privateConfigAbsent', 'stageAbsent')) { if ($Prepared.$taskAbsent -isnot [bool] -or -not $Prepared.$taskAbsent) { throw 'Preparation did not prove its private directory cleanup.' } }
    if (($Prepared.framework | ConvertTo-Json -Depth 40 -Compress) -cne ($Framework | ConvertTo-Json -Depth 40 -Compress)) { throw 'Current verified public framework differs from preparation.' }
    $taskIDs = @($Commands.Keys)
    if ($taskIDs.Count -ne 5) { throw 'Exactly five source checks must precede signing.' }
    Assert-ReleaseOrder $taskIDs @($Plan.checks | Select-Object -First 5)
    Assert-ReleaseOrder @($Prepared.checks | ForEach-Object { $_.id }) $taskIDs
    foreach ($taskCheck in $Prepared.checks) {
        if ($taskCheck.exitCode -ne 0) { throw 'Prepared check was unsuccessful.' }
        Assert-ReleaseBoundFile $taskCheck.evidence $Root -Nonempty
        if ($taskCheck.evidence.path -cne ('logs/' + $taskCheck.id + '.json')) { throw 'Prepared check evidence has the wrong identity.' }
        $taskEvidence = Read-ReleaseJSON (Join-Path $Root $taskCheck.evidence.path)
        if ($taskEvidence.schema -ne 1 -or $taskEvidence.id -cne $taskCheck.id -or $taskEvidence.version -cne $Version -or $taskEvidence.source -cne $Source -or $taskEvidence.inputDigest -cne $InputDigest -or $taskEvidence.pid -is [bool] -or $taskEvidence.pid -is [string] -or $taskEvidence.pid -le 0 -or $taskEvidence.exitCode -ne 0 -or $taskEvidence.elapsedMS -is [bool] -or $taskEvidence.elapsedMS -is [string] -or $taskEvidence.elapsedMS -lt 0 -or $taskEvidence.rootReaped -isnot [bool] -or -not $taskEvidence.rootReaped -or $taskEvidence.treeClosed -isnot [bool] -or -not $taskEvidence.treeClosed) { throw 'Prepared check cannot be relabeled as successful current-source evidence.' }
        Assert-ReleaseOrder @($taskEvidence.command) @($Commands[$taskCheck.id])
        foreach ($taskStream in @('stdout', 'stderr')) {
            if ($taskEvidence.$taskStream.path -cne ('logs/' + $taskCheck.id + '.' + $taskStream + '.log')) { throw 'Prepared captured log has the wrong identity.' }
            Assert-ReleaseBoundFile $taskEvidence.$taskStream $Root
        }
    }
    Assert-ReleaseOrder @($Prepared.artifacts | ForEach-Object { $_.path }) @(('artifacts/gocode-' + $Version + '-windows-amd64.msi'), ('artifacts/gocode-' + $Version + '-windows-amd64.zip'))
    foreach ($taskArtifact in $Prepared.artifacts) { Assert-ReleaseBoundFile $taskArtifact $Root -Nonempty }
    $taskChannelNames = @($Prepared.channels | ForEach-Object { $_.path })
    if ($taskChannelNames.Count -lt 1 -or $taskChannelNames.Count -gt 64 -or 'channels/manifest-input.json' -cnotin $taskChannelNames -or @($taskChannelNames | Where-Object { -not $_.StartsWith('channels/', [StringComparison]::Ordinal) }).Count) { throw 'Prepared channel set is invalid.' }
    $taskActualChannels = @(Get-ChildItem -LiteralPath (Join-Path $Root 'channels') -File -Recurse -Force | Sort-Object FullName | ForEach-Object { (Get-ReleaseFile $_.FullName $Root).path })
    Assert-ReleaseOrder $taskChannelNames $taskActualChannels
    foreach ($taskChannel in $Prepared.channels) { Assert-ReleaseBoundFile $taskChannel $Root -Nonempty }
    Assert-ReleaseBoundFile $Prepared.checker $Root -Nonempty
    if ($Prepared.checker.path -cne 'gocode-localreleasecheck.exe' -or [IO.Path]::GetFullPath($Checker) -cne [IO.Path]::GetFullPath((Join-Path $Root $Prepared.checker.path))) { throw 'Validation must use the original prepared checker.' }
}
function Resolve-ReleaseCommand {
    param([string]$Name)
    return (Get-Command -Name $Name -CommandType Application -ErrorAction Stop | Select-Object -First 1).Source
}
function Invoke-ReleaseProcess {
    param([string[]]$Command, [int]$TimeoutSeconds, [string]$LogStem, [Collections.Generic.Dictionary[string,string]]$Environment)
    if ($Command.Count -lt 1 -or $TimeoutSeconds -lt 1 -or $TimeoutSeconds -gt 1200) { throw 'Invalid owned process command/deadline.' }
    $taskExecutable = Resolve-ReleaseCommand $Command[0]
    $taskArguments = @($Command | Select-Object -Skip 1)
    $taskResult = [GocodeReleaseProcess]::Run($taskExecutable, [string[]]$taskArguments, $script:ReleaseProject, $Environment, $TimeoutSeconds * 1000)
    [IO.File]::WriteAllBytes($LogStem + '.stdout.log', $taskResult.Stdout)
    [IO.File]::WriteAllBytes($LogStem + '.stderr.log', $taskResult.Stderr)
    if ($taskResult.Error -or $taskResult.ExitCode -ne 0 -or -not $taskResult.RootReaped -or -not $taskResult.TreeClosed -or -not $taskResult.JobAssignedBeforeResume) {
        Write-ReleaseJSON -Path ($LogStem + '.failed.json') -Value ([ordered]@{ command = $Command; pid = $taskResult.PID; exitCode = $taskResult.ExitCode; elapsedMS = $taskResult.ElapsedMS; error = $taskResult.Error; rootReaped = $taskResult.RootReaped; treeClosed = $taskResult.TreeClosed })
        throw "Owned process failed; inspect $LogStem.failed.json and captured logs."
    }
    return $taskResult
}
function Get-ReleaseEnvironment {
    param([string]$Root)
    $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($taskName in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskEnvironment[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
    foreach ($taskName in @('TMP', 'TEMP', 'GOTMPDIR')) { $taskEnvironment[$taskName] = Join-Path $Root 'private-tmp' }
    $taskEnvironment['APPDATA'] = Join-Path $Root 'private-config'
    $taskEnvironment['LOCALAPPDATA'] = Join-Path $Root 'private-config'
    $taskEnvironment['GOWORK'] = 'off'; $taskEnvironment['GOFLAGS'] = ''; $taskEnvironment['CGO_ENABLED'] = '1'; $taskEnvironment['GOARCH'] = 'amd64'; $taskEnvironment['GOAMD64'] = 'v1'; $taskEnvironment['GOEXPERIMENT'] = 'cgocheck2'; $taskEnvironment['CC'] = 'gcc'; $taskEnvironment['CXX'] = 'g++'
    $taskEnvironment['GODESKTOP_GPU_DEBUG'] = '0'; $taskEnvironment['GODESKTOP_DX12_TRACE'] = '0'
    $taskEnvironment['GODESKTOP_READBACK'] = '1'; $taskEnvironment['GODESKTOP_TEST_INPUT_ISOLATION'] = '1'
    $taskEnvironment.Remove('GODESKTOP_TEST_DRAWABLE_SCALE') | Out-Null
    $taskEnvironment.Remove('GODESKTOP_GPU_ADAPTER') | Out-Null
    $taskEnvironment.Remove('GOCODE_CONPTY_DIR') | Out-Null
    foreach ($taskName in @($taskEnvironment.Keys)) {
        if ($taskName -like 'GOCODE_*SCREENSHOT*' -or $taskName -like 'GOCODE_*_REPORT' -or $taskName -eq 'GOCODE_POPUP_SHADOW_OUTPUT' -or $taskName -like 'GOCODE_TEST_*' -or $taskName -like 'GODESKTOP_TEST_*') {
            if ($taskName -ne 'GODESKTOP_TEST_INPUT_ISOLATION') { $taskEnvironment.Remove($taskName) | Out-Null }
        }
    }
    $taskEnvironment['GIT_CEILING_DIRECTORIES'] = Join-Path $script:ReleaseProject '.cache'
    return ,$taskEnvironment
}
function Set-ReleaseNativeOutput {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Internal explicit execution assigns private artifact paths and creates only its owner-validated stage subdirectory.')]
    param([Collections.Generic.Dictionary[string,string]]$Environment, [string]$Stage, [string]$ID)
    if ($ID -cnotmatch '^[a-z][a-z0-9-]{0,79}$') { throw 'Invalid native artifact identity.' }
    $taskOutput = Join-Path $Stage ('.cache/native-gates/' + $ID)
    New-Item -ItemType Directory -Path $taskOutput -Force | Out-Null
    foreach ($taskName in @('GOCODE_OPEN_SCREENSHOTS', 'GOCODE_FILEWATCH_SCREENSHOTS', 'GOCODE_CLOSE_SCREENSHOTS', 'GOCODE_AUTOSAVE_SCREENSHOTS', 'GOCODE_GROUPS_SCREENSHOTS', 'GOCODE_TABS_SCREENSHOTS', 'GOCODE_SCM_SCREENSHOTS', 'GOCODE_SEARCH_SCREENSHOTS', 'GOCODE_TERMINAL_SCREENSHOTS', 'GOCODE_REPLACE_SCREENSHOTS', 'GOCODE_TERMINAL_VSIX_SCREENSHOTS', 'GOCODE_WINDOWS_WORKBENCH_SCREENSHOTS', 'GOCODE_UI_SCREENSHOTS', 'GOCODE_POPUP_SHADOW_OUTPUT')) { $Environment[$taskName] = $taskOutput }
    foreach ($taskName in @('GOCODE_SCREENSHOT', 'GOCODE_AI_SCREENSHOT', 'GOCODE_LARGEFILE_SCREENSHOT', 'GOCODE_LSP_SCREENSHOT')) { $Environment[$taskName] = Join-Path $taskOutput ($taskName.ToLowerInvariant() + '.png') }
    foreach ($taskName in @('GOCODE_AUTOSAVE_MINIMIZED_REPORT', 'GOCODE_EXTENSION_DETAIL_REPORT')) { $Environment[$taskName] = Join-Path $taskOutput ($taskName.ToLowerInvariant() + '.json') }
}
function Get-ReleaseMSIProperty {
    param($Rows, [string]$Name)
    $taskMatches = 0; $taskValue = ''
    foreach ($taskRow in $Rows) {
        if (@($taskRow).Count -ne 2) { throw 'Invalid MSI property row shape.' }
        if ($taskRow[0] -ceq $Name) { $taskMatches++; $taskValue = [string]$taskRow[1] }
    }
    if ($taskMatches -ne 1 -or -not $taskValue) { throw 'MSI property must occur exactly once with a value.' }
    return $taskValue
}
function Assert-ReleaseSigningKeyPath {
    param([string]$Path)
    $taskPath = [IO.Path]::GetFullPath($Path)
    $taskCheckout = [IO.Path]::GetFullPath((Split-Path -Parent $script:ReleaseProject)).TrimEnd('\', '/')
    if ($taskPath.Equals($taskCheckout, [StringComparison]::OrdinalIgnoreCase) -or $taskPath.StartsWith($taskCheckout + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Existing production key must remain outside both checkouts.' }
    for ($taskAncestor = $taskPath; $taskAncestor; $taskAncestor = Split-Path -Parent $taskAncestor) {
        $taskItem = Get-Item -LiteralPath $taskAncestor -Force
        if (($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Production key path contains a reparse point.' }
        if ($taskAncestor -eq [IO.Path]::GetPathRoot($taskAncestor)) { break }
    }
    return $taskPath
}
function Test-ReleaseRemoteTagBinding {
    param([string]$Listing, [string]$Tag, [string]$Source)
    $taskRefs = @{}
    foreach ($taskLine in @($Listing -split "`r?`n" | Where-Object { $_ })) {
        $taskParts = $taskLine -split "`t"
        if ($taskParts.Count -ne 2 -or $taskParts[0] -cnotmatch '^[0-9a-f]{40}$' -or $taskParts[1] -cnotin @(('refs/tags/' + $Tag), ('refs/tags/' + $Tag + '^{}')) -or $taskRefs.ContainsKey($taskParts[1])) { throw 'Remote immutable tag listing is invalid or duplicated.' }
        $taskRefs[$taskParts[1]] = $taskParts[0]
    }
    if ($taskRefs.Count -eq 0) { return $false }
    if (-not $taskRefs.ContainsKey('refs/tags/' + $Tag)) { throw 'Remote peeled tag lacks its actual tag ref.' }
    $taskCommit = $taskRefs['refs/tags/' + $Tag]
    if ($taskRefs.ContainsKey('refs/tags/' + $Tag + '^{}')) { $taskCommit = $taskRefs['refs/tags/' + $Tag + '^{}'] }
    if ($taskCommit -cne $Source) { throw 'Existing remote tag points to a different source; immutable tags cannot move.' }
    return $true
}
function Get-ReleaseMissingAsset {
    param([object[]]$Assets, [string[]]$Files)
    $taskWanted = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal)
    $taskPresent = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($taskPath in $Files) {
        $taskName = Split-Path -Leaf $taskPath
        if ($taskWanted.ContainsKey($taskName)) { throw 'Duplicate local release asset name.' }
        $taskWanted[$taskName] = $taskPath
    }
    foreach ($taskAsset in $Assets) {
        if (-not $taskWanted.ContainsKey($taskAsset.name) -or -not $taskPresent.Add($taskAsset.name)) { throw 'Existing draft has an unknown or duplicate asset; do not overwrite it.' }
    }
    foreach ($taskPath in $Files) { if (-not $taskPresent.Contains((Split-Path -Leaf $taskPath))) { Write-Output $taskPath } }
}
function Remove-ReleasePrivateDirectory {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Internal owner-only cleanup must run after explicit execution and on failure; every path is validated before removal.')]
    param([string]$Path, [string]$Root)
    $taskPath = Assert-ReleaseOwnedPath -Path $Path -Root $Root
    if (Test-Path -LiteralPath $taskPath) {
        # Examine every entry before recursive removal, including junctions.
        $taskPending = [Collections.Generic.Queue[string]]::new(); $taskPending.Enqueue($taskPath)
        while ($taskPending.Count) {
            $taskItem = Get-Item -LiteralPath $taskPending.Dequeue() -Force
            if (($taskItem.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) { throw 'Cannot clean a reparsed private directory.' }
            if ($taskItem.PSIsContainer) { foreach ($taskChild in Get-ChildItem -LiteralPath $taskItem.FullName -Force) { $taskPending.Enqueue($taskChild.FullName) } }
        }
        Remove-Item -LiteralPath $taskPath -Recurse -Force
    }
    if (Test-Path -LiteralPath $taskPath) { throw 'Private release directory remains after cleanup.' }
}
function ConvertTo-ReleaseGate {
    param([string]$ID, $Bound, [string]$Stem, [string]$Root, [string]$OutputStem = '')
    if (-not $OutputStem) { $OutputStem = $Stem }
    return [ordered]@{ id = $ID; program = [ordered]@{ path = $Bound.launch.archiveEntry; bytes = $Bound.launch.bytes; sha256 = $Bound.launch.sha256 }; reportFile = Get-ReleaseFile ($Stem + '.json') $Root; stdoutFile = Get-ReleaseFile ($OutputStem + '.stdout.log') $Root; stderrFile = Get-ReleaseFile ($OutputStem + '.stderr.log') $Root; result = $Bound.report; launch = $Bound.launch }
}
