param([ValidatePattern('^\d+\.\d+\.\d+$')][string]$Version)
$ErrorActionPreference='Stop'
$taskRoot=Split-Path -Parent $PSScriptRoot
if(-not $Version){$Version=(Get-Content -LiteralPath (Join-Path $taskRoot 'VERSION') -Raw).Trim()}
$taskCache=Join-Path $taskRoot '.cache/resources'
New-Item -ItemType Directory -Path $taskCache -Force | Out-Null
$taskIcon=(Join-Path $taskRoot 'assets/code-oss/code.ico').Replace('\','\\')
$taskManifestFile=Join-Path $taskCache 'gocode.manifest'
$taskManifestText=(Get-Content -LiteralPath (Join-Path $taskRoot 'assets/gocode.manifest') -Raw).Replace('version="0.4.0.0"',('version="'+$Version+'.0"'))
[IO.File]::WriteAllText($taskManifestFile,$taskManifestText,[Text.UTF8Encoding]::new($false))
$taskManifest=$taskManifestFile.Replace('\','\\')
$taskTuple=($Version.Replace('.',','))+',0'
$taskResource=@"
#pragma code_page(65001)
#include <windows.h>
1 ICON "$taskIcon"
1 RT_MANIFEST "$taskManifest"
1 VERSIONINFO
FILEVERSION $taskTuple
PRODUCTVERSION $taskTuple
FILEFLAGSMASK 0x3fL
FILEFLAGS 0x0L
FILEOS VOS_NT_WINDOWS32
FILETYPE VFT_APP
FILESUBTYPE 0x0L
BEGIN
  BLOCK "StringFileInfo"
  BEGIN
    BLOCK "040904b0"
    BEGIN
      VALUE "CompanyName", "neko233-com"
      VALUE "FileDescription", "gocode native editor"
      VALUE "FileVersion", "$Version"
      VALUE "InternalName", "gocode"
      VALUE "OriginalFilename", "gocode.exe"
      VALUE "ProductName", "gocode"
      VALUE "ProductVersion", "$Version"
      VALUE "LegalCopyright", "gocode contributors; Code-OSS icon Microsoft (MIT)"
    END
  END
  BLOCK "VarFileInfo"
  BEGIN
    VALUE "Translation", 0x0409, 1200
  END
END
"@
$taskRC=Join-Path $taskCache 'gocode.rc'
[IO.File]::WriteAllText($taskRC,$taskResource,[Text.UTF8Encoding]::new($false))
& windres --target=pe-x86-64 -i $taskRC -o (Join-Path $taskRoot 'gocode_windows_amd64.syso')
if ($LASTEXITCODE -ne 0) {throw 'Windows icon/version resource compilation failed.'}
