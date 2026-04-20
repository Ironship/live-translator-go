param(
    [Parameter(Mandatory = $true)][string]$ButtonName,
    [string]$ProcessName = 'live-translator-go'
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes

$proc = Get-Process -Name $ProcessName -ErrorAction Stop | Where-Object { $_.MainWindowHandle -ne 0 } | Select-Object -First 1
if (-not $proc) { throw "Process $ProcessName has no visible main window." }
$hWnd = [IntPtr]$proc.MainWindowHandle

$root = [System.Windows.Automation.AutomationElement]::FromHandle($hWnd)
if (-not $root) { throw 'Failed to obtain AutomationElement for main window.' }

$cond = New-Object System.Windows.Automation.PropertyCondition(
    [System.Windows.Automation.AutomationElement]::ControlTypeProperty,
    [System.Windows.Automation.ControlType]::Button)
$buttons = $root.FindAll([System.Windows.Automation.TreeScope]::Descendants, $cond)

$target = $null
foreach ($b in $buttons) {
    $n = $b.Current.Name
    if ($n -like "*$ButtonName*") { $target = $b; break }
}

if (-not $target) {
    $names = @()
    foreach ($b in $buttons) { $names += $b.Current.Name }
    throw ("Button matching '$ButtonName' not found. Available: " + ($names -join ' | '))
}

$pattern = $null
if ($target.TryGetCurrentPattern([System.Windows.Automation.InvokePattern]::Pattern, [ref]$pattern)) {
    $pattern.Invoke()
    Write-Host "Invoked: $($target.Current.Name)"
} else {
    throw "Button '$($target.Current.Name)' does not support InvokePattern."
}

Start-Sleep -Milliseconds 600
