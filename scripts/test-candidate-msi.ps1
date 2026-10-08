[CmdletBinding()]
param(
    [Parameter(Mandatory)][string]$CandidateMSI,
    [Parameter(Mandatory)][string]$CandidateZIP,
    [Parameter(Mandatory)][string]$PriorMSI,
    [Parameter(Mandatory)][string]$PriorZIP,
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version,
    [Parameter(Mandatory)][ValidatePattern('^[0-9a-f]{40}$')][string]$Source,
    [Parameter(Mandatory)][string]$ValidationDirectory,
    [switch]$PrepareOnly
)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'local-release-functions.ps1')
Add-Type -AssemblyName System.IO.Compression.FileSystem
if (-not ('GocodeReleaseProcess' -as [type])) { Add-Type -Path (Join-Path $PSScriptRoot 'local-release-process.cs') }

function Invoke-CandidateMSIObject {
    param($Object, [string]$Method, [object[]]$Arguments = @())
    $taskNativeArguments = @($Arguments | ForEach-Object { if ($null -eq $_) { $null } else { $_.PSObject.BaseObject } })
    try { return $Object.GetType().InvokeMember($Method, 'InvokeMethod', $null, $Object.PSObject.BaseObject, $taskNativeArguments) }
    catch { throw "MSI COM $Method argumentTypes=$((@($Arguments | ForEach-Object { if ($null -eq $_) { 'null' } else { $_.GetType().FullName } })) -join ','): $($_.Exception.Message)" }
}
function Remove-CandidateMSIObject {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Releases only the local COM reference; it changes no installer registration or user state.')]
    param($Object)
    if ($null -ne $Object) { [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($Object) }
}
function Invoke-CandidateMSISQL {
    param($Database, [string]$SQL, [object[]]$Values = @())
    $taskView = Invoke-CandidateMSIObject $Database 'OpenView' @($SQL)
    $taskRecord = $null
    try {
        if ($Values.Count) {
            $taskRecord = Invoke-CandidateMSIObject $script:CandidateInstaller 'CreateRecord' @($Values.Count)
            for ($taskIndex = 0; $taskIndex -lt $Values.Count; $taskIndex++) {
                if ($null -eq $Values[$taskIndex]) { continue }
                $taskField = [int]($taskIndex + 1)
                if ($Values[$taskIndex] -is [int]) {
                    [void]$taskRecord.GetType().InvokeMember('IntegerData', 'SetProperty', $null, $taskRecord.PSObject.BaseObject, [object[]]@([int]$taskField, [int]$Values[$taskIndex]))
                } else {
                    [void]$taskRecord.GetType().InvokeMember('StringData', 'SetProperty', $null, $taskRecord.PSObject.BaseObject, [object[]]@([int]$taskField, [string]$Values[$taskIndex]))
                }
            }
        }
        [void](Invoke-CandidateMSIObject $taskView 'Execute' @($taskRecord))
    } finally { [void](Invoke-CandidateMSIObject $taskView 'Close'); Remove-CandidateMSIObject $taskRecord; Remove-CandidateMSIObject $taskView }
}
function Get-CandidateMSIRow {
    param($Database, [string]$SQL, [int]$Columns)
    $taskView = Invoke-CandidateMSIObject $Database 'OpenView' @($SQL)
    $taskRows = [Collections.Generic.List[object]]::new()
    try {
        [void](Invoke-CandidateMSIObject $taskView 'Execute' @($null))
        while ($null -ne ($taskRecord = Invoke-CandidateMSIObject $taskView 'Fetch')) {
            try {
                $taskRow = @()
                for ($taskIndex = 1; $taskIndex -le $Columns; $taskIndex++) { $taskRow += $taskRecord.GetType().InvokeMember('StringData', 'GetProperty', $null, $taskRecord, @($taskIndex)) }
                $taskRows.Add($taskRow)
                if ($taskRows.Count -gt 1000) { throw 'MSI table exceeds package bounds.' }
            } finally { Remove-CandidateMSIObject $taskRecord }
        }
    } finally { [void](Invoke-CandidateMSIObject $taskView 'Close'); Remove-CandidateMSIObject $taskView }
    return ,$taskRows
}
function Get-CandidateMSIGuid {
    param([string]$Identity)
    $taskHasher = [Security.Cryptography.SHA256]::Create()
    try { $taskHash = $taskHasher.ComputeHash($script:ReleaseUTF8.GetBytes($script:CandidateUpgrade + ':' + $Identity)) } finally { $taskHasher.Dispose() }
    $taskBytes = [byte[]]::new(16); [Array]::Copy($taskHash, $taskBytes, 16)
    return '{' + [Guid]::new($taskBytes).ToString().ToUpperInvariant() + '}'
}
function New-CandidateMSITransform {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification = 'Internal preparation writes only fresh owner-validated diagnostic copies; original MSI bytes and all registrations remain unchanged.')]
    param([string]$Original, [string]$Name)
    $taskModifiedPath = Join-Path $script:CandidateRoot ($Name + '-modified.msi')
    $taskTransform = Join-Path $script:CandidateRoot ($Name + '.mst')
    Copy-Item -LiteralPath $Original -Destination $taskModifiedPath
    $taskOriginal = Invoke-CandidateMSIObject $script:CandidateInstaller 'OpenDatabase' @($Original, 0)
    $taskModified = Invoke-CandidateMSIObject $script:CandidateInstaller 'OpenDatabase' @($taskModifiedPath, 1)
    try {
        $taskProperties = Get-CandidateMSIRow $taskOriginal 'SELECT `Property`,`Value` FROM `Property`' 2
        $taskVersion = Get-ReleaseMSIProperty $taskProperties 'ProductVersion'
        $taskOriginalUpgrade = Get-ReleaseMSIProperty $taskProperties 'UpgradeCode'
        $taskOriginalProduct = Get-ReleaseMSIProperty $taskProperties 'ProductCode'
        $taskOriginalState = $script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, [object[]]@([string]$taskOriginalProduct))
        if ($taskOriginalUpgrade -cne '{EED33BE1-E465-4A97-A598-A29F430F9324}') { throw 'MSI is not the genuine gocode product family.' }
        $taskProduct = Get-CandidateMSIGuid ('product:' + $taskVersion)
        foreach ($taskEntry in @(@('ProductCode', $taskProduct), @('UpgradeCode', $script:CandidateUpgrade), @('ProductName', $script:CandidateName))) {
            Invoke-CandidateMSISQL $taskModified 'UPDATE `Property` SET `Value` = ? WHERE `Property` = ?' @($taskEntry[1], $taskEntry[0])
        }
        $taskComponents = Get-CandidateMSIRow $taskOriginal 'SELECT `Component`,`ComponentId` FROM `Component`' 2
        foreach ($taskRow in $taskComponents) {
            if (-not $taskRow[1]) { throw 'Unexpected MSI component without identity.' }
            Invoke-CandidateMSISQL $taskModified 'UPDATE `Component` SET `ComponentId` = ? WHERE `Component` = ?' @((Get-CandidateMSIGuid ('component:' + $taskRow[1])), $taskRow[0])
        }
        foreach ($taskTable in @(@('Registry', 'Registry'), @('RegLocator', 'Signature_'))) {
            $taskRegistryRows = Get-CandidateMSIRow $taskOriginal ('SELECT `' + $taskTable[1] + '`,`Key` FROM `' + $taskTable[0] + '`') 2
            foreach ($taskRow in $taskRegistryRows) {
                if (-not $taskRow[1].StartsWith('Software\neko233-com\gocode\' + $taskOriginalUpgrade, [StringComparison]::OrdinalIgnoreCase)) { throw 'Unexpected MSI registry scope.' }
                Invoke-CandidateMSISQL $taskModified ('UPDATE `' + $taskTable[0] + '` SET `Key` = ? WHERE `' + $taskTable[1] + '` = ?') @($taskRow[1].Replace($taskOriginalUpgrade, $script:CandidateUpgrade), $taskRow[0])
            }
        }
        $taskUpgrades = Get-CandidateMSIRow $taskOriginal 'SELECT `UpgradeCode`,`VersionMin`,`VersionMax`,`Language`,`Attributes`,`Remove`,`ActionProperty` FROM `Upgrade`' 7
        if ($taskUpgrades.Count -ne 2) { throw 'Unexpected MSI upgrade rules.' }
        Invoke-CandidateMSISQL $taskModified 'DELETE FROM `Upgrade`'
        foreach ($taskRow in $taskUpgrades) {
            if ($taskRow[0] -cne $taskOriginalUpgrade) { throw 'MSI upgrade row targets another family.' }
            $taskValues = @($script:CandidateUpgrade)
            foreach ($taskIndex in 1..6) { if ($taskIndex -eq 4) { $taskValues += [int]$taskRow[$taskIndex] } elseif ($taskRow[$taskIndex] -eq '') { $taskValues += $null } else { $taskValues += $taskRow[$taskIndex] } }
            Invoke-CandidateMSISQL $taskModified 'INSERT INTO `Upgrade` (`UpgradeCode`,`VersionMin`,`VersionMax`,`Language`,`Attributes`,`Remove`,`ActionProperty`) VALUES (?,?,?,?,?,?,?)' $taskValues
        }
        Invoke-CandidateMSISQL $taskModified 'UPDATE `Directory` SET `DefaultDir` = ? WHERE `Directory` = ?' @($script:CandidateName, 'MenuFolder')
        Invoke-CandidateMSISQL $taskModified 'UPDATE `Directory` SET `DefaultDir` = ? WHERE `Directory` = ?' @($script:CandidateName, 'INSTALLDIR')
        Invoke-CandidateMSISQL $taskModified 'UPDATE `Shortcut` SET `Name` = ?' @($script:CandidateName)
        [void](Invoke-CandidateMSIObject $taskModified 'Commit')
        # Microsoft Gen.vbs: modified.GenerateTransform(original). Summary
        # validates language/product/full version equality, suppressing nothing.
        if (-not (Invoke-CandidateMSIObject $taskModified 'GenerateTransform' @($taskOriginal, $taskTransform))) { throw 'Isolated MSI transform was not generated.' }
        [void](Invoke-CandidateMSIObject $taskModified 'CreateTransformSummaryInfo' @($taskOriginal, $taskTransform, 0, 291))
        $taskAppliedPath = Join-Path $script:CandidateRoot ($Name + '-applied.msi')
        Copy-Item -LiteralPath $Original -Destination $taskAppliedPath
        $taskApplied = Invoke-CandidateMSIObject $script:CandidateInstaller 'OpenDatabase' @($taskAppliedPath, 1)
        try {
            [void](Invoke-CandidateMSIObject $taskApplied 'ApplyTransform' @($taskTransform, 0))
            $taskAppliedProperties = Get-CandidateMSIRow $taskApplied 'SELECT `Property`,`Value` FROM `Property`' 2
            foreach ($taskEntry in @(@('ProductCode', $taskProduct), @('UpgradeCode', $script:CandidateUpgrade), @('ProductName', $script:CandidateName), @('ProductVersion', $taskVersion))) {
                if ((Get-ReleaseMSIProperty $taskAppliedProperties $taskEntry[0]) -cne $taskEntry[1]) { throw 'Actual applied MST property differs from isolated plan.' }
            }
            # Read the applied transform, not just the SQL-modified template.
            # File payload metadata/order is unchanged; every component GUID
            # and nonfile integration key/shortcut is actually isolated.
            foreach ($taskQuery in @(
                @('SELECT `Component`,`ComponentId`,`Directory_`,`Attributes`,`Condition`,`KeyPath` FROM `Component` ORDER BY `Component`', 6),
                @('SELECT `Registry`,`Root`,`Key`,`Name`,`Value`,`Component_` FROM `Registry` ORDER BY `Registry`', 6),
                @('SELECT `Signature_`,`Root`,`Key`,`Name`,`Type` FROM `RegLocator` ORDER BY `Signature_`', 5),
                @('SELECT `Directory`,`Directory_Parent`,`DefaultDir` FROM `Directory` ORDER BY `Directory`', 3),
                @('SELECT `Shortcut`,`Directory_`,`Name`,`Component_`,`Target` FROM `Shortcut` ORDER BY `Shortcut`', 5),
                @('SELECT `UpgradeCode`,`VersionMin`,`VersionMax`,`Language`,`Attributes`,`Remove`,`ActionProperty` FROM `Upgrade` ORDER BY `ActionProperty`', 7)
            )) {
                $taskWantedRows = Get-CandidateMSIRow $taskModified $taskQuery[0] $taskQuery[1]
                $taskActualRows = Get-CandidateMSIRow $taskApplied $taskQuery[0] $taskQuery[1]
                if (($taskActualRows | ConvertTo-Json -Depth 8 -Compress) -cne ($taskWantedRows | ConvertTo-Json -Depth 8 -Compress)) { throw 'Actual MST integration rows do not match owner isolation.' }
            }
            $taskPayloadQuery = 'SELECT `File`,`Component_`,`FileName`,`FileSize`,`Version`,`Language`,`Attributes`,`Sequence` FROM `File` ORDER BY `Sequence`'
            $taskOriginalFiles = Get-CandidateMSIRow $taskOriginal $taskPayloadQuery 8
            $taskAppliedFiles = Get-CandidateMSIRow $taskApplied $taskPayloadQuery 8
            if (($taskAppliedFiles | ConvertTo-Json -Depth 8 -Compress) -cne ($taskOriginalFiles | ConvertTo-Json -Depth 8 -Compress)) { throw 'MST changed original payload file metadata.' }
            [void](Invoke-CandidateMSIObject $taskApplied 'Commit')
        } finally { Remove-CandidateMSIObject $taskApplied }
        return [ordered]@{ path = $taskTransform; version = $taskVersion; product = $taskProduct; original = $Original; originalProduct = $taskOriginalProduct; originalProductStateBefore = $taskOriginalState; appliedDatabase = $taskAppliedPath; appliedIsolationVerified = $true; originalFileMetadataUnchanged = $true }
    } finally { Remove-CandidateMSIObject $taskModified; Remove-CandidateMSIObject $taskOriginal }
}
function Assert-CandidateInstalledZIP {
    param([string]$Archive, [string]$Root)
    $taskZIP = [IO.Compression.ZipFile]::OpenRead($Archive)
    try {
        if ($taskZIP.Entries.Count -gt 1000) { throw 'Candidate ZIP entry bound exceeded.' }
        foreach ($taskEntry in $taskZIP.Entries) {
            if ($taskEntry.Name -eq '') { continue }
            $taskPath = Assert-ReleaseOwnedPath (Join-Path $Root $taskEntry.FullName.Replace('/', '\')) $Root
            $taskInstalled = Get-Item -LiteralPath $taskPath
            if ($taskInstalled.Length -ne $taskEntry.Length) { throw 'MSI installed payload size differs from original ZIP.' }
            $taskStream = $taskEntry.Open(); $taskHasher = [Security.Cryptography.SHA256]::Create()
            try { $taskWanted = [BitConverter]::ToString($taskHasher.ComputeHash($taskStream)).Replace('-', '').ToLowerInvariant() } finally { $taskStream.Dispose(); $taskHasher.Dispose() }
            if ((Get-FileHash -LiteralPath $taskPath -Algorithm SHA256).Hash.ToLowerInvariant() -cne $taskWanted) { throw 'MSI installed payload hash differs from original ZIP.' }
        }
    } finally { $taskZIP.Dispose() }
}
function Invoke-CandidateTransaction {
    param([string[]]$Arguments, [string]$Name, [switch]$ExpectedFailure)
    $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($taskKey in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskEnvironment[$taskKey] = [Environment]::GetEnvironmentVariable($taskKey, 'Process') }
    $taskMSIExec = Join-Path ([Environment]::GetFolderPath('System')) 'msiexec.exe'
    $taskResult = [GocodeReleaseProcess]::Run($taskMSIExec, $Arguments + @('/qn', '/norestart', '/L*v', (Join-Path $script:CandidateRoot ($Name + '.msi.log'))), $script:ReleaseProject, $taskEnvironment, 120000)
    Write-ReleaseJSON (Join-Path $script:CandidateRoot ($Name + '.process.json')) ([ordered]@{ pid = $taskResult.PID; exitCode = $taskResult.ExitCode; elapsedMS = $taskResult.ElapsedMS; rootReaped = $taskResult.RootReaped; treeClosed = $taskResult.TreeClosed; error = $taskResult.Error })
    if ($taskResult.Error -or -not $taskResult.RootReaped -or -not $taskResult.TreeClosed -or ($ExpectedFailure -and $taskResult.ExitCode -eq 0) -or (-not $ExpectedFailure -and $taskResult.ExitCode -ne 0)) { throw "Actual MSI transaction $Name failed its expected outcome." }
}

$script:CandidateRoot = Assert-ReleaseOwnedPath $ValidationDirectory (Join-Path $script:ReleaseProject '.cache')
if (Test-Path -LiteralPath $script:CandidateRoot) { throw 'Candidate installer validation requires a fresh owned directory.' }
New-Item -ItemType Directory -Path $script:CandidateRoot | Out-Null
$taskInputs = @($CandidateMSI, $CandidateZIP, $PriorMSI, $PriorZIP) | ForEach-Object { (Resolve-Path -LiteralPath $_).Path }
$CandidateMSI, $CandidateZIP, $PriorMSI, $PriorZIP = $taskInputs
$taskOriginalHashes = @($taskInputs | ForEach-Object { (Get-FileHash -LiteralPath $_ -Algorithm SHA256).Hash })
$script:CandidateName = 'gocode Candidate ' + [Guid]::NewGuid().ToString('N').Substring(0, 8)
$script:CandidateUpgrade = '{' + [Guid]::NewGuid().ToString().ToUpperInvariant() + '}'
$script:CandidateInstaller = New-Object -ComObject WindowsInstaller.Installer
$taskOriginalUserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$taskProducts = @(); $taskTransforms = @(); $taskSucceeded = $false
$taskTarget = Join-Path $script:CandidateRoot 'installed'; $taskWorkspace = Join-Path $script:CandidateRoot 'workspace'
try {
    $taskOld = New-CandidateMSITransform $PriorMSI 'prior'; $taskTransforms += $taskOld; $taskProducts += $taskOld.product
    $taskNew = New-CandidateMSITransform $CandidateMSI 'candidate'; $taskTransforms += $taskNew; $taskProducts += $taskNew.product
    if ($taskNew.version -cne $Version -or [version]$taskOld.version -ge [version]$Version) { throw 'Actual MSI versions do not describe an older-to-candidate upgrade.' }
    foreach ($taskTransform in @($taskOld, $taskNew)) {
        if ($script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, [object[]]@([string]$taskTransform.product)) -ge 1) { throw 'Fresh isolated MSI product is unexpectedly registered.' }
    }
    Write-ReleaseJSON (Join-Path $script:CandidateRoot 'prepared.json') ([ordered]@{ version = $Version; source = $Source; originalInputs = $taskInputs; originalSHA256 = $taskOriginalHashes; transforms = @($taskOld, $taskNew); productName = $script:CandidateName; upgradeCode = $script:CandidateUpgrade; installed = $false })
    if ($PrepareOnly) {
        foreach ($taskTransform in @($taskOld, $taskNew)) {
            $taskStateAfter = $script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, [object[]]@([string]$taskTransform.originalProduct))
            if ($taskStateAfter -ne $taskTransform.originalProductStateBefore) { throw 'File-only MSI preparation changed original registration state.' }
        }
        for ($taskIndex = 0; $taskIndex -lt $taskInputs.Count; $taskIndex++) { if ((Get-FileHash -LiteralPath $taskInputs[$taskIndex] -Algorithm SHA256).Hash -cne $taskOriginalHashes[$taskIndex]) { throw 'File-only MSI preparation changed an original package.' } }
        if ([Environment]::GetEnvironmentVariable('Path', 'User') -cne $taskOriginalUserPath) { throw 'File-only MSI preparation changed user PATH.' }
        Write-ReleaseJSON (Join-Path $script:CandidateRoot 'prepare-only.json') ([ordered]@{ version = $Version; source = $Source; originalInputSHA256 = $taskOriginalHashes; originalRegistrationsUnchanged = $true; isolatedProductsUnregistered = $true; userPathUnchanged = $true; noInstallerTransaction = $true; transforms = @($taskOld, $taskNew) })
        return
    }
    Invoke-CandidateTransaction @('/i', $PriorMSI, ('TRANSFORMS=' + $taskOld.path), 'MSINEWINSTANCE=1', ('INSTALLDIR=' + $taskTarget)) 'install-prior'
    Assert-CandidateInstalledZIP $PriorZIP $taskTarget
    New-Item -ItemType Directory -Path $taskWorkspace | Out-Null
    $taskSourceFile = Join-Path $taskWorkspace 'main.go'; [IO.File]::WriteAllText($taskSourceFile, "package main`r`n// preserved 用户 😀`r`n", $script:ReleaseUTF8)
    $taskSourceHash = (Get-FileHash -LiteralPath $taskSourceFile -Algorithm SHA256).Hash
    # Only a diagnostic byte-copy is corrupted. The actual candidate MSI and
    # embedded cabinet remain byte-exact and are used for successful upgrade.
    $taskBad = Join-Path $script:CandidateRoot 'bad-candidate.msi'; Copy-Item -LiteralPath $CandidateMSI -Destination $taskBad
    $taskBadCab = Join-Path $script:CandidateRoot 'bad.cab'; [IO.File]::WriteAllText($taskBadCab, 'intentionally invalid cabinet', $script:ReleaseUTF8)
    $taskDatabase = Invoke-CandidateMSIObject $script:CandidateInstaller 'OpenDatabase' @($taskBad, 1)
    $taskView = Invoke-CandidateMSIObject $taskDatabase 'OpenView' @('SELECT `Name`,`Data` FROM `_Streams` WHERE `Name` = ''payload.cab''')
    $taskRecord = $null
    try {
        [void](Invoke-CandidateMSIObject $taskView 'Execute' @($null)); $taskRecord = Invoke-CandidateMSIObject $taskView 'Fetch'
        [void](Invoke-CandidateMSIObject $taskRecord 'SetStream' @(2, $taskBadCab)); [void](Invoke-CandidateMSIObject $taskView 'Modify' @(2, $taskRecord)); [void](Invoke-CandidateMSIObject $taskDatabase 'Commit')
    } finally { [void](Invoke-CandidateMSIObject $taskView 'Close'); Remove-CandidateMSIObject $taskRecord; Remove-CandidateMSIObject $taskView; Remove-CandidateMSIObject $taskDatabase }
    Invoke-CandidateTransaction @('/i', $taskBad, ('TRANSFORMS=' + $taskNew.path), 'MSINEWINSTANCE=1') 'corrupted-upgrade' -ExpectedFailure
    Assert-CandidateInstalledZIP $PriorZIP $taskTarget
    if ($script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskOld.product)) -ne 5) { throw 'Corrupted transaction damaged prior registration.' }
    Invoke-CandidateTransaction @('/i', $CandidateMSI, ('TRANSFORMS=' + $taskNew.path), 'MSINEWINSTANCE=1') 'upgrade-candidate'
    Assert-CandidateInstalledZIP $CandidateZIP $taskTarget
    if ($script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskOld.product)) -ge 1 -or $script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskNew.product)) -ne 5) { throw 'Actual upgrade did not replace the isolated prior registration.' }
    $taskSelected = Read-ReleaseJSON (Join-Path $taskTarget 'base.json')
    if ($taskSelected.version -cne $Version -or $taskSelected.source -cne $Source) { throw 'Actual candidate MSI payload source/version differs.' }
    $taskShortcuts = @((Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) ($script:CandidateName + '.lnk')), (Join-Path ([Environment]::GetFolderPath('Programs')) ($script:CandidateName + '/' + $script:CandidateName + '.lnk')))
    foreach ($taskShortcut in $taskShortcuts) {
        if (-not (Test-Path -LiteralPath $taskShortcut)) { throw 'Isolated MSI shortcut missing.' }
        $taskShell = New-Object -ComObject WScript.Shell
        $taskLink = $null
        try { $taskLink = $taskShell.CreateShortcut($taskShortcut); if ($taskLink.TargetPath -ne (Join-Path $taskTarget 'gocode-launch.exe') -or -not $taskLink.IconLocation) { throw 'Candidate shortcut does not select owned native launcher/icon.' } } finally { Remove-CandidateMSIObject $taskLink; Remove-CandidateMSIObject $taskShell }
    }
    Invoke-CandidateTransaction @('/i', $PriorMSI, ('TRANSFORMS=' + $taskOld.path), 'MSINEWINSTANCE=1') 'downgrade' -ExpectedFailure
    Assert-CandidateInstalledZIP $CandidateZIP $taskTarget
    if ($script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskNew.product)) -ne 5) { throw 'Rejected downgrade damaged candidate registration.' }
    $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($taskKey in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskEnvironment[$taskKey] = [Environment]::GetEnvironmentVariable($taskKey, 'Process') }
    $taskNative = [GocodeReleaseProcess]::Run((Join-Path $taskTarget ('versions/' + $Version + '/gocode-app.exe')), @('-smoke', '-workspace', $taskWorkspace, '-extensions-dir', (Join-Path $script:CandidateRoot 'extensions'), '-copilot=false', '-lsp=false'), $script:ReleaseProject, $taskEnvironment, 60000)
    [IO.File]::WriteAllBytes((Join-Path $script:CandidateRoot 'installed-native.stdout.log'), $taskNative.Stdout)
    [IO.File]::WriteAllBytes((Join-Path $script:CandidateRoot 'installed-native.stderr.log'), $taskNative.Stderr)
    if ($taskNative.Error -or $taskNative.ExitCode -ne 0 -or -not $taskNative.RootReaped -or -not $taskNative.TreeClosed -or $script:ReleaseUTF8.GetString($taskNative.Stdout) -notmatch 'gocode smoke passed: native rendering \+ installed VSIX activation \+ command execution') { throw 'Actual MSI-installed native app acceptance failed.' }
    Invoke-CandidateTransaction @('/x', $taskNew.product) 'uninstall-candidate'
    foreach ($taskShortcut in $taskShortcuts) { if (Test-Path -LiteralPath $taskShortcut) { throw 'Candidate uninstall retained an isolated shortcut.' } }
    if ((Test-Path -LiteralPath (Join-Path $taskTarget 'gocode.exe')) -or (Test-Path -LiteralPath (Join-Path $taskTarget 'current.json'))) { throw 'Candidate uninstall retained its registered launcher/update pointer.' }
    if ((Get-FileHash -LiteralPath $taskSourceFile -Algorithm SHA256).Hash -cne $taskSourceHash) { throw 'Installer changed the owned user workspace.' }
    $taskSucceeded = $true
} finally {
    $taskCleanupErrors = @(); $taskCleanupIndex = 0
    if (-not $PrepareOnly) {
        foreach ($taskProduct in $taskProducts) {
            try {
                $taskCleanupIndex++
                $taskState = $script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskProduct))
                if ($taskState -ge 1) { Invoke-CandidateTransaction @('/x', $taskProduct, ('INSTALLDIR=' + $taskTarget)) ('cleanup-product-' + $taskCleanupIndex) }
                if ($script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, @($taskProduct)) -ge 1) { throw 'Isolated MSI product remains registered after cleanup.' }
            } catch { $taskCleanupErrors += $_.Exception.Message }
        }
    }
    foreach ($taskTransform in $taskTransforms) {
        try {
            $taskStateAfter = $script:CandidateInstaller.GetType().InvokeMember('ProductState', 'GetProperty', $null, $script:CandidateInstaller, [object[]]@([string]$taskTransform.originalProduct))
            if ($taskStateAfter -ne $taskTransform.originalProductStateBefore) { throw 'Candidate MSI lifecycle changed an original product registration.' }
        } catch { $taskCleanupErrors += $_.Exception.Message }
    }
    try { Remove-CandidateMSIObject $script:CandidateInstaller } catch { $taskCleanupErrors += $_.Exception.Message }
    for ($taskIndex = 0; $taskIndex -lt $taskInputs.Count; $taskIndex++) {
        try { if ((Get-FileHash -LiteralPath $taskInputs[$taskIndex] -Algorithm SHA256).Hash -cne $taskOriginalHashes[$taskIndex]) { throw 'Original final/prior package bytes changed during MSI acceptance.' } } catch { $taskCleanupErrors += $_.Exception.Message }
    }
    if ([Environment]::GetEnvironmentVariable('Path', 'User') -cne $taskOriginalUserPath) { $taskCleanupErrors += 'Candidate MSI lifecycle did not preserve the original per-user PATH.' }
    if ($taskCleanupErrors.Count) { throw ($taskCleanupErrors -join '; ') }
}
if ($taskSucceeded) {
    Write-ReleaseJSON (Join-Path $script:CandidateRoot 'current.json') ([ordered]@{ version = $Version; source = $Source; originalInputs = $taskInputs; originalSHA256 = $taskOriginalHashes; installedPayloadMatchesZIP = $true; genuineOriginalMSIUsed = $true; isolatedTransform = $true; installUpgradeRollbackDowngradeNativeUninstall = $true; userPathPreserved = $true; workspacePreserved = $true })
    Write-Output 'Candidate MSI acceptance passed: exact original MSI/ZIP bytes, isolated MST instance, real install/rollback/upgrade/downgrade/native/uninstall, PATH/workspace preserved.'
}
