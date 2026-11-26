<#
.SYNOPSIS
    Reproducible local release builder for sshm (Windows binary, Inno Setup EXE installer, SHA256SUMS).
    Copyright © 2026 Oktanetflow
#>
[CmdletBinding()]
param(
    [string]$Version = "1.0.0",
    [switch]$Sign,
    [switch]$SkipInstaller
)

$ErrorActionPreference = "Stop"

$distDir = Join-Path $PSScriptRoot "..\dist"
New-Item -ItemType Directory -Force -Path $distDir | Out-Null

Write-Host "==> [1/5] Generating Win32 PE VersionInfo & asInvoker manifest (.syso)..." -ForegroundColor Cyan
go generate ./cmd/sshm
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> [2/5] Building reproducible dist/sshm.exe (CGO_ENABLED=0, -trimpath, NO UPX)..." -ForegroundColor Cyan
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = "amd64"
$exePath = Join-Path $distDir "sshm.exe"
$ldflags = "-s -w -X github.com/oktaaa17/sshm/internal/cli.Version=$Version"
go build -trimpath -ldflags $ldflags -o $exePath ./cmd/sshm
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> [3/5] Verifying embedded PE Publisher metadata..." -ForegroundColor Cyan
$vi = (Get-Item $exePath).VersionInfo
Write-Host ("    Company   : {0}" -f $vi.CompanyName)
Write-Host ("    Product   : {0} ({1})" -f $vi.ProductName, $vi.ProductVersion)
Write-Host ("    Copyright : {0}" -f $vi.LegalCopyright)
if ($vi.CompanyName -ne "Oktanetflow") {
    throw "Expected CompanyName 'Oktanetflow' in PE VersionInfo, got '$($vi.CompanyName)'"
}

$artifacts = @($exePath)

if ($Sign) {
    & (Join-Path $PSScriptRoot "..\packaging\windows\sign.ps1") -Files $exePath
}

if (-not $SkipInstaller) {
    Write-Host "==> [4/5] Building Inno Setup per-user Windows EXE installer..." -ForegroundColor Cyan
    $isccPath = $null
    $isccCmd = Get-Command iscc.exe -ErrorAction SilentlyContinue
    if ($isccCmd) {
        $isccPath = $isccCmd.Source
    } else {
        $candidates = @(
            "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
            "$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe"
        )
        foreach ($c in $candidates) {
            if (Test-Path $c) { $isccPath = $c; break }
        }
    }
    if ($isccPath) {
        $issPath = Join-Path $PSScriptRoot "..\packaging\windows\sshm.iss"
        & $isccPath "/DMyAppVersion=$Version" $issPath
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        $installerPath = Join-Path $distDir "sshm-setup-$Version-amd64.exe"
        if (Test-Path $installerPath) {
            if ($Sign) {
                & (Join-Path $PSScriptRoot "..\packaging\windows\sign.ps1") -Files $installerPath
            }
            $artifacts += $installerPath
        }
    } else {
        Write-Warning "Inno Setup 6 (ISCC.exe) not found locally. Install via 'winget install JRSoftware.InnoSetup' or push a release tag to build automatically on GitHub Actions."
    }
}

Write-Host "==> [5/5] Writing dist/SHA256SUMS.txt..." -ForegroundColor Cyan
$sumLines = foreach ($a in $artifacts) {
    $hash = (Get-FileHash -Path $a -Algorithm SHA256).Hash.ToLowerInvariant()
    $name = Split-Path -Leaf $a
    "$hash  $name"
}
$sumPath = Join-Path $distDir "SHA256SUMS.txt"
$sumLines | Set-Content -Path $sumPath -Encoding ASCII
Get-Content $sumPath

Write-Host "`nRelease build complete!" -ForegroundColor Green
