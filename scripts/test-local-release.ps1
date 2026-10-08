[CmdletBinding()]
param([switch]$ProcessControls)
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
. (Join-Path $PSScriptRoot 'local-release-functions.ps1')
. (Join-Path $PSScriptRoot 'publish-local-release.ps1')
if (-not ('GocodeReleaseProcess' -as [type])) { Add-Type -Path (Join-Path $PSScriptRoot 'local-release-process.cs') }
$taskRoot = Join-Path $script:ReleaseProject '.cache/local-release-parser-tests'
$taskRoot = Assert-ReleaseOwnedPath $taskRoot (Join-Path $script:ReleaseProject '.cache')
New-Item -ItemType Directory -Path $taskRoot -Force | Out-Null
$taskCount = 0
function Assert-ReleaseControl {
    param([bool]$Condition, [string]$Name)
    if (-not $Condition) { throw "CPU release control failed: $Name" }
    $script:ReleaseControlCount++
}
function Assert-ReleaseRejected {
    param([scriptblock]$Action, [string]$Name)
    $taskRejected = $false
    try { & $Action } catch { $taskRejected = $true }
    Assert-ReleaseControl $taskRejected $Name
}
$script:ReleaseControlCount = 0
$taskValid = '{"platforms":["windows/amd64"],"checks":["first","second"],"gates":[{"id":"first","args":["-one"],"gui":false,"timeoutNanoseconds":60000000000},{"id":"second","args":["-two","1024"],"gui":true,"timeoutNanoseconds":120000000000}],"pendingCommands":null}'
$taskJSON = Join-Path $taskRoot 'plan.json'
try {
    Assert-ReleaseVersion '0.24.0' '0.24.0'
    Assert-ReleaseControl $true 'exact repository VERSION admitted'
    Assert-ReleaseRejected { Assert-ReleaseVersion '0.25.0' '0.24.0' } 'explicit version cannot relabel clean source'
    Assert-ReleaseRejected { Assert-ReleaseVersion '0.24.0' 'invalid' } 'malformed repository VERSION rejected'
    [IO.File]::WriteAllText($taskJSON, $taskValid, $script:ReleaseUTF8)
    $taskPlan = Read-ReleaseJSON $taskJSON; Assert-ReleasePlan $taskPlan
    Assert-ReleaseOrder @($taskPlan.gates | ForEach-Object { $_.id }) @('first', 'second')
    Assert-ReleaseControl $true 'valid lower-case generated JSON shape/null pending'
    foreach ($taskBad in @('{', '{"id":1,"id":2}', '{"id":1,"\u0069d":2}', '{"nested":{"id":1,"id":2}}', '[1,]', '{"id":01}', '{"id":NaN}', 'true false')) {
        [IO.File]::WriteAllText($taskJSON, $taskBad, $script:ReleaseUTF8)
        Assert-ReleaseRejected { Read-ReleaseJSON $taskJSON | Out-Null } ('malformed/duplicate JSON ' + $taskBad)
    }
    [IO.File]::WriteAllText($taskJSON, ('[' * 66 + '0' + ']' * 66), $script:ReleaseUTF8)
    Assert-ReleaseRejected { Read-ReleaseJSON $taskJSON | Out-Null } 'deep JSON bound'
    [IO.File]::WriteAllText($taskJSON, (' ' * 524289), $script:ReleaseUTF8)
    Assert-ReleaseRejected { Read-ReleaseJSON $taskJSON | Out-Null } '512KiB JSON bound'
    foreach ($taskBad in @($taskValid.Replace('"id":"second"', '"id":"first"'), $taskValid.Replace('"second"]', '"first"]'), $taskValid.Replace('60000000000', '121000000000'), $taskValid.Replace('"pendingCommands":null', '"pendingCommands":["unwired"]'), $taskValid.Replace('"windows/amd64"', '"darwin/arm64"'), $taskValid.Replace('"gui":false', '"gui":"false"'))) {
        [IO.File]::WriteAllText($taskJSON, $taskBad, $script:ReleaseUTF8)
        Assert-ReleaseRejected { Assert-ReleasePlan (Read-ReleaseJSON $taskJSON) } 'plan identity/bounds/type'
    }
    Assert-ReleaseRejected { Assert-ReleaseOrder @('first', 'first') @('first', 'second') } 'same-count wrong name'
    Assert-ReleaseRejected { Assert-ReleaseOrder @('second', 'first') @('first', 'second') } 'same-count reorder'
    Assert-ReleaseRejected { Assert-ReleaseOwnedPath (Join-Path $taskRoot '../escaped') $taskRoot | Out-Null } 'resolved root escape'
    $taskWork = Join-Path $taskRoot 'work'
    $taskPreparedWork = Initialize-ReleaseWorkDirectory $taskWork
    Assert-ReleaseControl ($taskPreparedWork -ceq $taskWork -and (Test-Path -LiteralPath $taskWork -PathType Container)) 'explicit new owned work directory'
    Assert-ReleaseControl ((Initialize-ReleaseWorkDirectory $taskWork) -ceq $taskWork) 'existing empty work directory'
    [IO.File]::WriteAllText((Join-Path $taskWork 'preserved.txt'), 'keep', $script:ReleaseUTF8)
    Assert-ReleaseRejected { Initialize-ReleaseWorkDirectory $taskWork | Out-Null } 'nonempty work directory rejected'
    Assert-ReleaseControl ([IO.File]::ReadAllText((Join-Path $taskWork 'preserved.txt')) -ceq 'keep') 'nonempty work contents preserved'
    Assert-ReleaseRejected { Initialize-ReleaseWorkDirectory (Join-Path $script:ReleaseProject 'user-workspace') | Out-Null } 'builder outside repo cache rejected'
    Remove-ReleasePrivateDirectory $taskWork $taskRoot
    $taskRows = [Collections.Generic.List[object]]::new(); $taskRows.Add([object[]]@('ProductVersion', '0.24.0')); $taskRows.Add([object[]]@('UpgradeCode', '{EED33BE1-E465-4A97-A598-A29F430F9324}'))
    Assert-ReleaseControl ((Get-ReleaseMSIProperty $taskRows 'ProductVersion') -ceq '0.24.0') 'PS5 MSI singleton row is a complete string'
    Assert-ReleaseRejected { Get-ReleaseMSIProperty $taskRows 'Absent' | Out-Null } 'MSI missing property'
    $taskRows.Add([object[]]@('ProductVersion', '0.23.0'))
    Assert-ReleaseRejected { Get-ReleaseMSIProperty $taskRows 'ProductVersion' | Out-Null } 'MSI duplicate property'
    Assert-ReleaseRejected { Assert-ReleaseSigningKeyPath (Join-Path (Split-Path -Parent $script:ReleaseProject) 'parent-key.pem') | Out-Null } 'key forbidden in parent checkout'
    $taskNativeEnvironment = Get-ReleaseEnvironment $taskRoot
    Assert-ReleaseControl ($taskNativeEnvironment['GODESKTOP_READBACK'] -ceq '1' -and $taskNativeEnvironment['GODESKTOP_TEST_INPUT_ISOLATION'] -ceq '1' -and -not $taskNativeEnvironment.ContainsKey('GOCODE_CONPTY_DIR')) 'actual native readback/isolation explicit with managed ConPTY'
    $taskSHA = 'a' * 40; $taskOtherSHA = 'b' * 40
    Assert-ReleaseControl (-not (Test-ReleaseRemoteTagBinding '' 'v0.24.0' $taskSHA)) 'absent remote tag'
    Assert-ReleaseControl (Test-ReleaseRemoteTagBinding ($taskSHA + "`trefs/tags/v0.24.0`n") 'v0.24.0' $taskSHA) 'exact lightweight remote tag source'
    Assert-ReleaseControl (Test-ReleaseRemoteTagBinding ($taskOtherSHA + "`trefs/tags/v0.24.0`n" + $taskSHA + "`trefs/tags/v0.24.0^{}`n") 'v0.24.0' $taskSHA) 'exact annotated peeled commit'
    Assert-ReleaseRejected { Test-ReleaseRemoteTagBinding ($taskOtherSHA + "`trefs/tags/v0.24.0`n") 'v0.24.0' $taskSHA | Out-Null } 'different remote source immutable'
    Assert-ReleaseRejected { Test-ReleaseRemoteTagBinding ($taskSHA + "`trefs/tags/v0.24.0^{}`n") 'v0.24.0' $taskSHA | Out-Null } 'orphan peeled remote ref'
    Assert-ReleaseRejected { Test-ReleaseRemoteTagBinding ($taskSHA + "`trefs/tags/v0.24.0`n" + $taskSHA + "`trefs/tags/v0.24.0`n") 'v0.24.0' $taskSHA | Out-Null } 'duplicate remote ref'
    $taskFiles = @((Join-Path $taskRoot 'first.zip'), (Join-Path $taskRoot 'second.msi'))
    $taskAssets = @([PSCustomObject]@{ name = 'first.zip' })
    $taskMissing = @(Get-ReleaseMissingAsset $taskAssets $taskFiles)
    Assert-ReleaseControl ($taskMissing.Count -eq 1 -and $taskMissing[0] -ceq $taskFiles[1]) 'matching draft resumes only missing assets'
    Assert-ReleaseRejected { Get-ReleaseMissingAsset @([PSCustomObject]@{ name = 'unknown.exe' }) $taskFiles | Out-Null } 'unknown draft asset'
    Assert-ReleaseRejected { Get-ReleaseMissingAsset @([PSCustomObject]@{ name = 'First.zip' }) $taskFiles | Out-Null } 'draft asset exact case'
    Assert-ReleaseRejected { Get-ReleaseMissingAsset @($taskAssets[0], $taskAssets[0]) $taskFiles | Out-Null } 'duplicate draft asset'
    Assert-ReleaseRejected { [GocodeReleaseProcess]::Quote("bad$([char]0)argument") | Out-Null } 'NUL argument'
    Assert-ReleaseControl ([GocodeReleaseProcess]::Quote('one "two"\') -ceq '"one \"two\"\\"') 'Windows backslash/quote convention'
    $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::Ordinal)
    $taskEnvironment['TMP'] = 'old'; $taskEnvironment['tMp'] = 'private'; $taskEnvironment['é'] = 'first'; $taskEnvironment['É'] = 'last'; $taskEnvironment['=C:'] = 'C:\owned'
    $taskBlock = [GocodeReleaseProcess]::EnvironmentBlock($taskEnvironment)
    Assert-ReleaseControl ($taskBlock.Contains('tMp=private') -and -not $taskBlock.Contains('TMP=old') -and $taskBlock.Contains('É=last') -and -not $taskBlock.Contains('é=first') -and $taskBlock.Contains('=C:=C:\owned')) 'actual OS-ordinal env override/pseudo-variable'
    $taskEnvironment['invalid=name'] = 'value'
    Assert-ReleaseRejected { [GocodeReleaseProcess]::EnvironmentBlock($taskEnvironment) | Out-Null } 'invalid env name'
    # Pure actual-file binding controls; placeholder bytes are deliberately not
    # claimed to be installers, a real checker or completed source/native tests.
    $taskPreparedRoot = Initialize-ReleaseWorkDirectory (Join-Path $taskRoot 'prepared')
    New-Item -ItemType Directory -Path (Join-Path $taskPreparedRoot 'logs'), (Join-Path $taskPreparedRoot 'artifacts'), (Join-Path $taskPreparedRoot 'channels') | Out-Null
    $taskPreparedCommands = [ordered]@{ first=@('tool','first'); second=@('tool','second'); third=@('tool','third'); fourth=@('tool','fourth'); fifth=@('tool','fifth') }
    $taskPreparedPlan = [PSCustomObject]@{ checks=@('first','second','third','fourth','fifth','msi','rollback') }
    $taskPreparedFramework = [PSCustomObject]@{ version='0.17.0'; source=('c'*40); verified=$true }
    $taskPreparedDigest = 'd'*64; $taskPreparedChecks=@()
    foreach ($taskID in $taskPreparedCommands.Keys) {
        $taskStem = Join-Path $taskPreparedRoot ('logs/'+$taskID)
        [IO.File]::WriteAllText(($taskStem+'.stdout.log'), 'bound output', $script:ReleaseUTF8)
        [IO.File]::WriteAllText(($taskStem+'.stderr.log'), '', $script:ReleaseUTF8)
        $taskCheck = [ordered]@{ schema=1; id=$taskID; version='0.24.0'; source=$taskSHA; inputDigest=$taskPreparedDigest; command=$taskPreparedCommands[$taskID]; pid=123; exitCode=0; elapsedMS=10; rootReaped=$true; treeClosed=$true; stdout=(Get-ReleaseFile ($taskStem+'.stdout.log') $taskPreparedRoot); stderr=(Get-ReleaseFile ($taskStem+'.stderr.log') $taskPreparedRoot) }
        Write-ReleaseJSON ($taskStem+'.json') $taskCheck
        $taskPreparedChecks += [ordered]@{ id=$taskID; exitCode=0; evidence=(Get-ReleaseFile ($taskStem+'.json') $taskPreparedRoot) }
    }
    foreach ($taskFile in @('artifacts/gocode-0.24.0-windows-amd64.msi','artifacts/gocode-0.24.0-windows-amd64.zip','channels/manifest-input.json','channels/SHA256SUMS','gocode-localreleasecheck.exe')) { [IO.File]::WriteAllText((Join-Path $taskPreparedRoot $taskFile),'CPU binding placeholder',$script:ReleaseUTF8) }
    $taskPreparedChecker=Join-Path $taskPreparedRoot 'gocode-localreleasecheck.exe'
    $taskPreparedMarker=[ordered]@{ schema=1; version='0.24.0'; source=$taskSHA; inputDigest=$taskPreparedDigest; framework=$taskPreparedFramework; checks=$taskPreparedChecks; artifacts=@((Get-ReleaseFile (Join-Path $taskPreparedRoot 'artifacts/gocode-0.24.0-windows-amd64.msi') $taskPreparedRoot),(Get-ReleaseFile (Join-Path $taskPreparedRoot 'artifacts/gocode-0.24.0-windows-amd64.zip') $taskPreparedRoot)); channels=@(Get-ChildItem -LiteralPath (Join-Path $taskPreparedRoot 'channels') -File | Sort-Object FullName | ForEach-Object {Get-ReleaseFile $_.FullName $taskPreparedRoot}); checker=(Get-ReleaseFile $taskPreparedChecker $taskPreparedRoot); complete=$false; prepared=$true; needsExistingSigningKey=$true; privateTMPAbsent=$true; privateConfigAbsent=$true; stageAbsent=$true }
    $taskMarkerPath=Join-Path $taskPreparedRoot 'prepared-candidate.json'; Write-ReleaseJSON $taskMarkerPath $taskPreparedMarker
    $taskValidatePrepared={param($Value) Assert-ReleasePrepared $Value $taskPreparedRoot '0.24.0' $taskSHA $taskPreparedDigest $taskPreparedFramework $taskPreparedPlan $taskPreparedCommands $taskPreparedChecker}
    & $taskValidatePrepared (Read-ReleaseJSON $taskMarkerPath)
    Assert-ReleaseControl $true 'exact prepared bytes and five ordered check bindings admitted'
    foreach ($taskProperty in @('complete','prepared','needsExistingSigningKey','privateTMPAbsent','privateConfigAbsent','stageAbsent')) {
        $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.$taskProperty=-not $taskBad.$taskProperty
        Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} ('prepared boolean '+$taskProperty)
    }
    foreach ($taskProperty in @('source','version','inputDigest')) {
        $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.$taskProperty='wrong'
        Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} ('prepared current identity '+$taskProperty)
    }
    $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.framework.verified=$false
    Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} 'prepared actual framework mismatch'
    $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.checks=@($taskBad.checks[1],$taskBad.checks[0],$taskBad.checks[2],$taskBad.checks[3],$taskBad.checks[4])
    Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} 'prepared same-count check reorder'
    $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.checks[1]=$taskBad.checks[0]
    Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} 'prepared same-count duplicate check'
    $taskFirstEvidence=Join-Path $taskPreparedRoot 'logs/first.json'; $taskFirstBytes=[IO.File]::ReadAllBytes($taskFirstEvidence)
    foreach ($taskProperty in @('source','command','pid','treeClosed')) {
        $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskEvidence=Read-ReleaseJSON $taskFirstEvidence
        switch($taskProperty){'source'{$taskEvidence.source=$taskOtherSHA};'command'{$taskEvidence.command=@('tool','other')};'pid'{$taskEvidence.pid=0};'treeClosed'{$taskEvidence.treeClosed=$false}}
        Write-ReleaseJSON $taskFirstEvidence $taskEvidence
        $taskBad.checks[0].evidence=Get-ReleaseFile $taskFirstEvidence $taskPreparedRoot
        Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} ('rehashed receipt cannot relabel '+$taskProperty)
        [IO.File]::WriteAllBytes($taskFirstEvidence,$taskFirstBytes)
    }
    foreach ($taskFile in @('logs/first.stdout.log','artifacts/gocode-0.24.0-windows-amd64.zip','channels/manifest-input.json','gocode-localreleasecheck.exe')) {
        $taskPath=Join-Path $taskPreparedRoot $taskFile; $taskBytes=[IO.File]::ReadAllBytes($taskPath)
        [IO.File]::AppendAllText($taskPath,'changed',$script:ReleaseUTF8)
        Assert-ReleaseRejected {& $taskValidatePrepared (Read-ReleaseJSON $taskMarkerPath)} ('actual prepared file mutation '+$taskFile)
        [IO.File]::WriteAllBytes($taskPath,$taskBytes)
    }
    $taskExtra=Join-Path $taskPreparedRoot 'channels/extra.json'; [IO.File]::WriteAllText($taskExtra,'extra',$script:ReleaseUTF8)
    Assert-ReleaseRejected {& $taskValidatePrepared (Read-ReleaseJSON $taskMarkerPath)} 'unbound added channel'
    Remove-Item -LiteralPath $taskExtra
    $taskMissing=Join-Path $taskPreparedRoot 'channels/SHA256SUMS'; $taskBytes=[IO.File]::ReadAllBytes($taskMissing); Remove-Item -LiteralPath $taskMissing
    Assert-ReleaseRejected {& $taskValidatePrepared (Read-ReleaseJSON $taskMarkerPath)} 'missing prepared channel'
    [IO.File]::WriteAllBytes($taskMissing,$taskBytes)
    $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.artifacts[0].path='../escape.msi'
    Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} 'prepared artifact escape'
    $taskBad=Read-ReleaseJSON $taskMarkerPath; $taskBad.checks[0].evidence.bytes=$null
    Assert-ReleaseRejected {& $taskValidatePrepared $taskBad} 'prepared null byte count'
    Remove-ReleasePrivateDirectory $taskPreparedRoot $taskRoot
    if ($ProcessControls) {
        # These children only exchange bytes/wait/spawn a console descendant;
        # they create no HWND, native app, GPU, MSI transaction or signing key.
        $taskChild = Join-Path $taskRoot 'console-child.ps1'
        $taskChildSource = @'
param([string]$Mode,[string]$Value,[string]$ChildPIDFile)
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
switch($Mode){
 'echo' { if([Console]::In.ReadToEnd() -ne ''){throw 'stdin is not EOF'};[Console]::Out.Write($Value);[Console]::Error.Write([Environment]::GetEnvironmentVariable('GOCODE_RELEASE_ENV'));exit 0 }
 'wait' { Start-Sleep -Seconds 30;exit 0 }
 'flood' { $block='x'*8192;for($i=0;$i-lt1024;$i++){[Console]::Out.Write($block)};exit 0 }
 'descendant' { $info=[Diagnostics.ProcessStartInfo]::new();$info.FileName=(Get-Command powershell.exe).Source;$info.Arguments='-NoProfile -Command "Start-Sleep -Seconds 30"';$info.UseShellExecute=$false;$info.CreateNoWindow=$true;$child=[Diagnostics.Process]::Start($info);[IO.File]::WriteAllText($ChildPIDFile,[string]$child.Id);exit 0 }
 default { throw 'unknown console test mode' }
}
'@
        [IO.File]::WriteAllText($taskChild, $taskChildSource, $script:ReleaseUTF8)
        $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
        foreach ($taskName in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskEnvironment[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
        $taskEnvironment['GOCODE_RELEASE_ENV'] = 'private Unicode 世界'
        $taskPowerShell = Resolve-ReleaseCommand 'powershell.exe'
        $taskMismatchedVersion='0.0.0'
        if ((Get-Content -LiteralPath (Join-Path $script:ReleaseProject 'VERSION') -Raw).Trim() -ceq $taskMismatchedVersion) { $taskMismatchedVersion='0.0.1' }
        $taskRejectedRoot=Join-Path $taskRoot 'rejected-version'
        $taskVersionResult=[GocodeReleaseProcess]::Run($taskPowerShell,@('-NoProfile','-ExecutionPolicy','Bypass','-File',(Join-Path $PSScriptRoot 'publish-local-release.ps1'),'-Mode','Prepare','-Version',$taskMismatchedVersion,'-Source',('a'*40),'-ReleaseDirectory',$taskRejectedRoot),$script:ReleaseProject,$taskEnvironment,10000)
        Assert-ReleaseControl ($taskVersionResult.Error -eq '' -and $taskVersionResult.ExitCode -ne 0 -and $taskVersionResult.RootReaped -and $taskVersionResult.TreeClosed -and -not (Test-Path -LiteralPath $taskRejectedRoot) -and $script:ReleaseUTF8.GetString($taskVersionResult.Stderr).Contains('Release version must exactly match')) 'actual publisher rejects version before directory creation/build'
        $taskValue = 'literal 世界😀 "quoted" $() `ticks \ trailing\'
        $taskResult = [GocodeReleaseProcess]::Run($taskPowerShell, @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $taskChild, '-Mode', 'echo', '-Value', $taskValue), $script:ReleaseProject, $taskEnvironment, 10000)
        Assert-ReleaseControl ($taskResult.Error -eq '' -and $taskResult.ExitCode -eq 0 -and $taskResult.PID -gt 0 -and $taskResult.JobAssignedBeforeResume -and $taskResult.RootReaped -and $taskResult.TreeClosed -and $script:ReleaseUTF8.GetString($taskResult.Stdout) -ceq $taskValue -and $script:ReleaseUTF8.GetString($taskResult.Stderr) -ceq 'private Unicode 世界') 'real child literal argv/env/EOF/output and owned reap'
        $taskResult = [GocodeReleaseProcess]::Run($taskPowerShell, @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $taskChild, '-Mode', 'wait'), $script:ReleaseProject, $taskEnvironment, 200)
        Assert-ReleaseControl ($taskResult.TimedOut -and $taskResult.RootReaped -and $taskResult.TreeClosed -and $taskResult.ElapsedMS -lt 6500 -and -not (Get-Process -Id $taskResult.PID -ErrorAction SilentlyContinue)) 'real deadline kill/reap'
        $taskResult = [GocodeReleaseProcess]::Run($taskPowerShell, @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $taskChild, '-Mode', 'flood'), $script:ReleaseProject, $taskEnvironment, 10000)
        Assert-ReleaseControl ($taskResult.OutputLimit -and $taskResult.Stdout.Length -eq 4194304 -and $taskResult.RootReaped -and $taskResult.TreeClosed) 'real flood admission/memory bound and joined readers'
        $taskPIDFile = Join-Path $taskRoot 'descendant.pid'
        $taskResult = [GocodeReleaseProcess]::Run($taskPowerShell, @('-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', $taskChild, '-Mode', 'descendant', '-ChildPIDFile', $taskPIDFile), $script:ReleaseProject, $taskEnvironment, 10000)
        $taskDescendantPID = [int][IO.File]::ReadAllText($taskPIDFile)
        Assert-ReleaseControl ($taskResult.Error -eq '' -and $taskResult.RootReaped -and $taskResult.TreeClosed -and -not (Get-Process -Id $taskDescendantPID -ErrorAction SilentlyContinue)) 'real descendant retirement before tree proof'
    }
    Write-Output "Local-release CPU parser/process controls passed: $script:ReleaseControlCount. Native/install/sign/upload were not executed."
} finally {
    foreach ($taskName in @('plan.json', 'console-child.ps1', 'descendant.pid')) { $taskFile = Join-Path $taskRoot $taskName; if (Test-Path -LiteralPath $taskFile) { Remove-Item -LiteralPath $taskFile -Force } }
    if (Test-Path -LiteralPath (Join-Path $taskRoot 'work')) { Remove-ReleasePrivateDirectory (Join-Path $taskRoot 'work') $taskRoot }
    if (Test-Path -LiteralPath (Join-Path $taskRoot 'prepared')) { Remove-ReleasePrivateDirectory (Join-Path $taskRoot 'prepared') $taskRoot }
    $taskCount = @(Get-ChildItem -LiteralPath $taskRoot -Force).Count
    if ($taskCount -ne 0) { throw 'CPU control left unexpected files in its fixed owned directory.' }
}
