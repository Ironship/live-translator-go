param(
    [string]$OutputDir = 'dist',
    [string]$BinaryName = 'live-translator-go.exe'
)

$ErrorActionPreference = 'Stop'

$repoRoot = Split-Path -Parent $PSScriptRoot
$mainPackageDir = Join-Path $repoRoot 'cmd\live-translator-go'
$manifestPath = Join-Path $repoRoot 'live-translator-go.exe.manifest'
$sysoPath = Join-Path $mainPackageDir 'zz_manifest_windows_amd64.syso'
$outputRoot = Join-Path $repoRoot $OutputDir
$outputPath = Join-Path $outputRoot $BinaryName

function Get-GoBinPath {
    $goBin = (go env GOBIN).Trim()
    if ($goBin) {
        return $goBin
    }

    $goPath = (go env GOPATH).Trim()
    if (-not $goPath) {
        throw 'Unable to resolve GOPATH for locating rsrc.exe.'
    }

    return (Join-Path $goPath 'bin')
}

if (-not (Test-Path $manifestPath)) {
    throw "Manifest file not found: $manifestPath"
}

$goBinPath = Get-GoBinPath
$rsrcExe = Join-Path $goBinPath 'rsrc.exe'
if (-not (Test-Path $rsrcExe)) {
    throw 'rsrc.exe is not installed. Run: go install github.com/akavel/rsrc@latest'
}

New-Item -ItemType Directory -Force -Path $outputRoot | Out-Null

Push-Location $repoRoot
try {
    if (Test-Path $sysoPath) {
        Remove-Item $sysoPath -Force
    }

    & $rsrcExe -arch amd64 -manifest $manifestPath -o $sysoPath
    if ($LASTEXITCODE -ne 0) {
        throw 'Failed to generate embedded manifest resource.'
    }

    $env:CGO_ENABLED = '0'
    $env:GOOS = 'windows'
    $env:GOARCH = 'amd64'

    go build -trimpath -ldflags='-s -w -H windowsgui' -o $outputPath ./cmd/live-translator-go
    if ($LASTEXITCODE -ne 0) {
        throw 'Go build failed.'
    }
}
finally {
    if (Test-Path $sysoPath) {
        Remove-Item $sysoPath -Force
    }
    Pop-Location
}

Write-Host "Standalone executable created: $outputPath"
