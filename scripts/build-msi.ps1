param(
    [Parameter(Mandatory)][string]$PayloadDirectory,
    [Parameter(Mandatory)][string]$OutputPath,
    [ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version='0.4.0',
    [ValidatePattern('^[A-Za-z0-9 -]{1,40}$')][string]$ProductName='gocode',
    [Guid]$UpgradeCode='{EED33BE1-E465-4A97-A598-A29F430F9324}',
    [string]$WorkDirectory=''
)
$ErrorActionPreference='Stop'
$taskPayload=(Resolve-Path -LiteralPath $PayloadDirectory).Path
$taskMSI=[IO.Path]::GetFullPath($OutputPath)
if(Test-Path -LiteralPath $taskMSI){throw "MSI already exists: $taskMSI"}
$taskBase=Get-Content -LiteralPath (Join-Path $taskPayload 'base.json') -Raw|ConvertFrom-Json
if($taskBase.owner -ne 'neko233-com/gocode' -or $taskBase.schema -ne 1 -or $taskBase.version -ne $Version){throw 'Payload ownership/version mismatch.'}
$taskBuild=Join-Path (Split-Path -Parent $taskMSI) ('msi-work-'+[Guid]::NewGuid().ToString('N'))
if ($WorkDirectory) {
    . (Join-Path $PSScriptRoot 'local-release-functions.ps1')
    $taskBuild = Initialize-ReleaseWorkDirectory $WorkDirectory
}
New-Item -ItemType Directory -Path $taskBuild -Force|Out-Null
$taskInstaller=New-Object -ComObject WindowsInstaller.Installer
$taskDB=$taskInstaller.GetType().InvokeMember('OpenDatabase','InvokeMethod',$null,$taskInstaller,@($taskMSI,3))
function Invoke-MSI($Object,[string]$Name,[object[]]$Arguments=@()) {return $Object.GetType().InvokeMember($Name,'InvokeMethod',$null,$Object,$Arguments)}
function Set-MSI {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification='Internal builder assigns only properties in its output MSI database, never installer registration.')]
    param($Object,[string]$Name,[object[]]$Arguments)
    [void]$Object.GetType().InvokeMember($Name,'SetProperty',$null,$Object,$Arguments)
}
function Invoke-SQL([string]$SQL,$Record=$null) {
    $taskView=Invoke-MSI $taskDB 'OpenView' @($SQL)
    try {[void](Invoke-MSI $taskView 'Execute' @($Record))}finally{[void](Invoke-MSI $taskView 'Close');[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskView)}
}
function Add-Row([string]$Table,[string[]]$Columns,[object[]]$Values) {
    $taskRecord=Invoke-MSI $taskInstaller 'CreateRecord' @($Values.Count)
    try {
        for($taskIndex=0;$taskIndex -lt $Values.Count;$taskIndex++){
            if($null -eq $Values[$taskIndex]){continue}
            if($Values[$taskIndex] -is [int]){Set-MSI $taskRecord 'IntegerData' @(($taskIndex+1),$Values[$taskIndex])}else{Set-MSI $taskRecord 'StringData' @(($taskIndex+1),[string]$Values[$taskIndex])}
        }
        $taskQuoted=($Columns|ForEach-Object{'`'+$_+'`'}) -join ','
        $taskMarkers=(@('?')*$Values.Count)-join ','
        Invoke-SQL ('INSERT INTO `'+$Table+'` ('+$taskQuoted+') VALUES ('+$taskMarkers+')') $taskRecord
    }finally{[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskRecord)}
}
function Add-Stream([string]$Table,[string]$Name,[string]$Path) {
    $taskView=Invoke-MSI $taskDB 'OpenView' @('SELECT `Name`,`Data` FROM `'+$Table+'`')
    $taskRecord=Invoke-MSI $taskInstaller 'CreateRecord' @(2)
    try {
        [void](Invoke-MSI $taskView 'Execute' @($null));Set-MSI $taskRecord 'StringData' @(1,$Name)
        [void](Invoke-MSI $taskRecord 'SetStream' @(2,$Path));[void](Invoke-MSI $taskView 'Modify' @(1,$taskRecord))
    }finally{[void](Invoke-MSI $taskView 'Close');[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskRecord);[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskView)}
}
function New-StableGuid {
    [Diagnostics.CodeAnalysis.SuppressMessageAttribute('PSUseShouldProcessForStateChangingFunctions', '', Justification='Pure deterministic GUID calculation allocates a value without modifying files or system state.')]
    param([string]$Name)
    $taskHash=[Security.Cryptography.SHA256]::Create().ComputeHash([Text.Encoding]::UTF8.GetBytes($UpgradeCode.ToString()+':'+$Name))
    $taskBytes=New-Object byte[] 16;[Array]::Copy($taskHash,$taskBytes,16)
    return '{'+([Guid]::new($taskBytes)).ToString().ToUpperInvariant()+'}'
}
try {
    $taskSchemas=@(
        'CREATE TABLE `Property` (`Property` CHAR(72) NOT NULL, `Value` CHAR(0) LOCALIZABLE PRIMARY KEY `Property`)',
        'CREATE TABLE `Directory` (`Directory` CHAR(72) NOT NULL, `Directory_Parent` CHAR(72), `DefaultDir` CHAR(255) NOT NULL LOCALIZABLE PRIMARY KEY `Directory`)',
        'CREATE TABLE `Component` (`Component` CHAR(72) NOT NULL, `ComponentId` CHAR(38), `Directory_` CHAR(72) NOT NULL, `Attributes` SHORT NOT NULL, `Condition` CHAR(255), `KeyPath` CHAR(72) PRIMARY KEY `Component`)',
        'CREATE TABLE `Feature` (`Feature` CHAR(38) NOT NULL, `Feature_Parent` CHAR(38), `Title` CHAR(64) LOCALIZABLE, `Description` CHAR(255) LOCALIZABLE, `Display` SHORT, `Level` SHORT NOT NULL, `Directory_` CHAR(72), `Attributes` SHORT NOT NULL PRIMARY KEY `Feature`)',
        'CREATE TABLE `FeatureComponents` (`Feature_` CHAR(38) NOT NULL, `Component_` CHAR(72) NOT NULL PRIMARY KEY `Feature_`,`Component_`)',
        'CREATE TABLE `File` (`File` CHAR(72) NOT NULL, `Component_` CHAR(72) NOT NULL, `FileName` CHAR(255) NOT NULL LOCALIZABLE, `FileSize` LONG NOT NULL, `Version` CHAR(72), `Language` CHAR(20), `Attributes` SHORT, `Sequence` SHORT NOT NULL PRIMARY KEY `File`)',
        'CREATE TABLE `Media` (`DiskId` SHORT NOT NULL, `LastSequence` SHORT NOT NULL, `DiskPrompt` CHAR(64) LOCALIZABLE, `Cabinet` CHAR(255), `VolumeLabel` CHAR(32), `Source` CHAR(72) PRIMARY KEY `DiskId`)',
        'CREATE TABLE `Registry` (`Registry` CHAR(72) NOT NULL, `Root` SHORT NOT NULL, `Key` CHAR(255) NOT NULL LOCALIZABLE, `Name` CHAR(255) LOCALIZABLE, `Value` CHAR(0) LOCALIZABLE, `Component_` CHAR(72) NOT NULL PRIMARY KEY `Registry`)',
        'CREATE TABLE `Environment` (`Environment` CHAR(72) NOT NULL, `Name` CHAR(255) NOT NULL LOCALIZABLE, `Value` CHAR(0) LOCALIZABLE, `Component_` CHAR(72) NOT NULL PRIMARY KEY `Environment`)',
        'CREATE TABLE `Shortcut` (`Shortcut` CHAR(72) NOT NULL, `Directory_` CHAR(72) NOT NULL, `Name` CHAR(128) NOT NULL LOCALIZABLE, `Component_` CHAR(72) NOT NULL, `Target` CHAR(72) NOT NULL, `Arguments` CHAR(255), `Description` CHAR(255) LOCALIZABLE, `Hotkey` SHORT, `Icon_` CHAR(72), `IconIndex` SHORT, `ShowCmd` SHORT, `WkDir` CHAR(72), `DisplayResourceDLL` CHAR(255), `DisplayResourceId` SHORT, `DescriptionResourceDLL` CHAR(255), `DescriptionResourceId` SHORT PRIMARY KEY `Shortcut`)',
        'CREATE TABLE `Icon` (`Name` CHAR(72) NOT NULL, `Data` OBJECT NOT NULL PRIMARY KEY `Name`)',
        'CREATE TABLE `RemoveFile` (`FileKey` CHAR(72) NOT NULL, `Component_` CHAR(72) NOT NULL, `FileName` CHAR(255) LOCALIZABLE, `DirProperty` CHAR(72) NOT NULL, `InstallMode` SHORT NOT NULL PRIMARY KEY `FileKey`)',
        'CREATE TABLE `Upgrade` (`UpgradeCode` CHAR(38) NOT NULL, `VersionMin` CHAR(20), `VersionMax` CHAR(20), `Language` CHAR(255), `Attributes` SHORT NOT NULL, `Remove` CHAR(255), `ActionProperty` CHAR(72) NOT NULL PRIMARY KEY `UpgradeCode`,`VersionMin`,`VersionMax`,`Language`,`Attributes`)',
        'CREATE TABLE `LaunchCondition` (`Condition` CHAR(255) NOT NULL, `Description` CHAR(255) NOT NULL LOCALIZABLE PRIMARY KEY `Condition`)',
        'CREATE TABLE `AppSearch` (`Property` CHAR(72) NOT NULL, `Signature_` CHAR(72) NOT NULL PRIMARY KEY `Property`,`Signature_`)',
        'CREATE TABLE `RegLocator` (`Signature_` CHAR(72) NOT NULL, `Root` SHORT NOT NULL, `Key` CHAR(255) NOT NULL, `Name` CHAR(255), `Type` SHORT PRIMARY KEY `Signature_`)',
        'CREATE TABLE `Signature` (`Signature` CHAR(72) NOT NULL, `FileName` CHAR(255) NOT NULL, `MinVersion` CHAR(20), `MaxVersion` CHAR(20), `MinSize` LONG, `MaxSize` LONG, `MinDate` LONG, `MaxDate` LONG, `Languages` CHAR(255) PRIMARY KEY `Signature`)',
        'CREATE TABLE `CompLocator` (`Signature_` CHAR(72) NOT NULL, `ComponentId` CHAR(38) NOT NULL, `Type` SHORT PRIMARY KEY `Signature_`)',
        'CREATE TABLE `DrLocator` (`Signature_` CHAR(72) NOT NULL, `Parent` CHAR(72), `Path` CHAR(255), `Depth` SHORT PRIMARY KEY `Signature_`,`Parent`,`Path`)',
        'CREATE TABLE `CustomAction` (`Action` CHAR(72) NOT NULL, `Type` SHORT NOT NULL, `Source` CHAR(72), `Target` CHAR(0), `ExtendedType` LONG PRIMARY KEY `Action`)',
        'CREATE TABLE `InstallExecuteSequence` (`Action` CHAR(72) NOT NULL, `Condition` CHAR(255), `Sequence` SHORT PRIMARY KEY `Action`)',
        'CREATE TABLE `InstallUISequence` (`Action` CHAR(72) NOT NULL, `Condition` CHAR(255), `Sequence` SHORT PRIMARY KEY `Action`)'
    )
    foreach($taskSQL in $taskSchemas){Invoke-SQL $taskSQL}
    $taskProduct=New-StableGuid ('product:'+ $Version)
    $taskUpgrade='{'+$UpgradeCode.ToString().ToUpperInvariant()+'}'
    $taskProperties=@{ProductCode=$taskProduct;UpgradeCode=$taskUpgrade;ProductName=$ProductName;ProductVersion=$Version;ProductLanguage='1033';Manufacturer='neko233-com';INSTALLLEVEL='1';MSIINSTALLPERUSER='1';ARPNOMODIFY='1';ARPPRODUCTICON='gocode.ico';ARPURLINFOABOUT='https://github.com/neko233-com/gocode';SecureCustomProperties='GOCODE_OLDPRODUCTS;GOCODE_NEWPRODUCTS';REBOOT='ReallySuppress'}
    foreach($taskKey in $taskProperties.Keys){Add-Row 'Property' @('Property','Value') @($taskKey,$taskProperties[$taskKey])}
    foreach($taskDirectory in @(@('TARGETDIR',$null,'SourceDir'),@('LocalAppDataFolder','TARGETDIR','.'),@('Programs','LocalAppDataFolder','Programs'),@('INSTALLDIR','Programs','gocode'),@('Versions','INSTALLDIR','versions'),@('AppVersion','Versions',$Version),@('ProgramMenuFolder','TARGETDIR','.'),@('MenuFolder','ProgramMenuFolder',$ProductName),@('DesktopFolder','TARGETDIR','.'))){Add-Row 'Directory' @('Directory','Directory_Parent','DefaultDir') $taskDirectory}
    Add-Row 'Feature' @('Feature','Feature_Parent','Title','Description','Display','Level','Directory_','Attributes') @('Core',$null,$ProductName,'Native Go desktop editor',1,1,'INSTALLDIR',0)
    $taskFiles=@(Get-ChildItem -LiteralPath $taskPayload -File -Recurse|Sort-Object FullName)
    if($taskFiles.Count -gt 1000){throw 'Unexpected package file count.'}
    $taskDDF=@('.OPTION EXPLICIT','.Set CabinetNameTemplate=payload.cab',('.Set DiskDirectoryTemplate="'+$taskBuild+'"'),('.Set InfFileName="'+(Join-Path $taskBuild 'setup.inf')+'"'),('.Set RptFileName="'+(Join-Path $taskBuild 'setup.rpt')+'"'),'.Set MaxDiskSize=0','.Set CompressionType=LZX','.Set CompressionMemory=21','.Set Cabinet=on','.Set Compress=on')
    $taskGUI=$null;$taskMaintenance=$null;$taskMaintenanceComponent=$null;$taskIndex=0
    foreach($taskFile in $taskFiles){
        $taskIndex++;$taskID='F'+$taskIndex;$taskComponent='C'+$taskIndex;$taskRegistry='R'+$taskIndex
        $taskRelative=$taskFile.FullName.Substring($taskPayload.Length+1).Replace('\','/')
        $taskDirectory='INSTALLDIR';if($taskRelative.StartsWith('versions/'+$Version+'/')){$taskDirectory='AppVersion'}elseif($taskRelative.Contains('/')){throw "Unsupported payload directory: $taskRelative"}
        if($taskRelative -eq 'gocode-launch.exe'){$taskGUI=$taskID}
        if($taskRelative -eq 'gocode-maintenance.exe'){$taskMaintenance=$taskID;$taskMaintenanceComponent=$taskComponent}
        Add-Row 'Component' @('Component','ComponentId','Directory_','Attributes','Condition','KeyPath') @($taskComponent,(New-StableGuid $taskRelative),$taskDirectory,260,$null,$taskRegistry)
        Add-Row 'FeatureComponents' @('Feature_','Component_') @('Core',$taskComponent)
        Add-Row 'Registry' @('Registry','Root','Key','Name','Value','Component_') @($taskRegistry,1,('Software\neko233-com\gocode\'+$taskUpgrade),$taskID,('[INSTALLDIR]'+$taskRelative),$taskComponent)
        $taskFileVersion=$null;if($taskFile.Extension -eq '.exe'){$taskFileVersion=$taskFile.VersionInfo.FileVersion}
        Add-Row 'File' @('File','Component_','FileName','FileSize','Version','Language','Attributes','Sequence') @($taskID,$taskComponent,$taskFile.Name,[int]$taskFile.Length,$taskFileVersion,$null,16384,$taskIndex)
        $taskDDF+=('"'+$taskFile.FullName+'" '+$taskID)
    }
    if(-not $taskGUI){throw 'GUI launcher missing.'}
    $taskDDFPath=Join-Path $taskBuild 'payload.ddf';[IO.File]::WriteAllLines($taskDDFPath,$taskDDF,[Text.Encoding]::Default)
    & makecab /F $taskDDFPath | Out-Null
    if($LASTEXITCODE -ne 0){throw 'Embedded cabinet creation failed.'}
    Add-Row 'Media' @('DiskId','LastSequence','DiskPrompt','Cabinet','VolumeLabel','Source') @(1,$taskIndex,$null,'#payload.cab',$null,$null)
    Add-Stream '_Streams' 'payload.cab' (Join-Path $taskBuild 'payload.cab')
    Add-Stream 'Icon' 'gocode.ico' (Join-Path $taskPayload 'gocode.ico')
    Add-Row 'Component' @('Component','ComponentId','Directory_','Attributes','Condition','KeyPath') @('Integration',(New-StableGuid 'integration'),'INSTALLDIR',260,$null,'IntegrationKey')
    Add-Row 'FeatureComponents' @('Feature_','Component_') @('Core','Integration')
    Add-Row 'Registry' @('Registry','Root','Key','Name','Value','Component_') @('IntegrationKey',1,('Software\neko233-com\gocode\'+$taskUpgrade),'InstallRoot','[INSTALLDIR]','Integration')
    Add-Row 'AppSearch' @('Property','Signature_') @('GOCODE_EXISTINGDIR','InstallRootSignature')
    Add-Row 'RegLocator' @('Signature_','Root','Key','Name','Type') @('InstallRootSignature',1,('Software\neko233-com\gocode\'+$taskUpgrade),'InstallRoot',18)
    Add-Row 'CustomAction' @('Action','Type','Source','Target','ExtendedType') @('RestoreInstallRoot',51,'INSTALLDIR','[GOCODE_EXISTINGDIR]',$null)
    if($taskMaintenance){Add-Row 'CustomAction' @('Action','Type','Source','Target','ExtendedType') @('CleanupUpdates',1042,$taskMaintenance,'-uninstall-cleanup "[INSTALLDIR]."',$null)}
    Add-Row 'Environment' @('Environment','Name','Value','Component_') @('UserPath','=-PATH','[~];[INSTALLDIR]','Integration')
    foreach($taskShortcut in @(@('StartMenu','MenuFolder'),@('Desktop','DesktopFolder'))){Add-Row 'Shortcut' @('Shortcut','Directory_','Name','Component_','Target','Arguments','Description','Hotkey','Icon_','IconIndex','ShowCmd','WkDir','DisplayResourceDLL','DisplayResourceId','DescriptionResourceDLL','DescriptionResourceId') @($taskShortcut[0],$taskShortcut[1],$ProductName,'Integration',('[#'+$taskGUI+']'),$null,'gocode native editor',$null,'gocode.ico',0,1,'INSTALLDIR',$null,$null,$null,$null)}
    Add-Row 'RemoveFile' @('FileKey','Component_','FileName','DirProperty','InstallMode') @('MenuCleanup','Integration',$null,'MenuFolder',2)
    Add-Row 'Upgrade' @('UpgradeCode','VersionMin','VersionMax','Language','Attributes','Remove','ActionProperty') @($taskUpgrade,$null,$Version,$null,1,$null,'GOCODE_OLDPRODUCTS')
    Add-Row 'Upgrade' @('UpgradeCode','VersionMin','VersionMax','Language','Attributes','Remove','ActionProperty') @($taskUpgrade,$Version,$null,$null,258,$null,'GOCODE_NEWPRODUCTS')
    Add-Row 'LaunchCondition' @('Condition','Description') @('VersionNT64','gocode requires 64-bit Windows.')
    Add-Row 'LaunchCondition' @('Condition','Description') @('NOT GOCODE_NEWPRODUCTS OR Installed','A newer or equal gocode version is already installed.')
    # Finish copying and validating the new payload before removing the old one.
    # Stable component GUIDs retain shared launchers; version directories differ.
    # A damaged cabinet therefore rolls back without unregistering the old app.
    foreach($taskAction in @(@('FindRelatedProducts',25),@('LaunchConditions',100),@('ValidateProductID',700),@('CostInitialize',800),@('FileCost',900),@('CostFinalize',1000),@('MigrateFeatureStates',1200),@('InstallValidate',1400),@('InstallInitialize',1500),@('ProcessComponents',1600),@('UnpublishFeatures',1800),@('RemoveShortcuts',3200),@('RemoveEnvironmentStrings',3300),@('RemoveRegistryValues',3500),@('RemoveFiles',4000),@('RemoveFolders',4100),@('CreateFolders',4200),@('InstallFiles',4500),@('CreateShortcuts',4600),@('WriteRegistryValues',5000),@('WriteEnvironmentStrings',5200),@('RegisterUser',6000),@('RegisterProduct',6100),@('PublishFeatures',6300),@('PublishProduct',6400),@('InstallExecute',6500),@('RemoveExistingProducts',6550),@('InstallFinalize',6600))){Add-Row 'InstallExecuteSequence' @('Action','Condition','Sequence') @($taskAction[0],$null,[int]$taskAction[1])}
    Add-Row 'InstallExecuteSequence' @('Action','Condition','Sequence') @('AppSearch',$null,50)
    Add-Row 'InstallExecuteSequence' @('Action','Condition','Sequence') @('RestoreInstallRoot','NOT INSTALLDIR AND GOCODE_EXISTINGDIR',70)
    if($taskMaintenance){Add-Row 'InstallExecuteSequence' @('Action','Condition','Sequence') @('CleanupUpdates',('REMOVE~="ALL" AND NOT UPGRADINGPRODUCTCODE AND ?'+$taskMaintenanceComponent+'=3'),3900)}
    foreach($taskAction in @(@('FindRelatedProducts',25),@('LaunchConditions',100),@('CostInitialize',800),@('FileCost',900),@('CostFinalize',1000),@('ExecuteAction',1300))){Add-Row 'InstallUISequence' @('Action','Condition','Sequence') @($taskAction[0],$null,[int]$taskAction[1])}
    Add-Row 'InstallUISequence' @('Action','Condition','Sequence') @('AppSearch',$null,50)
    Add-Row 'InstallUISequence' @('Action','Condition','Sequence') @('RestoreInstallRoot','NOT INSTALLDIR AND GOCODE_EXISTINGDIR',70)
    $taskSummary=$taskDB.GetType().InvokeMember('SummaryInformation','GetProperty',$null,$taskDB,@(20))
    try{foreach($taskEntry in @(@(1,1252),@(2,'Installation Database'),@(3,$ProductName),@(4,'neko233-com'),@(7,'x64;1033'),@(9,('{'+[Guid]::NewGuid().ToString().ToUpperInvariant()+'}')),@(14,500),@(15,10),@(18,'gocode free Windows Installer COM packager'),@(19,2))){Set-MSI $taskSummary 'Property' @($taskEntry[0],$taskEntry[1])};[void](Invoke-MSI $taskSummary 'Persist')}finally{[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskSummary)}
    [void](Invoke-MSI $taskDB 'Commit')
    Write-Output "MSI: $taskMSI"
    Write-Output "ProductCode: $taskProduct"
}finally{[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskDB);[void][Runtime.InteropServices.Marshal]::FinalReleaseComObject($taskInstaller)}
