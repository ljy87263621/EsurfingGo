[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [int]$ProcessId,
    [string]$OutputDirectory = (Join-Path (Get-Location) ("packet-capture-" + (Get-Date -Format "yyyyMMdd-HHmmss"))),
    [int]$DurationSeconds = 120,
    [switch]$NoNetshFallback
)

$ErrorActionPreference = "Stop"
if (-not (Get-Process -Id $ProcessId -ErrorAction SilentlyContinue)) {
    throw "Process $ProcessId is not running. Start the official client first and pass its current PID."
}

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$out = (Resolve-Path -LiteralPath $OutputDirectory).Path
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"

function Save-Text([string]$Name, [scriptblock]$Command) {
    & $Command 2>&1 | Out-File -LiteralPath (Join-Path $out $Name) -Encoding utf8
}

$proc = Get-CimInstance Win32_Process -Filter "ProcessId = $ProcessId"
[ordered]@{
    captured_at = (Get-Date).ToString("o")
    process_id = $ProcessId
    process_name = $proc.Name
    executable_path = $proc.ExecutablePath
    command_line = $proc.CommandLine
    duration_seconds = $DurationSeconds
} | ConvertTo-Json | Out-File -LiteralPath (Join-Path $out "metadata.json") -Encoding utf8

Save-Text "process.txt" { Get-Process -Id $ProcessId | Format-List * }
Save-Text "connections-before.txt" { Get-NetTCPConnection -OwningProcess $ProcessId -ErrorAction SilentlyContinue | Format-Table -AutoSize }
Save-Text "network-state.txt" { Get-NetIPConfiguration; route.exe print }

$pktmon = Get-Command pktmon.exe -ErrorAction SilentlyContinue
$netsh = Get-Command netsh.exe -ErrorAction SilentlyContinue
$captureStarted = $false
try {
    if ($pktmon) {
        $etl = Join-Path $out "official-client.etl"
        $pcap = Join-Path $out "official-client.pcapng"
        & $pktmon.Path stop 2>$null | Out-Null
        & $pktmon.Path filter remove 2>$null | Out-Null
        $ports = @(Get-NetTCPConnection -OwningProcess $ProcessId -ErrorAction SilentlyContinue |
            ForEach-Object { $_.LocalPort; $_.RemotePort } |
            Where-Object { $_ -gt 0 } | Select-Object -Unique)
        if ($ports.Count -eq 0) {
            Write-Warning "No TCP ports found for PID $ProcessId yet; start login traffic immediately after capture begins."
            foreach ($port in @(80, 443, 7001, 10001)) { & $pktmon.Path filter add -p $port | Out-Null }
        } else {
            foreach ($port in $ports) { & $pktmon.Path filter add -p $port | Out-Null }
        }
        & $pktmon.Path start --etw -p 0 -s 0 -f $etl | Out-Null
        $captureStarted = $true
        Write-Host "pktmon capture started for PID $ProcessId. Reproduce the official-client login now."
        Start-Sleep -Seconds $DurationSeconds
        & $pktmon.Path stop | Out-Null
        & $pktmon.Path pcapng $etl -o $pcap | Out-Null
        Save-Text "pktmon-filter.txt" { & $pktmon.Path filter list }
        Write-Host "Packet capture written to $pcap"
    } elseif (-not $NoNetshFallback -and $netsh) {
        $etl = Join-Path $out "official-client-netsh.etl"
        & $netsh.Path trace start capture=yes report=no persistent=no maxsize=512 file=$etl | Out-Null
        $captureStarted = $true
        Write-Host "netsh trace started (PID filtering unavailable). Reproduce the login now."
        Start-Sleep -Seconds $DurationSeconds
        & $netsh.Path trace stop | Out-Null
        Write-Host "System trace written to $etl; filter it by PID $ProcessId in WPA or Microsoft Message Analyzer-compatible tooling."
    } else {
        throw "Neither pktmon.exe nor netsh.exe is available."
    }
} finally {
    if ($captureStarted) {
        if ($pktmon) { & $pktmon.Path stop 2>$null | Out-Null }
        if ($netsh) { & $netsh.Path trace stop 2>$null | Out-Null }
    }
    Save-Text "connections-after.txt" { Get-NetTCPConnection -OwningProcess $ProcessId -ErrorAction SilentlyContinue | Format-Table -AutoSize }
}

Write-Host "Capture directory: $out"
Write-Host "Warning: packet captures may contain account credentials and session tokens; keep them local and redact before sharing."
