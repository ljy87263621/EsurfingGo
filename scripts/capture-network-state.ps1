[CmdletBinding()]
param(
    [string]$OutputDirectory = (Join-Path (Get-Location) ("tun-test-" + (Get-Date -Format "yyyyMMdd-HHmmss"))),
    [string]$Label = "state"
)

$ErrorActionPreference = "Continue"

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$resolvedOutputDirectory = (Resolve-Path -LiteralPath $OutputDirectory).Path

function Save-Capture {
    param(
        [Parameter(Mandatory = $true)]
        [string]$Name,

        [Parameter(Mandatory = $true)]
        [scriptblock]$Command
    )

    $path = Join-Path $resolvedOutputDirectory $Name
    & $Command 2>&1 | Out-File -LiteralPath $path -Encoding utf8
}

$metadata = [ordered]@{
    label = $Label
    captured_at = (Get-Date).ToString("o")
    computer = $env:COMPUTERNAME
    user = $env:USERNAME
    powershell = $PSVersionTable.PSVersion.ToString()
}
$metadata | ConvertTo-Json | Out-File -LiteralPath (Join-Path $resolvedOutputDirectory "metadata.json") -Encoding utf8

Save-Capture "net-adapter.txt" {
    Get-NetAdapter -IncludeHidden | Sort-Object ifIndex | Format-List Name, InterfaceDescription, ifIndex, Status, AdminStatus, LinkSpeed, MacAddress
}
Save-Capture "ip-interface.txt" {
    Get-NetIPInterface -AddressFamily IPv4 | Sort-Object InterfaceIndex | Format-Table -AutoSize InterfaceIndex, InterfaceAlias, ConnectionState, Dhcp, InterfaceMetric, NlMtu, AutomaticMetric
}
Save-Capture "ip-configuration.txt" {
    Get-NetIPConfiguration | Format-List InterfaceAlias, InterfaceIndex, IPv4Address, IPv4DefaultGateway, DNSServer
}
Save-Capture "route.txt" {
    Get-NetRoute -AddressFamily IPv4 | Sort-Object DestinationPrefix, RouteMetric | Format-Table -AutoSize ifIndex, InterfaceAlias, DestinationPrefix, NextHop, RouteMetric, PolicyStore
}
Save-Capture "route-print.txt" {
    & route.exe print
}
Save-Capture "dns-client.txt" {
    Get-DnsClientServerAddress -AddressFamily IPv4 | Sort-Object InterfaceIndex | Format-Table -AutoSize InterfaceIndex, InterfaceAlias, ServerAddresses
}
Save-Capture "proxy.txt" {
    Write-Output "=== WinHTTP ==="
    & netsh.exe winhttp show proxy
    Write-Output "=== Current user Internet Settings ==="
    Get-ItemProperty "HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings" | Select-Object ProxyEnable, ProxyServer, AutoConfigURL
}
Save-Capture "processes.txt" {
    Get-Process -Name "clash-verge*", "verge-mihomo*", "esurfing*", "ESurfing*" -ErrorAction SilentlyContinue |
        Sort-Object ProcessName, Id |
        Format-Table -AutoSize Id, ProcessName, Path, StartTime
}

Write-Host "Network state captured to $resolvedOutputDirectory"
Write-Host "This script is read-only: it does not change adapters, routes, DNS, proxy settings, or processes."
