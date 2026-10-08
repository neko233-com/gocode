[CmdletBinding()]
param()
$ErrorActionPreference = 'Stop'
$taskProject = Split-Path -Parent $PSScriptRoot
$taskParent = Split-Path -Parent $taskProject
& (Join-Path $taskParent 'scripts/validate-github-actions.ps1')
if ($LASTEXITCODE -ne 0) { throw 'Parent workflow lint failed.' }
Push-Location -LiteralPath $taskProject
try {
    $taskWorkflows = @(Get-ChildItem -LiteralPath '.github/workflows' -File | Where-Object { $_.Extension -in @('.yml', '.yaml') } | Select-Object -ExpandProperty FullName)
    & actionlint -shellcheck shellcheck @taskWorkflows
    if ($LASTEXITCODE -ne 0) { throw 'Application actionlint/ShellCheck failed.' }
    $taskScripts = @(Get-ChildItem -LiteralPath $PSScriptRoot -File -Filter '*.ps1')
    foreach ($taskScript in $taskScripts) {
        $taskTokens = $null; $taskErrors = $null
        [Management.Automation.Language.Parser]::ParseFile($taskScript.FullName, [ref]$taskTokens, [ref]$taskErrors) | Out-Null
        if ($taskErrors) { throw "PowerShell AST failed: $($taskScript.Name): $taskErrors" }
    }
    Write-Output 'Workflow actionlint/ShellCheck and PowerShell parser checks passed.'
} finally { Pop-Location }
