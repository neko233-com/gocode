param([switch]$SkipBuild,[string]$ValidationDirectory='')
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
if(-not $ValidationDirectory){$ValidationDirectory=Join-Path $taskRoot ('.cache/msi-validation/'+[Guid]::NewGuid().ToString('N'))}
$taskValidation=[IO.Path]::GetFullPath($ValidationDirectory)
if(-not $taskValidation.StartsWith((Join-Path $taskRoot '.cache')+'\',[StringComparison]::OrdinalIgnoreCase)){throw 'Installer acceptance must use this checkout-owned cache directory.'}
New-Item -ItemType Directory -Path $taskValidation -Force|Out-Null
$taskName='gocode Test '+[Guid]::NewGuid().ToString('N').Substring(0,8)
$taskUpgrade=[Guid]::NewGuid()
$taskTarget=Join-Path $taskValidation 'installed'
$taskInstaller=New-Object -ComObject WindowsInstaller.Installer
function Read-MSIProperty([string]$Path,[string]$Name){
    $taskDB=$taskInstaller.GetType().InvokeMember('OpenDatabase','InvokeMethod',$null,$taskInstaller,@($Path,0))
    $taskView=$taskDB.GetType().InvokeMember('OpenView','InvokeMethod',$null,$taskDB,@(('SELECT `Value` FROM `Property` WHERE `Property` = '+"'"+$Name+"'")))
    try{[void]$taskView.GetType().InvokeMember('Execute','InvokeMethod',$null,$taskView,@($null));$taskRecord=$taskView.GetType().InvokeMember('Fetch','InvokeMethod',$null,$taskView,@());try{return $taskRecord.GetType().InvokeMember('StringData','GetProperty',$null,$taskRecord,@(1))}finally{[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskRecord)}}finally{[void]$taskView.GetType().InvokeMember('Close','InvokeMethod',$null,$taskView,@());[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskView);[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskDB)}
}
function Invoke-TestMSI([string]$Action,[string]$Package,[string]$Log,[string[]]$Extra=@()){
    $taskArguments=@($Action,('"'+$Package+'"'),'/qn','/norestart','/L*v',('"'+(Join-Path $taskValidation $Log)+'"'))+$Extra
    $taskProcess=Start-Process -FilePath msiexec.exe -ArgumentList $taskArguments -WindowStyle Hidden -Wait -PassThru
    return $taskProcess.ExitCode
}
$taskProducts=@()
Push-Location -LiteralPath $taskRoot
try{
    foreach($taskVersion in @('0.0.1','0.0.2')){
        $taskOutput=Join-Path $taskValidation $taskVersion
        if(-not $SkipBuild){& (Join-Path $PSScriptRoot 'build-windows-package.ps1') -Version $taskVersion -AllowDirty -SkipMSI -OutputDirectory $taskOutput | Out-Null}
        $taskStage=Get-ChildItem -LiteralPath (Join-Path $taskRoot '.cache') -Directory -Filter ('package-'+$taskVersion+'-*')|Sort-Object LastWriteTime -Descending|Select-Object -First 1
        if(-not $taskStage){throw 'Synthetic payload not found.'}
        $taskMSI=Join-Path $taskValidation ($taskVersion+'.msi')
        & (Join-Path $PSScriptRoot 'build-msi.ps1') -PayloadDirectory (Join-Path $taskStage.FullName 'payload') -Version $taskVersion -ProductName $taskName -UpgradeCode $taskUpgrade -OutputPath $taskMSI | Out-Null
        $taskProducts+=Read-MSIProperty $taskMSI 'ProductCode'
    }
    $taskOld=Join-Path $taskValidation '0.0.1.msi';$taskNew=Join-Path $taskValidation '0.0.2.msi'
    if((Invoke-TestMSI '/i' $taskOld 'install.log' @(('INSTALLDIR="'+$taskTarget+'"'))) -ne 0){throw 'MSI install failed.'}
    $taskVersion=& (Join-Path $taskTarget 'gocode.exe') -version
    if($LASTEXITCODE -ne 0 -or $taskVersion -notmatch '^gocode 0\.0\.1 '){throw 'Installed command/version failed.'}
    $taskShell=New-Object -ComObject WScript.Shell
    foreach($taskShortcutPath in @((Join-Path ([Environment]::GetFolderPath('DesktopDirectory')) ($taskName+'.lnk')),(Join-Path ([Environment]::GetFolderPath('Programs')) ($taskName+'/'+$taskName+'.lnk')))){
        if(-not(Test-Path -LiteralPath $taskShortcutPath)){throw 'Shortcut missing.'}
        $taskShortcut=$taskShell.CreateShortcut($taskShortcutPath)
        if($taskShortcut.TargetPath -ne (Join-Path $taskTarget 'gocode-launch.exe') -or -not $taskShortcut.IconLocation){throw 'Shortcut target/icon invalid.'}
    }
    $taskPointer=Get-Content -LiteralPath (Join-Path $taskTarget 'base.json') -Raw
    [IO.File]::WriteAllText((Join-Path $taskTarget 'current.json'),$taskPointer,[Text.UTF8Encoding]::new($false))
    # Corrupt only this disposable package's embedded cabinet, exercising the
    # real Windows Installer transaction while the old product remains usable.
    $taskBad=Join-Path $taskValidation 'bad-upgrade.msi';Copy-Item -LiteralPath $taskNew -Destination $taskBad
    $taskBadCab=Join-Path $taskValidation 'bad.cab';[IO.File]::WriteAllText($taskBadCab,'invalid cabinet',[Text.UTF8Encoding]::new($false))
    $taskDB=$taskInstaller.GetType().InvokeMember('OpenDatabase','InvokeMethod',$null,$taskInstaller,[object[]]@([string]$taskBad,[int]1))
    $taskView=$taskDB.GetType().InvokeMember('OpenView','InvokeMethod',$null,$taskDB,@('SELECT `Name`,`Data` FROM `_Streams` WHERE `Name` = ''payload.cab'''))
    try{[void]$taskView.GetType().InvokeMember('Execute','InvokeMethod',$null,$taskView,@($null));$taskRecord=$taskView.GetType().InvokeMember('Fetch','InvokeMethod',$null,$taskView,@());try{[void]$taskRecord.GetType().InvokeMember('SetStream','InvokeMethod',$null,$taskRecord,[object[]]@([int]2,[string]$taskBadCab));[void]$taskView.GetType().InvokeMember('Modify','InvokeMethod',$null,$taskView,[object[]]@([int]2,$taskRecord));[void]$taskDB.GetType().InvokeMember('Commit','InvokeMethod',$null,$taskDB,@())}finally{[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskRecord)}}finally{[void]$taskView.GetType().InvokeMember('Close','InvokeMethod',$null,$taskView,@());[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskView);[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskDB)}
    if((Invoke-TestMSI '/i' $taskBad 'rollback.log') -eq 0){throw 'Corrupted cabinet upgrade succeeded.'}
    $taskVersion=& (Join-Path $taskTarget 'gocode.exe') -version
    if($LASTEXITCODE -ne 0 -or $taskVersion -notmatch '^gocode 0\.0\.1 '){throw 'Installer rollback failed to restore the previous app.'}
    if($taskInstaller.GetType().InvokeMember('ProductState','GetProperty',$null,$taskInstaller,@($taskProducts[0])) -ne 5){throw 'Rollback damaged the previous product registration.'}
    if((Invoke-TestMSI '/i' $taskNew 'upgrade.log') -ne 0){throw 'MSI remembered-root upgrade failed.'}
    $taskVersion=& (Join-Path $taskTarget 'gocode.exe') -version
    if($LASTEXITCODE -ne 0 -or $taskVersion -notmatch '^gocode 0\.0\.2 '){throw 'Upgrade did not supersede old pointer.'}
    if((Invoke-TestMSI '/i' $taskOld 'downgrade.log') -eq 0){throw 'MSI downgrade was not blocked.'}
    $taskWorkspace=Join-Path $taskValidation 'workspace';New-Item -ItemType Directory -Path $taskWorkspace -Force|Out-Null
    [IO.File]::WriteAllText((Join-Path $taskWorkspace 'main.go'),"package main`nfunc main() {}`n",[Text.UTF8Encoding]::new($false))
    $taskSavedReadback=$env:GODESKTOP_READBACK;$env:GODESKTOP_READBACK='1'
    try{& (Join-Path $taskTarget 'gocode.exe') -workspace $taskWorkspace -extensions-dir (Join-Path $taskValidation 'extensions') -smoke -copilot=false -lsp=false;if($LASTEXITCODE -ne 0){throw 'Installed native acceptance failed.'}}finally{$env:GODESKTOP_READBACK=$taskSavedReadback}
    if((Invoke-TestMSI '/x' $taskProducts[1] 'uninstall.log') -ne 0){throw 'MSI uninstall failed.'}
    if(Test-Path -LiteralPath (Join-Path $taskTarget 'gocode.exe')){throw 'Registered launcher retained after uninstall.'}
    if(Test-Path -LiteralPath (Join-Path $taskTarget 'current.json')){throw 'Owned update pointer retained after uninstall.'}
    if(-not(Test-Path -LiteralPath (Join-Path $taskWorkspace 'main.go'))){throw 'Uninstall removed user workspace.'}
    $taskUserPath=[Environment]::GetEnvironmentVariable('Path','User')
    if([bool]($taskUserPath -split ';'|Where-Object{$_.TrimEnd('\') -eq $taskTarget})){throw 'Installer PATH entry retained.'}
    Write-Output 'MSI acceptance passed: per-user install, icons/shortcuts, corrupted-cabinet rollback, versioned upgrade, downgrade rejection, native launch, owned cleanup and workspace/PATH preservation.'
}finally{
    foreach($taskProduct in $taskProducts){$taskState=$taskInstaller.GetType().InvokeMember('ProductState','GetProperty',$null,$taskInstaller,@($taskProduct));if($taskState -ge 1){[void](Invoke-TestMSI '/x' $taskProduct 'cleanup.log' @(('INSTALLDIR="'+$taskTarget+'"')))}}
    & (Join-Path $PSScriptRoot 'build-windows-resources.ps1')
    [void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskInstaller)
    Pop-Location
}
