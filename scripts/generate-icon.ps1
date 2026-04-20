param(
    [string]$OutputPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'assets\app.ico'),
    [string]$PngPath = (Join-Path (Split-Path -Parent $PSScriptRoot) 'assets\app.png')
)

$ErrorActionPreference = 'Stop'

Add-Type -AssemblyName System.Drawing

$sizes = 256, 128, 64, 48, 32, 16

function New-IconPng {
    param([int]$Size)

    $bmp = New-Object System.Drawing.Bitmap($Size, $Size, [System.Drawing.Imaging.PixelFormat]::Format32bppArgb)
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::AntiAlias
    $g.TextRenderingHint = [System.Drawing.Text.TextRenderingHint]::AntiAliasGridFit
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic

    # Transparent background
    $g.Clear([System.Drawing.Color]::Transparent)

    # Rounded square background with diagonal gradient.
    $pad = [Math]::Max(1, [int]($Size * 0.04))
    $rectF = New-Object System.Drawing.RectangleF($pad, $pad, ($Size - 2 * $pad), ($Size - 2 * $pad))
    $radius = [Math]::Max(2, [int]($Size * 0.22))

    $path = New-Object System.Drawing.Drawing2D.GraphicsPath
    $d = $radius * 2
    $x = $rectF.X; $y = $rectF.Y; $w = $rectF.Width; $h = $rectF.Height
    $path.AddArc($x, $y, $d, $d, 180, 90)
    $path.AddArc(($x + $w - $d), $y, $d, $d, 270, 90)
    $path.AddArc(($x + $w - $d), ($y + $h - $d), $d, $d, 0, 90)
    $path.AddArc($x, ($y + $h - $d), $d, $d, 90, 90)
    $path.CloseFigure()

    $gradTopLeft     = [System.Drawing.Color]::FromArgb(255, 31, 111, 235)   # #1F6FEB
    $gradBottomRight = [System.Drawing.Color]::FromArgb(255, 106, 92, 255)   # #6A5CFF
    $brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
        (New-Object System.Drawing.PointF($x, $y)),
        (New-Object System.Drawing.PointF(($x + $w), ($y + $h))),
        $gradTopLeft, $gradBottomRight)
    $g.FillPath($brush, $path)
    $brush.Dispose()

    # Subtle inner highlight along the top edge for depth.
    if ($Size -ge 48) {
        $highlight = New-Object System.Drawing.Drawing2D.LinearGradientBrush(
            (New-Object System.Drawing.PointF($x, $y)),
            (New-Object System.Drawing.PointF($x, ($y + $h / 2))),
            [System.Drawing.Color]::FromArgb(70, 255, 255, 255),
            [System.Drawing.Color]::FromArgb(0, 255, 255, 255))
        $clip = $g.Clip
        $g.SetClip($path)
        $g.FillRectangle($highlight, $rectF)
        $g.Clip = $clip
        $highlight.Dispose()
    }

    # "LT" monogram in white.
    $fontSize = [float]($Size * 0.52)
    $family = 'Segoe UI'
    try {
        $font = New-Object System.Drawing.Font($family, $fontSize, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
    } catch {
        $font = New-Object System.Drawing.Font('Arial', $fontSize, [System.Drawing.FontStyle]::Bold, [System.Drawing.GraphicsUnit]::Pixel)
    }
    $format = New-Object System.Drawing.StringFormat
    $format.Alignment = [System.Drawing.StringAlignment]::Center
    $format.LineAlignment = [System.Drawing.StringAlignment]::Center

    $textRect = New-Object System.Drawing.RectangleF(0, (-$Size * 0.02), $Size, $Size)
    $textBrush = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)
    $g.DrawString('LT', $font, $textBrush, $textRect, $format)
    $textBrush.Dispose()
    $font.Dispose()
    $format.Dispose()

    # Accent underline bar beneath the monogram.
    if ($Size -ge 32) {
        $barWidth = [int]($Size * 0.34)
        $barHeight = [Math]::Max(2, [int]($Size * 0.055))
        $barX = [int](($Size - $barWidth) / 2)
        $barY = [int]($Size * 0.78)
        $accent = [System.Drawing.Color]::FromArgb(255, 255, 159, 67)   # #FF9F43
        $accentBrush = New-Object System.Drawing.SolidBrush($accent)
        $barPath = New-Object System.Drawing.Drawing2D.GraphicsPath
        $br = [Math]::Min([int]($barHeight / 2), [int]($barWidth / 2))
        $bd = $br * 2
        if ($bd -gt 0) {
            $barPath.AddArc($barX, $barY, $bd, $bd, 180, 90)
            $barPath.AddArc(($barX + $barWidth - $bd), $barY, $bd, $bd, 270, 90)
            $barPath.AddArc(($barX + $barWidth - $bd), ($barY + $barHeight - $bd), $bd, $bd, 0, 90)
            $barPath.AddArc($barX, ($barY + $barHeight - $bd), $bd, $bd, 90, 90)
            $barPath.CloseFigure()
            $g.FillPath($accentBrush, $barPath)
        } else {
            $g.FillRectangle($accentBrush, $barX, $barY, $barWidth, $barHeight)
        }
        $accentBrush.Dispose()
        $barPath.Dispose()
    }

    $g.Dispose()
    $path.Dispose()

    $ms = New-Object System.IO.MemoryStream
    $bmp.Save($ms, [System.Drawing.Imaging.ImageFormat]::Png)
    $bmp.Dispose()
    return , $ms.ToArray()
}

# Build an ICO container with PNG-encoded entries (Vista+ compatible).
$pngs = @{}
foreach ($sz in $sizes) {
    $pngs[$sz] = New-IconPng -Size $sz
}

$outDir = Split-Path -Parent $OutputPath
if (-not (Test-Path $outDir)) {
    New-Item -ItemType Directory -Force -Path $outDir | Out-Null
}

$fs = [System.IO.File]::Open($OutputPath, [System.IO.FileMode]::Create, [System.IO.FileAccess]::Write)
$bw = New-Object System.IO.BinaryWriter($fs)
try {
    # ICONDIR
    $bw.Write([uint16]0)
    $bw.Write([uint16]1)
    $bw.Write([uint16]$sizes.Length)

    $headerSize = 6 + 16 * $sizes.Length
    $offset = $headerSize

    foreach ($sz in $sizes) {
        $data = $pngs[$sz]
        $w = if ($sz -ge 256) { 0 } else { $sz }
        $h = if ($sz -ge 256) { 0 } else { $sz }
        $bw.Write([byte]$w)
        $bw.Write([byte]$h)
        $bw.Write([byte]0)     # colors in palette
        $bw.Write([byte]0)     # reserved
        $bw.Write([uint16]1)   # color planes
        $bw.Write([uint16]32)  # bpp
        $bw.Write([uint32]$data.Length)
        $bw.Write([uint32]$offset)
        $offset += $data.Length
    }

    foreach ($sz in $sizes) {
        $bw.Write($pngs[$sz])
    }
}
finally {
    $bw.Dispose()
    $fs.Dispose()
}

Write-Host "Icon written: $OutputPath"

# Also emit a standalone 256x256 PNG for runtime use (walk NewIconFromImageForDPI).
$pngDir = Split-Path -Parent $PngPath
if (-not (Test-Path $pngDir)) {
    New-Item -ItemType Directory -Force -Path $pngDir | Out-Null
}
[System.IO.File]::WriteAllBytes($PngPath, $pngs[256])
Write-Host "PNG written:  $PngPath"

# Sync the embed-adjacent copy used by //go:embed in internal/ui/appicon.
$embedPath = Join-Path (Split-Path -Parent $PSScriptRoot) 'internal\ui\appicon\app.png'
$embedDir = Split-Path -Parent $embedPath
if (-not (Test-Path $embedDir)) {
    New-Item -ItemType Directory -Force -Path $embedDir | Out-Null
}
[System.IO.File]::WriteAllBytes($embedPath, $pngs[256])
Write-Host "Embed PNG:    $embedPath"
