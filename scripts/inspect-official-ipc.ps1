[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
if (-not ([System.Management.Automation.PSTypeName]'OfficialIpc').Type) {
Add-Type @'
using System;
using System.Runtime.InteropServices;
public static class OfficialIpc {
  [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)]
  public static extern IntPtr OpenFileMapping(uint access, bool inherit, string name);
  [DllImport("kernel32.dll", SetLastError=true, CharSet=CharSet.Unicode)]
  public static extern IntPtr OpenEvent(uint access, bool inherit, string name);
  [DllImport("kernel32.dll", SetLastError=true)]
  public static extern bool CloseHandle(IntPtr handle);
}
'@
}

$objects = @(
  @{ Name = 'Global\IPCerMemoryMsg'; Kind = 'FileMapping' },
  @{ Name = 'Global\IPCerMemoryEvent'; Kind = 'Event' }
)
$SYNCHRONIZE = 0x00100000
$FILE_MAP_READ = 0x0004
$EVENT_QUERY_STATE = 0x0001

foreach ($item in $objects) {
  $handle = if ($item.Kind -eq 'FileMapping') {
    [OfficialIpc]::OpenFileMapping($FILE_MAP_READ, $false, $item.Name)
  } else {
    [OfficialIpc]::OpenEvent(($EVENT_QUERY_STATE -bor $SYNCHRONIZE), $false, $item.Name)
  }
  if ($handle -eq [IntPtr]::Zero) {
    $code = [Runtime.InteropServices.Marshal]::GetLastWin32Error()
    [pscustomobject]@{ Name=$item.Name; Kind=$item.Kind; Exists=$false; ErrorCode=$code; Error=[ComponentModel.Win32Exception]::new($code).Message }
  } else {
    [OfficialIpc]::CloseHandle($handle) | Out-Null
    [pscustomobject]@{ Name=$item.Name; Kind=$item.Kind; Exists=$true; ErrorCode=0; Error='' }
  }
}
