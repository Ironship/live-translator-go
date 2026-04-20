param(
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [string]$ProcessName = 'live-translator-go',
    [int]$WaitSeconds = 1
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing
Add-Type -AssemblyName System.Windows.Forms

$typeDef = @'
using System;
using System.Runtime.InteropServices;

public static class Win32 {
    [DllImport("user32.dll")] public static extern bool GetWindowRect(IntPtr hWnd, out RECT lpRect);
    [DllImport("user32.dll")] public static extern bool IsIconic(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
    [DllImport("user32.dll")] public static extern bool BringWindowToTop(IntPtr hWnd);
    [DllImport("user32.dll")] public static extern IntPtr GetForegroundWindow();
    [DllImport("user32.dll")] public static extern bool PrintWindow(IntPtr hWnd, IntPtr hdcBlt, uint nFlags);
    [DllImport("dwmapi.dll")] public static extern int DwmGetWindowAttribute(IntPtr hWnd, int dwAttribute, out RECT pvAttribute, int cbAttribute);

    [StructLayout(LayoutKind.Sequential)]
    public struct RECT { public int Left, Top, Right, Bottom; }
}
'@
if (-not ([System.Management.Automation.PSTypeName]'Win32').Type) {
    Add-Type -TypeDefinition $typeDef
}

Start-Sleep -Seconds $WaitSeconds

$proc = Get-Process -Name $ProcessName -ErrorAction Stop | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $proc) { throw "Process $ProcessName has no visible main window." }
$hWnd = $proc.MainWindowHandle

if ([Win32]::IsIconic($hWnd)) { [Win32]::ShowWindow($hWnd, 9) | Out-Null }
[Win32]::BringWindowToTop($hWnd) | Out-Null
[Win32]::SetForegroundWindow($hWnd) | Out-Null
Start-Sleep -Milliseconds 400

# Prefer DWM extended frame bounds (excludes invisible resize border on Win10/11).
$r = New-Object Win32+RECT
$DWMWA_EXTENDED_FRAME_BOUNDS = 9
$hr = [Win32]::DwmGetWindowAttribute($hWnd, $DWMWA_EXTENDED_FRAME_BOUNDS, [ref]$r, [System.Runtime.InteropServices.Marshal]::SizeOf([type][Win32+RECT]))
if ($hr -ne 0) {
    [Win32]::GetWindowRect($hWnd, [ref]$r) | Out-Null
}

$w = $r.Right - $r.Left
$h = $r.Bottom - $r.Top
if ($w -le 0 -or $h -le 0) { throw "Invalid window rect: ${w}x${h}" }

$bmp = New-Object System.Drawing.Bitmap($w, $h)
$g = [System.Drawing.Graphics]::FromImage($bmp)

# PrintWindow with PW_RENDERFULLCONTENT=2 captures the window even when occluded.
$hdc = $g.GetHdc()
$ok = [Win32]::PrintWindow($hWnd, $hdc, 2)
$g.ReleaseHdc($hdc)
if (-not $ok) {
    $g.CopyFromScreen($r.Left, $r.Top, 0, 0, (New-Object System.Drawing.Size($w, $h)))
}
$g.Dispose()

$outDir = Split-Path -Parent $OutputPath
if ($outDir -and -not (Test-Path $outDir)) { New-Item -ItemType Directory -Force -Path $outDir | Out-Null }

$bmp.Save($OutputPath, [System.Drawing.Imaging.ImageFormat]::Png)
$bmp.Dispose()
Write-Host "Screenshot saved: $OutputPath  (${w}x${h})"
