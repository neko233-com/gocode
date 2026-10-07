param(
    [ValidateSet('auto','direct','mirror')][string]$Route='auto',
    [string]$Mirror='',
    [string]$InstallDirectory='',
    [string]$PackagePath='',
    [switch]$DownloadOnly
)
$ErrorActionPreference='Stop'
if(-not [Environment]::Is64BitOperatingSystem){throw 'gocode requires Windows x64.'}
$taskVersion='0.15.0'
$taskHash='a56f70df8d61d4bcf91a24f3cb5bf9b4159d8e1f001564989c92110a71cb8406'
$taskURL='https://github.com/neko233-com/gocode/releases/download/v'+$taskVersion+'/gocode-'+$taskVersion+'-windows-amd64.msi'
if($Mirror){$taskURI=[Uri]$Mirror;if($taskURI.Scheme -ne 'https' -or $taskURI.UserInfo -or $taskURI.Query -or $taskURI.Fragment){throw 'Mirror must be an HTTPS prefix without credentials/query/fragment.'}}
if($Route -eq 'mirror' -and -not $Mirror){throw 'Manual mirror route needs -Mirror.'}
$taskRoutes=@($taskURL)
if($Route -eq 'mirror'){$taskRoutes=@($Mirror.TrimEnd('/')+'/'+$taskURL)}elseif($Route -eq 'auto'){
    if($Mirror){$taskRoutes+=($Mirror.TrimEnd('/')+'/'+$taskURL)}
    $taskRoutes+=@('https://ghfast.top/'+$taskURL,'https://gh-proxy.com/'+$taskURL)
}
if(-not $PackagePath){
    $taskCache=Join-Path ([IO.Path]::GetTempPath()) ('gocode-install-'+[Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path $taskCache|Out-Null
    $PackagePath=Join-Path $taskCache 'gocode.msi'
    $taskSuccess=$false
    foreach($taskRoute in $taskRoutes){
        $taskResponse=$null;$taskInput=$null;$taskOutput=$null
        try{
            $taskRequest=[Net.HttpWebRequest]::Create($taskRoute)
            $taskRequest.Timeout=5000;$taskRequest.ReadWriteTimeout=30000;$taskRequest.UserAgent='gocode-installer/1'
            $taskResponse=$taskRequest.GetResponse()
            if($taskResponse.ContentLength -gt 536870912){throw 'Installer exceeds 512 MiB.'}
            $taskInput=$taskResponse.GetResponseStream();$taskOutput=[IO.File]::Create($PackagePath)
            $taskBuffer=New-Object byte[] 65536;$taskTotal=0L;$taskDeadline=[DateTime]::UtcNow.AddMinutes(3)
            while(($taskRead=$taskInput.Read($taskBuffer,0,$taskBuffer.Length)) -gt 0){
                $taskTotal+=$taskRead
                if($taskTotal -gt 536870912 -or [DateTime]::UtcNow -gt $taskDeadline){throw 'Installer download exceeds size/time policy.'}
                $taskOutput.Write($taskBuffer,0,$taskRead)
            }
            $taskOutput.Dispose();$taskOutput=$null
            if((Get-FileHash -LiteralPath $PackagePath -Algorithm SHA256).Hash -ne $taskHash){throw 'Installer SHA256 mismatch.'}
            $taskSuccess=$true;Write-Output ('Verified installer route: '+$taskRoute);break
        }catch{Write-Warning ('Route failed: '+$taskRoute+' ('+$_.Exception.Message+')')}finally{
            if($taskOutput){$taskOutput.Dispose()};if($taskInput){$taskInput.Dispose()};if($taskResponse){$taskResponse.Dispose()}
        }
    }
    if(-not $taskSuccess){throw 'No route returned the pinned installer bytes.'}
}
$PackagePath=[IO.Path]::GetFullPath($PackagePath)
if((Get-FileHash -LiteralPath $PackagePath -Algorithm SHA256).Hash -ne $taskHash){throw 'Installer SHA256 mismatch; no program was executed.'}
if($DownloadOnly){Write-Output $PackagePath;return}
if(-not $InstallDirectory){$InstallDirectory=Join-Path $env:LOCALAPPDATA 'Programs/gocode'}
$InstallDirectory=[IO.Path]::GetFullPath($InstallDirectory)
if(Test-Path -LiteralPath $InstallDirectory){
    $taskBase=Join-Path $InstallDirectory 'base.json'
    if(-not(Test-Path -LiteralPath $taskBase)){if(@(Get-ChildItem -LiteralPath $InstallDirectory -Force).Count){throw 'Target contains unrelated files.'}}
    else{if((Get-Content -LiteralPath $taskBase -Raw|ConvertFrom-Json).owner -ne 'neko233-com/gocode'){throw 'Target belongs to another application.'}}
}
for($taskParent=$InstallDirectory;$taskParent -and $taskParent -ne [IO.Path]::GetPathRoot($taskParent);$taskParent=Split-Path -Parent $taskParent){
    if((Test-Path -LiteralPath $taskParent) -and ((Get-Item -LiteralPath $taskParent -Force).Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Installation must not traverse a junction/symlink.'}
}
$taskInstall=Start-Process msiexec.exe -ArgumentList @('/i',('"'+$PackagePath+'"'),'/qn','/norestart',('INSTALLDIR="'+$InstallDirectory+'"')) -WindowStyle Hidden -Wait -PassThru
if($taskInstall.ExitCode -ne 0){throw ('Windows Installer failed: '+$taskInstall.ExitCode)}
& (Join-Path $InstallDirectory 'gocode.exe') -version
if($LASTEXITCODE -ne 0){throw 'Installed launcher did not pass its version check.'}
& (Join-Path $InstallDirectory 'gocode.exe') -configure-updates -update-mode $Route "-update-mirror=$Mirror"
if($LASTEXITCODE -ne 0){throw 'Update route configuration failed.'}
Write-Output ('Installed gocode. New terminals can run gocode. Location: '+$InstallDirectory)
