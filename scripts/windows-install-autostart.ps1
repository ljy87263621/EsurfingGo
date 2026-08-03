[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$ExePath,

    [Parameter(Mandatory = $true)]
    [string]$ConfigPath,

    [string]$LogFilePath = "",

    [string]$TaskName = "EsurfingGo"
)

$ErrorActionPreference = "Stop"

$resolvedExe = (Resolve-Path -LiteralPath $ExePath).Path
$resolvedConfig = (Resolve-Path -LiteralPath $ConfigPath).Path
$workingDirectory = Split-Path -Parent $resolvedExe

$arguments = "-config `"$resolvedConfig`""
if ($LogFilePath.Trim() -ne "") {
    $logDirectory = Split-Path -Parent $LogFilePath
    if ($logDirectory -and -not (Test-Path -LiteralPath $logDirectory)) {
        New-Item -ItemType Directory -Path $logDirectory | Out-Null
    }
    $arguments += " -log-file `"$LogFilePath`""
}

$action = New-ScheduledTaskAction -Execute $resolvedExe -Argument $arguments -WorkingDirectory $workingDirectory
$trigger = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -StartWhenAvailable -RestartCount 10 -RestartInterval (New-TimeSpan -Minutes 1)

Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger -Settings $settings -Description "Run EsurfingGo at user logon" -Force | Out-Null

Write-Host "Registered scheduled task '$TaskName'."
