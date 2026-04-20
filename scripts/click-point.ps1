param(
    [Parameter(Mandatory = $true)][double]$RelX,
    [Parameter(Mandatory = $true)][double]$RelY,
    [string]$ProcessName = 'live-translator-go'
)

$ErrorActionPreference = 'Stop'

$typeDef = @'
using System;
using System.Runtime.InteropServices;
public static class Win32Click {
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetCursorPos(int X, int Y);
    [DllImport("user32.dll")] public static extern void mouse_event(uint dwFlags, uint dx, uint dy, uint dwData, UIntPtr dwExtraInfo);
    [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr hWnd, int dwAttribute, out RECT pvAttribute, int cbAttribute);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
    public const uint MOUSEEVENTF_LEFTDOWN = 0x02;
    public const uint MOUSEEVENTF_LEFTUP   = 0x04;
}
'@
if (-not ([System.Management.Automation.PSTypeName]'Win32Click').Type) { Add-Type -TypeDefinition $typeDef }

$proc = Get-Process -Name $ProcessName -ErrorAction Stop | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
$hWnd = [IntPtr]$proc.MainWindowHandle

[Win32Click]::BringWindowToTop($hWnd) | Out-Null
[Win32Click]::SetForegroundWindow($hWnd) | Out-Null
Start-Sleep -Milliseconds 300

$r = New-Object Win32Click+RECT
$hr = [Win32Click]::DwmGetWindowAttribute($hWnd, 9, [ref]$r, [System.Runtime.InteropServices.Marshal]::SizeOf([type][Win32Click+RECT]))
if ($hr -ne 0) { [Win32Click]::GetWindowRect($hWnd, [ref]$r) | Out-Null }

$w = $r.Right - $r.Left
$h = $r.Bottom - $r.Top
$x = $r.Left + [int]($RelX * $w)
$y = $r.Top  + [int]($RelY * $h)

[Win32Click]::SetCursorPos($x, $y) | Out-Null
Start-Sleep -Milliseconds 150
[Win32Click]::mouse_event([Win32Click]::MOUSEEVENTF_LEFTDOWN, 0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 60
[Win32Click]::mouse_event([Win32Click]::MOUSEEVENTF_LEFTUP,   0, 0, 0, [UIntPtr]::Zero)
Start-Sleep -Milliseconds 600

Write-Host "Clicked at ($x, $y) inside window ${w}x${h}"
