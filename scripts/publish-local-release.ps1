[CmdletBinding()]
param(
    [ValidateSet('Plan','Prepare','Validate','Publish')][string]$Mode = 'Plan',
    [ValidatePattern('^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')][string]$Version,
    [ValidatePattern('^[0-9a-f]{40}$')][string]$Source,
    [string]$ReleaseDirectory,
    [string]$SigningKeyPath,
    [string]$PriorArchive,
    [string]$PriorManifest,
    [string]$PriorMSI,
    [string]$ReleaseNotes
)
. (Join-Path $PSScriptRoot 'local-release-functions.ps1')

function Assert-ReleaseVersion {
    param([string]$Requested, [string]$RepositoryVersion)
    if ($RepositoryVersion -cnotmatch '^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$' -or $Requested -cne $RepositoryVersion) { throw 'Release version must exactly match the clean repository VERSION.' }
}

# Dot-sourcing loads pure parser/helpers for CPU tests and executes no commands.
if ($MyInvocation.InvocationName -eq '.') { return }
if (-not [Environment]::Is64BitOperatingSystem -or -not [Environment]::Is64BitProcess) { throw 'Local publication requires native 64-bit Windows PowerShell.' }
$taskRepositoryVersion = (Get-Content -LiteralPath (Join-Path $script:ReleaseProject 'VERSION') -Raw).Trim()
if (-not $Version) { $Version = $taskRepositoryVersion }
if ($Mode -in @('Prepare', 'Validate', 'Publish')) { Assert-ReleaseVersion $Version $taskRepositoryVersion }
if (-not $Source) { throw 'Supply the exact immutable source commit, including for Plan.' }
if (-not $ReleaseDirectory) { $ReleaseDirectory = Join-Path $script:ReleaseProject ('.cache/local-release/' + $Version + '-' + $Source.Substring(0, 12)) }
$taskRoot = Assert-ReleaseOwnedPath -Path $ReleaseDirectory -Root (Join-Path $script:ReleaseProject '.cache')
$taskResumePrepared = $Mode -eq 'Validate' -and (Test-Path -LiteralPath (Join-Path $taskRoot 'prepared-candidate.json'))
if ($Mode -notin @('Publish','Validate') -and (Test-Path -LiteralPath $taskRoot)) { throw 'Use a fresh owned release directory; existing success/failure evidence is immutable.' }
if ($Mode -eq 'Validate' -and (Test-Path -LiteralPath $taskRoot) -and -not $taskResumePrepared) { throw 'Validate may resume only an exact prepared-candidate, never an incomplete native/signing attempt.' }
if ($taskResumePrepared) {
    foreach ($taskPrevious in @('incomplete.json','staged.json','receipt.pending.json','receipt.json','channels/update-manifest.json','private-tmp','private-config','private-stage')) {
        if (Test-Path -LiteralPath (Join-Path $taskRoot $taskPrevious)) { throw 'Prepared continuation requires preserved unsigned evidence and actual prior private-directory cleanup.' }
    }
}
if ($Mode -eq 'Validate') {
    foreach ($taskInput in @($SigningKeyPath, $PriorArchive, $PriorManifest, $PriorMSI)) { if (-not $taskInput -or -not (Test-Path -LiteralPath $taskInput -PathType Leaf)) { throw 'Validate needs an existing production key and genuine prior ZIP/envelope/MSI.' } }
    $SigningKeyPath = Assert-ReleaseSigningKeyPath $SigningKeyPath
}
New-Item -ItemType Directory -Path $taskRoot -Force | Out-Null
$taskTemporary = Join-Path $taskRoot 'private-tmp'; $taskConfig = Join-Path $taskRoot 'private-config'; $taskStage = Join-Path $taskRoot 'private-stage'; $taskLogs = Join-Path $taskRoot 'logs'
New-Item -ItemType Directory -Path $taskLogs -Force | Out-Null
$taskTelemetry = Join-Path $taskConfig 'go/telemetry/mode'
$taskTelemetryHash = ''
if ($Mode -ne 'Publish') {
    New-Item -ItemType Directory -Path $taskTemporary, (Split-Path -Parent $taskTelemetry) -Force | Out-Null
    [IO.File]::WriteAllText($taskTelemetry, ('off ' + [DateTime]::UtcNow.ToString('yyyy-MM-dd')), $script:ReleaseUTF8)
    $taskTelemetryHash = (Get-FileHash -LiteralPath $taskTelemetry -Algorithm SHA256).Hash
    $taskEnvironment = Get-ReleaseEnvironment $taskRoot
} else {
    $taskEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($taskName in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskEnvironment[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
    $taskEnvironment['GOWORK'] = 'off'; $taskEnvironment['GOFLAGS'] = ''
}
if (-not ('GocodeReleaseProcess' -as [type])) { Add-Type -Path (Join-Path $PSScriptRoot 'local-release-process.cs') }
$taskPowerShell = Resolve-ReleaseCommand 'powershell.exe'
$taskGo = Resolve-ReleaseCommand 'go.exe'
$taskChecker = Join-Path $taskRoot 'gocode-localreleasecheck.exe'
$taskComplete = $false
try {
    if ($taskResumePrepared) {
        $taskPrepared = Read-ReleaseJSON (Join-Path $taskRoot 'prepared-candidate.json')
        Assert-ReleaseBoundFile $taskPrepared.checker $taskRoot -Nonempty
        if ($taskPrepared.checker.path -cne 'gocode-localreleasecheck.exe') { throw 'Continuation must use the original prepared checker identity.' }
    }
    if ($Mode -ne 'Publish' -and -not $taskResumePrepared) { $null = Invoke-ReleaseProcess @($taskGo, 'build', '-o', $taskChecker, './cmd/gocode-localreleasecheck') 120 (Join-Path $taskLogs 'build-checker') $taskEnvironment }
    if (-not (Test-Path -LiteralPath $taskChecker -PathType Leaf)) { throw 'The exact prepared release checker is absent.' }
    $taskCommon = @('-project', $script:ReleaseProject, '-version', $Version, '-source', $Source)
    $taskQueryPrefix = ''; if ($taskResumePrepared) { $taskQueryPrefix = 'resume-' }
    $taskPlanPath = Join-Path $taskRoot ($taskQueryPrefix + 'plan.json')
    $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'plan', '-output', $taskPlanPath) + $taskCommon) 30 (Join-Path $taskLogs ($taskQueryPrefix + 'plan')) $taskEnvironment
    $taskPlan = Read-ReleaseJSON $taskPlanPath; Assert-ReleasePlan $taskPlan
    if ($Mode -eq 'Plan') { $taskPlan | ConvertTo-Json -Depth 12; return }
    $taskReceiptPath = Join-Path $taskRoot 'receipt.json'
    if ($Mode -eq 'Publish') {
        if (-not $ReleaseNotes -or -not (Test-Path -LiteralPath $ReleaseNotes -PathType Leaf)) { throw 'Publish needs reviewed release notes.' }
        $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'verify', '-root', $taskRoot, '-receipt', $taskReceiptPath) + $taskCommon) 120 (Join-Path $taskLogs 'pre-publish-verify') $taskEnvironment
        $taskReceipt = Read-ReleaseJSON $taskReceiptPath
        $taskGH = Resolve-ReleaseCommand 'gh.exe'; $taskGit = Resolve-ReleaseCommand 'git.exe'
        $taskTag = 'v' + $Version
        $taskRepository = 'https://github.com/neko233-com/gocode.git'
        $taskRemote = Invoke-ReleaseProcess @($taskGit, 'ls-remote', '--tags', $taskRepository, ('refs/tags/' + $taskTag), ('refs/tags/' + $taskTag + '^{}')) 120 (Join-Path $taskLogs 'remote-tag') $taskEnvironment
        $taskRemoteExists = Test-ReleaseRemoteTagBinding ($script:ReleaseUTF8.GetString($taskRemote.Stdout)) $taskTag $Source
        $taskLocal = Invoke-ReleaseProcess @($taskGit, 'tag', '--list', $taskTag) 30 (Join-Path $taskLogs 'local-tag') $taskEnvironment
        if ($script:ReleaseUTF8.GetString($taskLocal.Stdout).Trim()) {
            $taskLocalCommit = Invoke-ReleaseProcess @($taskGit, 'rev-parse', ($taskTag + '^{commit}')) 30 (Join-Path $taskLogs 'local-tag-source') $taskEnvironment
            if ($script:ReleaseUTF8.GetString($taskLocalCommit.Stdout).Trim() -cne $Source) { throw 'Existing local tag belongs to a different immutable source.' }
        } elseif (-not $taskRemoteExists) {
            $null = Invoke-ReleaseProcess @($taskGit, 'tag', '-a', $taskTag, $Source, '-m', ('gocode ' + $Version)) 30 (Join-Path $taskLogs 'create-tag') $taskEnvironment
        }
        if (-not $taskRemoteExists) {
            $null = Invoke-ReleaseProcess @($taskGit, 'push', $taskRepository, ('refs/tags/' + $taskTag)) 120 (Join-Path $taskLogs 'push-tag') $taskEnvironment
            $taskRemote = Invoke-ReleaseProcess @($taskGit, 'ls-remote', '--tags', $taskRepository, ('refs/tags/' + $taskTag), ('refs/tags/' + $taskTag + '^{}')) 120 (Join-Path $taskLogs 'verify-tag') $taskEnvironment
            if (-not (Test-ReleaseRemoteTagBinding ($script:ReleaseUTF8.GetString($taskRemote.Stdout)) $taskTag $Source)) { throw 'Pushed immutable tag is absent.' }
        }
        $taskUpload = @($taskReceipt.artifacts | ForEach-Object { Join-Path $taskRoot $_.path }) + @((Join-Path $taskRoot $taskReceipt.envelope.path), $taskReceiptPath, (Join-Path $taskRoot 'channels/SHA256SUMS'), (Join-Path $taskRoot 'channels/install.ps1'), (Join-Path $taskRoot 'channels/release-platforms.json'))
        $taskReleases = Invoke-ReleaseProcess @($taskGH, 'release', 'list', '--repo', 'neko233-com/gocode', '--limit', '100', '--json', 'tagName,isDraft') 120 (Join-Path $taskLogs 'release-list') $taskEnvironment
        [GocodeReleaseJson]::Validate($script:ReleaseUTF8.GetString($taskReleases.Stdout))
        $taskFound = @($script:ReleaseUTF8.GetString($taskReleases.Stdout) | ConvertFrom-Json | Where-Object { $_.tagName -ceq $taskTag })
        if ($taskFound.Count -gt 1 -or ($taskFound.Count -eq 1 -and -not $taskFound[0].isDraft)) { throw 'An already published release is immutable; do not overwrite or republish it.' }
        if ($taskFound.Count -eq 0) {
            $null = Invoke-ReleaseProcess @($taskGH, 'release', 'create', $taskTag, '--draft', '--repo', 'neko233-com/gocode', '--verify-tag', '--target', $Source, '--title', ('gocode ' + $Version), '--notes-file', ([IO.Path]::GetFullPath($ReleaseNotes))) 120 (Join-Path $taskLogs 'create-draft') $taskEnvironment
        }
        $taskDraft = Invoke-ReleaseProcess @($taskGH, 'release', 'view', $taskTag, '--repo', 'neko233-com/gocode', '--json', 'tagName,isDraft,assets') 120 (Join-Path $taskLogs 'draft-state') $taskEnvironment
        [GocodeReleaseJson]::Validate($script:ReleaseUTF8.GetString($taskDraft.Stdout))
        $taskDraftState = $script:ReleaseUTF8.GetString($taskDraft.Stdout) | ConvertFrom-Json
        if (-not $taskDraftState.isDraft -or $taskDraftState.tagName -cne $taskTag) { throw 'Only this exact draft may resume upload.' }
        $taskMissing = @(Get-ReleaseMissingAsset @($taskDraftState.assets) $taskUpload)
        if ($taskMissing.Count) { $null = Invoke-ReleaseProcess (@($taskGH, 'release', 'upload', $taskTag, '--repo', 'neko233-com/gocode') + $taskMissing) 120 (Join-Path $taskLogs 'upload-draft-assets') $taskEnvironment }
        $taskDownloadRoot = Join-Path $taskRoot 'upload-verification'
        if (Test-Path -LiteralPath $taskDownloadRoot) { throw 'Preserved prior upload-verification evidence must be archived before a fresh remote check.' }
        New-Item -ItemType Directory -Path $taskDownloadRoot | Out-Null
        $taskDownloaded = @()
        foreach ($taskFile in $taskUpload) {
            $taskName = Split-Path -Leaf $taskFile
            $null = Invoke-ReleaseProcess @($taskGH, 'release', 'download', $taskTag, '--repo', 'neko233-com/gocode', '--pattern', $taskName, '--dir', $taskDownloadRoot) 120 (Join-Path $taskLogs ('download-' + $taskName)) $taskEnvironment
            $taskExpected = Get-ReleaseFile $taskFile $taskRoot; $taskActual = Get-ReleaseFile (Join-Path $taskDownloadRoot $taskName) $taskRoot
            if ($taskActual.bytes -ne $taskExpected.bytes -or $taskActual.sha256 -cne $taskExpected.sha256) { throw 'Downloaded draft asset differs from exact locally tested bytes; draft remains unpublished.' }
            $taskDownloaded += $taskActual
        }
        Write-ReleaseJSON (Join-Path $taskRoot 'publication-ready.json') ([ordered]@{ version = $Version; source = $Source; tag = $taskTag; allDraftAssetBytesVerified = $true; downloaded = $taskDownloaded })
        $null = Invoke-ReleaseProcess @($taskGH, 'release', 'edit', $taskTag, '--repo', 'neko233-com/gocode', '--draft=false', '--verify-tag') 120 (Join-Path $taskLogs 'publish-complete-draft') $taskEnvironment
        $taskComplete = $true
        return
    }
    $taskInputsPath = Join-Path $taskRoot ($taskQueryPrefix + 'inputs.json'); $taskFrameworkPath = Join-Path $taskRoot ($taskQueryPrefix + 'framework.json')
    $taskGit = Resolve-ReleaseCommand 'git.exe'
    $taskHead = Invoke-ReleaseProcess @($taskGit, 'rev-parse', 'HEAD') 30 (Join-Path $taskLogs ($taskQueryPrefix + 'exact-head')) $taskEnvironment
    $taskStatus = Invoke-ReleaseProcess @($taskGit, 'status', '--porcelain=v1', '--untracked-files=normal') 30 (Join-Path $taskLogs ($taskQueryPrefix + 'clean-source')) $taskEnvironment
    if ($script:ReleaseUTF8.GetString($taskHead.Stdout).Trim() -cne $Source -or $taskStatus.Stdout.Length -ne 0) { throw 'Validation requires the exact clean committed candidate.' }
    $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'inputs', '-output', $taskInputsPath) + $taskCommon) 30 (Join-Path $taskLogs ($taskQueryPrefix + 'inputs')) $taskEnvironment
    $taskDigest = (Read-ReleaseJSON $taskInputsPath).inputDigest
    $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'framework', '-output', $taskFrameworkPath) + $taskCommon) 120 (Join-Path $taskLogs ($taskQueryPrefix + 'framework')) $taskEnvironment
    $taskChecks = @(); $taskGates = @()
    $taskPython = Resolve-ReleaseCommand 'python.exe'
    $taskCheckCommands = [ordered]@{
        'source-windows-amd64-repeat3' = @($taskPowerShell, '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'test-windows-amd64.ps1'), '-Repeat', '3')
        'source-no-cgo' = @($taskGo, 'test', '-p=1', '-shuffle=on', '-count=3', '-timeout=12m', './...')
        'source-vet' = @($taskGo, 'vet', './...')
        'workflow-lint' = @($taskPowerShell, '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'local-release-lint.ps1'))
        'distribution-tests' = @($taskPython, '-m', 'unittest', 'discover', '-s', 'scripts', '-p', 'generate_distribution_test.py')
    }
    if ($taskResumePrepared) {
        $taskPrepared = Read-ReleaseJSON (Join-Path $taskRoot 'prepared-candidate.json')
        Assert-ReleasePrepared $taskPrepared $taskRoot $Version $Source $taskDigest (Read-ReleaseJSON $taskFrameworkPath) $taskPlan $taskCheckCommands $taskChecker
        $taskChecks = @($taskPrepared.checks)
    }
    foreach ($taskID in $taskPlan.checks) {
        if (-not $taskCheckCommands.Contains($taskID)) { continue }
        if ($taskResumePrepared) { continue }
        $taskStem = Join-Path $taskLogs $taskID
        $taskCheckEnvironment = [Collections.Generic.Dictionary[string,string]]::new($taskEnvironment, [StringComparer]::OrdinalIgnoreCase)
        if ($taskID -eq 'source-no-cgo') { $taskCheckEnvironment['CGO_ENABLED'] = '0'; $taskCheckEnvironment['GOEXPERIMENT'] = '' }
        $taskDeadline = 120; if ($taskID -like 'source-*') { $taskDeadline = 1200 }
        $taskResult = Invoke-ReleaseProcess $taskCheckCommands[$taskID] $taskDeadline $taskStem $taskCheckEnvironment
        $taskProof = [ordered]@{ schema = 1; id = $taskID; version = $Version; source = $Source; command = $taskCheckCommands[$taskID]; pid = $taskResult.PID; exitCode = $taskResult.ExitCode; elapsedMS = $taskResult.ElapsedMS; rootReaped = $taskResult.RootReaped; treeClosed = $taskResult.TreeClosed; inputDigest = $taskDigest; stdout = Get-ReleaseFile ($taskStem + '.stdout.log') $taskRoot; stderr = Get-ReleaseFile ($taskStem + '.stderr.log') $taskRoot }
        Write-ReleaseJSON ($taskStem + '.json') $taskProof
        $taskChecks += [ordered]@{ id = $taskID; exitCode = 0; evidence = Get-ReleaseFile ($taskStem + '.json') $taskRoot }
    }
    $taskArtifacts = Join-Path $taskRoot 'artifacts'; $taskChannels = Join-Path $taskRoot 'channels'
    if (-not $taskResumePrepared) {
        $null = Invoke-ReleaseProcess @($taskPowerShell, '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'build-windows-package.ps1'), '-Version', $Version, '-OutputDirectory', $taskArtifacts, '-WorkDirectory', (Join-Path $taskTemporary 'package-work')) 1200 (Join-Path $taskLogs 'package') $taskEnvironment
        $null = Invoke-ReleaseProcess @($taskPython, (Join-Path $PSScriptRoot 'generate-distribution.py'), '--version', $Version, '--source', $Source, '--artifacts', $taskArtifacts, '--output', $taskChannels, '--platforms', 'windows/amd64') 120 (Join-Path $taskLogs 'generate-channels') $taskEnvironment
    }
    $taskArchive = Join-Path $taskArtifacts ('gocode-' + $Version + '-windows-amd64.zip'); $taskMSI = Join-Path $taskArtifacts ('gocode-' + $Version + '-windows-amd64.msi'); $taskEnvelope = Join-Path $taskChannels 'update-manifest.json'
    if ($Mode -eq 'Prepare') {
        Assert-ReleaseOrder @($taskChecks | ForEach-Object { $_.id }) @($taskPlan.checks | Select-Object -First 5)
        $taskChannelsProof = @(Get-ChildItem -LiteralPath $taskChannels -File -Recurse | Sort-Object FullName | ForEach-Object { Get-ReleaseFile $_.FullName $taskRoot })
        if ((Get-FileHash -LiteralPath $taskTelemetry -Algorithm SHA256).Hash -cne $taskTelemetryHash) { throw 'Private Go telemetry mode changed.' }
        $taskPrepared = [ordered]@{ schema=1; version=$Version; source=$Source; inputDigest=$taskDigest; framework=(Read-ReleaseJSON $taskFrameworkPath); checks=$taskChecks; artifacts=@((Get-ReleaseFile $taskMSI $taskRoot),(Get-ReleaseFile $taskArchive $taskRoot)); channels=$taskChannelsProof; checker=(Get-ReleaseFile $taskChecker $taskRoot); complete=$false; prepared=$true; needsExistingSigningKey=$true; privateTMPAbsent=$true; privateConfigAbsent=$true; stageAbsent=$true }
        foreach ($taskPrivate in @($taskStage,$taskTemporary,$taskConfig)) { Remove-ReleasePrivateDirectory $taskPrivate $taskRoot }
        Write-ReleaseJSON (Join-Path $taskRoot 'prepared-candidate.json') $taskPrepared
        $taskComplete=$true # preparation succeeded; the file remains complete=false.
        Write-Output 'Exact clean source/unsigned Windows artifacts prepared. This is not a signed or complete release; Validate requires the existing production key and consumes these same bytes.'
        return
    }
    $null = Invoke-ReleaseProcess @($taskGo, 'run', './cmd/gocode-manifest', '-key', $SigningKeyPath, '-input', (Join-Path $taskChannels 'manifest-input.json'), '-output', $taskEnvelope) 120 (Join-Path $taskLogs 'sign-envelope') $taskEnvironment
    New-Item -ItemType Directory -Path $taskStage | Out-Null
    $taskStagedPath = Join-Path $taskRoot 'staged.json'
    $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'stage', '-root', $taskStage, '-archive', $taskArchive, '-manifest', $taskEnvelope, '-output', $taskStagedPath) + $taskCommon) 120 (Join-Path $taskLogs 'stage') $taskEnvironment
    $taskApp = Join-Path $taskStage ('versions/' + $Version + '/gocode-app.exe')
    foreach ($taskLanguage in @('go', 'typescript')) { $null = Invoke-ReleaseProcess @($taskApp, '-install-language-extension', $taskLanguage, '-extensions-dir', (Join-Path $taskStage 'acceptance-extensions'), '-copilot=false', '-lsp=false') 120 (Join-Path $taskLogs ('install-' + $taskLanguage)) $taskEnvironment }
    foreach ($taskGate in $taskPlan.gates) {
        Set-ReleaseNativeOutput $taskEnvironment $taskStage $taskGate.id
        $taskStem = Join-Path $taskLogs ('gate-' + $taskGate.id)
        $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'gate', '-root', $taskStage, '-gate', $taskGate.id, '-output', ($taskStem + '.json')) + $taskCommon) 120 (Join-Path $taskLogs ('gate-owner-' + $taskGate.id)) $taskEnvironment
        $taskGates += ConvertTo-ReleaseGate $taskGate.id (Read-ReleaseJSON ($taskStem + '.json')) $taskStem $taskRoot
    }
    $taskMSIStem = Join-Path $taskLogs 'msi-install-upgrade-rollback-uninstall'
    Set-ReleaseNativeOutput $taskEnvironment $taskStage 'msi-installed'
    $taskMSICommand = @($taskPowerShell, '-NoProfile', '-ExecutionPolicy', 'Bypass', '-File', (Join-Path $PSScriptRoot 'test-candidate-msi.ps1'), '-CandidateMSI', $taskMSI, '-CandidateZIP', $taskArchive, '-PriorMSI', ([IO.Path]::GetFullPath($PriorMSI)), '-PriorZIP', ([IO.Path]::GetFullPath($PriorArchive)), '-Version', $Version, '-Source', $Source, '-ValidationDirectory', (Join-Path $taskStage 'msi-validation'))
    $taskMSIResult = Invoke-ReleaseProcess $taskMSICommand 1200 $taskMSIStem $taskEnvironment
    $taskFinalArtifacts = @(Get-ReleaseFile $taskMSI $taskRoot; Get-ReleaseFile $taskArchive $taskRoot)
    $taskMSIProof = [ordered]@{ schema = 1; id = 'msi-install-upgrade-rollback-uninstall'; version = $Version; source = $Source; command = $taskMSICommand; pid = $taskMSIResult.PID; exitCode = 0; elapsedMS = $taskMSIResult.ElapsedMS; rootReaped = $taskMSIResult.RootReaped; treeClosed = $taskMSIResult.TreeClosed; inputDigest = $taskDigest; stdout = Get-ReleaseFile ($taskMSIStem + '.stdout.log') $taskRoot; stderr = Get-ReleaseFile ($taskMSIStem + '.stderr.log') $taskRoot; packageInputs = $taskFinalArtifacts }
    Write-ReleaseJSON ($taskMSIStem + '.json') $taskMSIProof
    $taskChecks += [ordered]@{ id = 'msi-install-upgrade-rollback-uninstall'; exitCode = 0; evidence = Get-ReleaseFile ($taskMSIStem + '.json') $taskRoot }
    $taskPriorDir = Join-Path $taskRoot 'prior'; New-Item -ItemType Directory -Path $taskPriorDir | Out-Null
    $taskOldArchive = Join-Path $taskPriorDir (Split-Path -Leaf $PriorArchive); $taskOldEnvelope = Join-Path $taskPriorDir 'update-manifest.json'
    Copy-Item -LiteralPath $PriorArchive -Destination $taskOldArchive; Copy-Item -LiteralPath $PriorManifest -Destination $taskOldEnvelope
    $taskRollbackStem = Join-Path $taskLogs 'real-prior-release-rollback'
    $taskRollbackCommand = @($taskChecker, '-mode', 'rollback', '-root', $taskStage, '-receipt', $taskStagedPath, '-prior-archive', $taskOldArchive, '-prior-manifest', $taskOldEnvelope, '-output', ($taskRollbackStem + '.json')) + $taskCommon
    Set-ReleaseNativeOutput $taskEnvironment $taskStage 'prior-rollback'
    $taskRollbackResult = Invoke-ReleaseProcess $taskRollbackCommand 120 (Join-Path $taskLogs 'rollback-owner') $taskEnvironment
    $taskRollback = Read-ReleaseJSON ($taskRollbackStem + '.json')
    $taskRollback.archive = Get-ReleaseFile $taskOldArchive $taskRoot; $taskRollback.envelope = Get-ReleaseFile $taskOldEnvelope $taskRoot
    $taskRollback.native = ConvertTo-ReleaseGate 'prior-rollback' (Read-ReleaseJSON ($taskRollbackStem + '.native.json')) ($taskRollbackStem + '.native') $taskRoot $taskRollbackStem
    $taskRollbackProof = [ordered]@{ schema = 1; id = 'real-prior-release-rollback'; version = $Version; source = $Source; command = $taskRollbackCommand; pid = $taskRollbackResult.PID; exitCode = 0; elapsedMS = $taskRollbackResult.ElapsedMS; rootReaped = $taskRollbackResult.RootReaped; treeClosed = $taskRollbackResult.TreeClosed; inputDigest = $taskDigest; stdout = Get-ReleaseFile (Join-Path $taskLogs 'rollback-owner.stdout.log') $taskRoot; stderr = Get-ReleaseFile (Join-Path $taskLogs 'rollback-owner.stderr.log') $taskRoot; rollback = $taskRollback }
    Write-ReleaseJSON (Join-Path $taskLogs 'rollback-check.json') $taskRollbackProof
    $taskChecks += [ordered]@{ id = 'real-prior-release-rollback'; exitCode = 0; evidence = Get-ReleaseFile (Join-Path $taskLogs 'rollback-check.json') $taskRoot }
    Assert-ReleaseOrder @($taskChecks | ForEach-Object { $_.id }) @($taskPlan.checks)
    Assert-ReleaseOrder @($taskGates | ForEach-Object { $_.id }) @($taskPlan.gates | ForEach-Object { $_.id })
    if ((Get-FileHash -LiteralPath $taskTelemetry -Algorithm SHA256).Hash -cne $taskTelemetryHash) { throw 'Private Go telemetry mode changed.' }
    $taskNativeEvidence = Join-Path $taskStage '.cache/native-gates'
    if (Test-Path -LiteralPath $taskNativeEvidence) { Copy-Item -LiteralPath $taskNativeEvidence -Destination (Join-Path $taskRoot 'native-gates') -Recurse }
    Copy-Item -LiteralPath (Join-Path $taskStage 'msi-validation') -Destination (Join-Path $taskRoot 'msi-validation') -Recurse
    foreach ($taskPrivate in @($taskStage, $taskTemporary, $taskConfig)) { Remove-ReleasePrivateDirectory $taskPrivate $taskRoot }
    $taskReceipt = [ordered]@{ schema = 1; version = $Version; source = $Source; platforms = @($taskPlan.platforms); artifacts = $taskFinalArtifacts; envelope = Get-ReleaseFile $taskEnvelope $taskRoot; framework = Read-ReleaseJSON $taskFrameworkPath; checks = $taskChecks; gates = $taskGates; privateTMPAbsent = $true; privateConfigAbsent = $true; stageAbsent = $true; complete = $true; inputDigest = $taskDigest }
    $taskPendingReceipt = Join-Path $taskRoot 'receipt.pending.json'
    Write-ReleaseJSON $taskPendingReceipt $taskReceipt
    # Verification uses the caller environment after the three private roots
    # are gone. It never writes into a path whose absence is being proved.
    $taskVerifyEnvironment = [Collections.Generic.Dictionary[string,string]]::new([StringComparer]::OrdinalIgnoreCase)
    foreach ($taskName in [Environment]::GetEnvironmentVariables('Process').Keys) { $taskVerifyEnvironment[$taskName] = [Environment]::GetEnvironmentVariable($taskName, 'Process') }
    $taskVerifyEnvironment['GOWORK'] = 'off'; $taskVerifyEnvironment['GOFLAGS'] = ''
    $null = Invoke-ReleaseProcess (@($taskChecker, '-mode', 'verify', '-root', $taskRoot, '-receipt', $taskPendingReceipt) + $taskCommon) 120 (Join-Path $taskLogs 'final-verify') $taskVerifyEnvironment
    Move-Item -LiteralPath $taskPendingReceipt -Destination $taskReceiptPath
    $taskComplete = $true
    Write-Output "Locally validated immutable Windows release: $taskReceiptPath. Publish is a separate explicit invocation."
} finally {
    $taskCleanupErrors = @()
    if (-not $taskComplete -and $Mode -ne 'Plan') { Write-ReleaseJSON (Join-Path $taskRoot 'incomplete.json') ([ordered]@{ version = $Version; source = $Source; complete = $false; mode = $Mode }) }
    if ($Mode -ne 'Publish') {
        if (Test-Path -LiteralPath $taskTelemetry) {
            if ((Get-FileHash -LiteralPath $taskTelemetry -Algorithm SHA256).Hash -cne $taskTelemetryHash) { $taskCleanupErrors += 'Private Go telemetry baseline changed.' }
        }
        foreach ($taskPrivate in @($taskStage, $taskTemporary, $taskConfig)) { try { Remove-ReleasePrivateDirectory $taskPrivate $taskRoot } catch { $taskCleanupErrors += $_.Exception.Message } }
    }
    if ($taskCleanupErrors.Count) { throw ($taskCleanupErrors -join '; ') }
}
